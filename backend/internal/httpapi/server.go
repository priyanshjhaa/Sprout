package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func NewServer(address string) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           NewRouter(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

func NewRouter() http.Handler {
	router := chi.NewRouter()
	router.Get("/health/live", handleLiveness)

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
