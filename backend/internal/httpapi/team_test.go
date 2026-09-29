package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/identity"
	"github.com/priyanshjhaa/Sprout/backend/internal/team"
)

type stubTeam struct {
	TeamService
	create func(context.Context, string, string, string, string) (team.CreatedInvitation, error)
	accept func(context.Context, string, string, string) (identity.Workspace, error)
}

func (s stubTeam) Create(ctx context.Context, slug, actor, email, role string) (team.CreatedInvitation, error) {
	return s.create(ctx, slug, actor, email, role)
}
func (s stubTeam) Accept(ctx context.Context, actor, subject, token string) (identity.Workspace, error) {
	return s.accept(ctx, actor, subject, token)
}

func teamRouter(service TeamService) *chi.Mux {
	router := NewRouter(discardLogger(), AlwaysReady, "")
	router.Route("/api/v1", func(api chi.Router) {
		api.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-session" {
					writeAPIError(w, 401, "authentication_required", "A valid session is required.", requestIDFromContext(r.Context()))
					return
				}
				ctx := context.WithValue(r.Context(), identityContextKey{}, handlerTestUserID)
				claims := &clerk.SessionClaims{}
				claims.Subject = "clerk-test-user"
				ctx = clerk.ContextWithSessionClaims(ctx, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		RegisterTeamRoutes(api, service, discardLogger())
	})
	return router
}
func TestTeamRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		serviceErr error
		want       int
		code       string
	}{
		{"success", `{"email":"person@example.test","role":"viewer"}`, nil, 201, ""},
		{"unknown field", `{"email":"person@example.test","role":"viewer","actor":"spoofed"}`, nil, 400, "invalid_request"},
		{"forbidden", `{"email":"person@example.test","role":"viewer"}`, team.ErrForbidden, 403, "access_forbidden"},
		{"duplicate", `{"email":"person@example.test","role":"viewer"}`, team.ErrConflict, 409, "invitation_conflict"},
		{"not found", `{"email":"person@example.test","role":"viewer"}`, team.ErrNotFound, 404, "resource_not_found"},
		{"internal", `{"email":"person@example.test","role":"viewer"}`, errors.New("sensitive database detail"), 500, "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			router := teamRouter(stubTeam{create: func(_ context.Context, slug, actor, email, role string) (team.CreatedInvitation, error) {
				called = true
				if slug != "acme" || actor != handlerTestUserID || email != "person@example.test" || role != "viewer" {
					t.Fatal("incorrect request trace")
				}
				return team.CreatedInvitation{Token: "one-time-token"}, tc.serviceErr
			}})
			request := httptest.NewRequest("POST", "/api/v1/workspaces/acme/invitations", strings.NewReader(tc.body))
			request.Header.Set("Authorization", "Bearer test-session")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
			if tc.code != "" {
				assertErrorCode(t, response, tc.code)
			}
			if tc.want == 400 && called {
				t.Fatal("invalid JSON reached service")
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("invitation response is cacheable")
			}
			if strings.Contains(response.Body.String(), "sensitive") {
				t.Fatal("internal details exposed")
			}
		})
	}
}
func TestInvitationAcceptanceTrace(t *testing.T) {
	router := teamRouter(stubTeam{accept: func(_ context.Context, actor, subject, token string) (identity.Workspace, error) {
		if actor != handlerTestUserID || subject != "clerk-test-user" || token != "invitation-token" {
			t.Fatal("identity/token not propagated")
		}
		return identity.Workspace{Slug: "shared"}, nil
	}})
	request := httptest.NewRequest("POST", "/api/v1/invitations/accept", strings.NewReader(`{"token":"invitation-token"}`))
	request.Header.Set("Authorization", "Bearer test-session")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("status %d", response.Code)
	}
	for _, route := range []struct{ method, path string }{
		{"POST", "/api/v1/invitations/accept"}, {"GET", "/api/v1/workspaces"},
		{"GET", "/api/v1/workspaces/acme/invitations"}, {"POST", "/api/v1/workspaces/acme/invitations"},
		{"DELETE", "/api/v1/workspaces/acme/invitations/invitation"},
		{"PATCH", "/api/v1/workspaces/acme/members/user"}, {"DELETE", "/api/v1/workspaces/acme/members/user"},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(route.method, route.path, nil))
		if response.Code != 401 {
			t.Fatalf("unauthenticated %s status %d", route.path, response.Code)
		}
	}
}
