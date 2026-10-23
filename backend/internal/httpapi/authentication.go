package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/priyanshjhaa/Sprout/backend/internal/apitoken"
	"github.com/priyanshjhaa/Sprout/backend/internal/identity"
)

type PrincipalResolver interface {
	Resolve(context.Context, string) (identity.Principal, error)
	ResolveUser(context.Context, string) (identity.Principal, error)
}

// TokenAuthenticator verifies a personal access token and names its owner.
type TokenAuthenticator interface {
	Authenticate(ctx context.Context, secret string) (userID string, err error)
}

type principalContextKey struct{}
type viaTokenContextKey struct{}

// authenticatedByToken reports that the request used a personal access token
// rather than a browser session. Endpoints that mint or manage credentials use
// it to demand a real session.
func authenticatedByToken(ctx context.Context) bool {
	value, _ := ctx.Value(viaTokenContextKey{}).(bool)
	return value
}

// AuthenticationMiddleware accepts two credentials on the same header: a
// Sprout personal access token (recognised by its prefix) or a Clerk session.
// A token is restricted to the routes in tokenRoutes.
func AuthenticationMiddleware(
	resolver PrincipalResolver, tokens TokenAuthenticator, logger *slog.Logger, webOrigin string,
) func(http.Handler) http.Handler {
	session := sessionAuthentication(resolver, logger, webOrigin)
	token := tokenAuthentication(resolver, tokens, logger)
	return func(next http.Handler) http.Handler {
		viaSession, viaToken := session(next), token(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if credential, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && apitoken.IsToken(credential) {
				viaToken.ServeHTTP(w, r)
				return
			}
			viaSession.ServeHTTP(w, r)
		})
	}
}

// tokenRoutes is what a personal access token may call: read the account, and
// create applications and deployments. It is an allowlist, so a route added
// later is closed to tokens until someone decides otherwise. Notably absent
// are tokens themselves, team, invitations, access grants and lifecycle
// changes. Patterns match the escaped URL path, the form the router uses, so
// an encoded slash cannot be read as a separator by one and not the other.
var tokenRoutes = []struct {
	methods []string
	path    *regexp.Regexp
}{
	{[]string{http.MethodGet}, regexp.MustCompile(`^/api/v1/(me|capabilities|workspaces)/?$`)},
	{[]string{http.MethodGet}, regexp.MustCompile(`^/api/v1/workspaces/[^/]+/?$`)},
	{[]string{http.MethodGet, http.MethodPost}, regexp.MustCompile(`^/api/v1/workspaces/[^/]+/applications/?$`)},
	{[]string{http.MethodGet}, regexp.MustCompile(`^/api/v1/workspaces/[^/]+/applications/[^/]+/?$`)},
	{[]string{http.MethodGet, http.MethodPost}, regexp.MustCompile(`^/api/v1/workspaces/[^/]+/applications/[^/]+/deployments/?$`)},
	{[]string{http.MethodGet}, regexp.MustCompile(`^/api/v1/workspaces/[^/]+/applications/[^/]+/deployments/[^/]+/?$`)},
	{[]string{http.MethodPost}, regexp.MustCompile(`^/api/v1/workspaces/[^/]+/applications/[^/]+/deployments/[^/]+/cancel/?$`)},
}

func tokenMayCall(method, escapedPath string) bool {
	for _, route := range tokenRoutes {
		if route.path.MatchString(escapedPath) {
			for _, allowed := range route.methods {
				if allowed == method {
					return true
				}
			}
		}
	}
	return false
}

func tokenAuthentication(resolver PrincipalResolver, tokens TokenAuthenticator, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := requestIDFromContext(r.Context())
			secret, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if tokens == nil {
				writeAPIError(w, http.StatusUnauthorized, "authentication_required", "A valid session is required.", requestID)
				return
			}
			userID, err := tokens.Authenticate(r.Context(), secret)
			if errors.Is(err, apitoken.ErrUnauthenticated) {
				writeAPIError(w, http.StatusUnauthorized, "authentication_required", "The access token is invalid, expired or revoked.", requestID)
				return
			}
			if err != nil {
				logger.ErrorContext(r.Context(), "token lookup failed", "request_id", requestID)
				writeAPIError(w, http.StatusServiceUnavailable, "identity_unavailable", "Your account could not be loaded.", requestID)
				return
			}
			// Authenticate first, so an invalid token is always a 401 and the
			// allowlist is not revealed to anyone without a working token.
			if !tokenMayCall(r.Method, r.URL.EscapedPath()) {
				writeAPIError(w, http.StatusForbidden, "token_not_permitted", "Access tokens cannot be used for this request. Use the dashboard.", requestID)
				return
			}
			principal, err := resolver.ResolveUser(r.Context(), userID)
			if errors.Is(err, identity.ErrNoWorkspace) {
				writeAPIError(w, http.StatusForbidden, "workspace_access_required", "You do not have access to a workspace.", requestID)
				return
			}
			if err != nil {
				logger.ErrorContext(r.Context(), "identity lookup failed", "request_id", requestID)
				writeAPIError(w, http.StatusServiceUnavailable, "identity_unavailable", "Your account could not be loaded.", requestID)
				return
			}
			ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
			ctx = context.WithValue(ctx, identityContextKey{}, principal.ID)
			ctx = context.WithValue(ctx, viaTokenContextKey{}, true)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func sessionAuthentication(
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
