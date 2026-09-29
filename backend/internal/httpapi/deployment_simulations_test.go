package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/deployment"
)

type simulationStub struct {
	DeploymentSimulations
	submit func(context.Context, deployment.Scope) (deployment.Job, error)
}

func (s simulationStub) Submit(ctx context.Context, scope deployment.Scope) (deployment.Job, error) {
	return s.submit(ctx, scope)
}
func simulationRouter(service DeploymentSimulations) *chi.Mux {
	router := NewRouter(discardLogger(), AlwaysReady, "")
	router.Route("/api/v1", func(api chi.Router) {
		api.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-session" {
					writeAPIError(w, 401, "authentication_required", "A valid session is required.", requestIDFromContext(r.Context()))
					return
				}
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityContextKey{}, handlerTestUserID)))
			})
		})
		RegisterDeploymentSimulationRoutes(api, service, discardLogger())
	})
	return router
}

const simulationPath = "/api/v1/workspaces/acme/applications/app-id/deployment-simulations"

func TestSimulationHTTPContract(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		err        error
		status     int
		code       string
	}{
		{"accepted", `{}`, nil, 201, ""},
		{"untrusted input", `{"command":"echo secret"}`, nil, 400, "invalid_request"},
		{"malformed", `{`, nil, 400, "invalid_request"},
		{"null", `null`, nil, 400, "invalid_request"},
		{"forbidden", `{}`, deployment.ErrForbidden, 403, "operation_forbidden"},
		{"not found", `{}`, deployment.ErrNotFound, 404, "deployment_not_found"},
		{"active job", `{}`, deployment.ErrConflict, 409, "deployment_conflict"},
		{"saturated", `{}`, deployment.ErrFull, 503, "deployment_queue_full"},
		{"shutdown", `{}`, deployment.ErrUnavailable, 503, "deployment_worker_unavailable"},
		{"database failure", `{}`, errors.New("secret internal SQL detail"), 500, "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := simulationStub{submit: func(ctx context.Context, scope deployment.Scope) (deployment.Job, error) {
				if tc.status == 400 {
					t.Fatal("invalid input reached worker")
				}
				if scope.WorkspaceSlug != "acme" || scope.ApplicationID != "app-id" || scope.UserID != handlerTestUserID {
					t.Fatalf("scope=%+v", scope)
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("missing request deadline")
				}
				return deployment.Job{ID: "job-id", ApplicationID: "app-id", Status: "queued", Simulated: true, Stages: []deployment.Stage{}}, tc.err
			}}
			request := httptest.NewRequest("POST", simulationPath, strings.NewReader(tc.body))
			request.Header.Set("Authorization", "Bearer test-session")
			response := httptest.NewRecorder()
			simulationRouter(service).ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
			if tc.code != "" {
				assertErrorCode(t, response, tc.code)
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("response cacheable")
			}
			if tc.status == 201 && response.Header().Get("Location") != simulationPath+"/job-id" {
				t.Fatal("missing job location")
			}
			if errors.Is(tc.err, deployment.ErrFull) && response.Header().Get("Retry-After") == "" {
				t.Fatal("missing backpressure hint")
			}
			if strings.Contains(response.Body.String(), "secret") {
				t.Fatal("internal details leaked")
			}
		})
	}
}
func TestSimulationRoutesRequireAuthentication(t *testing.T) {
	router := simulationRouter(simulationStub{})
	for _, route := range []struct{ method, path string }{{"POST", simulationPath}, {"GET", simulationPath}, {"GET", simulationPath + "/job-id"}, {"POST", simulationPath + "/job-id/cancel"}} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(route.method, route.path, nil))
		if response.Code != 401 {
			t.Fatalf("%s status=%d", route.path, response.Code)
		}
	}
}
