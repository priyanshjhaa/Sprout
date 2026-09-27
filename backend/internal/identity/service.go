package identity

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrNotFound          = errors.New("identity not found")
	ErrNoWorkspace       = errors.New("identity has no workspace")
	ErrIncompleteProfile = errors.New("identity has no verified primary email")
)

type Profile struct {
	Email       string
	DisplayName string
}

type Workspace struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type Principal struct {
	ID          string
	Email       string
	DisplayName string
	Workspace   Workspace
}

type ProfileProvider interface {
	GetProfile(context.Context, string) (Profile, error)
}

type Repository interface {
	Find(context.Context, string) (Principal, error)
	Create(context.Context, string, Profile) (Principal, error)
	GetWorkspace(context.Context, string, string) (Workspace, error)
}

type Service struct {
	provider   ProfileProvider
	repository Repository
}

func NewService(provider ProfileProvider, repository Repository) *Service {
	return &Service{provider: provider, repository: repository}
}

func (service *Service) Resolve(ctx context.Context, subject string) (Principal, error) {
	if strings.TrimSpace(subject) == "" {
		return Principal{}, ErrNotFound
	}

	principal, err := service.repository.Find(ctx, subject)
	if err == nil {
		return principal, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Principal{}, err
	}

	profile, err := service.provider.GetProfile(ctx, subject)
	if err != nil {
		return Principal{}, err
	}
	if strings.TrimSpace(profile.Email) == "" {
		return Principal{}, ErrIncompleteProfile
	}
	return service.repository.Create(ctx, subject, profile)
}

func (service *Service) GetWorkspace(ctx context.Context, slug, userID string) (Workspace, error) {
	return service.repository.GetWorkspace(ctx, slug, userID)
}
