package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

type ReadinessCheck func(context.Context) error

func AlwaysReady(context.Context) error {
	return nil
}

func NewServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

func NewRouter(logger *slog.Logger, readiness ReadinessCheck, webOrigin string) *chi.Mux {
	router := chi.NewRouter()
	router.Use(corsMiddleware(webOrigin))
	router.Use(requestIDMiddleware)
	router.Use(requestLogger(logger))
	router.Use(recoverPanic(logger))

	router.Get("/health/live", handleLiveness)
	router.Get("/health/ready", handleReadiness(readiness))

	return router
}

func Serve(ctx context.Context, listener net.Listener, server *http.Server) error {
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.Serve(listener)
	}()

	select {
	case err := <-serveErrors:
		return normalizeServeError(err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		shutdownErr := server.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			closeErr := server.Close()
			serveErr := <-serveErrors

			return errors.Join(
				fmt.Errorf("graceful shutdown: %w", shutdownErr),
				closeErr,
				normalizeServeError(serveErr),
			)
		}

		return normalizeServeError(<-serveErrors)
	}
}

func handleLiveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Service: "sprout-api",
	})
}

func handleReadiness(readiness ReadinessCheck) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := readiness(r.Context()); err != nil {
			writeAPIError(
				w,
				http.StatusServiceUnavailable,
				"service_not_ready",
				"The server is not ready to receive traffic.",
				requestIDFromContext(r.Context()),
			)
			return
		}

		writeJSON(w, http.StatusOK, healthResponse{
			Status:  "ready",
			Service: "sprout-api",
		})
	}
}

func writeAPIError(w http.ResponseWriter, status int, code, message, requestID string) {
	writeJSON(w, status, errorEnvelope{
		Error: apiError{
			Code:      code,
			Message:   message,
			RequestID: requestID,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}

func normalizeServeError(err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return fmt.Errorf("serve HTTP: %w", err)
}
