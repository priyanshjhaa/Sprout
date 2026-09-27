package httpapi

import "net/http"

const allowedRequestHeaders = "Content-Type, Authorization"
const allowedRequestMethods = "GET, POST, PATCH, OPTIONS"

func corsMiddleware(webOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")
			origin := r.Header.Get("Origin")
			if origin == webOrigin {
				w.Header().Set("Access-Control-Allow-Origin", webOrigin)
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
