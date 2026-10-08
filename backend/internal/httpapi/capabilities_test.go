package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestCapabilities(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		router := chi.NewRouter()
		RegisterCapabilityRoutes(router, Capabilities{LocalBuilds: enabled})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/capabilities", nil))
		want := `{"localBuilds":false}`
		if enabled {
			want = `{"localBuilds":true}`
		}
		if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != want {
			t.Fatalf("capabilities: %d %s", response.Code, response.Body.String())
		}
	}
}
