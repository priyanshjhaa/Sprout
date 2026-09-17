package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/application"
)

const handlerTestUserID = "11111111-1111-4111-8111-111111111111"

type stubApplicationService struct {
	list   func(context.Context, string, string) ([]application.Application, error)
	get    func(context.Context, string, string, string) (application.Application, error)
	create func(context.Context, application.CreateInput) (application.Application, error)
	update func(context.Context, application.UpdateInput) (application.Application, error)
}

func (service *stubApplicationService) List(
	ctx context.Context,
	workspaceSlug, userID string,
) ([]application.Application, error) {
	return service.list(ctx, workspaceSlug, userID)
}

func (service *stubApplicationService) Get(
	ctx context.Context,
	workspaceSlug, applicationID, userID string,
) (application.Application, error) {
	return service.get(ctx, workspaceSlug, applicationID, userID)
}

func (service *stubApplicationService) Create(
	ctx context.Context,
	input application.CreateInput,
) (application.Application, error) {
	return service.create(ctx, input)
}

func (service *stubApplicationService) Update(
	ctx context.Context,
	input application.UpdateInput,
) (application.Application, error) {
	return service.update(ctx, input)
}

func TestApplicationRoutesRequireDevelopmentIdentity(t *testing.T) {
	t.Parallel()

	service := &stubApplicationService{
		list: func(context.Context, string, string) ([]application.Application, error) {
			t.Fatal("service should not be called")
			return nil, nil
		},
	}
	router := NewRouter(discardLogger(), AlwaysReady)
	RegisterApplicationRoutes(router, service, discardLogger())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/acme/applications", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	assertErrorCode(t, response, "authentication_required")
}

func TestCreateApplicationReturnsExplicitDTO(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)
	service := &stubApplicationService{
		create: func(_ context.Context, input application.CreateInput) (application.Application, error) {
			if input.WorkspaceSlug != "acme" || input.UserID != handlerTestUserID {
				t.Fatalf("create input = %#v", input)
			}
			return application.Application{
				ID:              "22222222-2222-4222-8222-222222222222",
				Name:            input.Name,
				Slug:            input.Slug,
				Description:     input.Description,
				Lifecycle:       application.LifecycleActive,
				DefaultHostname: "invoice-approvals.acme.sprout.local",
				CreatedAt:       createdAt,
				UpdatedAt:       createdAt,
			}, nil
		},
	}
	router := NewRouter(discardLogger(), AlwaysReady)
	RegisterApplicationRoutes(router, service, discardLogger())
	body := bytes.NewBufferString(`{"name":"Invoice approvals","slug":"invoice-approvals","description":"Review invoices"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/acme/applications", body)
	request.Header.Set(developmentUserHeader, handlerTestUserID)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if response.Header().Get("Location") != "/api/v1/workspaces/acme/applications/22222222-2222-4222-8222-222222222222" {
		t.Fatalf("Location = %q", response.Header().Get("Location"))
	}

	var payload applicationResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Lifecycle != application.LifecycleActive || payload.DefaultHostname == "" {
		t.Fatalf("response = %#v", payload)
	}
}

func TestCreateApplicationRejectsUnknownJSONField(t *testing.T) {
	t.Parallel()

	service := &stubApplicationService{
		create: func(context.Context, application.CreateInput) (application.Application, error) {
			t.Fatal("service should not be called")
			return application.Application{}, nil
		},
	}
	router := NewRouter(discardLogger(), AlwaysReady)
	RegisterApplicationRoutes(router, service, discardLogger())
	body := bytes.NewBufferString(`{"name":"Invoices","slug":"invoices","unexpected":true}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/acme/applications", body)
	request.Header.Set(developmentUserHeader, handlerTestUserID)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	assertErrorCode(t, response, "invalid_request")
}

func TestCreateApplicationMapsConflict(t *testing.T) {
	t.Parallel()

	service := &stubApplicationService{
		create: func(context.Context, application.CreateInput) (application.Application, error) {
			return application.Application{}, application.ErrConflict
		},
	}
	router := NewRouter(discardLogger(), AlwaysReady)
	RegisterApplicationRoutes(router, service, discardLogger())
	body := bytes.NewBufferString(`{"name":"Invoices","slug":"invoices"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/acme/applications", body)
	request.Header.Set(developmentUserHeader, handlerTestUserID)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
	assertErrorCode(t, response, "application_slug_conflict")
}

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, expected string) {
	t.Helper()

	var payload errorEnvelope
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Error.Code != expected {
		t.Fatalf("error code = %q, want %q", payload.Error.Code, expected)
	}
	if payload.Error.RequestID == "" || payload.Error.RequestID != response.Header().Get(requestIDHeader) {
		t.Fatalf("error request ID = %q, header = %q", payload.Error.RequestID, response.Header().Get(requestIDHeader))
	}
}
