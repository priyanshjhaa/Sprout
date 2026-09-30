// Package deployment implements explicitly simulated deployment work. It never
// executes application code or reports a simulation as a live application.
package deployment

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalid     = errors.New("invalid deployment simulation request")
	ErrNotFound    = errors.New("application or simulation not found")
	ErrForbidden   = errors.New("application edit access required")
	ErrConflict    = errors.New("application inactive, already has work, or job is terminal")
	ErrFull        = errors.New("simulation queue full")
	ErrUnavailable = errors.New("simulation worker unavailable")
)

type Scope struct{ WorkspaceSlug, ApplicationID, UserID string }
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
	Finish(context.Context, string, string, string) error
}

type Runner func(context.Context, string) error

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
