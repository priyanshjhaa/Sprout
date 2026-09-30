package nodeapp

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os/exec"
	"regexp"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/source"
)

var (
	ErrImage        = errors.New("node_build_image_invalid")
	ErrDependencies = errors.New("node_build_dependencies_unsupported")
	ErrBuild        = errors.New("node_build_failed")
	ErrDocker       = errors.New("node_build_docker_unavailable")
	ErrCleanup      = errors.New("node_build_cleanup_failed")
)

var pinnedImage = regexp.MustCompile(`^node@sha256:[a-f0-9]{64}$`)

// DefaultImage is the official Node 24.21.0 Bookworm slim image pinned to the
// multi-platform registry digest pulled for this milestone.
const DefaultImage = "node@sha256:0e0ff40c39bc087845bfb27465a0df4ea419520094bc35842ff83dd8cbe6f9b6"

const buildCommand = "tar -x --no-same-owner --no-same-permissions -C /workspace && cd /workspace && npm ci --offline --ignore-scripts --no-audit --no-fund && npm --ignore-scripts run build"

type Runner struct {
	image   string
	timeout time.Duration
}

func NewRunner(image string, timeout time.Duration) (*Runner, error) {
	if !pinnedImage.MatchString(image) || timeout <= 0 || timeout > 2*time.Minute {
		return nil, ErrImage
	}
	return &Runner{image: image, timeout: timeout}, nil
}

// RunArchive extracts, validates, and builds a dependency-free Node app in a
// disposable offline Docker container. No build output is exported.
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

// RunFiles validates and builds a prepared filesystem. The caller owns files
// and must keep them available until this call returns.
func (r *Runner) RunFiles(ctx context.Context, files fs.FS) error {
	if r == nil {
		return ErrImage
	}
	if err := Validate(ctx, files); err != nil {
		return err
	}
	if err := offlineOnly(files); err != nil {
		return err
	}
	payload, err := pack(ctx, files)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.execute(runCtx, payload)
}

func offlineOnly(files fs.FS) error {
	var manifest Manifest
	if err := readJSON(files, "package.json", 256<<10, &manifest); err != nil {
		return ErrContract
	}
	if len(manifest.Dependencies) != 0 || len(manifest.DevDependencies) != 0 || len(manifest.OptionalDependencies) != 0 {
		return ErrDependencies
	}
	var lock lockfile
	if err := readJSON(files, "package-lock.json", 1<<20, &lock); err != nil {
		return ErrContract
	}
	if len(lock.Packages) != 1 {
		return ErrDependencies
	}
	return nil
}

func pack(ctx context.Context, files fs.FS) ([]byte, error) {
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
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
		if entry.IsDir() {
			return writer.WriteHeader(&tar.Header{Name: name + "/", Mode: 0700, Typeflag: tar.TypeDir})
		}
		content, err := fs.ReadFile(files, name)
		if err != nil {
			return ErrContract
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			return ErrContract
		}
		if _, err := writer.Write(content); err != nil {
			return ErrContract
		}
		if buffer.Len() > source.MaxSourceBytes+2*source.MaxEntries*512 {
			return source.ErrLimit
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if writer.Close() != nil {
		return nil, ErrContract
	}
	return buffer.Bytes(), nil
}

func (r *Runner) execute(ctx context.Context, payload []byte) (err error) {
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return ErrDocker
	}
	name := "sprout-local-build-" + hex.EncodeToString(random[:])
	command := exec.CommandContext(ctx, "docker", dockerArgs(name, r.image)...)
	command.Stdin = bytes.NewReader(payload)
	command.Stdout = io.Discard // Never return or persist untrusted build output.
	command.Stderr = io.Discard
	command.WaitDelay = 5 * time.Second

	// Cleanup has its own bounded context so cancellation still stops the
	// specific container after the local Docker CLI has been interrupted.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if cleanupContainer(cleanupCtx, name, ctx.Err() != nil) != nil {
			err = errors.Join(err, ErrCleanup)
		}
	}()

	runErr := command.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if runErr == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(runErr, &exit) && exit.ExitCode() != 125 && exit.ExitCode() != 126 && exit.ExitCode() != 127 {
		return ErrBuild
	}
	return ErrDocker
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
		"--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "1g", "--memory-swap", "1g", "--pids-limit", "128",
		"--tmpfs", "/workspace:rw,nosuid,nodev,size=536870912,uid=65532,gid=65532,mode=0700",
		"--tmpfs", "/tmp:rw,nosuid,nodev,size=67108864,uid=65532,gid=65532,mode=0700",
		"--workdir", "/workspace", "--env", "HOME=/tmp", "--env", "npm_config_cache=/tmp/npm-cache",
		"--env", "HTTP_PROXY=", "--env", "HTTPS_PROXY=", "--env", "ALL_PROXY=", "--env", "NO_PROXY=",
		"--env", "http_proxy=", "--env", "https_proxy=", "--env", "all_proxy=", "--env", "no_proxy=",
		image, "sh", "-c", buildCommand,
	}
}
