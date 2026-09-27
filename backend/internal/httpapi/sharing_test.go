package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/sharing"
)

type stubSharingService struct {
	getAccess func(context.Context, string, string, string) (sharing.Access, error)
	setMode   func(context.Context, string, string, string, string) (sharing.Access, error)
}

func (service stubSharingService) ListMembers(context.Context, string, string) ([]sharing.Member, error) {
	return []sharing.Member{}, nil
}

func (service stubSharingService) GetAccess(ctx context.Context, workspace, application, actor string) (sharing.Access, error) {
	return service.getAccess(ctx, workspace, application, actor)
}

func (service stubSharingService) SetMode(ctx context.Context, workspace, application, actor, mode string) (sharing.Access, error) {
	return service.setMode(ctx, workspace, application, actor, mode)
}

func (service stubSharingService) Grant(context.Context, string, string, string, string, string) (sharing.Access, error) {
	return sharing.Access{}, nil
}

func (service stubSharingService) Revoke(context.Context, string, string, string, string) (sharing.Access, error) {
	return sharing.Access{}, nil
}

func sharingRouterForTest(service SharingService) *chi.Mux {
	router := NewRouter(discardLogger(), AlwaysReady, "")
	router.Route("/api/v1", func(api chi.Router) {
		api.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-session" {
					writeAPIError(w, http.StatusUnauthorized, "authentication_required", "A valid session is required.", requestIDFromContext(r.Context()))
					return
				}
				ctx := context.WithValue(r.Context(), identityContextKey{}, handlerTestUserID)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		RegisterSharingRoutes(api, service, discardLogger())
	})
	return router
}

func TestSharingRoutesRequireAuthentication(t *testing.T) {
	service := stubSharingService{}
	router := sharingRouterForTest(service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/acme/members", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}

func TestSharingRouteScopesIdentityAndMapsForbidden(t *testing.T) {
	service := stubSharingService{
		getAccess: func(_ context.Context, workspace, application, actor string) (sharing.Access, error) {
			if workspace != "acme" || application != "app-id" || actor != handlerTestUserID {
				t.Fatalf("route context = %q, %q, %q", workspace, application, actor)
			}
			return sharing.Access{}, sharing.ErrForbidden
		},
	}
	router := sharingRouterForTest(service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/acme/applications/app-id/access", nil)
	request.Header.Set("Authorization", "Bearer test-session")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
	assertErrorCode(t, response, "access_forbidden")
}

func TestSharingRouteRejectsUnknownFields(t *testing.T) {
	service := stubSharingService{
		setMode: func(context.Context, string, string, string, string) (sharing.Access, error) {
			t.Fatal("service should not be called")
			return sharing.Access{}, nil
		},
	}
	router := sharingRouterForTest(service)
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/workspaces/acme/applications/app-id/access",
		bytes.NewBufferString(`{"accessMode":"restricted","unexpected":true}`))
	request.Header.Set("Authorization", "Bearer test-session")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	assertErrorCode(t, response, "invalid_request")
}
