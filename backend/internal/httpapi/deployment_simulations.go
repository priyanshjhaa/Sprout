package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/deployment"
)

type DeploymentSimulations interface {
	Submit(context.Context, deployment.Scope) (deployment.Job, error)
	List(context.Context, deployment.Scope) ([]deployment.Job, error)
	Get(context.Context, deployment.Scope, string) (deployment.Job, error)
	Cancel(context.Context, deployment.Scope, string) (deployment.Job, error)
}
type simulationHandler struct {
	service DeploymentSimulations
	logger  *slog.Logger
}

func RegisterDeploymentSimulationRoutes(router chi.Router, service DeploymentSimulations, logger *slog.Logger) {
	h := &simulationHandler{service, logger}
	router.Route("/workspaces/{workspaceSlug}/applications/{applicationId}/deployment-simulations", func(r chi.Router) {
		r.Post("/", h.submit)
		r.Get("/", h.list)
		r.Get("/{deploymentId}", h.get)
		r.Post("/{deploymentId}/cancel", h.cancel)
	})
}
func simulationScope(r *http.Request) deployment.Scope {
	return deployment.Scope{WorkspaceSlug: chi.URLParam(r, "workspaceSlug"), ApplicationID: chi.URLParam(r, "applicationId"), UserID: userIDFromContext(r.Context()), Simulated: true}
}
func (h *simulationHandler) submit(w http.ResponseWriter, r *http.Request) {
	var body *struct{}
	if err := decodeJSON(w, r, &body); err != nil || body == nil {
		h.respond(w, r, nil, deployment.ErrInvalid, 0)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	job, err := h.service.Submit(ctx, simulationScope(r))
	if err == nil {
		w.Header().Set("Location", fmt.Sprintf("/api/v1/workspaces/%s/applications/%s/deployment-simulations/%s", chi.URLParam(r, "workspaceSlug"), job.ApplicationID, job.ID))
	}
	h.respond(w, r, job, err, http.StatusCreated)
}
func (h *simulationHandler) list(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	jobs, err := h.service.List(ctx, simulationScope(r))
	h.respond(w, r, struct {
		Deployments []deployment.Job `json:"deployments"`
	}{jobs}, err, http.StatusOK)
}
func (h *simulationHandler) get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	job, err := h.service.Get(ctx, simulationScope(r), chi.URLParam(r, "deploymentId"))
	h.respond(w, r, job, err, http.StatusOK)
}
func (h *simulationHandler) cancel(w http.ResponseWriter, r *http.Request) {
	var body *struct{}
	if err := decodeJSON(w, r, &body); err != nil || body == nil {
		h.respond(w, r, nil, deployment.ErrInvalid, 0)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	job, err := h.service.Cancel(ctx, simulationScope(r), chi.URLParam(r, "deploymentId"))
	h.respond(w, r, job, err, http.StatusOK)
}
func (h *simulationHandler) respond(w http.ResponseWriter, r *http.Request, value any, err error, status int) {
	w.Header().Set("Cache-Control", "no-store")
	if err == nil {
		writeJSON(w, status, value)
		return
	}
	code, message := "internal_error", "The simulation request could not be completed."
	status = 500
	switch {
	case errors.Is(err, deployment.ErrInvalid):
		status = 400
		code = "invalid_request"
		message = "Use valid identifiers and an empty JSON object."
	case errors.Is(err, deployment.ErrNotFound):
		status = 404
		code = "deployment_not_found"
		message = "The application or simulation could not be found."
	case errors.Is(err, deployment.ErrForbidden):
		status = 403
		code = "operation_forbidden"
		message = "Application edit access is required."
	case errors.Is(err, deployment.ErrConflict):
		status = 409
		code = "deployment_conflict"
		message = "The app is inactive, already has work, or the simulation is already finished."
	case errors.Is(err, deployment.ErrFull):
		status = 503
		code = "deployment_queue_full"
		message = "The simulation queue is full. Try again shortly."
		w.Header().Set("Retry-After", "3")
	case errors.Is(err, deployment.ErrUnavailable):
		status = 503
		code = "deployment_worker_unavailable"
		message = "The simulation worker is not accepting work."
	default:
		h.logger.ErrorContext(r.Context(), "simulation request failed", "request_id", requestIDFromContext(r.Context()))
	}
	writeAPIError(w, status, code, message, requestIDFromContext(r.Context()))
}
