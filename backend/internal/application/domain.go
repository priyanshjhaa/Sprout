package application

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound        = errors.New("application or workspace not found")
	ErrForbidden       = errors.New("workspace role does not allow this operation")
	ErrConflict        = errors.New("application conflicts with existing state")
	ErrInvalidIdentity = errors.New("identity is not a valid UUID")
)

type Lifecycle string

const (
	LifecycleActive   Lifecycle = "active"
	LifecyclePaused   Lifecycle = "paused"
	LifecycleArchived Lifecycle = "archived"
)

type Application struct {
	ID              string
	WorkspaceID     string
	Name            string
	Slug            string
	Description     string
	Lifecycle       Lifecycle
	DefaultHostname string
	CreatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateInput struct {
	WorkspaceSlug string
	UserID        string
	Name          string
	Slug          string
	Description   string
}

type UpdateInput struct {
	WorkspaceSlug string
	ApplicationID string
	UserID        string
	Name          *string
	Description   *string
	Lifecycle     *Lifecycle
}

type ValidationError struct {
	Field   string
	Message string
}

func (err *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", err.Field, err.Message)
}
