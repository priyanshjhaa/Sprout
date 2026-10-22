package httpapi

import (
	"net/http"
	"slices"
)

const allowedRequestHeaders = "Content-Type, Authorization, Last-Event-ID"
const allowedRequestMethods = "GET, POST, PATCH, PUT, DELETE, OPTIONS"

// corsMiddleware answers only the configured origins, echoing the one that
// matched; it never allows an arbitrary origin.
func corsMiddleware(webOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(webOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Headers", allowedRequestHeaders)
				w.Header().Set("Access-Control-Allow-Methods", allowedRequestMethods)
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
