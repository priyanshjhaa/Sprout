// Package deployment runs deployment jobs on a bounded, process-local worker.
// A job is either an explicit simulation or a real build; the manager owns
// scheduling, cancellation and persistence, and a Runner owns the actual work.
// Nothing here reports a job as a live application.
package deployment

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
)

var (
	ErrInvalid     = errors.New("invalid deployment simulation request")
	ErrNotFound    = errors.New("application or simulation not found")
	ErrForbidden   = errors.New("application edit access required")
	ErrConflict    = errors.New("application inactive, already has work, or job is terminal")
	ErrFull        = errors.New("simulation queue full")
	ErrUnavailable = errors.New("simulation worker unavailable")
)

// Scope names the application a caller addresses and which kind of deployments
// it may see. Simulation endpoints set Simulated; real-build endpoints do not.
type Scope struct {
	WorkspaceSlug, ApplicationID, UserID string
	Simulated                            bool
}
type Stage struct {
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	FailureCode *string `json:"failureCode"`
}
type Job struct {
	ID            string     `json:"id"`
	ApplicationID string     `json:"applicationId"`
	Simulated     bool       `json:"simulated"`
	Status        string     `json:"status"`
	FailureCode   *string    `json:"failureCode"`
	CreatedAt     time.Time  `json:"createdAt"`
	StartedAt     *time.Time `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
	Stages        []Stage    `json:"stages"`
}

var stages = [...]string{"source", "build", "package"}

// Repository owns durable state; Manager owns process-local scheduling only.
type Repository interface {
	Authorize(context.Context, Scope, bool) error
	Create(context.Context, Scope) (Job, error)
	Get(context.Context, Scope, string) (Job, error)
	List(context.Context, Scope) ([]Job, error)
	Cancel(context.Context, Scope, string) (Job, error)
	Start(context.Context, Scope, string) error
	Advance(context.Context, string, string, string) error
	// Finish records a terminal state. It reports false when the job was already
	// terminal (cancellation won), in which case nothing, including an artifact,
	// was recorded.
	Finish(ctx context.Context, id, status, code string, output *artifact.Descriptor) (bool, error)
}

// Work describes one accepted job to its Runner.
type Work struct {
	DeploymentID string
	Simulated    bool
	// Source is the uploaded archive for a real build; nil for simulations.
	Source *artifact.Descriptor
}

// Steps lets a Runner report stage boundaries. The manager persists each one
// and notifies progress listeners; an error means persistence failed.
type Steps interface {
	Begin(stage string) error
	Complete(stage string) error
}

// Outcome is what a successful run produced. Simulations produce nothing.
type Outcome struct{ Artifact *artifact.Descriptor }

// Runner performs a job's work. Run must honour ctx: Go cannot stop a function
// that ignores cancellation. Release is called exactly once after the job's
// terminal state is persisted (or that failed), and must free everything the
// job owned. kept reports whether the outcome's artifact is now referenced by
// the deployment and must be preserved.
type Runner interface {
	Run(ctx context.Context, work Work, steps Steps) (Outcome, error)
	Release(ctx context.Context, work Work, outcome Outcome, kept bool)
}

// Failure is a runner error carrying a stable, user-safe failure code. Any
// other runner error is recorded under a generic code so raw details never
// reach the database or API.
type Failure struct {
	Code string
	Err  error
}

func (f *Failure) Error() string { return f.Code }
func (f *Failure) Unwrap() error { return f.Err }

var failureCode = regexp.MustCompile(`^[a-z][a-z0-9_]{2,79}$`)

// StageRunner performs one named stage. Staged adapts it into a Runner that
// walks the standard stages in order; it is how simulations run.
type StageRunner func(context.Context, string) error

func Staged(run StageRunner) Runner { return staged{run} }

type staged struct{ run StageRunner }

func (s staged) Run(ctx context.Context, _ Work, steps Steps) (Outcome, error) {
	for _, stage := range stages {
		if err := steps.Begin(stage); err != nil {
			return Outcome{}, err
		}
		if err := s.run(ctx, stage); err != nil {
			return Outcome{}, err
		}
		if err := steps.Complete(stage); err != nil {
			return Outcome{}, err
		}
	}
	return Outcome{}, nil
}
func (staged) Release(context.Context, Work, Outcome, bool) {}

// Simulate waits rather than invoking a repository, subprocess or container.
func Simulate(ctx context.Context, _ string) error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
