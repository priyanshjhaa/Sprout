package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/priyanshjhaa/Sprout/backend/internal/apitoken"
	"github.com/priyanshjhaa/Sprout/backend/internal/identity"
)

type stubPrincipalResolver struct {
	called      bool
	resolve     func(context.Context, string) (identity.Principal, error)
	resolveUser func(context.Context, string) (identity.Principal, error)
}

func (resolver *stubPrincipalResolver) ResolveUser(ctx context.Context, userID string) (identity.Principal, error) {
	resolver.called = true
	if resolver.resolveUser == nil {
		return identity.Principal{ID: userID}, nil
	}
	return resolver.resolveUser(ctx, userID)
}

func (resolver *stubPrincipalResolver) Resolve(ctx context.Context, subject string) (identity.Principal, error) {
	resolver.called = true
	return resolver.resolve(ctx, subject)
}

func TestAuthenticationRejectsMissingAndMalformedSessions(t *testing.T) {
	t.Parallel()
	resolver := &stubPrincipalResolver{resolve: func(context.Context, string) (identity.Principal, error) {
		t.Fatal("identity resolver must not run without verified claims")
		return identity.Principal{}, nil
	}}
	handler := AuthenticationMiddleware(resolver, nil, discardLogger(), "http://localhost:3000")(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("protected handler ran") }),
	)

	for _, authorization := range []string{"", "Bearer malformed-token"} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		request.Header.Set("Authorization", authorization)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("authorization %q: status = %d, want 401", authorization, response.Code)
		}
	}
	if resolver.called {
		t.Fatal("identity resolver ran without verified claims")
	}
}

func TestAuthenticationMapsVerifiedClaimsToLocalUser(t *testing.T) {
	t.Parallel()
	resolver := &stubPrincipalResolver{resolve: func(_ context.Context, subject string) (identity.Principal, error) {
		if subject != "user_clerk" {
			t.Fatalf("subject = %q, want user_clerk", subject)
		}
		return identity.Principal{ID: handlerTestUserID}, nil
	}}
	handler := AuthenticationMiddleware(resolver, nil, discardLogger(), "http://localhost:3000")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userIDFromContext(r.Context()) != handlerTestUserID {
				t.Fatalf("local user ID = %q", userIDFromContext(r.Context()))
			}
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request = request.WithContext(clerk.ContextWithSessionClaims(request.Context(), &clerk.SessionClaims{
		RegisteredClaims: clerk.RegisteredClaims{Subject: "user_clerk"},
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || !resolver.called {
		t.Fatalf("status = %d; resolver called = %v", response.Code, resolver.called)
	}
}

type stubTokens struct {
	calls int
	user  string
	err   error
}

func (tokens *stubTokens) Authenticate(_ context.Context, secret string) (string, error) {
	tokens.calls++
	return tokens.user, tokens.err
}

const testSecret = "sprout_pat_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func tokenRequest(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer "+testSecret)
	return request
}

func tokenHandlerForTest(resolver *stubPrincipalResolver, tokens TokenAuthenticator, next http.HandlerFunc) http.Handler {
	return requestIDMiddleware(AuthenticationMiddleware(resolver, tokens, discardLogger(), "http://localhost:3000")(next))
}

func TestTokenAuthenticationAllowsTheDeployRoutes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/me"},
		{http.MethodGet, "/api/v1/capabilities"},
		{http.MethodGet, "/api/v1/workspaces"},
		{http.MethodGet, "/api/v1/workspaces/acme"},
		{http.MethodGet, "/api/v1/workspaces/acme/applications"},
		{http.MethodPost, "/api/v1/workspaces/acme/applications/"},
		{http.MethodGet, "/api/v1/workspaces/acme/applications/app-1"},
		{http.MethodPost, "/api/v1/workspaces/acme/applications/app-1/deployments"},
		{http.MethodGet, "/api/v1/workspaces/acme/applications/app-1/deployments"},
		{http.MethodGet, "/api/v1/workspaces/acme/applications/app-1/deployments/d-1"},
		{http.MethodPost, "/api/v1/workspaces/acme/applications/app-1/deployments/d-1/cancel"},
	} {
		resolver := &stubPrincipalResolver{}
		tokens := &stubTokens{user: handlerTestUserID}
		ran := false
		handler := tokenHandlerForTest(resolver, tokens, func(w http.ResponseWriter, r *http.Request) {
			ran = true
			if userIDFromContext(r.Context()) != handlerTestUserID || !authenticatedByToken(r.Context()) {
				t.Errorf("%s %s: identity not established by the token", tc.method, tc.path)
			}
		})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, tokenRequest(tc.method, tc.path))
		if !ran || response.Code != http.StatusOK {
			t.Errorf("%s %s: ran=%v status=%d", tc.method, tc.path, ran, response.Code)
		}
	}
}

func TestTokenAuthenticationRefusesEverythingElse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/tokens"},
		{http.MethodPost, "/api/v1/tokens"},
		{http.MethodDelete, "/api/v1/tokens/t-1"},
		{http.MethodGet, "/api/v1/workspaces/acme/members"},
		{http.MethodPatch, "/api/v1/workspaces/acme/members/u-1"},
		{http.MethodDelete, "/api/v1/workspaces/acme/members/u-1"},
		{http.MethodGet, "/api/v1/workspaces/acme/invitations"},
		{http.MethodPost, "/api/v1/workspaces/acme/invitations"},
		{http.MethodPost, "/api/v1/invitations/accept"},
		{http.MethodPatch, "/api/v1/workspaces/acme/applications/app-1"},
		{http.MethodDelete, "/api/v1/workspaces/acme/applications/app-1"},
		{http.MethodGet, "/api/v1/workspaces/acme/applications/app-1/access"},
		{http.MethodPatch, "/api/v1/workspaces/acme/applications/app-1/access"},
		{http.MethodPut, "/api/v1/workspaces/acme/applications/app-1/access/u-1"},
		{http.MethodDelete, "/api/v1/workspaces/acme/applications/app-1/access/u-1"},
		{http.MethodPost, "/api/v1/workspaces/acme/applications/app-1/deployment-simulations"},
		// The progress stream needs a browser session; token clients poll the deployment.
		{http.MethodGet, "/api/v1/workspaces/acme/applications/app-1/deployments/d-1/events"},
		{http.MethodPut, "/api/v1/workspaces/acme/applications/app-1/deployments"},
		{http.MethodDelete, "/api/v1/workspaces/acme/applications/app-1/deployments/d-1"},
		{http.MethodPost, "/api/v1/workspaces"},
		{http.MethodPost, "/api/v1/me"},
		// Segment games: extra, missing or traversing segments never match an allowed route.
		{http.MethodGet, "/api/v1/workspaces/acme/applications/app-1/deployments/d-1/events/x"},
		{http.MethodGet, "/api/v1/workspaces/acme/applications/../../members"},
		{http.MethodGet, "/api/v1/workspaces//members"},
		{http.MethodGet, "/api/v1/tokens/"},
		{http.MethodGet, "/api/v2/me"},
		{http.MethodGet, "/health/ready/../../api/v1/tokens"},
	} {
		resolver := &stubPrincipalResolver{}
		handler := tokenHandlerForTest(resolver, &stubTokens{user: handlerTestUserID}, func(http.ResponseWriter, *http.Request) {
			t.Errorf("%s %s reached a handler with a token", tc.method, tc.path)
		})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, tokenRequest(tc.method, tc.path))
		if response.Code != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want 403", tc.method, tc.path, response.Code)
			continue
		}
		assertErrorCode(t, response, "token_not_permitted")
		if resolver.called {
			t.Errorf("%s %s: identity was loaded for a refused request", tc.method, tc.path)
		}
	}
}

func TestTokenAuthenticationFailures(t *testing.T) {
	t.Parallel()
	next := func(http.ResponseWriter, *http.Request) { t.Error("handler ran") }

	// An invalid token is a 401 even on a route tokens may not use: the
	// allowlist is not revealed to someone without a working token.
	for _, path := range []string{"/api/v1/me", "/api/v1/tokens"} {
		response := httptest.NewRecorder()
		tokenHandlerForTest(&stubPrincipalResolver{}, &stubTokens{err: apitoken.ErrUnauthenticated}, next).ServeHTTP(response, tokenRequest(http.MethodGet, path))
		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", path, response.Code)
		}
	}

	// Infrastructure failure is not reported as bad credentials.
	response := httptest.NewRecorder()
	tokenHandlerForTest(&stubPrincipalResolver{}, &stubTokens{err: errors.New("database down")}, next).ServeHTTP(response, tokenRequest(http.MethodGet, "/api/v1/me"))
	if response.Code != http.StatusServiceUnavailable || response.Body.String() == "" {
		t.Errorf("storage failure: status = %d", response.Code)
	}

	// No token verifier configured: refuse rather than fall through to a session check.
	response = httptest.NewRecorder()
	tokenHandlerForTest(&stubPrincipalResolver{}, nil, next).ServeHTTP(response, tokenRequest(http.MethodGet, "/api/v1/me"))
	if response.Code != http.StatusUnauthorized {
		t.Errorf("no verifier: status = %d", response.Code)
	}

	// An account without a workspace is refused like a session without one.
	response = httptest.NewRecorder()
	resolver := &stubPrincipalResolver{resolveUser: func(context.Context, string) (identity.Principal, error) {
		return identity.Principal{}, identity.ErrNoWorkspace
	}}
	tokenHandlerForTest(resolver, &stubTokens{user: handlerTestUserID}, next).ServeHTTP(response, tokenRequest(http.MethodGet, "/api/v1/me"))
	if response.Code != http.StatusForbidden {
		t.Errorf("no workspace: status = %d", response.Code)
	}
	assertErrorCode(t, response, "workspace_access_required")
}

// A lowercase or otherwise odd scheme is not a token: it falls to the session
// path, which rejects it. A token cannot be smuggled past the allowlist.
func TestOnlyTheExactBearerPrefixSelectsTokenAuthentication(t *testing.T) {
	t.Parallel()
	tokens := &stubTokens{user: handlerTestUserID}
	handler := tokenHandlerForTest(&stubPrincipalResolver{}, tokens, func(http.ResponseWriter, *http.Request) { t.Error("handler ran") })
	for _, header := range []string{"bearer " + testSecret, "Token " + testSecret, testSecret, "Bearer  " + testSecret, "Bearer x" + testSecret} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/tokens", nil)
		request.Header.Set("Authorization", header)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Errorf("%q: status = %d, want 401 from the session path", header, response.Code)
		}
	}
	if tokens.calls != 0 {
		t.Fatalf("token verifier ran %d times for non-token credentials", tokens.calls)
	}
}
