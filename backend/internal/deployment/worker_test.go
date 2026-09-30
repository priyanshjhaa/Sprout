package deployment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memoryRepository struct {
	mu         sync.Mutex
	jobs       map[string]Job
	order      []string
	done       chan string
	failFinish bool
	deny       bool
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{jobs: map[string]Job{}, done: make(chan string, 100)}
}
func (r *memoryRepository) Authorize(context.Context, Scope, bool) error {
	if r.deny {
		return ErrForbidden
	}
	return nil
}
func (r *memoryRepository) Create(ctx context.Context, s Scope) (Job, error) {
	if err := ctx.Err(); err != nil {
		return Job{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	job := Job{ID: fmt.Sprint(len(r.jobs) + 1), ApplicationID: s.ApplicationID, Simulated: true, Status: "queued"}
	r.jobs[job.ID] = job
	return job, nil
}
func (r *memoryRepository) Get(_ context.Context, _ Scope, id string) (Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.jobs[id], nil
}
func (r *memoryRepository) List(context.Context, Scope) ([]Job, error) { return nil, nil }
func (r *memoryRepository) Cancel(_ context.Context, _ Scope, id string) (Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job := r.jobs[id]
	job.Status = "cancelled"
	r.jobs[id] = job
	return job, nil
}
func (r *memoryRepository) Start(ctx context.Context, s Scope, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	job := r.jobs[id]
	if job.Status != "queued" {
		return ErrConflict
	}
	job.Status = "building"
	r.jobs[id] = job
	r.order = append(r.order, s.ApplicationID)
	return nil
}
func (r *memoryRepository) Advance(ctx context.Context, id, stage, status string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	job := r.jobs[id]
	if job.Status != "building" {
		return ErrConflict
	}
	job.Stages = append(job.Stages, Stage{Name: stage, Status: status})
	r.jobs[id] = job
	return nil
}
func (r *memoryRepository) Finish(_ context.Context, id, status, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failFinish {
		return errors.New("private database details")
	}
	job := r.jobs[id]
	if job.Status != "cancelled" {
		job.Status = status
		job.FailureCode = &code
		r.jobs[id] = job
	}
	r.done <- id
	return nil
}
func managerForTest(t *testing.T, r *memoryRepository, run Runner, workers, queue int, timeout time.Duration) *Manager {
	t.Helper()
	m, err := NewManager(r, run, slog.New(slog.NewTextHandler(io.Discard, nil)), workers, queue, timeout)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}
func waitSignal[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for worker")
		var zero T
		return zero
	}
}
func submit(t *testing.T, m *Manager, app string) Job {
	t.Helper()
	job, err := m.Submit(context.Background(), Scope{ApplicationID: app})
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func TestBoundedQueueAndFIFO(t *testing.T) {
	repo := newMemoryRepository()
	started := make(chan struct{}, 10)
	release := make(chan struct{})
	runner := func(ctx context.Context, stage string) error {
		if stage != "source" {
			return nil
		}
		started <- struct{}{}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m := managerForTest(t, repo, runner, 1, 1, time.Minute)
	first := submit(t, m, "first")
	waitSignal(t, started)
	second := submit(t, m, "second")
	if _, err := m.Submit(context.Background(), Scope{ApplicationID: "first"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := m.Submit(context.Background(), Scope{ApplicationID: "third"}); !errors.Is(err, ErrFull) {
		t.Fatalf("full: %v", err)
	}
	close(release)
	if waitSignal(t, repo.done) != first.ID || waitSignal(t, repo.done) != second.ID {
		t.Fatal("accepted jobs did not finish FIFO")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.jobs) != 2 {
		t.Fatal("rejected request persisted a job")
	}
	if len(repo.order) != 2 || repo.order[0] != "first" || repo.order[1] != "second" {
		t.Fatalf("order: %v", repo.order)
	}
	for _, job := range repo.jobs {
		if job.Status != "succeeded" || len(job.Stages) != 6 {
			t.Fatalf("job: %+v", job)
		}
		for i, stage := range stages {
			if job.Stages[2*i].Name != stage || job.Stages[2*i].Status != "running" || job.Stages[2*i+1].Status != "succeeded" {
				t.Fatal("unordered stages")
			}
		}
	}
}

func TestConcurrencyLimitAndRequestLifetime(t *testing.T) {
	repo := newMemoryRepository()
	entered := make(chan struct{}, 10)
	release := make(chan struct{})
	var running, maximum atomic.Int32
	run := func(ctx context.Context, _ string) error {
		n := running.Add(1)
		defer running.Add(-1)
		for old := maximum.Load(); n > old; old = maximum.Load() {
			if maximum.CompareAndSwap(old, n) {
				break
			}
		}
		entered <- struct{}{}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m := managerForTest(t, repo, run, 2, 2, time.Minute)
	request, cancel := context.WithCancel(context.Background())
	first, err := m.Submit(request, Scope{ApplicationID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	waitSignal(t, entered)
	cancel() // Accepted jobs are owned by the manager, not HTTP.
	submit(t, m, "b")
	waitSignal(t, entered)
	submit(t, m, "c")
	if maximum.Load() != 2 {
		t.Fatalf("parallel runners: %d", maximum.Load())
	}
	close(release)
	for range 3 {
		waitSignal(t, repo.done)
	}
	if err = m.Close(); err != nil {
		t.Fatal(err)
	}
	job, _ := repo.Get(context.Background(), Scope{}, first.ID)
	if job.Status != "succeeded" || maximum.Load() > 2 {
		t.Fatalf("job=%+v max=%d", job, maximum.Load())
	}
}

func TestCancellationAndShutdown(t *testing.T) {
	repo := newMemoryRepository()
	started := make(chan struct{}, 10)
	m := managerForTest(t, repo, func(ctx context.Context, _ string) error { started <- struct{}{}; <-ctx.Done(); return ctx.Err() }, 1, 2, time.Minute)
	active := submit(t, m, "active")
	waitSignal(t, started)
	queued := submit(t, m, "queued")
	if _, err := m.Cancel(context.Background(), Scope{ApplicationID: "queued"}, queued.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Cancel(context.Background(), Scope{ApplicationID: "active"}, active.ID); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, repo.done)
	waitSignal(t, repo.done)
	shutdown := submit(t, m, "shutdown")
	waitSignal(t, started)
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{active.ID, queued.ID, shutdown.ID} {
		job, _ := repo.Get(context.Background(), Scope{}, id)
		if job.Status != "cancelled" {
			t.Fatalf("job not cancelled: %+v", job)
		}
	}
	if _, err := m.Submit(context.Background(), Scope{ApplicationID: "late"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("submit after shutdown: %v", err)
	}
	if m.Ready() == nil {
		t.Fatal("drained worker still ready")
	}
}

func TestFailuresAndPanicContainment(t *testing.T) {
	for _, tc := range []struct {
		name    string
		run     Runner
		timeout time.Duration
		code    string
	}{
		{"failure", func(context.Context, string) error { return errors.New("private runner output") }, time.Second, "simulated_step_failed"},
		{"panic", func(context.Context, string) error { panic("private panic text") }, time.Second, "worker_panic"},
		{"timeout", func(ctx context.Context, _ string) error { <-ctx.Done(); return ctx.Err() }, time.Millisecond, "job_timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemoryRepository()
			m := managerForTest(t, repo, tc.run, 1, 2, tc.timeout)
			job := submit(t, m, "first")
			waitSignal(t, repo.done)
			result, _ := repo.Get(context.Background(), Scope{}, job.ID)
			if result.Status != "failed" || result.FailureCode == nil || *result.FailureCode != tc.code {
				t.Fatalf("result: %+v", result)
			}
			submit(t, m, "another")
			waitSignal(t, repo.done) // Worker remains usable after a job panic.
		})
	}
}
func TestPersistenceFailureStopsAdmission(t *testing.T) {
	repo := newMemoryRepository()
	repo.failFinish = true
	m := managerForTest(t, repo, func(context.Context, string) error { return nil }, 1, 1, time.Second)
	submit(t, m, "a")
	if err := m.Close(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("finish failure was ignored: %v", err)
	}
	if m.Ready() == nil {
		t.Fatal("failed persistence still ready")
	}
}
func TestAdmissionValidation(t *testing.T) {
	repo := newMemoryRepository()
	repo.deny = true
	m := managerForTest(t, repo, Simulate, 1, 1, time.Second)
	if _, err := m.Submit(context.Background(), Scope{}); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if _, err := NewManager(repo, Simulate, slog.Default(), 0, 1, time.Second); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
