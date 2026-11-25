package nodeapp

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"regexp"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/npmfetch"
	"github.com/priyanshjhaa/Sprout/backend/internal/source"
)

var (
	ErrImage        = errors.New("node_build_image_invalid")
	ErrDependencies = errors.New("node_build_dependencies_unsupported")
	ErrInstall      = errors.New("node_build_install_failed")
	ErrBuild        = errors.New("node_build_failed")
	ErrArtifact     = errors.New("node_build_artifact_invalid")
	ErrArtifactSize = errors.New("node_build_artifact_too_large")
	ErrDocker       = errors.New("node_build_docker_unavailable")
	ErrCleanup      = errors.New("node_build_cleanup_failed")
)

var pinnedImage = regexp.MustCompile(`^node@sha256:[a-f0-9]{64}$`)

// DefaultImage is the official Node 24.21.0 Bookworm slim image pinned to the
// multi-platform registry digest pulled for this milestone.
const DefaultImage = "node@sha256:0e0ff40c39bc087845bfb27465a0df4ea419520094bc35842ff83dd8cbe6f9b6"

// buildCommand runs inside the sealed container. The input tar holds the app
// under app/ and, when it uses packages, the verified tarballs under
// packages/. npm only ever reads those tarballs: the container has no
// network, and install scripts never run. After the build, devDependencies
// are pruned so the release carries only what `npm start` needs.
// node_modules/.bin is left out because it holds symlinks; the runtime
// recreates it when it unpacks the release. Stdout is the output tar, so every
// other command is silenced. Exit 3 means the install failed, 4 the build.
const buildCommand = `set -e
tar -x --no-same-owner --no-same-permissions -C /workspace
cd /workspace/app
if [ -d /workspace/packages ]; then
  npm cache add /workspace/packages/*.tgz --offline --no-audit --no-fund >/dev/null 2>&1 || exit 3
  rm -rf /workspace/packages
fi
npm ci --offline --ignore-scripts --no-audit --no-fund >/dev/null 2>&1 || exit 3
npm --ignore-scripts run build >/dev/null 2>&1 || exit 4
if [ -d node_modules ]; then
  npm prune --omit=dev --offline --ignore-scripts --no-audit --no-fund >/dev/null 2>&1 || exit 3
fi
if [ -d node_modules ]; then set -- dist package.json node_modules; else set -- dist package.json; fi
tar --format=ustar --exclude=node_modules/.bin -cf - "$@"`

// Packages downloads the tarballs a lockfile names. *npmfetch.Fetcher
// implements it; tests substitute a fake.
type Packages interface {
	Fetch(ctx context.Context, lock []byte) ([]npmfetch.Package, error)
}

type Runner struct {
	image    string
	timeout  time.Duration
	packages Packages
}

// NewRunner builds Node apps in the pinned image. Without packages (nil) it
// only builds apps that use no npm packages, and never touches the network.
func NewRunner(image string, timeout time.Duration, packages Packages) (*Runner, error) {
	if !pinnedImage.MatchString(image) || timeout <= 0 || timeout > 5*time.Minute {
		return nil, ErrImage
	}
	return &Runner{image: image, timeout: timeout, packages: packages}, nil
}

// RunArchive extracts, validates, and builds a Node app in a disposable
// offline Docker container. The validated output is discarded.
func (r *Runner) RunArchive(ctx context.Context, archive io.Reader) (summary source.Summary, err error) {
	if r == nil {
		return summary, ErrImage
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return source.WithArchive(ctx, archive, func(ctx context.Context, files fs.FS) error {
		return r.RunFiles(ctx, files)
	})
}

// RunFiles validates and builds a prepared filesystem, discarding the output.
func (r *Runner) RunFiles(ctx context.Context, files fs.FS) error {
	_, err := r.BuildFiles(ctx, files)
	return err
}

// BuildFiles returns a bounded, normalized tar of dist/, package.json and the
// production node_modules from a prepared source filesystem. The caller owns
// files and must keep them available until return.
func (r *Runner) BuildFiles(ctx context.Context, files fs.FS) ([]byte, error) {
	if r == nil {
		return nil, ErrImage
	}
	if err := Validate(ctx, files); err != nil {
		return nil, err
	}
	packages, err := r.fetch(ctx, files)
	if err != nil {
		return nil, err
	}
	input, err := writeInput(ctx, files, packages)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = input.Close()
		_ = os.Remove(input.Name())
	}()
	runCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	raw, err := r.execute(runCtx, input)
	if err != nil {
		return nil, err
	}
	var artifact []byte
	_, err = source.WithArchiveLimits(runCtx, bytes.NewReader(raw), source.ArtifactLimits, func(ctx context.Context, built fs.FS) error {
		info, statErr := fs.Stat(built, "dist")
		if statErr != nil || !info.IsDir() {
			return ErrArtifact
		}
		// The release must be self-contained: the runtime starts it with
		// npm, which reads package.json.
		manifest, statErr := fs.Stat(built, "package.json")
		if statErr != nil || !manifest.Mode().IsRegular() {
			return ErrArtifact
		}
		var buffer bytes.Buffer
		if err := pack(ctx, built, "", tar.NewWriter(&buffer), &buffer, source.ArtifactLimits.ArchiveBytes); err != nil {
			return err
		}
		artifact = buffer.Bytes()
		return nil
	})
	if err != nil {
		if runCtx.Err() != nil {
			return nil, runCtx.Err()
		}
		if errors.Is(err, source.ErrLimit) {
			return nil, ErrArtifactSize
		}
		return nil, ErrArtifact
	}
	return artifact, nil
}

// fetch downloads the app's packages. An app with none needs no network.
func (r *Runner) fetch(ctx context.Context, files fs.FS) ([]npmfetch.Package, error) {
	var lock lockfile
	if err := readJSON(files, "package-lock.json", 1<<20, &lock); err != nil {
		return nil, ErrContract
	}
	if len(lock.Packages) <= 1 {
		return nil, nil
	}
	if r.packages == nil {
		return nil, ErrDependencies
	}
	raw, err := fs.ReadFile(files, "package-lock.json")
	if err != nil {
		return nil, ErrContract
	}
	return r.packages.Fetch(ctx, raw)
}

// writeInput writes the container's input tar to a private temporary file:
// the app under app/ and each verified tarball under packages/. A file, not
// memory, because an app's packages can be far larger than its source.
func writeInput(ctx context.Context, files fs.FS, packages []npmfetch.Package) (input *os.File, err error) {
	input, err = os.CreateTemp("", "sprout-build-input-")
	if err != nil {
		return nil, ErrDocker
	}
	defer func() {
		if err != nil {
			_ = input.Close()
			_ = os.Remove(input.Name())
		}
	}()
	writer := tar.NewWriter(input)
	if err := pack(ctx, files, "app", writer, nil, 0); err != nil {
		return nil, err
	}
	for i, p := range packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		header := &tar.Header{Name: fmt.Sprintf("packages/%05d.tgz", i), Mode: 0o600, Size: p.Bytes, Typeflag: tar.TypeReg}
		if err := writer.WriteHeader(header); err != nil {
			return nil, ErrDocker
		}
		if err := npmfetch.Verify(p, writer); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, ErrDocker
	}
	if _, err := input.Seek(0, io.SeekStart); err != nil {
		return nil, ErrDocker
	}
	return input, nil
}

// pack writes files into writer under prefix with platform-owned metadata.
// When limit is positive, size (the bytes written so far) must stay within it.
func pack(ctx context.Context, files fs.FS, prefix string, writer *tar.Writer, size *bytes.Buffer, limit int64) error {
	err := fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return ErrContract
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		target := name
		if prefix != "" {
			target = path.Join(prefix, name)
		}
		if entry.IsDir() {
			return writer.WriteHeader(&tar.Header{Name: target + "/", Mode: 0700, Typeflag: tar.TypeDir})
		}
		content, err := fs.ReadFile(files, name)
		if err != nil {
			return ErrContract
		}
		if err := writer.WriteHeader(&tar.Header{Name: target, Mode: 0600, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			return ErrContract
		}
		if _, err := writer.Write(content); err != nil {
			return ErrContract
		}
		if limit > 0 && int64(size.Len()) > limit {
			return source.ErrLimit
		}
		return nil
	})
	if err != nil {
		return err
	}
	if prefix == "" {
		if writer.Close() != nil {
			return ErrContract
		}
		if limit > 0 && int64(size.Len()) > limit {
			return source.ErrLimit
		}
	}
	return nil
}

func (r *Runner) execute(ctx context.Context, input io.Reader) (artifact []byte, err error) {
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return nil, ErrDocker
	}
	name := "sprout-local-build-" + hex.EncodeToString(random[:])
	commandCtx, stop := context.WithCancel(ctx)
	defer stop()
	output := &boundedOutput{limit: int(source.MaxArchiveBytes), cancel: stop}
	command := exec.CommandContext(commandCtx, "docker", dockerArgs(name, r.image)...)
	command.Stdin = input
	command.Stdout = output
	command.Stderr = io.Discard
	command.WaitDelay = 5 * time.Second

	// Cleanup has its own bounded context so cancellation still stops the
	// specific container after the local Docker CLI has been interrupted.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if cleanupContainer(cleanupCtx, name, commandCtx.Err() != nil) != nil {
			err = errors.Join(err, ErrCleanup)
		}
	}()

	runErr := command.Run()
	if output.exceeded {
		return nil, ErrArtifactSize
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if runErr == nil {
		return output.buffer.Bytes(), nil
	}
	var exit *exec.ExitError
	if errors.As(runErr, &exit) {
		switch exit.ExitCode() {
		case 3:
			return nil, ErrInstall
		case 125, 126, 127: // Docker itself could not run the container
		default:
			return nil, ErrBuild
		}
	}
	return nil, ErrDocker
}

type boundedOutput struct {
	buffer   bytes.Buffer
	limit    int
	cancel   context.CancelFunc
	exceeded bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.exceeded {
		return len(p), nil
	}
	remaining := b.limit - b.buffer.Len()
	if len(p) > remaining {
		if remaining > 0 {
			_, _ = b.buffer.Write(p[:remaining])
		}
		b.exceeded = true
		b.cancel()
		return len(p), nil
	}
	_, _ = b.buffer.Write(p)
	return len(p), nil
}

func cleanupContainer(ctx context.Context, name string, waitForLateCreate bool) error {
	for {
		inspect := exec.CommandContext(ctx, "docker", "container", "inspect", name)
		inspect.Stdout = io.Discard
		inspect.Stderr = io.Discard
		if inspect.Run() == nil {
			remove := exec.CommandContext(ctx, "docker", "container", "rm", "--force", "--", name)
			remove.Stdout = io.Discard
			remove.Stderr = io.Discard
			if remove.Run() == nil {
				return nil
			}
		} else if !waitForLateCreate {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func dockerArgs(name, image string) []string {
	return []string{
		"run", "--name", name, "--pull=never", "--interactive", "--log-driver=none",
		"--label", "io.sprout.purpose=local-node-build", "--label", "io.sprout.owned=" + name,
		"--network", "none", "--user", "65532:65532", "--read-only", "--cap-drop", "ALL",
		"--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2g", "--memory-swap", "2g", "--pids-limit", "256",
		"--tmpfs", "/workspace:rw,nosuid,nodev,size=1073741824,uid=65532,gid=65532,mode=0700",
		"--tmpfs", "/tmp:rw,nosuid,nodev,size=536870912,uid=65532,gid=65532,mode=0700",
		"--workdir", "/workspace", "--env", "HOME=/tmp", "--env", "npm_config_cache=/tmp/npm-cache",
		"--env", "HTTP_PROXY=", "--env", "HTTPS_PROXY=", "--env", "ALL_PROXY=", "--env", "NO_PROXY=",
		"--env", "http_proxy=", "--env", "https_proxy=", "--env", "all_proxy=", "--env", "no_proxy=",
		image, "sh", "-c", buildCommand,
	}
}
