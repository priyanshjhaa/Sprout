package pythonapp

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"regexp"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/buildbox"
	"github.com/priyanshjhaa/Sprout/backend/internal/pypifetch"
	"github.com/priyanshjhaa/Sprout/backend/internal/source"
)

var (
	ErrImage        = errors.New("python_build_image_invalid")
	ErrDependencies = errors.New("python_build_dependencies_unsupported")
	ErrInstall      = errors.New("python_build_install_failed")
	ErrBuild        = errors.New("python_build_failed")
	ErrArtifact     = errors.New("python_build_artifact_invalid")
	ErrArtifactSize = errors.New("python_build_artifact_too_large")
	ErrDocker       = errors.New("python_build_docker_unavailable")
	ErrCleanup      = errors.New("python_build_cleanup_failed")
)

var pinnedImage = regexp.MustCompile(`^python@sha256:[a-f0-9]{64}$`)

// DefaultImage is the official Python 3.13 Bookworm slim image pinned to the
// multi-platform registry digest pulled for this milestone. The runtime runs
// releases in the same image, so the interpreter that installed the packages
// is the one that runs them.
const DefaultImage = "python@sha256:f0fa218d5b06f9b9235024fa061cde2cb141f726de52134471053d2d9b5324a3"

// buildScript runs inside the sealed container. The input tar holds the app
// under app/ and, when it has requirements, the verified wheels under
// wheels/. pip reads only those wheels: --no-index means it never looks for
// another, --only-binary means it never runs a package's setup code, and
// --require-hashes means it checks every wheel against requirements.txt once
// more. Then every .py file is compiled in memory, so a syntax error fails the
// build instead of the health check. Stdout is the output tar, so every other
// command is silenced. Exit 3 means the install failed, 4 a syntax error.
const buildScript = `set -e
tar -x --no-same-owner --no-same-permissions -C /workspace
cd /workspace/app
if [ -d /workspace/wheels ]; then
  python -m pip install --no-index --find-links /workspace/wheels --only-binary :all: --require-hashes \
    --no-compile --no-input --disable-pip-version-check --no-cache-dir --target site-packages \
    -r requirements.txt >/dev/null 2>&1 || exit 3
  rm -rf /workspace/wheels
fi
python -I - >/dev/null 2>&1 <<'CHECK' || exit 4
import os
for root, dirs, files in os.walk("."):
    if root == ".":
        dirs[:] = [d for d in dirs if d != "site-packages"]
    for name in files:
        if name.endswith(".py"):
            path = os.path.join(root, name)
            with open(path, "rb") as source:
                compile(source.read(), path, "exec", dont_inherit=True)
CHECK
set -- *
tar --format=ustar -cf - -- "$@"`

// Packages downloads the wheels a requirements.txt names.
// *pypifetch.Fetcher implements it; tests substitute a fake.
type Packages interface {
	Fetch(ctx context.Context, requirements []byte) ([]pypifetch.Package, error)
}

type Runner struct {
	image    string
	timeout  time.Duration
	packages Packages
}

// NewRunner builds Python apps in the pinned image. Without packages (nil) it
// only builds apps with an empty requirements.txt, and never touches the
// network.
func NewRunner(image string, timeout time.Duration, packages Packages) (*Runner, error) {
	if !pinnedImage.MatchString(image) || timeout <= 0 || timeout > 5*time.Minute {
		return nil, ErrImage
	}
	return &Runner{image: image, timeout: timeout, packages: packages}, nil
}

// BuildFiles returns a bounded, normalized tar of the app with its packages
// installed in site-packages/. The caller owns files and must keep them
// available until return.
func (r *Runner) BuildFiles(ctx context.Context, files fs.FS) ([]byte, error) {
	if r == nil {
		return nil, ErrImage
	}
	if err := Validate(ctx, files); err != nil {
		return nil, err
	}
	packages, wanted, err := r.fetch(ctx, files)
	if err != nil {
		return nil, err
	}
	input, err := writeInput(ctx, files, packages, wanted)
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
	_, err = source.WithArchiveLimits(ctx, bytes.NewReader(raw), source.ArtifactLimits, func(ctx context.Context, built fs.FS) error {
		// The release must be self-contained: the runtime reads its Procfile.
		for _, name := range []string{"Procfile", "requirements.txt"} {
			if info, statErr := fs.Stat(built, name); statErr != nil || !info.Mode().IsRegular() {
				return ErrArtifact
			}
		}
		var buffer bytes.Buffer
		if err := buildbox.Pack(ctx, built, "", tar.NewWriter(&buffer), &buffer, source.ArtifactLimits.ArchiveBytes); err != nil {
			return err
		}
		artifact = buffer.Bytes()
		return nil
	})
	switch {
	case err == nil:
		return artifact, nil
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case errors.Is(err, source.ErrLimit), errors.Is(err, buildbox.ErrLimit):
		return nil, ErrArtifactSize
	}
	return nil, ErrArtifact
}

// fetch downloads the app's wheels. It also reports whether pip must run at
// all: a line with a marker may need no wheel here, but pip still decides.
func (r *Runner) fetch(ctx context.Context, files fs.FS) ([]pypifetch.Package, bool, error) {
	requirements, err := readSmall(files, "requirements.txt", pypifetch.MaxRequirementsBytes)
	if err != nil {
		return nil, false, broke(reasonRequirements)
	}
	parsed, err := pypifetch.ParseRequirements(requirements)
	if err != nil {
		return nil, false, broke(reasonRequirements)
	}
	if len(parsed) == 0 {
		return nil, false, nil
	}
	if r.packages == nil {
		return nil, false, ErrDependencies
	}
	packages, err := r.packages.Fetch(ctx, requirements)
	return packages, true, err
}

// writeInput writes the container's input tar to a private temporary file:
// the app under app/ and each verified wheel under wheels/, by its own
// filename, which is where pip reads a wheel's tags from.
func writeInput(ctx context.Context, files fs.FS, packages []pypifetch.Package, wanted bool) (input *os.File, err error) {
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
	if wanted {
		if err := writer.WriteHeader(&tar.Header{Name: "wheels/", Mode: 0o700, Typeflag: tar.TypeDir}); err != nil {
			return nil, ErrDocker
		}
	}
	for _, p := range packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := writer.WriteHeader(&tar.Header{Name: "wheels/" + p.Filename, Mode: 0o600, Size: p.Bytes, Typeflag: tar.TypeReg}); err != nil {
			return nil, ErrDocker
		}
		if err := pypifetch.Verify(p, writer); err != nil {
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

// execute runs the build in the sealed container and gives its exit codes
// and failures Python's own codes.
func (r *Runner) execute(ctx context.Context, input io.Reader) ([]byte, error) {
	box := buildbox.Box{Image: r.image, Purpose: "local-python-build", Env: []string{"PIP_NO_INDEX=1", "PYTHONDONTWRITEBYTECODE=1"}}
	output, err := box.Run(ctx, input, buildScript, int(source.MaxArchiveBytes))
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
