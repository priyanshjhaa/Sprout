package application

import (
	"context"
	"regexp"
	"strings"
)

const maxDescriptionLength = 2000

var (
	slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

type Repository interface {
	List(context.Context, string, string) ([]Application, error)
	Get(context.Context, string, string, string) (Application, error)
	Create(context.Context, CreateInput) (Application, error)
	Update(context.Context, UpdateInput) (Application, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (service *Service) List(ctx context.Context, workspaceSlug, userID string) ([]Application, error) {
	if err := validateWorkspaceAndUser(workspaceSlug, userID); err != nil {
		return nil, err
	}

	return service.repository.List(ctx, workspaceSlug, userID)
}

func (service *Service) Get(
	ctx context.Context,
	workspaceSlug, applicationID, userID string,
) (Application, error) {
	if err := validateWorkspaceAndUser(workspaceSlug, userID); err != nil {
		return Application{}, err
	}
	if !uuidPattern.MatchString(applicationID) {
		return Application{}, &ValidationError{Field: "applicationId", Message: "must be a UUID"}
	}

	return service.repository.Get(ctx, workspaceSlug, applicationID, userID)
}

func (service *Service) Create(ctx context.Context, input CreateInput) (Application, error) {
	if err := validateWorkspaceAndUser(input.WorkspaceSlug, input.UserID); err != nil {
		return Application{}, err
	}

	input.Name = strings.TrimSpace(input.Name)
	input.Slug = strings.TrimSpace(input.Slug)
	input.Description = strings.TrimSpace(input.Description)

	if err := validateName(input.Name); err != nil {
		return Application{}, err
	}
	if !slugPattern.MatchString(input.Slug) || len(input.Slug) > 63 {
		return Application{}, &ValidationError{
			Field:   "slug",
			Message: "must contain lowercase letters, numbers, and single hyphens only",
		}
	}
	if len(input.Description) > maxDescriptionLength {
		return Application{}, &ValidationError{Field: "description", Message: "must be 2000 characters or fewer"}
	}

	return service.repository.Create(ctx, input)
}

func (service *Service) Update(ctx context.Context, input UpdateInput) (Application, error) {
	if err := validateWorkspaceAndUser(input.WorkspaceSlug, input.UserID); err != nil {
		return Application{}, err
	}
	if !uuidPattern.MatchString(input.ApplicationID) {
		return Application{}, &ValidationError{Field: "applicationId", Message: "must be a UUID"}
	}
	if input.Name == nil && input.Description == nil && input.Lifecycle == nil {
		return Application{}, &ValidationError{Field: "body", Message: "must include a field to update"}
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if err := validateName(name); err != nil {
			return Application{}, err
		}
		input.Name = &name
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		if len(description) > maxDescriptionLength {
			return Application{}, &ValidationError{Field: "description", Message: "must be 2000 characters or fewer"}
		}
		input.Description = &description
	}
	if input.Lifecycle != nil {
		if !validLifecycle(*input.Lifecycle) {
			return Application{}, &ValidationError{Field: "lifecycle", Message: "must be active, paused, or archived"}
		}

		current, err := service.repository.Get(ctx, input.WorkspaceSlug, input.ApplicationID, input.UserID)
		if err != nil {
			return Application{}, err
		}
		if !validTransition(current.Lifecycle, *input.Lifecycle) {
			return Application{}, &ValidationError{
				Field:   "lifecycle",
				Message: "cannot transition directly from archived to paused",
			}
		}
	}

	return service.repository.Update(ctx, input)
}

func validateWorkspaceAndUser(workspaceSlug, userID string) error {
	if !slugPattern.MatchString(workspaceSlug) || len(workspaceSlug) > 63 {
		return &ValidationError{Field: "workspaceSlug", Message: "is invalid"}
	}
	if !uuidPattern.MatchString(userID) {
		return ErrInvalidIdentity
	}
	return nil
}

func validateName(name string) error {
	if name == "" {
		return &ValidationError{Field: "name", Message: "is required"}
	}
	if len(name) > 120 {
		return &ValidationError{Field: "name", Message: "must be 120 characters or fewer"}
	}
	return nil
}

func validLifecycle(lifecycle Lifecycle) bool {
	return lifecycle == LifecycleActive || lifecycle == LifecyclePaused || lifecycle == LifecycleArchived
}

func validTransition(from, to Lifecycle) bool {
	if from == to {
		return true
	}
	return !(from == LifecycleArchived && to == LifecyclePaused)
}
