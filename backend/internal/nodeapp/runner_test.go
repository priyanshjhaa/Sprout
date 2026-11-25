package nodeapp

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
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/npmfetch"
)

// Run with SPROUT_DOCKER_TEST=1 when the pinned image is available locally.
// Ordinary unit tests do not require access to a Docker daemon.
func TestDockerSmokeRunner(t *testing.T) {
	if os.Getenv("SPROUT_DOCKER_TEST") != "1" {
		t.Skip("set SPROUT_DOCKER_TEST=1 to run the local Docker smoke test")
	}
	inspectCtx, cancelInspect := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelInspect()
	if err := exec.CommandContext(inspectCtx, "docker", "image", "inspect", DefaultImage).Run(); err != nil {
		t.Fatalf("pinned image unavailable: %v", err)
	}
	before := localBuildContainers(t)
	runner, err := NewRunner(DefaultImage, 30*time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		script string
		want   error
		limit  time.Duration
	}{
		{name: "success", script: `node -e 'require("fs").mkdirSync("dist");require("fs").copyFileSync("server.mjs","dist/server.mjs")'`, limit: 20 * time.Second},
		{name: "build failure", script: `node -e 'console.log("secret-marker");process.exit(7)'`, want: ErrBuild, limit: 20 * time.Second},
		{name: "unsafe output", script: "ln -s /etc dist", want: ErrArtifact, limit: 20 * time.Second},
		{name: "oversized output", script: `node -e 'require("fs").mkdirSync("dist");require("fs").writeFileSync("dist/large",Buffer.alloc(17*1024*1024))'`, want: ErrArtifactSize, limit: 20 * time.Second},
		{name: "cancellation", script: "node -e 'setInterval(() => {}, 1000)'", want: context.DeadlineExceeded, limit: 2 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest, lock := sample()
			manifest.Scripts["build"] = tc.script
			ctx, cancel := context.WithTimeout(context.Background(), tc.limit)
			defer cancel()
			artifact, err := runner.BuildFiles(ctx, filesFor(t, manifest, lock))
			if errors.Is(err, ErrCleanup) {
				t.Fatalf("container cleanup failed: %v", err)
			}
			if !errors.Is(err, tc.want) && !(err == nil && tc.want == nil) {
				t.Fatalf("build result: got %v, want %v", err, tc.want)
			}
			if strings.Contains(errString(err), "secret-marker") {
				t.Fatal("build output leaked through error")
			}
			if tc.want == nil {
				checkArtifact(t, artifact)
			} else if len(artifact) != 0 {
				t.Fatal("failed build returned an artifact")
			}
			after := localBuildContainers(t)
			if after != before {
				t.Fatalf("build left a container behind: before %q, after %q", before, after)
			}
		})
	}
}

func checkArtifact(t *testing.T, artifact []byte) {
	t.Helper()
	reader := tar.NewReader(bytes.NewReader(artifact))
	var sawServer, sawManifest bool
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch header.Name {
		case "package.json":
			sawManifest = true
		case "dist/server.mjs":
			content, err := io.ReadAll(reader)
			if err != nil || string(content) != "// not executed" {
				t.Fatalf("unexpected built file: %q (%v)", content, err)
			}
			sawServer = true
		}
	}
	if !sawServer || !sawManifest {
		t.Fatalf("artifact incomplete: dist/server.mjs=%v package.json=%v", sawServer, sawManifest)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func localBuildContainers(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "docker", "ps", "--all", "--filter", "label=io.sprout.purpose=local-node-build", "--format", "{{.Names}}").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(output))
}

// noPackages pretends every package is already provided, but hands over none,
// so npm has to fail offline instead of reaching for the network.
type noPackages struct{}

func (noPackages) Fetch(context.Context, []byte) ([]npmfetch.Package, error) { return nil, nil }

// TestDockerBuildsAnAppWithPackages builds dev/node-express-example for real:
// packages are downloaded from the npm registry into a temporary cache, then
// installed inside the sealed container with no network. It needs Docker, the
// pinned image and internet access, so it runs only with SPROUT_DOCKER_TEST=1.
func TestDockerBuildsAnAppWithPackages(t *testing.T) {
	if os.Getenv("SPROUT_DOCKER_TEST") != "1" {
		t.Skip("set SPROUT_DOCKER_TEST=1 to run the local Docker build test")
	}
	before := localBuildContainers(t)
	example := os.DirFS("../../dev/node-express-example")
	fetcher, err := npmfetch.New(filepath.Join(t.TempDir(), "npm"), npmfetch.BuildPlatform())
	if err != nil {
		t.Fatal(err)
	}
	runner, err := NewRunner(DefaultImage, 2*time.Minute, fetcher)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	artifact, err := runner.BuildFiles(ctx, example)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	names := map[string]bool{}
	reader := tar.NewReader(bytes.NewReader(artifact))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names[header.Name] = true
	}
	for _, want := range []string{"package.json", "dist/server.mjs", "node_modules/express/package.json"} {
		if !names[want] {
			t.Errorf("the release lacks %s", want)
		}
	}
	for name := range names {
		if strings.HasPrefix(name, "node_modules/is-number/") {
			t.Errorf("a devDependency reached the release: %s", name)
		}
		if strings.HasPrefix(name, "node_modules/.bin/") {
			t.Errorf("node_modules/.bin reached the release: %s", name)
		}
	}
	if after := localBuildContainers(t); after != before {
		t.Fatalf("build left a container behind: before %q, after %q", before, after)
	}

	// Without its packages, the offline install fails, and says so.
	runner, _ = NewRunner(DefaultImage, time.Minute, noPackages{})
	if _, err := runner.BuildFiles(ctx, example); !errors.Is(err, ErrInstall) {
		t.Fatalf("missing packages: got %v, want %v", err, ErrInstall)
	}
}
