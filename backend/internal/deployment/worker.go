package deployment

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

type task struct {
	job    Job
	scope  Scope
	ctx    context.Context
	cancel context.CancelFunc
}

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
}

func NewManager(repository Repository, runner Runner, logger *slog.Logger, workers, capacity int, timeout time.Duration) (*Manager, error) {
	if repository == nil || runner == nil || logger == nil || workers < 1 || workers > 8 || capacity < 1 || capacity > 100 || timeout <= 0 {
		return nil, ErrInvalid
	}
	ctx, cancel := context.WithCancel(context.Background())
	manager := &Manager{repository: repository, runner: runner, logger: logger, queue: make(chan *task, capacity), ctx: ctx, cancel: cancel, active: make(map[string]*task), timeout: timeout}
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

func (m *Manager) Submit(ctx context.Context, scope Scope) (Job, error) {
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
	item := &task{job: job, scope: scope, ctx: jobCtx, cancel: cancel}
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
		// Request/job cancellation must not prevent recording the terminal result.
		finishCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := m.repository.Finish(finishCtx, item.job.ID, status, code); err != nil {
			m.mu.Lock()
			m.unhealthy = true
			m.cancel()
			m.mu.Unlock()
			m.logger.Error("simulation persistence failed", "deployment_id", item.job.ID)
		}
	}()
	if err := m.repository.Start(ctx, item.scope, item.job.ID); err != nil {
		status, code = "failed", "start_rejected"
		return
	}
	for _, stage := range stages {
		if err := m.repository.Advance(ctx, item.job.ID, stage, "running"); err != nil {
			status, code = "failed", "progress_unavailable"
			return
		}
		if err := m.runner(ctx, stage); err != nil {
			status, code = "failed", "simulated_step_failed"
			return
		}
		if err := m.repository.Advance(ctx, item.job.ID, stage, "succeeded"); err != nil {
			status, code = "failed", "progress_unavailable"
			return
		}
	}
}
