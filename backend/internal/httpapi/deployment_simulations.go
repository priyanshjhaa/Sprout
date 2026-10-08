package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/deployment"
	"github.com/priyanshjhaa/Sprout/backend/internal/source"
)

type DeploymentSimulations interface {
	Submit(context.Context, deployment.Scope) (deployment.Job, error)
	List(context.Context, deployment.Scope) ([]deployment.Job, error)
	Get(context.Context, deployment.Scope, string) (deployment.Job, error)
	Cancel(context.Context, deployment.Scope, string) (deployment.Job, error)
}

// simulationHandler serves one kind of deployment: simulations at
// /deployment-simulations, or real builds at /deployments. Each kind only ever
// sees its own jobs, because Scope.Simulated filters every repository read.
type simulationHandler struct {
	service   DeploymentSimulations
	logger    *slog.Logger
	simulated bool
}

func RegisterDeploymentSimulationRoutes(router chi.Router, service DeploymentSimulations, logger *slog.Logger) {
	h := &simulationHandler{service: service, logger: logger, simulated: true}
	router.Route("/workspaces/{workspaceSlug}/applications/{applicationId}/deployment-simulations", func(r chi.Router) {
		r.Post("/", h.submit)
		r.Get("/", h.list)
		r.Get("/{deploymentId}", h.get)
		r.Post("/{deploymentId}/cancel", h.cancel)
	})
}

// BuildIntake accepts uploaded source archives for real builds.
type BuildIntake interface {
	Check(context.Context, deployment.Scope) error
	Submit(context.Context, deployment.Scope, []byte) (deployment.Job, error)
}

// RegisterDeploymentRoutes serves real builds. It is registered only when local
// builds are enabled; otherwise these paths do not exist.
func RegisterDeploymentRoutes(router chi.Router, service DeploymentSimulations, intake BuildIntake, logger *slog.Logger) {
	h := &simulationHandler{service: service, logger: logger}
	router.Route("/workspaces/{workspaceSlug}/applications/{applicationId}/deployments", func(r chi.Router) {
		r.Post("/", func(w http.ResponseWriter, r *http.Request) { h.upload(w, r, intake) })
		r.Get("/", h.list)
		r.Get("/{deploymentId}", h.get)
		r.Post("/{deploymentId}/cancel", h.cancel)
	})
}

func (h *simulationHandler) scope(r *http.Request) deployment.Scope {
	return deployment.Scope{WorkspaceSlug: chi.URLParam(r, "workspaceSlug"), ApplicationID: chi.URLParam(r, "applicationId"), UserID: userIDFromContext(r.Context()), Simulated: h.simulated}
}

func (h *simulationHandler) collection() string {
	if h.simulated {
		return "deployment-simulations"
	}
	return "deployments"
}

// upload receives a source archive. Permission and capacity are checked before
// the body is read, so a refused request costs neither time nor disk.
func (h *simulationHandler) upload(w http.ResponseWriter, r *http.Request, intake BuildIntake) {
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/x-tar" {
		w.Header().Set("Cache-Control", "no-store")
		writeAPIError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Upload a plain .tar archive with Content-Type application/x-tar.", requestIDFromContext(r.Context()))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	scope := h.scope(r)
	if err := intake.Check(ctx, scope); err != nil {
		h.respond(w, r, nil, err, 0)
		return
	}
	archive, err := io.ReadAll(http.MaxBytesReader(w, r.Body, source.MaxArchiveBytes))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		w.Header().Set("Cache-Control", "no-store")
		writeAPIError(w, http.StatusRequestEntityTooLarge, "source_too_large", "The archive is larger than 16 MiB.", requestIDFromContext(r.Context()))
		return
	}
	if err != nil {
		h.respond(w, r, nil, deployment.ErrInvalid, 0)
		return
	}
	job, err := intake.Submit(ctx, scope, archive)
	if err == nil {
		w.Header().Set("Location", fmt.Sprintf("/api/v1/workspaces/%s/applications/%s/deployments/%s", chi.URLParam(r, "workspaceSlug"), job.ApplicationID, job.ID))
	}
	h.respond(w, r, job, err, http.StatusCreated)
}
func (h *simulationHandler) submit(w http.ResponseWriter, r *http.Request) {
	var body *struct{}
	if err := decodeJSON(w, r, &body); err != nil || body == nil {
		h.respond(w, r, nil, deployment.ErrInvalid, 0)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	job, err := h.service.Submit(ctx, h.scope(r))
	if err == nil {
		w.Header().Set("Location", fmt.Sprintf("/api/v1/workspaces/%s/applications/%s/%s/%s", chi.URLParam(r, "workspaceSlug"), job.ApplicationID, h.collection(), job.ID))
	}
	h.respond(w, r, job, err, http.StatusCreated)
}
func (h *simulationHandler) list(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	jobs, err := h.service.List(ctx, h.scope(r))
	h.respond(w, r, struct {
		Deployments []deployment.Job `json:"deployments"`
	}{jobs}, err, http.StatusOK)
}
func (h *simulationHandler) get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	job, err := h.service.Get(ctx, h.scope(r), chi.URLParam(r, "deploymentId"))
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
	job, err := h.service.Cancel(ctx, h.scope(r), chi.URLParam(r, "deploymentId"))
	h.respond(w, r, job, err, http.StatusOK)
}
func (h *simulationHandler) respond(w http.ResponseWriter, r *http.Request, value any, err error, status int) {
	w.Header().Set("Cache-Control", "no-store")
	if err == nil {
		writeJSON(w, status, value)
		return
	}
	noun := "deployment"
	if h.simulated {
		noun = "simulation"
	}
	code, message := "internal_error", "The "+noun+" request could not be completed."
	status = 500
	switch {
	case errors.Is(err, deployment.ErrInvalid):
		status = 400
		code = "invalid_request"
		message = "Use valid identifiers and an empty JSON object."
		if !h.simulated {
			message = "Use valid identifiers and a non-empty source archive."
		}
	case errors.Is(err, deployment.ErrNotFound):
		status = 404
		code = "deployment_not_found"
		message = "The application or " + noun + " could not be found."
	case errors.Is(err, deployment.ErrForbidden):
		status = 403
		code = "operation_forbidden"
		message = "Application edit access is required."
	case errors.Is(err, deployment.ErrConflict):
		status = 409
		code = "deployment_conflict"
		message = "The app is inactive, already has work, or the " + noun + " is already finished."
	case errors.Is(err, deployment.ErrFull):
		status = 503
		code = "deployment_queue_full"
		message = "The deployment queue is full. Try again shortly."
		w.Header().Set("Retry-After", "3")
	case errors.Is(err, deployment.ErrUnavailable):
		status = 503
		code = "deployment_worker_unavailable"
		message = "The deployment worker is not accepting work."
	default:
		h.logger.ErrorContext(r.Context(), noun+" request failed", "request_id", requestIDFromContext(r.Context()))
	}
	writeAPIError(w, status, code, message, requestIDFromContext(r.Context()))
}
