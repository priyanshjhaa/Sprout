package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/apitoken"
)

type TokenService interface {
	Create(ctx context.Context, userID, name string, ttl time.Duration) (apitoken.Token, string, error)
	List(ctx context.Context, userID string) ([]apitoken.Token, error)
	Revoke(ctx context.Context, userID, id string) error
}

type tokenHandler struct {
	service TokenService
	logger  *slog.Logger
	now     func() time.Time
}

type tokenResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Hint       string     `json:"hint"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	Expired    bool       `json:"expired"`
}

type createdTokenResponse struct {
	tokenResponse
	// Token is the secret. It is returned only here and cannot be retrieved again.
	Token string `json:"token"`
}

// RegisterTokenRoutes manages a user's own personal access tokens. These
// routes need a browser session: a token can never mint, list or revoke
// tokens, so a leaked one cannot extend its own life or spawn others.
func RegisterTokenRoutes(router chi.Router, service TokenService, logger *slog.Logger) {
	handler := &tokenHandler{service: service, logger: logger, now: time.Now}
	router.Route("/tokens", func(tokens chi.Router) {
		tokens.Use(requireSession)
		tokens.Get("/", handler.list)
		tokens.Post("/", handler.create)
		tokens.Delete("/{tokenId}", handler.revoke)
	})
}

// requireSession is a second lock behind the route allowlist: if the allowlist
// is ever widened by mistake, credential management still refuses tokens.
func requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authenticatedByToken(r.Context()) {
			writeAPIError(w, http.StatusForbidden, "session_required", "Sign in to the dashboard to manage access tokens.", requestIDFromContext(r.Context()))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (handler *tokenHandler) respond(token apitoken.Token) tokenResponse {
	return tokenResponse{ID: token.ID, Name: token.Name, Hint: token.Hint, CreatedAt: token.CreatedAt, ExpiresAt: token.ExpiresAt,
		LastUsedAt: token.LastUsedAt, Expired: !token.ExpiresAt.After(handler.now())}
}

func (handler *tokenHandler) list(w http.ResponseWriter, r *http.Request) {
	tokens, err := handler.service.List(r.Context(), userIDFromContext(r.Context()))
	if err != nil {
		handler.fail(w, r, err)
		return
	}
	response := make([]tokenResponse, 0, len(tokens))
	for _, token := range tokens {
		response = append(response, handler.respond(token))
	}
	writeJSON(w, http.StatusOK, struct {
		Tokens []tokenResponse `json:"tokens"`
	}{Tokens: response})
}

func (handler *tokenHandler) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name          string `json:"name"`
		ExpiresInDays *int   `json:"expiresInDays"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "The request body must be valid JSON.", requestIDFromContext(r.Context()))
		return
	}
	var ttl time.Duration // zero means the default
	if body.ExpiresInDays != nil {
		days := *body.ExpiresInDays
		if days < 1 || days > int(apitoken.MaxTTL/(24*time.Hour)) {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "expiresInDays must be between 1 and 365.", requestIDFromContext(r.Context()))
			return
		}
		ttl = time.Duration(days) * 24 * time.Hour
	}
	token, secret, err := handler.service.Create(r.Context(), userIDFromContext(r.Context()), body.Name, ttl)
	if err != nil {
		handler.fail(w, r, err)
		return
	}
	// The secret must not be stored by browsers, proxies or caches.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, createdTokenResponse{tokenResponse: handler.respond(token), Token: secret})
}

func (handler *tokenHandler) revoke(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.Revoke(r.Context(), userIDFromContext(r.Context()), chi.URLParam(r, "tokenId")); err != nil {
		handler.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *tokenHandler) fail(w http.ResponseWriter, r *http.Request, err error) {
	requestID := requestIDFromContext(r.Context())
	switch {
	case errors.Is(err, apitoken.ErrInvalid):
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "Give the token a name of 1 to 80 characters.", requestID)
	case errors.Is(err, apitoken.ErrLimit):
		writeAPIError(w, http.StatusConflict, "token_limit_reached", "You have the maximum number of active tokens. Revoke one first.", requestID)
	case errors.Is(err, apitoken.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "token_not_found", "The token could not be found.", requestID)
	default:
		handler.logger.ErrorContext(r.Context(), "token request failed", "request_id", requestID)
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "The server could not complete the request.", requestID)
	}
}
