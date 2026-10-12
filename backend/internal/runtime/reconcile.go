package runtime

import (
	"context"
	"errors"
	"time"
)

const (
	DefaultReconcileInterval = 15 * time.Second
	DefaultIdleTimeout       = 10 * time.Minute
	reconcileTimeout         = 60 * time.Second
)

// Desired is what should exist according to the database: every live release
// with its application's lifecycle, and the deployments still being built,
// whose runtime resources are legitimate even though nothing is live yet.
type Desired struct {
	Live       []Route
	InProgress []string // deployment IDs
}

type DesiredSource func(ctx context.Context) (Desired, error)

// StartReconciler runs a loop that makes reality match the database: apps that
// are idle, paused or archived are stopped, and resources nothing owns are
// removed. It runs on a goroutine the supervisor owns and joins on Close.
func (s *Supervisor) StartReconciler(source DesiredSource, interval, idle time.Duration) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
			case <-s.kick:
			}
			ctx, cancel := context.WithTimeout(s.ctx, reconcileTimeout)
			if err := s.reconcile(ctx, source, idle); err != nil && s.ctx.Err() == nil {
				// Retried on the next tick; one failed Docker call must not stop the loop.
				s.logger.Error("runtime reconcile failed", "error", err.Error())
			}
			cancel()
		}
	}()
}

// Kick asks the loop to reconcile now, for example after a pause or archive.
// It never blocks; a pending kick already covers the request.
func (s *Supervisor) Kick() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// reconcile compares the driver's inventory with the database's desired state.
// The inventory is read first: any resource in it was created before the
// database is read, so its deployment row already exists and is either live,
// in progress, or finished — never "unknown yet".
func (s *Supervisor) reconcile(ctx context.Context, source DesiredSource, idle time.Duration) error {
	inventory, err := s.driver.List(ctx)
	if err != nil {
		return err
	}
	desired, err := source(ctx)
	if err != nil {
		return err
	}
	live := make(map[string]Route, len(desired.Live))
	for _, route := range desired.Live {
		live[route.Release.DeploymentID] = route
	}
	protected := make(map[string]bool, len(desired.InProgress))
	for _, id := range desired.InProgress {
		protected[id] = true
	}

	var failures []error
	for _, container := range inventory.Containers {
		if protected[container.DeploymentID] || s.isStarting(container.DeploymentID) {
			continue
		}
		route, isLive := live[container.DeploymentID]
		if isLive && route.Lifecycle == "active" && !s.expire(ctx, container, idle) {
			continue
		}
		s.forget(container)
		if err := s.driver.Stop(ctx, container); err != nil {
			failures = append(failures, err)
		}
	}
	for _, release := range inventory.Releases {
		if protected[release.DeploymentID] || s.isStarting(release.DeploymentID) {
			continue
		}
		// Files stay only for live releases of apps that are not archived.
		if route, isLive := live[release.DeploymentID]; isLive && route.Lifecycle != "archived" {
			continue
		}
		s.forget(release)
		if err := s.driver.Destroy(ctx, release); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// expire reports whether an active app's container has been idle too long. A
// container the supervisor has not seen (started by a deploy) is adopted now,
// so its idle clock starts at first sight rather than at zero.
func (s *Supervisor) expire(ctx context.Context, container Release, idle time.Duration) bool {
	s.mu.Lock()
	known := s.running[container.DeploymentID]
	s.mu.Unlock()
	if known == nil {
		endpoint, err := s.driver.Endpoint(ctx, container)
		if err != nil {
			return false // not actually running; nothing to stop
		}
		s.mu.Lock()
		if s.running[container.DeploymentID] == nil {
			s.running[container.DeploymentID] = &instance{release: container, endpoint: endpoint, lastUsed: s.now()}
		}
		s.mu.Unlock()
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.now().Sub(known.lastUsed) > idle
}

func (s *Supervisor) isStarting(deploymentID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.starting[deploymentID] != nil
}

func (s *Supervisor) forget(release Release) {
	s.mu.Lock()
	delete(s.running, release.DeploymentID)
	s.mu.Unlock()
}
