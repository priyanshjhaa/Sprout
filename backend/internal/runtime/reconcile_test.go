package runtime

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"
)

// worldDriver is a Driver over an in-memory inventory, recording what the
// reconciler stops and destroys.
type worldDriver struct {
	scriptedDriver
	mu2        sync.Mutex
	containers []Release
	releases   []Release
	stopped    []Release
	destroyed  []Release
}

func (d *worldDriver) List(context.Context) (Inventory, error) {
	d.mu2.Lock()
	defer d.mu2.Unlock()
	return Inventory{Containers: slices.Clone(d.containers), Releases: slices.Clone(d.releases)}, nil
}
func (d *worldDriver) Endpoint(_ context.Context, r Release) (string, error) {
	d.mu2.Lock()
	defer d.mu2.Unlock()
	if slices.Contains(d.containers, r) {
		return "127.0.0.1:1", nil
	}
	return "", ErrNotRunning
}
func (d *worldDriver) Stop(_ context.Context, r Release) error {
	d.mu2.Lock()
	defer d.mu2.Unlock()
	d.stopped = append(d.stopped, r)
	d.containers = slices.DeleteFunc(d.containers, func(c Release) bool { return c == r })
	return nil
}
func (d *worldDriver) Destroy(_ context.Context, r Release) error {
	d.mu2.Lock()
	defer d.mu2.Unlock()
	d.destroyed = append(d.destroyed, r)
	d.containers = slices.DeleteFunc(d.containers, func(c Release) bool { return c == r })
	d.releases = slices.DeleteFunc(d.releases, func(c Release) bool { return c == r })
	return nil
}

func rel(n string) Release {
	return Release{ApplicationID: "11111111-1111-4111-8111-11111111111" + n, DeploymentID: "22222222-2222-4222-8222-22222222222" + n}
}

func routeOf(r Release, lifecycle string) Route {
	return Route{Release: r, Lifecycle: lifecycle, Live: true}
}

func reconcileWith(t *testing.T, world *worldDriver, desired Desired, clock *time.Time) *Supervisor {
	t.Helper()
	s := newSupervisor(t, world, staticArtifacts{}, 5)
	s.now = func() time.Time { return *clock }
	return s
}

func source(d Desired) DesiredSource { return func(context.Context) (Desired, error) { return d, nil } }

func TestReconcileStopsIdleAppsOnlyAfterTheyHaveBeenSeen(t *testing.T) {
	a := rel("1")
	world := &worldDriver{containers: []Release{a}, releases: []Release{a}}
	clock := time.Unix(1_000_000, 0)
	desired := Desired{Live: []Route{routeOf(a, "active")}}
	s := reconcileWith(t, world, desired, &clock)
	ctx := context.Background()

	// First sight (started by a deploy): adopted, never stopped, however old it is.
	if err := s.reconcile(ctx, source(desired), time.Minute); err != nil || len(world.stopped) != 0 {
		t.Fatalf("first sight: %v stopped=%v", err, world.stopped)
	}
	clock = clock.Add(30 * time.Second)
	_, _ = s.Ensure(ctx, routeOf(a, "active")) // traffic resets the idle clock
	clock = clock.Add(45 * time.Second)
	_ = s.reconcile(ctx, source(desired), time.Minute)
	if len(world.stopped) != 0 {
		t.Fatalf("stopped an app that was used 45s ago: %v", world.stopped)
	}
	clock = clock.Add(30 * time.Second) // now 75s since last use
	_ = s.reconcile(ctx, source(desired), time.Minute)
	if !slices.Equal(world.stopped, []Release{a}) {
		t.Fatalf("idle app not stopped: %v", world.stopped)
	}
	if len(world.destroyed) != 0 {
		t.Fatalf("sleeping must keep the release files: %v", world.destroyed)
	}
}

func TestReconcilePauseArchiveAndOrphans(t *testing.T) {
	paused, archived, orphan, building, healthy := rel("1"), rel("2"), rel("3"), rel("4"), rel("5")
	all := []Release{paused, archived, orphan, building, healthy}
	world := &worldDriver{containers: slices.Clone(all), releases: slices.Clone(all)}
	clock := time.Unix(1_000_000, 0)
	desired := Desired{
		Live:       []Route{routeOf(paused, "paused"), routeOf(archived, "archived"), routeOf(healthy, "active")},
		InProgress: []string{building.DeploymentID},
	}
	s := reconcileWith(t, world, desired, &clock)
	if err := s.reconcile(context.Background(), source(desired), time.Hour); err != nil {
		t.Fatal(err)
	}
	for _, want := range []Release{paused, archived, orphan} {
		if !slices.Contains(world.stopped, want) {
			t.Errorf("container %v not stopped", want.DeploymentID)
		}
	}
	// Files: paused keeps them (resume is instant), archived and orphans lose them.
	if !slices.Contains(world.destroyed, archived) || !slices.Contains(world.destroyed, orphan) || slices.Contains(world.destroyed, paused) {
		t.Errorf("destroyed = %v", world.destroyed)
	}
	// An in-progress deployment's resources and a healthy active app are untouched.
	for _, safe := range []Release{building, healthy} {
		if slices.Contains(world.stopped, safe) || slices.Contains(world.destroyed, safe) {
			t.Errorf("%v was reclaimed", safe.DeploymentID)
		}
	}
}

func TestReconcileLeavesStartsInFlightAlone(t *testing.T) {
	a := rel("1")
	world := &worldDriver{containers: []Release{a}, releases: []Release{a}}
	clock := time.Unix(1_000_000, 0)
	s := reconcileWith(t, world, Desired{}, &clock)
	s.mu.Lock()
	s.starting[a.DeploymentID] = &flight{done: make(chan struct{})}
	s.mu.Unlock()
	if err := s.reconcile(context.Background(), source(Desired{}), time.Minute); err != nil {
		t.Fatal(err)
	}
	if len(world.stopped)+len(world.destroyed) != 0 {
		t.Fatalf("reclaimed a release that is starting: %v %v", world.stopped, world.destroyed)
	}
}

func TestReconcilerLoopRunsOnKickAndStopsOnClose(t *testing.T) {
	a := rel("1")
	world := &worldDriver{containers: []Release{a}, releases: []Release{a}}
	s, err := NewSupervisor(world, staticArtifacts{}, testLogger(), 5)
	if err != nil {
		t.Fatal(err)
	}
	desired := Desired{Live: []Route{routeOf(a, "paused")}}
	s.StartReconciler(source(desired), time.Hour, time.Hour) // only a kick can trigger it
	s.Kick()
	s.Kick() // a second kick must not block
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		world.mu2.Lock()
		done := len(world.stopped) > 0
		world.mu2.Unlock()
		if done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.Close() // joins the loop; a leaked goroutine would hang this test
	if !slices.Contains(world.stopped, a) {
		t.Fatal("kick did not stop the paused app")
	}
}
