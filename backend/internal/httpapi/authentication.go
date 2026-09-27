package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/priyanshjhaa/Sprout/backend/internal/identity"
)

type PrincipalResolver interface {
	Resolve(context.Context, string) (identity.Principal, error)
}

type principalContextKey struct{}

func AuthenticationMiddleware(
	resolver PrincipalResolver, logger *slog.Logger, webOrigin string,
) func(http.Handler) http.Handler {
	verify := clerkhttp.WithHeaderAuthorization(
		clerkhttp.AuthorizedParty(func(party string) bool { return party == webOrigin }),
		clerkhttp.AuthorizationFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeAPIError(w, http.StatusUnauthorized, "authentication_required", "A valid session is required.", requestIDFromContext(r.Context()))
		})),
	)

	return func(next http.Handler) http.Handler {
		return verify(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := clerk.SessionClaimsFromContext(r.Context())
			if !ok || claims == nil || claims.Subject == "" {
				writeAPIError(w, http.StatusUnauthorized, "authentication_required", "A valid session is required.", requestIDFromContext(r.Context()))
				return
			}

			principal, err := resolver.Resolve(r.Context(), claims.Subject)
			if errors.Is(err, identity.ErrIncompleteProfile) {
				writeAPIError(w, http.StatusForbidden, "profile_incomplete", "A verified email is required.", requestIDFromContext(r.Context()))
				return
			}
			if errors.Is(err, identity.ErrNoWorkspace) {
				writeAPIError(w, http.StatusForbidden, "workspace_access_required", "You do not have access to a workspace.", requestIDFromContext(r.Context()))
				return
			}
			if err != nil {
				logger.ErrorContext(r.Context(), "identity lookup failed", "request_id", requestIDFromContext(r.Context()))
				writeAPIError(w, http.StatusServiceUnavailable, "identity_unavailable", "Your account could not be loaded.", requestIDFromContext(r.Context()))
				return
			}

			ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
			ctx = context.WithValue(ctx, identityContextKey{}, principal.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		}))
	}
}

func principalFromContext(ctx context.Context) identity.Principal {
	principal, _ := ctx.Value(principalContextKey{}).(identity.Principal)
	return principal
}
