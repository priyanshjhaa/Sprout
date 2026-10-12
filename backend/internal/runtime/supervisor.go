package runtime

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
)

const (
	// MaxRunning bounds how many applications run at once on this machine.
	MaxRunning   = 10
	startTimeout = 45 * time.Second
	adoptTimeout = 5 * time.Second
	healthBudget = 30 * time.Second
)

var ErrCapacity = errors.New("runtime_capacity_reached")

// ArtifactLoader returns a stored release archive, verified against its digest.
type ArtifactLoader interface {
	Load(ctx context.Context, descriptor artifact.Descriptor) ([]byte, error)
}

// Supervisor owns which applications are running right now. Ensure starts a
// release on demand (waking a sleeping app), at most once at a time per
// release, with a global cap. Starts run on supervisor-owned goroutines that
// are cancelled and joined by Close, so a disconnecting visitor neither aborts
// nor leaks a start.
type Supervisor struct {
	driver    Driver
	artifacts ArtifactLoader
	logger    *slog.Logger
	max       int

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu       sync.Mutex // guards the three fields below
	closed   bool
	running  map[string]*instance // by deployment ID
	starting map[string]*flight
}

type instance struct {
	release  Release
	endpoint string
	lastUsed time.Time
}

type flight struct {
	done     chan struct{}
	endpoint string
	err      error
}

func NewSupervisor(driver Driver, artifacts ArtifactLoader, logger *slog.Logger, max int) (*Supervisor, error) {
	if driver == nil || artifacts == nil || logger == nil || max < 1 {
		return nil, ErrInvalid
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Supervisor{driver: driver, artifacts: artifacts, logger: logger, max: max, ctx: ctx, cancel: cancel,
		running: map[string]*instance{}, starting: map[string]*flight{}}, nil
}

// Ensure returns the endpoint of the route's live release, starting it first if
// needed. Concurrent callers for the same release share one start.
func (s *Supervisor) Ensure(ctx context.Context, route Route) (string, error) {
	key := route.Release.DeploymentID
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", ErrUnavailable
	}
	if running := s.running[key]; running != nil {
		running.lastUsed = time.Now()
		endpoint := running.endpoint
		s.mu.Unlock()
		return endpoint, nil
	}
	pending := s.starting[key]
	if pending == nil {
		if len(s.running)+len(s.starting) >= s.max {
			s.mu.Unlock()
			return "", ErrCapacity
		}
		pending = &flight{done: make(chan struct{})}
		s.starting[key] = pending
		s.wg.Add(1)
		go s.start(route, pending)
	}
	s.mu.Unlock()
	select {
	case <-pending.done:
		return pending.endpoint, pending.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (s *Supervisor) start(route Route, pending *flight) {
	defer s.wg.Done()
	ctx, cancel := context.WithTimeout(s.ctx, startTimeout)
	defer cancel()
	endpoint, err := s.bringUp(ctx, route)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.starting, route.Release.DeploymentID)
	if err == nil && s.closed {
		err = ErrUnavailable
	}
	if err == nil {
		s.running[route.Release.DeploymentID] = &instance{release: route.Release, endpoint: endpoint, lastUsed: time.Now()}
	}
	pending.endpoint, pending.err = endpoint, err
	close(pending.done)
}

// bringUp adopts a container the deploy already started, or prepares (if its
// files are gone) and starts one, and only returns once /health answers.
func (s *Supervisor) bringUp(ctx context.Context, route Route) (string, error) {
	release := route.Release
	if endpoint, err := s.driver.Endpoint(ctx, release); err == nil {
		if WaitHealthy(ctx, endpoint, adoptTimeout) == nil {
			return endpoint, nil
		}
	}
	inventory, err := s.driver.List(ctx)
	if err != nil {
		return "", err
	}
	if !contains(inventory.Releases, release) {
		archive, err := s.artifacts.Load(ctx, route.Artifact)
		if err != nil {
			return "", ErrStart
		}
		if err := s.driver.Prepare(ctx, release, archive); err != nil {
			return "", err
		}
	}
	// A previous container of this release may still exist, stopped or unhealthy.
	if err := s.driver.Stop(ctx, release); err != nil {
		return "", err
	}
	instance, err := s.driver.Start(ctx, release)
	if err != nil {
		return "", err
	}
	if err := WaitHealthy(ctx, instance.Endpoint, healthBudget); err != nil {
		_ = s.driver.Stop(context.WithoutCancel(ctx), release)
		return "", err
	}
	return instance.Endpoint, nil
}

// Failed forgets a release whose instance stopped answering, so the next
// request starts it again.
func (s *Supervisor) Failed(release Release) {
	s.mu.Lock()
	delete(s.running, release.DeploymentID)
	s.mu.Unlock()
}

// Close stops accepting work, cancels in-flight starts and joins them. Running
// containers are stopped by the caller (StopAll) afterwards.
func (s *Supervisor) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.cancel()
	s.wg.Wait()
}

func contains(releases []Release, want Release) bool {
	for _, release := range releases {
		if release == want {
			return true
		}
	}
	return false
}
