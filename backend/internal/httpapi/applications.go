package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/application"
)

const maxRequestBodyBytes = 64 * 1024

type identityContextKey struct{}

type ApplicationService interface {
	List(context.Context, string, string) ([]application.Application, error)
	Get(context.Context, string, string, string) (application.Application, error)
	Create(context.Context, application.CreateInput) (application.Application, error)
	Update(context.Context, application.UpdateInput) (application.Application, error)
}

type applicationHandler struct {
	service ApplicationService
	logger  *slog.Logger
}

type createApplicationRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type updateApplicationRequest struct {
	Name        *string                `json:"name"`
	Description *string                `json:"description"`
	Lifecycle   *application.Lifecycle `json:"lifecycle"`
}

type applicationResponse struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Slug            string                `json:"slug"`
	Description     string                `json:"description"`
	Lifecycle       application.Lifecycle `json:"lifecycle"`
	DefaultHostname string                `json:"defaultHostname"`
	CreatedAt       time.Time             `json:"createdAt"`
	UpdatedAt       time.Time             `json:"updatedAt"`
}

type applicationListResponse struct {
	Applications []applicationResponse `json:"applications"`
}

func RegisterApplicationRoutes(router chi.Router, service ApplicationService, logger *slog.Logger) {
	handler := &applicationHandler{service: service, logger: logger}

	router.Route("/workspaces/{workspaceSlug}/applications", func(applications chi.Router) {
		applications.Get("/", handler.list)
		applications.Post("/", handler.create)
		applications.Get("/{applicationId}", handler.get)
		applications.Patch("/{applicationId}", handler.update)
	})
}

func (handler *applicationHandler) list(w http.ResponseWriter, r *http.Request) {
	applications, err := handler.service.List(
		r.Context(),
		chi.URLParam(r, "workspaceSlug"),
		userIDFromContext(r.Context()),
	)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}

	response := make([]applicationResponse, 0, len(applications))
	for _, item := range applications {
		response = append(response, mapApplicationResponse(item))
	}

	writeJSON(w, http.StatusOK, applicationListResponse{Applications: response})
}

func (handler *applicationHandler) get(w http.ResponseWriter, r *http.Request) {
	item, err := handler.service.Get(
		r.Context(),
		chi.URLParam(r, "workspaceSlug"),
		chi.URLParam(r, "applicationId"),
		userIDFromContext(r.Context()),
	)
	if err != nil {
		handler.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, mapApplicationResponse(item))
}

func (handler *applicationHandler) create(w http.ResponseWriter, r *http.Request) {
	var request createApplicationRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(
			w,
			http.StatusBadRequest,
			"invalid_request",
			"The request body must be a single valid JSON object.",
			requestIDFromContext(r.Context()),
		)
		return
	}

	item, err := handler.service.Create(r.Context(), application.CreateInput{
		WorkspaceSlug: chi.URLParam(r, "workspaceSlug"),
		UserID:        userIDFromContext(r.Context()),
		Name:          request.Name,
		Slug:          request.Slug,
		Description:   request.Description,
	})
	if err != nil {
		handler.writeError(w, r, err)
		return
	}

	w.Header().Set("Location", fmt.Sprintf(
		"/api/v1/workspaces/%s/applications/%s",
		chi.URLParam(r, "workspaceSlug"),
		item.ID,
	))
	writeJSON(w, http.StatusCreated, mapApplicationResponse(item))
}

func (handler *applicationHandler) update(w http.ResponseWriter, r *http.Request) {
	var request updateApplicationRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAPIError(
			w,
			http.StatusBadRequest,
			"invalid_request",
			"The request body must be a single valid JSON object.",
			requestIDFromContext(r.Context()),
		)
		return
	}

	item, err := handler.service.Update(r.Context(), application.UpdateInput{
		WorkspaceSlug: chi.URLParam(r, "workspaceSlug"),
		ApplicationID: chi.URLParam(r, "applicationId"),
		UserID:        userIDFromContext(r.Context()),
		Name:          request.Name,
		Description:   request.Description,
		Lifecycle:     request.Lifecycle,
	})
	if err != nil {
		handler.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, mapApplicationResponse(item))
}

func (handler *applicationHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	requestID := requestIDFromContext(r.Context())

	var validationError *application.ValidationError
	switch {
	case errors.Is(err, application.ErrInvalidIdentity):
		writeAPIError(w, http.StatusUnauthorized, "authentication_required", "The development identity is invalid.", requestID)
	case errors.As(err, &validationError):
		writeAPIError(w, http.StatusBadRequest, "invalid_request", validationError.Error(), requestID)
	case errors.Is(err, application.ErrForbidden):
		writeAPIError(w, http.StatusForbidden, "operation_forbidden", "Your workspace role does not allow this operation.", requestID)
	case errors.Is(err, application.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "application_not_found", "The application or workspace could not be found.", requestID)
	case errors.Is(err, application.ErrConflict):
		writeAPIError(w, http.StatusConflict, "application_slug_conflict", "An application with this slug already exists in the workspace.", requestID)
	default:
		handler.logger.ErrorContext(r.Context(), "application request failed", "request_id", requestID)
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "The server could not complete the request.", requestID)
	}
}

func mapApplicationResponse(item application.Application) applicationResponse {
	return applicationResponse{
		ID:              item.ID,
		Name:            item.Name,
		Slug:            item.Slug,
		Description:     item.Description,
		Lifecycle:       item.Lifecycle,
		DefaultHostname: item.DefaultHostname,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func userIDFromContext(ctx context.Context) string {
	userID, _ := ctx.Value(identityContextKey{}).(string)
	return userID
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}

	return nil
}
