package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Capabilities tells clients which optional features this API process offers,
// so the dashboard, CLI and agents ask instead of guessing from 404s.
type Capabilities struct {
	LocalBuilds bool `json:"localBuilds"`
}

func RegisterCapabilityRoutes(router chi.Router, capabilities Capabilities) {
	router.Get("/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, capabilities)
	})
}
