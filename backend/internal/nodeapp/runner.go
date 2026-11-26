package nodeapp

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/buildbox"
	"github.com/priyanshjhaa/Sprout/backend/internal/npmfetch"
	"github.com/priyanshjhaa/Sprout/backend/internal/source"
	"github.com/priyanshjhaa/Sprout/backend/internal/staticsite"
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

// The build runs inside the sealed container. The input tar holds the app
// under app/ and, when it uses packages, the verified tarballs under
// packages/. npm only ever reads those tarballs: the container has no
// network, and install scripts never run. Stdout is the output tar, so every
// other command is silenced. Exit 3 means the install failed, 4 the build, 5
// that the build made no dist/ folder.
const installAndBuild = `set -e
tar -x --no-same-owner --no-same-permissions -C /workspace
cd /workspace/app
if [ -d /workspace/packages ]; then
  npm cache add /workspace/packages/*.tgz --offline --no-audit --no-fund >/dev/null 2>&1 || exit 3
  rm -rf /workspace/packages
fi
npm ci --offline --ignore-scripts --no-audit --no-fund >/dev/null 2>&1 || exit 3
npm --ignore-scripts run build >/dev/null 2>&1 || exit 4
[ -d dist ] || exit 5
`

// serverCommand then prunes devDependencies, so the release carries only what
// `npm start` needs. node_modules/.bin is left out because it holds symlinks;
// the runtime recreates it when it unpacks the release.
const serverCommand = installAndBuild + `if [ -d node_modules ]; then
  npm prune --omit=dev --offline --ignore-scripts --no-audit --no-fund >/dev/null 2>&1 || exit 3
fi
if [ -d node_modules ]; then set -- dist package.json node_modules; else set -- dist package.json; fi
tar --format=ustar --exclude=node_modules/.bin -cf - "$@"`

// websiteCommand returns only what is inside dist/: the website itself.
// Hidden files are not served, so the glob leaving them out is fine.
const websiteCommand = installAndBuild + `cd dist
tar --format=ustar -cf - -- *`

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
	raw, err := r.run(ctx, files, serverCommand)
	if err != nil {
		return nil, err
	}
	var artifact []byte
	_, err = source.WithArchiveLimits(ctx, bytes.NewReader(raw), source.ArtifactLimits, func(ctx context.Context, built fs.FS) error {
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
		if err := buildbox.Pack(ctx, built, "", tar.NewWriter(&buffer), &buffer, source.ArtifactLimits.ArchiveBytes); err != nil {
			return err
		}
		artifact = buffer.Bytes()
		return nil
	})
	return artifact, outputError(ctx, err)
}

// BuildWebsite builds a project without a start script and returns its dist/
// folder as a static site archive, checked by the same rules as a plain
// static site: an index.html at the top, web-safe names, the source limits.
func (r *Runner) BuildWebsite(ctx context.Context, files fs.FS) ([]byte, error) {
	raw, err := r.run(ctx, files, websiteCommand)
	if err != nil {
		return nil, err
	}
	var site []byte
	_, err = source.WithArchive(ctx, bytes.NewReader(raw), func(ctx context.Context, built fs.FS) error {
		if err := staticsite.Validate(ctx, built); err != nil {
			return err
		}
		site, err = staticsite.Pack(ctx, built)
		return err
	})
	if errors.Is(err, staticsite.ErrContract) {
		return nil, err
	}
	return site, outputError(ctx, err)
}

// run validates the app, fetches its packages and runs command in the sealed
// container, returning the raw output tar.
func (r *Runner) run(ctx context.Context, files fs.FS, command string) ([]byte, error) {
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
	return r.execute(runCtx, input, command)
}

// outputError words a problem with the build's output.
func outputError(ctx context.Context, err error) error {
	switch {
	case err == nil:
		return nil
	case ctx.Err() != nil:
		return ctx.Err()
	case errors.Is(err, source.ErrLimit), errors.Is(err, buildbox.ErrLimit):
		return ErrArtifactSize
	}
	return ErrArtifact
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
	if err := buildbox.Pack(ctx, files, "app", writer, nil, 0); err != nil {
		if errors.Is(err, buildbox.ErrFiles) {
			return nil, ErrContract
		}
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

// box is the sealed build container for Node.
func (r *Runner) box() buildbox.Box {
	return buildbox.Box{Image: r.image, Purpose: "local-node-build", Env: []string{"npm_config_cache=/tmp/npm-cache"}}
}

// execute runs script in the sealed container and gives its exit codes and
// failures Node's own codes.
func (r *Runner) execute(ctx context.Context, input io.Reader, script string) ([]byte, error) {
	output, err := r.box().Run(ctx, input, script, int(source.MaxArchiveBytes))
	var cleanup error
	if errors.Is(err, buildbox.ErrCleanup) {
		cleanup = ErrCleanup
	}
	var exit *buildbox.ExitError
	switch {
	case err == nil:
		return output, nil
	case errors.As(err, &exit) && exit.Code == 3:
		err = ErrInstall
	case errors.As(err, &exit) && exit.Code == 5:
		err = ErrArtifact
	case errors.As(err, &exit):
		err = ErrBuild
	case errors.Is(err, buildbox.ErrTooLarge):
		err = ErrArtifactSize
	case ctx.Err() != nil:
		err = ctx.Err()
	case errors.Is(err, buildbox.ErrDocker):
		err = ErrDocker
	case cleanup != nil:
		err = nil
	}
	return nil, errors.Join(err, cleanup)
}
