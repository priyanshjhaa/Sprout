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
