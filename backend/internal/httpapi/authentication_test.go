package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/priyanshjhaa/Sprout/backend/internal/identity"
)

type stubPrincipalResolver struct {
	called  bool
	resolve func(context.Context, string) (identity.Principal, error)
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
	handler := AuthenticationMiddleware(resolver, discardLogger(), "http://localhost:3000")(
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
	handler := AuthenticationMiddleware(resolver, discardLogger(), "http://localhost:3000")(
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
