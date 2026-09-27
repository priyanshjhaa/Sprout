package identity

import (
	"context"
	"errors"
	"testing"
)

type stubProvider struct {
	profile Profile
	called  bool
}

func (provider *stubProvider) GetProfile(context.Context, string) (Profile, error) {
	provider.called = true
	return provider.profile, nil
}

type stubRepository struct {
	findResult Principal
	findError  error
	createFunc func(context.Context, string, Profile) (Principal, error)
}

func (repository *stubRepository) Find(context.Context, string) (Principal, error) {
	return repository.findResult, repository.findError
}

func (repository *stubRepository) Create(ctx context.Context, subject string, profile Profile) (Principal, error) {
	return repository.createFunc(ctx, subject, profile)
}

func (repository *stubRepository) GetWorkspace(context.Context, string, string) (Workspace, error) {
	return Workspace{}, nil
}

func TestExistingIdentityDoesNotFetchClerkProfile(t *testing.T) {
	t.Parallel()
	provider := &stubProvider{}
	want := Principal{ID: "known-user"}
	service := NewService(provider, &stubRepository{findResult: want})

	got, err := service.Resolve(context.Background(), "user_clerk")
	if err != nil || got != want {
		t.Fatalf("Resolve() = %#v, %v; want %#v", got, err, want)
	}
	if provider.called {
		t.Fatal("Clerk profile was fetched for an existing identity")
	}
}

func TestNewIdentityRequiresVerifiedEmailBeforeCreation(t *testing.T) {
	t.Parallel()
	provider := &stubProvider{profile: Profile{}}
	service := NewService(provider, &stubRepository{
		findError: ErrNotFound,
		createFunc: func(context.Context, string, Profile) (Principal, error) {
			t.Fatal("identity must not be created without email")
			return Principal{}, nil
		},
	})

	_, err := service.Resolve(context.Background(), "user_clerk")
	if !errors.Is(err, ErrIncompleteProfile) {
		t.Fatalf("Resolve() error = %v, want ErrIncompleteProfile", err)
	}
}

func TestRemovedWorkspaceAccessDoesNotRecreateMembership(t *testing.T) {
	t.Parallel()
	provider := &stubProvider{}
	service := NewService(provider, &stubRepository{findError: ErrNoWorkspace})

	_, err := service.Resolve(context.Background(), "user_clerk")
	if !errors.Is(err, ErrNoWorkspace) {
		t.Fatalf("Resolve() error = %v, want ErrNoWorkspace", err)
	}
	if provider.called {
		t.Fatal("Clerk was contacted after workspace access was removed")
	}
}
