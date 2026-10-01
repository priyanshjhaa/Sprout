package nodeapp

import (
	"context"
	"errors"
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
		{name: "success", script: "node -e 'process.exit(0)'", limit: 20 * time.Second},
		{name: "build failure", script: "node -e 'process.exit(7)'", want: ErrBuild, limit: 20 * time.Second},
		{name: "cancellation", script: "node -e 'setInterval(() => {}, 1000)'", want: context.DeadlineExceeded, limit: 2 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest, lock := sample()
			manifest.Scripts["build"] = tc.script
			ctx, cancel := context.WithTimeout(context.Background(), tc.limit)
			defer cancel()
			err := runner.RunFiles(ctx, filesFor(t, manifest, lock))
			if errors.Is(err, ErrCleanup) {
				t.Fatalf("container cleanup failed: %v", err)
			}
			if !errors.Is(err, tc.want) && !(err == nil && tc.want == nil) {
				t.Fatalf("build result: got %v, want %v", err, tc.want)
			}
			after := localBuildContainers(t)
			if after != before {
				t.Fatalf("build left a container behind: before %q, after %q", before, after)
			}
		})
	}
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
