package nodeapp

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
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
	runner, err := NewRunner(DefaultImage, 30*time.Second)
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
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Name == "dist/server.mjs" {
			content, err := io.ReadAll(reader)
			if err != nil || string(content) != "// not executed" {
				t.Fatalf("unexpected built file: %q (%v)", content, err)
			}
			return
		}
	}
	t.Fatal("built file missing from artifact")
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
