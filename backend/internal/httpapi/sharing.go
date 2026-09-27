package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/sharing"
)

type SharingService interface {
	ListMembers(context.Context, string, string) ([]sharing.Member, error)
	GetAccess(context.Context, string, string, string) (sharing.Access, error)
	SetMode(context.Context, string, string, string, string) (sharing.Access, error)
	Grant(context.Context, string, string, string, string, string) (sharing.Access, error)
	Revoke(context.Context, string, string, string, string) (sharing.Access, error)
}

type sharingHandler struct {
	service SharingService
	logger  *slog.Logger
}

func RegisterSharingRoutes(router chi.Router, service SharingService, logger *slog.Logger) {
	handler := &sharingHandler{service: service, logger: logger}
	router.Get("/workspaces/{workspaceSlug}/members", handler.listMembers)
	router.Get("/workspaces/{workspaceSlug}/applications/{applicationId}/access", handler.getAccess)
	router.Patch("/workspaces/{workspaceSlug}/applications/{applicationId}/access", handler.setMode)
	router.Put("/workspaces/{workspaceSlug}/applications/{applicationId}/access/{userId}", handler.grant)
	router.Delete("/workspaces/{workspaceSlug}/applications/{applicationId}/access/{userId}", handler.revoke)
}

func (handler *sharingHandler) listMembers(w http.ResponseWriter, r *http.Request) {
	members, err := handler.service.ListMembers(r.Context(), chi.URLParam(r, "workspaceSlug"), userIDFromContext(r.Context()))
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Members []sharing.Member `json:"members"`
	}{Members: members})
}

func (handler *sharingHandler) getAccess(w http.ResponseWriter, r *http.Request) {
	access, err := handler.service.GetAccess(r.Context(), chi.URLParam(r, "workspaceSlug"),
		chi.URLParam(r, "applicationId"), userIDFromContext(r.Context()))
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, access)
}

func (handler *sharingHandler) setMode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccessMode string `json:"accessMode"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "The request body must be valid JSON.", requestIDFromContext(r.Context()))
		return
	}
	access, err := handler.service.SetMode(r.Context(), chi.URLParam(r, "workspaceSlug"),
		chi.URLParam(r, "applicationId"), userIDFromContext(r.Context()), body.AccessMode)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, access)
}

func (handler *sharingHandler) grant(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "The request body must be valid JSON.", requestIDFromContext(r.Context()))
		return
	}
	access, err := handler.service.Grant(r.Context(), chi.URLParam(r, "workspaceSlug"),
		chi.URLParam(r, "applicationId"), userIDFromContext(r.Context()), chi.URLParam(r, "userId"), body.Role)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, access)
}

func (handler *sharingHandler) revoke(w http.ResponseWriter, r *http.Request) {
	access, err := handler.service.Revoke(r.Context(), chi.URLParam(r, "workspaceSlug"),
		chi.URLParam(r, "applicationId"), userIDFromContext(r.Context()), chi.URLParam(r, "userId"))
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, access)
}

func (handler *sharingHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	requestID := requestIDFromContext(r.Context())
	switch {
	case errors.Is(err, sharing.ErrInvalid):
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "The access request is invalid.", requestID)
	case errors.Is(err, sharing.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "resource_not_found", "The resource could not be found.", requestID)
	case errors.Is(err, sharing.ErrForbidden):
		writeAPIError(w, http.StatusForbidden, "access_forbidden", "You cannot manage this access.", requestID)
	default:
		handler.logger.ErrorContext(r.Context(), "sharing request failed", "request_id", requestID)
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "The server could not complete the request.", requestID)
	}
}
