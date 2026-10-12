package application

import (
	"context"
	"errors"
	"testing"
)

const (
	testUserID        = "11111111-1111-4111-8111-111111111111"
	testApplicationID = "22222222-2222-4222-8222-222222222222"
)

type stubRepository struct {
	list   func(context.Context, string, string) ([]Application, error)
	get    func(context.Context, string, string, string) (Application, error)
	create func(context.Context, CreateInput) (Application, error)
	update func(context.Context, UpdateInput) (Application, error)
}

func (repository *stubRepository) List(ctx context.Context, workspaceSlug, userID string) ([]Application, error) {
	return repository.list(ctx, workspaceSlug, userID)
}

func (repository *stubRepository) Get(
	ctx context.Context,
	workspaceSlug, applicationID, userID string,
) (Application, error) {
	return repository.get(ctx, workspaceSlug, applicationID, userID)
}

func (repository *stubRepository) Create(ctx context.Context, input CreateInput) (Application, error) {
	return repository.create(ctx, input)
}

func (repository *stubRepository) Update(ctx context.Context, input UpdateInput) (Application, error) {
	return repository.update(ctx, input)
}

func TestCreateValidatesAndNormalizesInput(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{
		create: func(_ context.Context, input CreateInput) (Application, error) {
			if input.Name != "Invoice approvals" || input.Description != "Review invoices" {
				t.Fatalf("normalized input = %#v", input)
			}
			return Application{Name: input.Name, Slug: input.Slug}, nil
		},
	}
	service := NewService(repository)

	application, err := service.Create(context.Background(), CreateInput{
		WorkspaceSlug: "acme",
		UserID:        testUserID,
		Name:          "  Invoice approvals  ",
		Slug:          "invoice-approvals",
		Description:   "  Review invoices  ",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if application.Name != "Invoice approvals" {
		t.Fatalf("application name = %q", application.Name)
	}
}

func TestCreateRejectsInvalidSlugBeforeRepository(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{
		create: func(context.Context, CreateInput) (Application, error) {
			t.Fatal("repository should not be called")
			return Application{}, nil
		},
	}
	service := NewService(repository)

	_, err := service.Create(context.Background(), CreateInput{
		WorkspaceSlug: "acme",
		UserID:        testUserID,
		Name:          "Invoice approvals",
		Slug:          "Invoice Approvals",
	})

	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Field != "slug" {
		t.Fatalf("Create() error = %v, want slug validation error", err)
	}
}

func TestUpdateRejectsArchivedToPausedTransition(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{
		get: func(context.Context, string, string, string) (Application, error) {
			return Application{Lifecycle: LifecycleArchived}, nil
		},
		update: func(context.Context, UpdateInput) (Application, error) {
			t.Fatal("repository update should not be called")
			return Application{}, nil
		},
	}
	service := NewService(repository)
	target := LifecyclePaused

	_, err := service.Update(context.Background(), UpdateInput{
		WorkspaceSlug: "acme",
		ApplicationID: testApplicationID,
		UserID:        testUserID,
		Lifecycle:     &target,
	})

	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Field != "lifecycle" {
		t.Fatalf("Update() error = %v, want lifecycle validation error", err)
	}
}

func TestUpdateAllowsArchivedToActiveTransition(t *testing.T) {
	t.Parallel()

	repository := &stubRepository{
		get: func(context.Context, string, string, string) (Application, error) {
			return Application{Lifecycle: LifecycleArchived}, nil
		},
		update: func(_ context.Context, input UpdateInput) (Application, error) {
			return Application{ID: input.ApplicationID, Lifecycle: *input.Lifecycle}, nil
		},
	}
	service := NewService(repository)
	target := LifecycleActive

	application, err := service.Update(context.Background(), UpdateInput{
		WorkspaceSlug: "acme",
		ApplicationID: testApplicationID,
		UserID:        testUserID,
		Lifecycle:     &target,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if application.Lifecycle != LifecycleActive {
		t.Fatalf("lifecycle = %q, want active", application.Lifecycle)
	}
}

// A committed lifecycle change tells the runtime; a rename or a failed update does not.
func TestLifecycleChangeNotifiesRuntimeOnlyWhenCommitted(t *testing.T) {
	t.Parallel()

	fail := false
	repository := &stubRepository{
		get: func(context.Context, string, string, string) (Application, error) {
			return Application{Lifecycle: LifecycleActive}, nil
		},
		update: func(context.Context, UpdateInput) (Application, error) {
			if fail {
				return Application{}, ErrConflict
			}
			return Application{}, nil
		},
	}
	service := NewService(repository)
	notified := 0
	service.OnLifecycleChange(func() { notified++ })
	input := UpdateInput{WorkspaceSlug: "acme", ApplicationID: testApplicationID, UserID: testUserID}

	paused, name := LifecyclePaused, "New name"
	input.Lifecycle = &paused
	if _, err := service.Update(context.Background(), input); err != nil || notified != 1 {
		t.Fatalf("pause: err=%v notified=%d", err, notified)
	}
	input.Lifecycle, input.Name = nil, &name
	if _, err := service.Update(context.Background(), input); err != nil || notified != 1 {
		t.Fatalf("rename notified the runtime: err=%v notified=%d", err, notified)
	}
	fail, input.Name, input.Lifecycle = true, nil, &paused
	if _, err := service.Update(context.Background(), input); err == nil || notified != 1 {
		t.Fatalf("failed update notified the runtime: err=%v notified=%d", err, notified)
	}
}
