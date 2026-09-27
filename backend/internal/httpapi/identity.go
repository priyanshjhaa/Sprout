package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/identity"
)

type WorkspaceReader interface {
	GetWorkspace(context.Context, string, string) (identity.Workspace, error)
}

func RegisterIdentityRoutes(router chi.Router, reader WorkspaceReader, logger *slog.Logger) {
	router.Get("/me", func(w http.ResponseWriter, r *http.Request) {
		principal := principalFromContext(r.Context())
		writeJSON(w, http.StatusOK, principalResponse(principal))
	})
	router.Get("/workspaces/{workspaceSlug}", func(w http.ResponseWriter, r *http.Request) {
		workspace, err := reader.GetWorkspace(r.Context(), chi.URLParam(r, "workspaceSlug"), userIDFromContext(r.Context()))
		if errors.Is(err, identity.ErrNotFound) {
			writeAPIError(w, http.StatusNotFound, "workspace_not_found", "The workspace could not be found.", requestIDFromContext(r.Context()))
			return
		}
		if err != nil {
			logger.ErrorContext(r.Context(), "workspace request failed", "request_id", requestIDFromContext(r.Context()))
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "The server could not complete the request.", requestIDFromContext(r.Context()))
			return
		}
		writeJSON(w, http.StatusOK, workspace)
	})
}

type meResponse struct {
	ID          string             `json:"id"`
	Email       string             `json:"email"`
	DisplayName string             `json:"displayName"`
	Workspace   identity.Workspace `json:"workspace"`
}

func principalResponse(principal identity.Principal) meResponse {
	return meResponse{ID: principal.ID, Email: principal.Email, DisplayName: principal.DisplayName, Workspace: principal.Workspace}
}
