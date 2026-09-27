package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

const requestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

var fallbackRequestSequence atomic.Uint64

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		ctx := withRequestID(r.Context(), requestID)

		w.Header().Set(requestIDHeader, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			response := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(response, r)

			logger.InfoContext(
				r.Context(),
				"request completed",
				"request_id", requestIDFromContext(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", response.Status(),
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
		})
	}
}

func recoverPanic(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recover() == nil {
					return
				}

				requestID := requestIDFromContext(r.Context())
				logger.ErrorContext(r.Context(), "panic recovered", "request_id", requestID)
				writeAPIError(
					w,
					http.StatusInternalServerError,
					"internal_error",
					"The server could not complete the request.",
					requestID,
				)
			}()

			next.ServeHTTP(w, r)
		})
	}
}

func newRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}

	return fmt.Sprintf("fallback-%d-%d", os.Getpid(), fallbackRequestSequence.Add(1))
}
