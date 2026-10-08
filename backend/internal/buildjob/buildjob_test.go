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
	"strings"
	"testing"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
	"github.com/priyanshjhaa/Sprout/backend/internal/deployment"
	"github.com/priyanshjhaa/Sprout/backend/internal/nodeapp"
)

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
	events []string
	failOn string
}

func (s *recordedSteps) Begin(stage string) error    { return s.record("begin " + stage) }
func (s *recordedSteps) Complete(stage string) error { return s.record("complete " + stage) }
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
	sources, artifacts := openStore(t), openStore(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRunner(sources, artifacts, builder, deployment.Staged(deployment.Simulate), logger), sources, artifacts
}

func TestBuildSuccessAndRelease(t *testing.T) {
	ctx := context.Background()
	runner, sources, artifacts := newRunner(t, fakeBuilder{output: []byte("built output")})
	stored, err := sources.Save(ctx, archive(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	work := deployment.Work{DeploymentID: "d1", Source: &stored}
	steps := &recordedSteps{}
	outcome, err := runner.Run(ctx, work, steps)
	if err != nil || outcome.Artifact == nil {
		t.Fatalf("build: %+v %v", outcome, err)
	}
	want := []string{"begin source", "complete source", "begin build", "complete build", "begin package", "complete package"}
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
	runner.Release(ctx, deployment.Work{DeploymentID: "d1"}, outcome, false)
	if _, err := artifacts.Load(ctx, *outcome.Artifact); !errors.Is(err, artifact.ErrMissing) {
		t.Fatalf("discarded artifact survived: %v", err)
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
		{"not a node app", func() []byte {
			var buffer bytes.Buffer
			writer := tar.NewWriter(&buffer)
			_ = writer.WriteHeader(&tar.Header{Name: "index.html", Mode: 0600, Size: 2, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR})
			_, _ = writer.Write([]byte("hi"))
			_ = writer.Close()
			return buffer.Bytes()
		}(), fakeBuilder{}, "node_contract_invalid"},
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
	runner, sources, artifacts := newRunner(t, builder)
	ctx := context.Background()
	stored, err := sources.Save(ctx, archive(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	work := deployment.Work{DeploymentID: "docker", Source: &stored}
	outcome, err := runner.Run(ctx, work, &recordedSteps{})
	if err != nil || outcome.Artifact == nil {
		t.Fatalf("docker build: %+v %v", outcome, err)
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
}
