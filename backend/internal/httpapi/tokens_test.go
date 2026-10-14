package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/priyanshjhaa/Sprout/backend/internal/apitoken"
)

type stubTokenService struct {
	created    []time.Duration
	createErr  error
	listed     []apitoken.Token
	revokeErr  error
	revokedID  string
	secret     string
	serviceHit int
}

func (s *stubTokenService) Create(_ context.Context, userID, name string, ttl time.Duration) (apitoken.Token, string, error) {
	s.serviceHit++
	s.created = append(s.created, ttl)
	if s.createErr != nil {
		return apitoken.Token{}, "", s.createErr
	}
	now := time.Now()
	return apitoken.Token{ID: "tok-1", Name: name, Hint: "wxyz", CreatedAt: now, ExpiresAt: now.Add(90 * 24 * time.Hour)}, s.secret, nil
}
func (s *stubTokenService) List(context.Context, string) ([]apitoken.Token, error) {
	s.serviceHit++
	return s.listed, nil
}
func (s *stubTokenService) Revoke(_ context.Context, _, id string) error {
	s.serviceHit++
	s.revokedID = id
	return s.revokeErr
}

func tokenRouter(service TokenService, viaToken bool) *chi.Mux {
	router := NewRouter(discardLogger(), AlwaysReady, "")
	router.Route("/api/v1", func(api chi.Router) {
		api.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), identityContextKey{}, handlerTestUserID)
				ctx = context.WithValue(ctx, viaTokenContextKey{}, viaToken)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		RegisterTokenRoutes(api, service, discardLogger())
	})
	return router
}

func send(router http.Handler, method, target, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestCreateTokenReturnsTheSecretOnceAndNeverCaches(t *testing.T) {
	t.Parallel()
	service := &stubTokenService{secret: "sprout_pat_thesecret"}
	response := send(tokenRouter(service, false), http.MethodPost, "/api/v1/tokens", `{"name":"laptop","expiresInDays":30}`)
	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status %d cache %q", response.Code, response.Header().Get("Cache-Control"))
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["token"] != "sprout_pat_thesecret" || body["name"] != "laptop" || body["hint"] != "wxyz" || body["expired"] != false {
		t.Fatalf("body = %v", body)
	}
	if len(service.created) != 1 || service.created[0] != 30*24*time.Hour {
		t.Fatalf("ttl = %v", service.created)
	}
	// Omitting expiresInDays asks for the service's default.
	send(tokenRouter(service, false), http.MethodPost, "/api/v1/tokens", `{"name":"ci"}`)
	if service.created[1] != 0 {
		t.Fatalf("default ttl = %v", service.created[1])
	}
}

func TestListNeverContainsSecrets(t *testing.T) {
	t.Parallel()
	past := time.Now().Add(-time.Hour)
	service := &stubTokenService{secret: "sprout_pat_thesecret", listed: []apitoken.Token{
		{ID: "a", Name: "live", Hint: "1111", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)},
		{ID: "b", Name: "old", Hint: "2222", CreatedAt: past.Add(-time.Hour), ExpiresAt: past},
	}}
	response := send(tokenRouter(service, false), http.MethodGet, "/api/v1/tokens", "")
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "sprout_pat_") || strings.Contains(response.Body.String(), `"token"`) {
		t.Fatalf("list: %d %s", response.Code, response.Body.String())
	}
	var body struct {
		Tokens []struct {
			ID      string `json:"id"`
			Expired bool   `json:"expired"`
		} `json:"tokens"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &body)
	if len(body.Tokens) != 2 || body.Tokens[0].Expired || !body.Tokens[1].Expired {
		t.Fatalf("expiry flags: %+v", body.Tokens)
	}
	// An empty list is [], not null.
	empty := send(tokenRouter(&stubTokenService{}, false), http.MethodGet, "/api/v1/tokens", "")
	if !strings.Contains(empty.Body.String(), `"tokens":[]`) {
		t.Fatalf("empty list: %s", empty.Body.String())
	}
}

func TestCreateTokenValidationAndErrors(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		body string
		err  error
		code int
	}{
		"not json":        {`nope`, nil, http.StatusBadRequest},
		"unknown field":   {`{"name":"x","scope":"admin"}`, nil, http.StatusBadRequest},
		"zero days":       {`{"name":"x","expiresInDays":0}`, nil, http.StatusBadRequest},
		"negative days":   {`{"name":"x","expiresInDays":-5}`, nil, http.StatusBadRequest},
		"too many days":   {`{"name":"x","expiresInDays":366}`, nil, http.StatusBadRequest},
		"service rejects": {`{"name":""}`, apitoken.ErrInvalid, http.StatusBadRequest},
		"limit":           {`{"name":"x"}`, apitoken.ErrLimit, http.StatusConflict},
		"internal":        {`{"name":"x"}`, context.DeadlineExceeded, http.StatusInternalServerError},
	} {
		service := &stubTokenService{createErr: tc.err}
		response := send(tokenRouter(service, false), http.MethodPost, "/api/v1/tokens", tc.body)
		if response.Code != tc.code {
			t.Errorf("%s: status %d, want %d (%s)", name, response.Code, tc.code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "deadline") {
			t.Errorf("%s: internal error text leaked", name)
		}
	}
}

func TestRevokeToken(t *testing.T) {
	t.Parallel()
	service := &stubTokenService{}
	if response := send(tokenRouter(service, false), http.MethodDelete, "/api/v1/tokens/tok-9", ""); response.Code != http.StatusNoContent || service.revokedID != "tok-9" {
		t.Fatalf("revoke: %d %q", response.Code, service.revokedID)
	}
	missing := &stubTokenService{revokeErr: apitoken.ErrNotFound}
	response := send(tokenRouter(missing, false), http.MethodDelete, "/api/v1/tokens/nope", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing: %d", response.Code)
	}
	assertErrorCode(t, response, "token_not_found")
}

// Behind the allowlist, credential management refuses tokens on its own.
func TestTokenManagementRequiresASession(t *testing.T) {
	t.Parallel()
	service := &stubTokenService{secret: "sprout_pat_thesecret"}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/tokens", `{"name":"escalate"}`},
		{http.MethodGet, "/api/v1/tokens", ""},
		{http.MethodDelete, "/api/v1/tokens/tok-1", ""},
	} {
		response := send(tokenRouter(service, true), tc.method, tc.path, tc.body)
		if response.Code != http.StatusForbidden {
			t.Errorf("%s %s with a token: %d", tc.method, tc.path, response.Code)
			continue
		}
		assertErrorCode(t, response, "session_required")
	}
	if service.serviceHit != 0 {
		t.Fatal("the token service was reached by a token-authenticated request")
	}
}
