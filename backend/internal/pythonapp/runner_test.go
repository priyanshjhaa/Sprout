package pythonapp

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/pypifetch"
)

func TestNewRunnerWantsAPinnedImage(t *testing.T) {
	for _, image := range []string{"python:3.13", "node@sha256:" + strings.Repeat("a", 64), "python@sha256:short"} {
		if _, err := NewRunner(image, time.Minute, nil); !errors.Is(err, ErrImage) {
			t.Errorf("%s: %v", image, err)
		}
	}
	if _, err := NewRunner(DefaultImage, 10*time.Minute, nil); !errors.Is(err, ErrImage) {
		t.Error("an unbounded timeout was accepted")
	}
}

func TestRequirementsNeedAFetcher(t *testing.T) {
	runner, err := NewRunner(DefaultImage, time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.BuildFiles(context.Background(), project(nil)); !errors.Is(err, ErrDependencies) {
		t.Fatalf("err %v", err)
	}
}

func dockerTest(t *testing.T) {
	t.Helper()
	if os.Getenv("SPROUT_DOCKER_TEST") != "1" {
		t.Skip("set SPROUT_DOCKER_TEST=1 to run the local Docker build test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "docker", "image", "inspect", DefaultImage).Run(); err != nil {
		t.Fatalf("pinned image unavailable: %v", err)
	}
}

func names(t *testing.T, artifact []byte) map[string]bool {
	t.Helper()
	found := map[string]bool{}
	reader := tar.NewReader(bytes.NewReader(artifact))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return found
		}
		if err != nil {
			t.Fatal(err)
		}
		found[header.Name] = true
	}
}

// TestDockerBuildsTheFlaskExample builds dev/python-flask-example for real:
// wheels are downloaded from PyPI into a temporary cache, then installed
// inside the sealed container with no network. It needs Docker, the pinned
// image and internet access.
func TestDockerBuildsTheFlaskExample(t *testing.T) {
	dockerTest(t)
	fetcher, err := pypifetch.New(filepath.Join(t.TempDir(), "wheels"), pypifetch.BuildPlatform())
	if err != nil {
		t.Fatal(err)
	}
	runner, err := NewRunner(DefaultImage, 2*time.Minute, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	artifact, err := runner.BuildFiles(ctx, os.DirFS("../../dev/python-flask-example"))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	found := names(t, artifact)
	for _, want := range []string{"app.py", "Procfile", "requirements.txt", "site-packages/flask/__init__.py", "site-packages/bin/gunicorn"} {
		if !found[want] {
			t.Errorf("the release lacks %s", want)
		}
	}
	compiled := false
	for name := range found {
		if strings.Contains(name, "markupsafe/_speedups") && strings.HasSuffix(name, ".so") {
			compiled = true
		}
		if strings.Contains(name, "__pycache__") || strings.HasSuffix(name, ".pyc") {
			t.Errorf("bytecode reached the release: %s", name)
		}
	}
	if !compiled {
		t.Error("markupsafe's compiled wheel for this platform was not installed")
	}
}

// fixed hands the build a fixed set of wheels, or none.
type fixed []pypifetch.Package

func (f fixed) Fetch(context.Context, []byte) ([]pypifetch.Package, error) { return f, nil }

func TestDockerBuildFailures(t *testing.T) {
	dockerTest(t)
	runner, err := NewRunner(DefaultImage, time.Minute, fixed(nil))
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		files fstest.MapFS
		want  error
	}{
		// With no wheels handed in, pip has nothing to install from and
		// cannot reach the network to look.
		"missing wheel": {project(nil), ErrInstall},
		"syntax error":  {project(map[string]string{"requirements.txt": "\n", "app.py": "def broken(:\n"}), ErrBuild},
		"no packages":   {project(map[string]string{"requirements.txt": "\n", "lib/helpers.py": "VALUE = 1\n"}), nil},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			artifact, err := runner.BuildFiles(ctx, tc.files)
			if !errors.Is(err, tc.want) && !(err == nil && tc.want == nil) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if tc.want == nil && !names(t, artifact)["lib/helpers.py"] {
				t.Fatal("the release lacks the app's own files")
			}
		})
	}
}
