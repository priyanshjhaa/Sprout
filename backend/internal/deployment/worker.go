package deployment

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
)

type task struct {
	job    Job
	scope  Scope
	work   Work
	ctx    context.Context
	cancel context.CancelFunc
}

// errProgress marks a failure to persist a stage boundary, as opposed to a
// failure in the runner's own work.
var errProgress = errors.New("progress_unavailable")

type Manager struct {
	repository Repository
	runner     Runner
	logger     *slog.Logger
	queue      chan *task
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex // admission, active-app ownership, and draining state
	active     map[string]*task
	closing    bool
	unhealthy  bool
	workers    sync.WaitGroup
	timeout    time.Duration
	progress   *progressBroker
}

func NewManager(repository Repository, runner Runner, logger *slog.Logger, workers, capacity int, timeout time.Duration) (*Manager, error) {
	if repository == nil || runner == nil || logger == nil || workers < 1 || workers > 8 || capacity < 1 || capacity > 100 || timeout <= 0 {
		return nil, ErrInvalid
	}
	ctx, cancel := context.WithCancel(context.Background())
	manager := &Manager{repository: repository, runner: runner, logger: logger, queue: make(chan *task, capacity), ctx: ctx, cancel: cancel, active: make(map[string]*task), timeout: timeout, progress: newProgressBroker()}
	for range workers {
		manager.workers.Add(1)
		go manager.work()
	}
	return manager, nil
}

func (m *Manager) Ready() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing || m.unhealthy {
		return ErrUnavailable
	}
	return nil
}

// Submit queues a simulation.
func (m *Manager) Submit(ctx context.Context, scope Scope) (Job, error) {
	scope.Simulated = true
	return m.admit(ctx, scope, nil)
}

// SubmitBuild queues a real build of an already-stored source archive. If it
// returns an error the job was not accepted and the caller still owns source;
// on success the job owns it and the runner releases it.
func (m *Manager) SubmitBuild(ctx context.Context, scope Scope, source artifact.Descriptor) (Job, error) {
	scope.Simulated = false
	return m.admit(ctx, scope, &source)
}

// CheckBuild reports whether a build for scope would currently be admitted,
// without creating anything. Admission is checked again on submission; this
// only lets intake refuse early, before receiving an upload.
func (m *Manager) CheckBuild(ctx context.Context, scope Scope) error {
	ctx, stop := context.WithTimeout(ctx, 5*time.Second)
	defer stop()
	scope.Simulated = false
	if err := m.repository.Authorize(ctx, scope, true); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case m.closing || m.unhealthy:
		return ErrUnavailable
	case m.active[scope.ApplicationID] != nil:
		return ErrConflict
	case len(m.queue) == cap(m.queue):
		return ErrFull
	}
	return nil
}

func (m *Manager) admit(ctx context.Context, scope Scope, source *artifact.Descriptor) (Job, error) {
	ctx, stop := context.WithTimeout(ctx, 5*time.Second)
	defer stop()
	if err := m.repository.Authorize(ctx, scope, true); err != nil {
		return Job{}, err
	}
	// Serialize admission with shutdown and other producers. Persist before sending
	// to the channel; workers never observe a job that has not been committed.
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing || m.unhealthy {
		return Job{}, ErrUnavailable
	}
	if _, exists := m.active[scope.ApplicationID]; exists {
		return Job{}, ErrConflict
	}
	if len(m.queue) == cap(m.queue) {
		return Job{}, ErrFull
	}
	job, err := m.repository.Create(ctx, scope)
	if err != nil {
		// A lost commit acknowledgement can leave a persisted job without an
		// in-memory task. Fail closed so restart recovery cannot be overlooked.
		if !errors.Is(err, ErrInvalid) && !errors.Is(err, ErrForbidden) && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrConflict) {
			m.unhealthy = true
			m.cancel()
		}
		return Job{}, err
	}
	jobCtx, cancel := context.WithCancel(m.ctx)
	item := &task{job: job, scope: scope, work: Work{DeploymentID: job.ID, Simulated: job.Simulated, Source: source}, ctx: jobCtx, cancel: cancel}
	m.active[scope.ApplicationID] = item
	m.queue <- item // only this locked producer can add; capacity was reserved above
	return job, nil
}
func (m *Manager) List(ctx context.Context, scope Scope) ([]Job, error) {
	return m.repository.List(ctx, scope)
}
func (m *Manager) Get(ctx context.Context, scope Scope, id string) (Job, error) {
	return m.repository.Get(ctx, scope, id)
}
func (m *Manager) Cancel(ctx context.Context, scope Scope, id string) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, err := m.repository.Cancel(ctx, scope, id)
	if err != nil {
		return Job{}, err
	}
	if item := m.active[scope.ApplicationID]; item != nil && item.job.ID == id {
		item.cancel()
	}
	m.progress.notify(id)
	return job, nil
}

// Close stops admission, cancels active work and drains queued jobs into terminal
// cancelled states. It joins all workers before the database pool can close.
func (m *Manager) Close() error {
	m.mu.Lock()
	if !m.closing {
		m.closing = true
		m.cancel()
		close(m.queue)
	}
	m.mu.Unlock()
	m.workers.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.unhealthy {
		return ErrUnavailable
	}
	return nil
}
func (m *Manager) work() {
	defer m.workers.Done()
	for item := range m.queue {
		m.execute(item)
		item.cancel()
		m.mu.Lock()
		delete(m.active, item.scope.ApplicationID)
		m.mu.Unlock()
	}
}
func (m *Manager) execute(item *task) {
	ctx, cancel := context.WithTimeout(item.ctx, m.timeout)
	defer cancel()
	status, code := "succeeded", ""
	var outcome Outcome
	defer func() {
		if recover() != nil {
			status, code = "failed", "worker_panic"
		}
		if ctx.Err() != nil {
			status, code = "cancelled", "job_cancelled"
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				status, code = "failed", "job_timeout"
			} else if m.ctx.Err() != nil {
				code = "server_shutdown"
			}
		}
		// Only a successful run may leave an artifact behind.
		output := outcome.Artifact
		if status != "succeeded" {
			output = nil
		}
		// Request/job cancellation must not prevent recording the terminal result.
		finishCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		recorded, err := m.repository.Finish(finishCtx, item.job.ID, status, code, output)
		if err != nil {
			m.mu.Lock()
			m.unhealthy = true
			m.cancel()
			m.mu.Unlock()
			m.logger.Error("deployment persistence failed", "deployment_id", item.job.ID)
		}
		m.progress.notify(item.job.ID)
		m.release(finishCtx, item, outcome, err == nil && recorded && output != nil)
	}()
	if err := m.repository.Start(ctx, item.scope, item.job.ID); err != nil {
		status, code = "failed", "start_rejected"
		return
	}
	m.progress.notify(item.job.ID)
	result, err := m.runner.Run(ctx, item.work, steps{manager: m, ctx: ctx, id: item.job.ID})
	outcome = result
	if err != nil {
		status, code = "failed", failureCodeFor(err, item.work.Simulated)
	}
}

// release frees what the job owned. A panicking Release must not take down the
// worker goroutine, which other jobs depend on.
func (m *Manager) release(ctx context.Context, item *task, outcome Outcome, kept bool) {
	defer func() {
		if recover() != nil {
			m.logger.Error("deployment release panicked", "deployment_id", item.job.ID)
		}
	}()
	m.runner.Release(ctx, item.work, outcome, kept)
}

func failureCodeFor(err error, simulated bool) string {
	var failure *Failure
	switch {
	case errors.Is(err, errProgress):
		return "progress_unavailable"
	case errors.As(err, &failure) && failureCode.MatchString(failure.Code):
		return failure.Code
	case simulated:
		return "simulated_step_failed"
	default:
		return "build_failed"
	}
}

// steps persists stage boundaries in order and notifies progress listeners.
type steps struct {
	manager *Manager
	ctx     context.Context
	id      string
}

func (s steps) Begin(stage string) error    { return s.advance(stage, "running") }
func (s steps) Complete(stage string) error { return s.advance(stage, "succeeded") }
func (s steps) advance(stage, status string) error {
	if err := s.manager.repository.Advance(s.ctx, s.id, stage, status); err != nil {
		return errProgress
	}
	s.manager.progress.notify(s.id)
	return nil
}
