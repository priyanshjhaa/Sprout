package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowsConfiguredWebOrigin(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/workspaces/acme/applications", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	response := httptest.NewRecorder()

	NewRouter(discardLogger(), AlwaysReady, "http://localhost:3000").ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("allow origin = %q, want configured origin", response.Header().Get("Access-Control-Allow-Origin"))
	}
	if response.Header().Get("Access-Control-Allow-Headers") != "Content-Type, Authorization, Last-Event-ID" {
		t.Fatalf("allow headers = %q, want bearer auth header", response.Header().Get("Access-Control-Allow-Headers"))
	}
	if response.Header().Get("Access-Control-Allow-Methods") != allowedRequestMethods {
		t.Fatalf("allow methods = %q, want sharing methods", response.Header().Get("Access-Control-Allow-Methods"))
	}
}

func TestCORSDoesNotAllowAnotherOrigin(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	request.Header.Set("Origin", "https://untrusted.example")
	response := httptest.NewRecorder()

	NewRouter(discardLogger(), AlwaysReady, "http://localhost:3000").ServeHTTP(response, request)

	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("allow origin = %q, want empty", response.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSAnswersEachConfiguredOriginWithItself(t *testing.T) {
	t.Parallel()

	router := NewRouter(discardLogger(), AlwaysReady, "http://localhost:3000", "http://localhost:3001")
	for _, origin := range []string{"http://localhost:3000", "http://localhost:3001"} {
		request := httptest.NewRequest(http.MethodOptions, "/api/v1/me", nil)
		request.Header.Set("Origin", origin)
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if got := response.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Fatalf("allow origin for %s = %q, want the same origin", origin, got)
		}
	}
}

func TestCORSIgnoresRequestsWithoutAnOrigin(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()

	NewRouter(discardLogger(), AlwaysReady, "").ServeHTTP(response, request)

	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("allow origin = %q, want empty", response.Header().Get("Access-Control-Allow-Origin"))
	}
}
