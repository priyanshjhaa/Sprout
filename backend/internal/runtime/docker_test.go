package runtime

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
)

var testRelease = Release{ApplicationID: "11111111-1111-4111-8111-111111111111", DeploymentID: "22222222-2222-4222-8222-222222222222"}

const testImage = "node@sha256:0e0ff40c39bc087845bfb27465a0df4ea419520094bc35842ff83dd8cbe6f9b6"

// Command construction is a security boundary, so the flags are asserted.
func TestRunArgsLockDownTheApplication(t *testing.T) {
	d, err := NewDocker(testImage)
	if err != nil {
		t.Fatal(err)
	}
	args := d.runArgs(testRelease)
	joined := " " + strings.Join(args, " ") + " "
	for _, want := range []string{
		" --user 65532:65532 ", " --read-only ", " --cap-drop ALL ", " --security-opt no-new-privileges ",
		" --pids-limit 64 ", " --memory 256m ", " --memory-swap 256m ", " --cpus 0.5 ", " --network sprout-runtime ",
		" --publish 127.0.0.1::3000 ", " --pull=never ", " --restart no ",
		" --mount type=volume,src=sprout-release-" + testRelease.DeploymentID + ",dst=/app,readonly ",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("run args missing %q", want)
		}
	}
	for _, banned := range []string{"--privileged", "--network host", "--ipc", "docker.sock", "--volume", " -v ", "type=bind", "--cap-add", "--device"} {
		if strings.Contains(joined, banned) {
			t.Errorf("run args contain %q", banned)
		}
	}
	for _, arg := range args {
		if arg == "--pid" || strings.HasPrefix(arg, "--pid=") || arg == "--uts" || strings.HasPrefix(arg, "--uts=") {
			t.Errorf("run args share a host namespace: %s", arg)
		}
	}
	if !slices.Equal(args[len(args)-4:], []string{testImage, "npm", "--ignore-scripts", "start"}) {
		t.Errorf("platform-owned start command not last: %v", args[len(args)-4:])
	}
}

func TestPrepareArgsHaveNoNetworkOrCapabilities(t *testing.T) {
	d, _ := NewDocker(testImage)
	joined := " " + strings.Join(d.prepareArgs(testRelease), " ") + " "
	for _, want := range []string{" --network none ", " --read-only ", " --cap-drop ALL ", " --security-opt no-new-privileges "} {
		if !strings.Contains(joined, want) {
			t.Errorf("prepare args missing %q", want)
		}
	}
}

func TestInvalidReleaseAndImageAreRejected(t *testing.T) {
	if _, err := NewDocker("node:24"); err == nil {
		t.Fatal("unpinned image accepted")
	}
	d, _ := NewDocker(testImage)
	bad := Release{ApplicationID: "x; rm -rf /", DeploymentID: testRelease.DeploymentID}
	ctx := context.Background()
	if d.Prepare(ctx, bad, []byte("x")) != ErrInvalid || d.Stop(ctx, bad) != ErrInvalid || d.Destroy(ctx, bad) != ErrInvalid {
		t.Fatal("invalid release reached the Docker CLI")
	}
	if _, err := d.Start(ctx, bad); err != ErrInvalid {
		t.Fatal("invalid release started")
	}
}

func TestLoopbackEndpointAndInventoryParsing(t *testing.T) {
	if got := loopbackEndpoint("0.0.0.0:5000\n127.0.0.1:49153\n[::]:5000\n"); got != "127.0.0.1:49153" {
		t.Fatalf("endpoint = %q", got)
	}
	if loopbackEndpoint("0.0.0.0:5000\n") != "" {
		t.Fatal("non-loopback mapping accepted")
	}
	got := parseReleases(testRelease.ApplicationID + " " + testRelease.DeploymentID + "\n<no value> junk\n\n")
	if len(got) != 1 || got[0] != testRelease {
		t.Fatalf("parsed = %v", got)
	}
}

func exampleArchive(t *testing.T) []byte {
	t.Helper()
	server, err := os.ReadFile("../../dev/node-example/server.mjs")
	if err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"sprout-node-example","version":"1.0.0","private":true,"type":"module","scripts":{"start":"node dist/server.mjs"}}`
	var buffer bytes.Buffer
	w := tar.NewWriter(&buffer)
	add := func(name string, mode int64, typ byte, body []byte) {
		if err := w.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(body)), Typeflag: typ}); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(body)
	}
	add("dist/", 0700, tar.TypeDir, nil)
	add("dist/server.mjs", 0600, tar.TypeReg, server)
	add("package.json", 0600, tar.TypeReg, []byte(manifest))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// Run with SPROUT_DOCKER_TEST=1; prepares, starts, probes, stops and destroys
// the example app, then checks nothing labelled for it remains.
func TestDockerRuntimeLifecycle(t *testing.T) {
	if os.Getenv("SPROUT_DOCKER_TEST") != "1" {
		t.Skip("set SPROUT_DOCKER_TEST=1 to run the local Docker runtime test")
	}
	d, _ := NewDocker(testImage)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	defer d.Destroy(context.Background(), testRelease)

	if err := d.Prepare(ctx, testRelease, exampleArchive(t)); err != nil {
		t.Fatal(err)
	}
	instance, err := d.Start(ctx, testRelease)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(instance.Endpoint, "127.0.0.1:") {
		t.Fatalf("endpoint %q is not loopback", instance.Endpoint)
	}
	var status int
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(300 * time.Millisecond) {
		resp, err := http.Get("http://" + instance.Endpoint + "/health")
		if err == nil {
			status = resp.StatusCode
			resp.Body.Close()
			break
		}
	}
	if status != http.StatusOK {
		t.Fatalf("/health status = %d", status)
	}
	inventory, err := d.List(ctx)
	if err != nil || !slices.Contains(inventory.Containers, testRelease) || !slices.Contains(inventory.Releases, testRelease) {
		t.Fatalf("inventory = %+v (%v)", inventory, err)
	}
	if err := d.Stop(ctx, testRelease); err != nil {
		t.Fatal(err)
	}
	if inventory, _ = d.List(ctx); slices.Contains(inventory.Containers, testRelease) || !slices.Contains(inventory.Releases, testRelease) {
		t.Fatalf("after Stop: %+v", inventory)
	}
	if err := d.Destroy(ctx, testRelease); err != nil {
		t.Fatal(err)
	}
	if inventory, _ = d.List(ctx); slices.Contains(inventory.Releases, testRelease) {
		t.Fatalf("after Destroy: %+v", inventory)
	}
	if err := d.Destroy(ctx, testRelease); err != nil {
		t.Fatalf("destroying twice must succeed: %v", err)
	}
}

// Run with SPROUT_DOCKER_TEST=1. A request for the application's public host
// wakes a real container through the supervisor and proxy; once the container
// is gone (sleep, crash) the next request starts it again.
func TestProxyWakesARealContainer(t *testing.T) {
	if os.Getenv("SPROUT_DOCKER_TEST") != "1" {
		t.Skip("set SPROUT_DOCKER_TEST=1 to run the local Docker runtime test")
	}
	driver, _ := NewDocker(testImage)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	defer driver.Destroy(context.Background(), testRelease)

	// The release volume does not exist: the supervisor must rebuild it from the artifact.
	supervisor := newSupervisor(t, driver, archiveLoader{exampleArchive(t)}, 2)
	proxy, err := NewProxy("localhost", func(context.Context, string, string) (Route, error) { return testRoute, nil }, supervisor, archiveLoader{exampleArchive(t)}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(proxy)
	defer front.Close()

	fetch := func() string {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, front.URL+"/", nil)
		request.Host = "shop.acme.localhost"
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status %d: %s", response.StatusCode, body)
		}
		return string(body)
	}
	if body := fetch(); !strings.Contains(body, "ready for Sprout") {
		t.Fatalf("cold start body = %q", body)
	}
	// Simulate sleep: the container disappears behind the supervisor's back.
	if err := driver.Stop(ctx, testRelease); err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, front.URL+"/", nil)
	request.Host = "shop.acme.localhost"
	if response, err := http.DefaultClient.Do(request); err == nil {
		if response.StatusCode != http.StatusBadGateway {
			t.Fatalf("stopped app status = %d, want 502", response.StatusCode)
		}
		response.Body.Close()
	}
	// The failure made the supervisor forget it; the next request wakes it again.
	if body := fetch(); !strings.Contains(body, "ready for Sprout") {
		t.Fatalf("wake body = %q", body)
	}
}

type archiveLoader struct{ archive []byte }

func (l archiveLoader) Load(context.Context, artifact.Descriptor) ([]byte, error) {
	return l.archive, nil
}

// Run with SPROUT_DOCKER_TEST=1. Idle sleep stops the container but keeps the
// release files; archiving removes the files as well.
func TestReconcileSleepsAndArchivesARealApp(t *testing.T) {
	if os.Getenv("SPROUT_DOCKER_TEST") != "1" {
		t.Skip("set SPROUT_DOCKER_TEST=1 to run the local Docker runtime test")
	}
	driver, _ := NewDocker(testImage)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	defer driver.Destroy(context.Background(), testRelease)
	supervisor := newSupervisor(t, driver, archiveLoader{exampleArchive(t)}, 2)
	clock := time.Unix(1_000_000, 0)
	supervisor.now = func() time.Time { return clock }

	if _, err := supervisor.Ensure(ctx, testRoute); err != nil {
		t.Fatal(err)
	}
	active := source(Desired{Live: []Route{testRoute}})
	state := func() Inventory { inv, _ := driver.List(ctx); return inv }

	clock = clock.Add(11 * time.Minute)
	if err := supervisor.reconcile(ctx, active, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	if inv := state(); slices.Contains(inv.Containers, testRelease) || !slices.Contains(inv.Releases, testRelease) {
		t.Fatalf("after idle sleep: %+v (want container gone, files kept)", inv)
	}
	// The app wakes again from its kept files.
	if _, err := supervisor.Ensure(ctx, testRoute); err != nil {
		t.Fatal(err)
	}
	archived := testRoute
	archived.Lifecycle = "archived"
	if err := supervisor.reconcile(ctx, source(Desired{Live: []Route{archived}}), 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	if inv := state(); slices.Contains(inv.Containers, testRelease) || slices.Contains(inv.Releases, testRelease) {
		t.Fatalf("after archive: %+v (want everything gone)", inv)
	}
}
