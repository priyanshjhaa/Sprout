package buildjob

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
	"github.com/priyanshjhaa/Sprout/backend/internal/deployment"
	"github.com/priyanshjhaa/Sprout/backend/internal/nodeapp"
	rt "github.com/priyanshjhaa/Sprout/backend/internal/runtime"
	"github.com/priyanshjhaa/Sprout/backend/internal/staticsite"
)

const (
	testApp = "11111111-1111-4111-8111-111111111111"
	testDep = "22222222-2222-4222-8222-222222222222"
	oldDep  = "33333333-3333-4333-8333-333333333333"
)

// fakeDriver records calls and holds an inventory; errors are injected per method.
type fakeDriver struct {
	prepareErr, startErr error
	inventory            rt.Inventory
	calls                []string
	destroyed            []rt.Release
}

func (d *fakeDriver) Prepare(_ context.Context, r rt.Release, archive []byte) error {
	d.calls = append(d.calls, "prepare")
	return d.prepareErr
}
func (d *fakeDriver) Start(_ context.Context, r rt.Release) (rt.Instance, error) {
	d.calls = append(d.calls, "start")
	return rt.Instance{Release: r, Endpoint: "127.0.0.1:1"}, d.startErr
}
func (d *fakeDriver) Endpoint(context.Context, rt.Release) (string, error) {
	return "", rt.ErrNotRunning
}
func (d *fakeDriver) Stop(context.Context, rt.Release) error { return nil }
func (d *fakeDriver) Destroy(_ context.Context, r rt.Release) error {
	d.destroyed = append(d.destroyed, r)
	return nil
}
func (d *fakeDriver) List(context.Context) (rt.Inventory, error) { return d.inventory, nil }

func healthy(context.Context, string) error { return nil }

func openStore(t *testing.T) *artifact.Store {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := artifact.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// archive packs the named files from the committed Node example, plus extras.
func archive(t *testing.T, extra map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	add := func(name string, data []byte) {
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(data)), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"package.json", "package-lock.json", "build.mjs", "server.mjs"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "dev", "node-example", name))
		if err != nil {
			t.Fatal(err)
		}
		add(name, data)
	}
	for name, data := range extra {
		add(name, []byte(data))
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

type fakeBuilder struct {
	output []byte
	err    error
}

func (b fakeBuilder) BuildFiles(context.Context, fs.FS) ([]byte, error) { return b.output, b.err }

type recordedSteps struct {
	runtimes []string
	events   []string
	failOn   string
}

func (s *recordedSteps) Begin(stage string) error    { return s.record("begin " + stage) }
func (s *recordedSteps) Complete(stage string) error { return s.record("complete " + stage) }
func (s *recordedSteps) SetRuntime(runtime string) error {
	s.runtimes = append(s.runtimes, runtime)
	return nil
}
func (s *recordedSteps) record(event string) error {
	if event == s.failOn {
		return errors.New("progress persistence failed")
	}
	s.events = append(s.events, event)
	return nil
}

func failureCode(err error) string {
	var failure *deployment.Failure
	if errors.As(err, &failure) {
		return failure.Code
	}
	return ""
}

func newRunner(t *testing.T, builder Builder) (*Runner, *artifact.Store, *artifact.Store) {
	t.Helper()
	runner, sources, artifacts, _ := newRuntimeRunner(t, builder, &fakeDriver{}, healthy)
	return runner, sources, artifacts
}

func newRuntimeRunner(t *testing.T, builder Builder, driver *fakeDriver, check HealthCheck) (*Runner, *artifact.Store, *artifact.Store, *fakeDriver) {
	t.Helper()
	sources, artifacts := openStore(t), openStore(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRunner(sources, artifacts, builder, driver, check, deployment.Staged(deployment.Simulate), logger), sources, artifacts, driver
}

func TestBuildSuccessAndRelease(t *testing.T) {
	ctx := context.Background()
	runner, sources, artifacts := newRunner(t, fakeBuilder{output: []byte("built output")})
	stored, err := sources.Save(ctx, archive(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	work := deployment.Work{DeploymentID: testDep, ApplicationID: testApp, Source: &stored}
	steps := &recordedSteps{}
	outcome, err := runner.Run(ctx, work, steps)
	if err != nil || outcome.Artifact == nil || !outcome.Live || !outcome.Provisioned {
		t.Fatalf("build: %+v %v", outcome, err)
	}
	want := []string{"begin source", "complete source", "begin build", "complete build", "begin package", "complete package",
		"begin provision", "complete provision", "begin release", "complete release", "begin health_check", "complete health_check"}
	if !reflect.DeepEqual(steps.events, want) {
		t.Fatalf("stages: %v", steps.events)
	}
	if data, err := artifacts.Load(ctx, *outcome.Artifact); err != nil || string(data) != "built output" {
		t.Fatalf("artifact: %q %v", data, err)
	}
	// Kept: the source goes, the artifact stays.
	runner.Release(ctx, work, outcome, true)
	if _, err := sources.Load(ctx, stored); !errors.Is(err, artifact.ErrMissing) {
		t.Fatalf("source not released: %v", err)
	}
	if _, err := artifacts.Load(ctx, *outcome.Artifact); err != nil {
		t.Fatalf("kept artifact deleted: %v", err)
	}
	// Not kept (e.g. cancellation won): the artifact goes too.
	runner.Release(ctx, deployment.Work{DeploymentID: testDep, ApplicationID: testApp}, outcome, false)
	if _, err := artifacts.Load(ctx, *outcome.Artifact); !errors.Is(err, artifact.ErrMissing) {
		t.Fatalf("discarded artifact survived: %v", err)
	}
}

func TestRuntimeFailuresAreClassifiedAndCleanedUp(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		driver *fakeDriver
		check  HealthCheck
		code   string
		calls  []string
	}{
		{"prepare fails", &fakeDriver{prepareErr: rt.ErrStart}, healthy, "provision_failed", []string{"prepare"}},
		{"start fails", &fakeDriver{startErr: rt.ErrStart}, healthy, "release_failed", []string{"prepare", "start"}},
		{"docker down", &fakeDriver{prepareErr: rt.ErrUnavailable}, healthy, "runtime_unavailable", []string{"prepare"}},
		{"never healthy", &fakeDriver{}, func(context.Context, string) error { return rt.ErrUnhealthy }, "health_check_failed", []string{"prepare", "start"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner, sources, artifacts, driver := newRuntimeRunner(t, fakeBuilder{output: []byte("out")}, tc.driver, tc.check)
			stored, _ := sources.Save(ctx, archive(t, nil))
			work := deployment.Work{DeploymentID: testDep, ApplicationID: testApp, Source: &stored}
			outcome, err := runner.Run(ctx, work, &recordedSteps{})
			if failureCode(err) != tc.code || outcome.Live || !outcome.Provisioned || outcome.Artifact == nil {
				t.Fatalf("code %q outcome %+v err %v", failureCode(err), outcome, err)
			}
			if !reflect.DeepEqual(driver.calls, tc.calls) {
				t.Fatalf("calls %v", driver.calls)
			}
			// The manager reports failure, so kept is false: everything goes.
			runner.Release(ctx, work, outcome, false)
			if len(driver.destroyed) != 1 || driver.destroyed[0] != (rt.Release{ApplicationID: testApp, DeploymentID: testDep}) {
				t.Fatalf("failed release not destroyed: %v", driver.destroyed)
			}
			if _, err := artifacts.Load(ctx, *outcome.Artifact); !errors.Is(err, artifact.ErrMissing) {
				t.Fatalf("artifact survived failure: %v", err)
			}
		})
	}
}

func TestCancellationDuringHealthCheckIsNotAHealthFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner, sources, _, _ := newRuntimeRunner(t, fakeBuilder{output: []byte("out")}, &fakeDriver{}, func(ctx context.Context, _ string) error {
		cancel()
		return rt.ErrUnhealthy
	})
	stored, _ := sources.Save(context.Background(), archive(t, nil))
	_, err := runner.Run(ctx, deployment.Work{DeploymentID: testDep, ApplicationID: testApp, Source: &stored}, &recordedSteps{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want cancellation", err)
	}
}

func TestGoingLiveRetiresOnlyTheSameApplicationsOldReleases(t *testing.T) {
	ctx := context.Background()
	other := rt.Release{ApplicationID: "44444444-4444-4444-8444-444444444444", DeploymentID: "55555555-5555-4555-8555-555555555555"}
	old := rt.Release{ApplicationID: testApp, DeploymentID: oldDep}
	driver := &fakeDriver{inventory: rt.Inventory{
		Containers: []rt.Release{old, {ApplicationID: testApp, DeploymentID: testDep}, other},
		Releases:   []rt.Release{old, {ApplicationID: testApp, DeploymentID: testDep}, other},
	}}
	runner, _, artifacts, _ := newRuntimeRunner(t, fakeBuilder{}, driver, healthy)
	stored, _ := artifacts.Save(ctx, []byte("out"))
	runner.Release(ctx, deployment.Work{DeploymentID: testDep, ApplicationID: testApp}, deployment.Outcome{Artifact: &stored, Provisioned: true, Live: true}, true)
	if !reflect.DeepEqual(driver.destroyed, []rt.Release{old}) {
		t.Fatalf("destroyed %v, want only the previous release of the same app", driver.destroyed)
	}
}

func TestBuildFailureCodes(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		files   []byte
		builder fakeBuilder
		code    string
	}{
		{"neither a node app nor a static site", func() []byte {
			var buffer bytes.Buffer
			writer := tar.NewWriter(&buffer)
			_ = writer.WriteHeader(&tar.Header{Name: "readme.txt", Mode: 0600, Size: 2, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR})
			_, _ = writer.Write([]byte("hi"))
			_ = writer.Close()
			return buffer.Bytes()
		}(), fakeBuilder{}, "static_contract_invalid"},
		{"sensitive file", archive(t, map[string]string{".env": "SECRET=1"}), fakeBuilder{}, "source_sensitive_path"},
		{"build script fails", archive(t, nil), fakeBuilder{err: nodeapp.ErrBuild}, "node_build_failed"},
		{"not an archive", []byte(strings.Repeat("x", 600)), fakeBuilder{}, "source_archive_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner, sources, artifacts := newRunner(t, tc.builder)
			stored, err := sources.Save(ctx, tc.files)
			if err != nil {
				t.Fatal(err)
			}
			outcome, err := runner.Run(ctx, deployment.Work{DeploymentID: "d", Source: &stored}, &recordedSteps{})
			if code := failureCode(err); code != tc.code || outcome.Artifact != nil {
				t.Fatalf("code %q, artifact %v, err %v", code, outcome.Artifact, err)
			}
			if removed, _ := artifacts.Sweep(ctx, func(string) bool { return true }); removed != 0 {
				t.Fatal("failed build left a partial artifact")
			}
		})
	}
}

func TestMissingSourceAndProgressErrors(t *testing.T) {
	ctx := context.Background()
	runner, sources, _ := newRunner(t, fakeBuilder{output: []byte("out")})
	stored, err := sources.Save(ctx, archive(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := sources.Delete(ctx, stored); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Run(ctx, deployment.Work{Source: &stored}, &recordedSteps{}); failureCode(err) != "source_unavailable" {
		t.Fatalf("missing source: %v", err)
	}
	if _, err := runner.Run(ctx, deployment.Work{}, &recordedSteps{}); failureCode(err) != "source_missing" {
		t.Fatalf("no source: %v", err)
	}
	// A persistence failure is not a build failure; it passes through for the
	// manager to classify, and an artifact already saved is still returned.
	stored, _ = sources.Save(ctx, archive(t, nil))
	outcome, err := runner.Run(ctx, deployment.Work{Source: &stored}, &recordedSteps{failOn: "complete package"})
	if err == nil || failureCode(err) != "" || outcome.Artifact == nil {
		t.Fatalf("progress error: %+v %v", outcome, err)
	}
}

type fakeWorker struct {
	checkErr, submitErr error
	submitted           *artifact.Descriptor
}

func (w *fakeWorker) CheckBuild(context.Context, deployment.Scope) error { return w.checkErr }
func (w *fakeWorker) SubmitBuild(_ context.Context, _ deployment.Scope, source artifact.Descriptor) (deployment.Job, error) {
	w.submitted = &source
	return deployment.Job{ID: "job"}, w.submitErr
}

func TestIntake(t *testing.T) {
	ctx := context.Background()
	sources := openStore(t)
	rejecting := &fakeWorker{submitErr: deployment.ErrFull}
	if _, err := NewIntake(sources, rejecting).Submit(ctx, deployment.Scope{}, []byte("tar")); !errors.Is(err, deployment.ErrFull) {
		t.Fatalf("rejection: %v", err)
	}
	if _, err := sources.Load(ctx, *rejecting.submitted); !errors.Is(err, artifact.ErrMissing) {
		t.Fatalf("rejected upload kept on disk: %v", err)
	}
	accepting := &fakeWorker{}
	intake := NewIntake(sources, accepting)
	if _, err := intake.Submit(ctx, deployment.Scope{}, []byte("tar")); err != nil {
		t.Fatal(err)
	}
	if _, err := sources.Load(ctx, *accepting.submitted); err != nil {
		t.Fatalf("accepted upload missing: %v", err)
	}
	if _, err := intake.Submit(ctx, deployment.Scope{}, nil); !errors.Is(err, deployment.ErrInvalid) {
		t.Fatalf("empty upload: %v", err)
	}
}

func TestSweep(t *testing.T) {
	ctx := context.Background()
	sources, artifacts := openStore(t), openStore(t)
	upload, _ := sources.Save(ctx, []byte("left by a crash"))
	referenced, _ := artifacts.Save(ctx, []byte("deployed"))
	orphan, _ := artifacts.Save(ctx, []byte("never recorded"))
	removed, err := Sweep(ctx, sources, artifacts, map[string]bool{referenced.ID: true})
	if err != nil || removed != 2 {
		t.Fatalf("sweep removed %d: %v", removed, err)
	}
	if _, err := sources.Load(ctx, upload); !errors.Is(err, artifact.ErrMissing) {
		t.Fatal("upload survived sweep")
	}
	if _, err := artifacts.Load(ctx, orphan); !errors.Is(err, artifact.ErrMissing) {
		t.Fatal("orphan artifact survived sweep")
	}
	if _, err := artifacts.Load(ctx, referenced); err != nil {
		t.Fatalf("referenced artifact swept: %v", err)
	}
}

// Run with SPROUT_DOCKER_TEST=1 when the pinned Node image is available locally.
// It runs the real sealed build of the committed example application.
func TestDockerBuildJob(t *testing.T) {
	if os.Getenv("SPROUT_DOCKER_TEST") != "1" {
		t.Skip("set SPROUT_DOCKER_TEST=1 to run the Docker build job test")
	}
	builder, err := nodeapp.NewRunner(nodeapp.DefaultImage, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	docker, err := rt.NewDocker(nodeapp.DefaultImage)
	if err != nil {
		t.Fatal(err)
	}
	sources, artifacts := openStore(t), openStore(t)
	runner := NewRunner(sources, artifacts, builder, docker, DefaultHealthCheck, deployment.Staged(deployment.Simulate), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	stored, err := sources.Save(ctx, archive(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	work := deployment.Work{DeploymentID: testDep, ApplicationID: testApp, Source: &stored}
	t.Cleanup(func() {
		_ = docker.Destroy(context.Background(), rt.Release{ApplicationID: testApp, DeploymentID: testDep})
	})
	outcome, err := runner.Run(ctx, work, &recordedSteps{})
	if err != nil || outcome.Artifact == nil || !outcome.Live {
		t.Fatalf("docker build: %+v %v", outcome, err)
	}
	inventory, err := docker.List(ctx)
	if err != nil || !slices.Contains(inventory.Containers, rt.Release{ApplicationID: testApp, DeploymentID: testDep}) {
		t.Fatalf("live app is not running: %+v %v", inventory, err)
	}
	data, err := artifacts.Load(ctx, *outcome.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(bytes.NewReader(data))
	found := false
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		found = found || header.Name == "dist/server.mjs"
	}
	if !found {
		t.Fatal("built artifact has no dist/server.mjs")
	}
	runner.Release(ctx, work, outcome, false)
	if _, err := artifacts.Load(ctx, *outcome.Artifact); !errors.Is(err, artifact.ErrMissing) {
		t.Fatalf("artifact not released: %v", err)
	}
	if inventory, _ := docker.List(ctx); slices.Contains(inventory.Containers, rt.Release{ApplicationID: testApp, DeploymentID: testDep}) || slices.Contains(inventory.Releases, rt.Release{ApplicationID: testApp, DeploymentID: testDep}) {
		t.Fatalf("runtime resources survived release: %+v", inventory)
	}
}

// plainArchive packs exactly the given files, with no Node example mixed in.
func plainArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for name, body := range files {
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(body)), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}); err != nil {
			t.Fatal(err)
		}
		_, _ = writer.Write([]byte(body))
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestStaticSiteGoesLiveWithoutTouchingTheRuntime(t *testing.T) {
	ctx := context.Background()
	runner, sources, artifacts, driver := newRuntimeRunner(t, fakeBuilder{err: errors.New("a static site must not be built")}, &fakeDriver{prepareErr: errors.New("nor provisioned")}, func(context.Context, string) error {
		return errors.New("nor health-checked over HTTP")
	})
	stored, err := sources.Save(ctx, plainArchive(t, map[string]string{"index.html": "<h1>hi</h1>", "css/site.css": "h1{}"}))
	if err != nil {
		t.Fatal(err)
	}
	work := deployment.Work{DeploymentID: testDep, ApplicationID: testApp, Source: &stored}
	steps := &recordedSteps{}
	outcome, err := runner.Run(ctx, work, steps)
	if err != nil || !outcome.Live || outcome.Provisioned || outcome.Artifact == nil {
		t.Fatalf("static run: %+v %v", outcome, err)
	}
	want := []string{"begin source", "complete source", "begin build", "complete build", "begin package", "complete package",
		"begin provision", "complete provision", "begin release", "complete release", "begin health_check", "complete health_check"}
	if !reflect.DeepEqual(steps.events, want) || !reflect.DeepEqual(steps.runtimes, []string{"static"}) {
		t.Fatalf("events %v runtimes %v", steps.events, steps.runtimes)
	}
	if len(driver.calls) != 0 {
		t.Fatalf("static site used the runtime driver: %v", driver.calls)
	}
	// The stored artifact is a servable site.
	data, err := artifacts.Load(ctx, *outcome.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	site, err := staticsite.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if file, ok := site.Lookup("/"); !ok || string(file.Data) != "<h1>hi</h1>" {
		t.Fatalf("index not servable: %q %v", file.Data, ok)
	}
	// Going live retires nothing and destroys nothing, because there is no runtime state.
	runner.Release(ctx, work, outcome, true)
	if len(driver.destroyed) != 0 {
		t.Fatalf("release touched the runtime: %v", driver.destroyed)
	}
}

func TestStaticAndNodeAreToldApartByPackageJSON(t *testing.T) {
	ctx := context.Background()
	runner, sources, _ := newRunner(t, fakeBuilder{output: []byte("out")})
	stored, _ := sources.Save(ctx, archive(t, nil))
	steps := &recordedSteps{}
	if _, err := runner.Run(ctx, deployment.Work{DeploymentID: testDep, ApplicationID: testApp, Source: &stored}, steps); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(steps.runtimes, []string{"node"}) {
		t.Fatalf("node app recorded runtime %v", steps.runtimes)
	}
}

func TestStaticContractFailures(t *testing.T) {
	ctx := context.Background()
	for name, files := range map[string]map[string]string{
		"no index":          {"about.html": "x"},
		"unsafe file name":  {"index.html": "x", "my file.png": "x"},
		"index in a folder": {"site/index.html": "x"},
	} {
		t.Run(name, func(t *testing.T) {
			runner, sources, artifacts := newRunner(t, fakeBuilder{})
			stored, _ := sources.Save(ctx, plainArchive(t, files))
			outcome, err := runner.Run(ctx, deployment.Work{DeploymentID: testDep, ApplicationID: testApp, Source: &stored}, &recordedSteps{})
			if failureCode(err) != "static_contract_invalid" || outcome.Artifact != nil || outcome.Live {
				t.Fatalf("code %q outcome %+v err %v", failureCode(err), outcome, err)
			}
			if removed, _ := artifacts.Sweep(ctx, func(string) bool { return true }); removed != 0 {
				t.Fatal("rejected site left an artifact")
			}
		})
	}
}
