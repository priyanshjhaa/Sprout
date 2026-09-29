package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/identity"
	"github.com/priyanshjhaa/Sprout/backend/internal/team"
)

type TeamService interface {
	Workspaces(context.Context, string) ([]identity.Workspace, error)
	List(context.Context, string, string) ([]team.Invitation, error)
	Create(context.Context, string, string, string, string) (team.CreatedInvitation, error)
	Accept(context.Context, string, string, string) (identity.Workspace, error)
	Revoke(context.Context, string, string, string) error
	ChangeRole(context.Context, string, string, string, string) error
	Remove(context.Context, string, string, string) error
}
type teamHandler struct {
	service TeamService
	logger  *slog.Logger
}

func RegisterTeamRoutes(router chi.Router, service TeamService, logger *slog.Logger) {
	h := &teamHandler{service, logger}
	router.Get("/workspaces", h.workspaces)
	router.Get("/workspaces/{workspaceSlug}/invitations", h.list)
	router.Post("/workspaces/{workspaceSlug}/invitations", h.create)
	router.Delete("/workspaces/{workspaceSlug}/invitations/{invitationId}", h.revoke)
	router.Post("/invitations/accept", h.accept)
	router.Patch("/workspaces/{workspaceSlug}/members/{userId}", h.changeRole)
	router.Delete("/workspaces/{workspaceSlug}/members/{userId}", h.remove)
}
func (h *teamHandler) workspaces(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Workspaces(r.Context(), userIDFromContext(r.Context()))
	h.respond(w, r, struct {
		Workspaces []identity.Workspace `json:"workspaces"`
	}{result}, err, http.StatusOK)
}
func (h *teamHandler) list(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.List(r.Context(), chi.URLParam(r, "workspaceSlug"), userIDFromContext(r.Context()))
	h.respond(w, r, struct {
		Invitations []team.Invitation `json:"invitations"`
	}{result}, err, http.StatusOK)
}
func (h *teamHandler) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		h.respond(w, r, nil, team.ErrInvalid, 0)
		return
	}
	result, err := h.service.Create(r.Context(), chi.URLParam(r, "workspaceSlug"), userIDFromContext(r.Context()), body.Email, body.Role)
	h.respond(w, r, result, err, http.StatusCreated)
}
func (h *teamHandler) accept(w http.ResponseWriter, r *http.Request) {
	claims, ok := clerk.SessionClaimsFromContext(r.Context())
	if !ok || claims == nil || claims.Subject == "" {
		writeAPIError(w, 401, "authentication_required", "A valid session is required.", requestIDFromContext(r.Context()))
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		h.respond(w, r, nil, team.ErrInvalid, 0)
		return
	}
	result, err := h.service.Accept(r.Context(), userIDFromContext(r.Context()), claims.Subject, body.Token)
	h.respond(w, r, result, err, http.StatusOK)
}
func (h *teamHandler) revoke(w http.ResponseWriter, r *http.Request) {
	err := h.service.Revoke(r.Context(), chi.URLParam(r, "workspaceSlug"), userIDFromContext(r.Context()), chi.URLParam(r, "invitationId"))
	h.respond(w, r, struct {
		OK bool `json:"ok"`
	}{true}, err, http.StatusOK)
}
func (h *teamHandler) changeRole(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		h.respond(w, r, nil, team.ErrInvalid, 0)
		return
	}
	err := h.service.ChangeRole(r.Context(), chi.URLParam(r, "workspaceSlug"), userIDFromContext(r.Context()), chi.URLParam(r, "userId"), body.Role)
	h.respond(w, r, struct {
		OK bool `json:"ok"`
	}{true}, err, http.StatusOK)
}
func (h *teamHandler) remove(w http.ResponseWriter, r *http.Request) {
	err := h.service.Remove(r.Context(), chi.URLParam(r, "workspaceSlug"), userIDFromContext(r.Context()), chi.URLParam(r, "userId"))
	h.respond(w, r, struct {
		OK bool `json:"ok"`
	}{true}, err, http.StatusOK)
}
func (h *teamHandler) respond(w http.ResponseWriter, r *http.Request, result any, err error, status int) {
	w.Header().Set("Cache-Control", "no-store")
	if err == nil {
		writeJSON(w, status, result)
		return
	}
	code, message := "internal_error", "The team request could not be completed."
	status = http.StatusInternalServerError
	switch {
	case errors.Is(err, team.ErrInvalid):
		status = 400
		code = "invalid_request"
		message = "Enter a valid email and an editor or viewer role."
	case errors.Is(err, team.ErrForbidden):
		status = 403
		code = "access_forbidden"
		message = "Only owners can manage the team. Owners cannot be removed or demoted."
	case errors.Is(err, team.ErrNotFound):
		status = 404
		code = "resource_not_found"
		message = "The resource could not be found."
	case errors.Is(err, team.ErrConflict):
		status = 409
		code = "invitation_conflict"
		message = "This person is already a member or has a pending invitation, or the workspace has 100 pending invitations."
	case errors.Is(err, team.ErrInvitation):
		status = 404
		code = "invitation_unavailable"
		message = "This invitation is expired, revoked, already used, or not for your verified primary email."
	case errors.Is(err, team.ErrIdentity):
		status = 503
		code = "identity_unavailable"
		message = "Your verified email could not be checked. Try again."
	default:
		h.logger.ErrorContext(r.Context(), "team request failed", "request_id", requestIDFromContext(r.Context()))
	}
	writeAPIError(w, status, code, message, requestIDFromContext(r.Context()))
}
