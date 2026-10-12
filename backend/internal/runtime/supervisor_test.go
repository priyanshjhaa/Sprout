package runtime

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
)

// scriptedDriver is an in-memory Driver. Start returns the endpoint of a real
// test server, so health checks and proxying exercise real HTTP.
type scriptedDriver struct {
	mu        sync.Mutex
	endpoint  string
	adoptable bool // Endpoint reports a running instance
	prepared  map[Release]bool
	starts    atomic.Int32
	prepares  atomic.Int32
	startErr  error
	startGate chan struct{} // when set, Start blocks until closed
}

func newScripted(endpoint string) *scriptedDriver {
	return &scriptedDriver{endpoint: endpoint, prepared: map[Release]bool{}}
}
func (d *scriptedDriver) Prepare(_ context.Context, r Release, _ []byte) error {
	d.prepares.Add(1)
	d.mu.Lock()
	d.prepared[r] = true
	d.mu.Unlock()
	return nil
}
func (d *scriptedDriver) Start(ctx context.Context, r Release) (Instance, error) {
	d.starts.Add(1)
	if d.startGate != nil {
		select {
		case <-d.startGate:
		case <-ctx.Done():
			return Instance{}, ctx.Err()
		}
	}
	return Instance{Release: r, Endpoint: d.endpoint}, d.startErr
}
func (d *scriptedDriver) Endpoint(context.Context, Release) (string, error) {
	if d.adoptable {
		return d.endpoint, nil
	}
	return "", ErrNotRunning
}
func (d *scriptedDriver) Stop(context.Context, Release) error    { return nil }
func (d *scriptedDriver) Destroy(context.Context, Release) error { return nil }
func (d *scriptedDriver) List(context.Context) (Inventory, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var inventory Inventory
	for r := range d.prepared {
		inventory.Releases = append(inventory.Releases, r)
	}
	return inventory, nil
}

type staticArtifacts struct{ err error }

func (a staticArtifacts) Load(context.Context, artifact.Descriptor) ([]byte, error) {
	return []byte("tar"), a.err
}

func healthyServer(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			return
		}
		_, _ = io.WriteString(w, "host="+r.Host+" fwd="+r.Header.Get("X-Forwarded-Host")+" sprout="+r.Header.Get("X-Sprout-Admin"))
	}))
	t.Cleanup(server.Close)
	return endpointOf(server)
}

func newSupervisor(t *testing.T, driver Driver, artifacts ArtifactLoader, max int) *Supervisor {
	t.Helper()
	s, err := NewSupervisor(driver, artifacts, slog.New(slog.NewTextHandler(io.Discard, nil)), max)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

var testRoute = Route{Release: testRelease, Lifecycle: "active", Live: true}

func TestEnsureSharesOneStartAndPreparesMissingFiles(t *testing.T) {
	driver := newScripted(healthyServer(t))
	driver.startGate = make(chan struct{})
	s := newSupervisor(t, driver, staticArtifacts{}, 3)
	var wg sync.WaitGroup
	results := make(chan string, 5)
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			endpoint, err := s.Ensure(context.Background(), testRoute)
			if err != nil {
				t.Error(err)
			}
			results <- endpoint
		}()
	}
	time.Sleep(100 * time.Millisecond) // let every caller reach the shared start
	close(driver.startGate)
	wg.Wait()
	if driver.starts.Load() != 1 || driver.prepares.Load() != 1 {
		t.Fatalf("starts=%d prepares=%d, want one shared start that re-prepared the files", driver.starts.Load(), driver.prepares.Load())
	}
	// Now running: no further work.
	if _, err := s.Ensure(context.Background(), testRoute); err != nil || driver.starts.Load() != 1 {
		t.Fatalf("second Ensure restarted: %v starts=%d", err, driver.starts.Load())
	}
}

func TestEnsureAdoptsARunningContainerAndForgetsFailedOnes(t *testing.T) {
	driver := newScripted(healthyServer(t))
	driver.adoptable = true
	s := newSupervisor(t, driver, staticArtifacts{}, 3)
	if _, err := s.Ensure(context.Background(), testRoute); err != nil || driver.starts.Load() != 0 || driver.prepares.Load() != 0 {
		t.Fatalf("adoption should not start or prepare: %v", err)
	}
	driver.adoptable = false
	s.Failed(testRelease)
	if _, err := s.Ensure(context.Background(), testRoute); err != nil || driver.starts.Load() != 1 {
		t.Fatalf("after Failed the next Ensure must start again: %v starts=%d", err, driver.starts.Load())
	}
}

func TestEnsureEnforcesCapacityAndReportsFailures(t *testing.T) {
	driver := newScripted(healthyServer(t))
	s := newSupervisor(t, driver, staticArtifacts{}, 1)
	if _, err := s.Ensure(context.Background(), testRoute); err != nil {
		t.Fatal(err)
	}
	other := testRoute
	other.Release.DeploymentID = "33333333-3333-4333-8333-333333333333"
	if _, err := s.Ensure(context.Background(), other); !errors.Is(err, ErrCapacity) {
		t.Fatalf("err = %v, want capacity", err)
	}

	failing := newScripted("127.0.0.1:1")
	failing.startErr = ErrStart
	s2 := newSupervisor(t, failing, staticArtifacts{}, 2)
	if _, err := s2.Ensure(context.Background(), testRoute); !errors.Is(err, ErrStart) {
		t.Fatalf("err = %v", err)
	}
	// A failed start is not remembered as running.
	if _, err := s2.Ensure(context.Background(), testRoute); !errors.Is(err, ErrStart) || failing.starts.Load() != 2 {
		t.Fatalf("failed start was cached: %v starts=%d", err, failing.starts.Load())
	}

	missing := newSupervisor(t, newScripted("127.0.0.1:1"), staticArtifacts{err: artifact.ErrMissing}, 2)
	if _, err := missing.Ensure(context.Background(), testRoute); !errors.Is(err, ErrStart) {
		t.Fatalf("missing artifact err = %v", err)
	}
}

func TestVisitorLeavingDoesNotCancelTheStart(t *testing.T) {
	driver := newScripted(healthyServer(t))
	driver.startGate = make(chan struct{})
	s := newSupervisor(t, driver, staticArtifacts{}, 2)
	ctx, leave := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := s.Ensure(ctx, testRoute); done <- err }()
	time.Sleep(50 * time.Millisecond)
	leave()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	close(driver.startGate)
	// The start finishes on its own; the next visitor gets the running app.
	endpoint, err := s.Ensure(context.Background(), testRoute)
	if err != nil || endpoint == "" || driver.starts.Load() != 1 {
		t.Fatalf("endpoint=%q err=%v starts=%d", endpoint, err, driver.starts.Load())
	}
}
