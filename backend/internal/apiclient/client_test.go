package apiclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeBaseURL(t *testing.T) {
	accepted := map[string]string{
		"http://127.0.0.1:8080":   "http://127.0.0.1:8080",
		"http://127.0.0.1:8080/":  "http://127.0.0.1:8080",
		"http://LOCALHOST:8080":   "http://localhost:8080",
		"http://[::1]:8080":       "http://[::1]:8080",
		"https://api.example.com": "https://api.example.com",
	}
	for raw, want := range accepted {
		if got, err := NormalizeBaseURL(raw); err != nil || got != want {
			t.Errorf("NormalizeBaseURL(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{
		"http://api.example.com", // a token in clear text over the network
		"http://10.0.0.5:8080",
		"ftp://127.0.0.1",
		"127.0.0.1:8080",
		"https://user:pass@api.example.com",
		"https://api.example.com/v1",
		"https://api.example.com?x=1",
		"",
	} {
		if _, err := NormalizeBaseURL(raw); err == nil {
			t.Errorf("NormalizeBaseURL(%q) accepted an address it must refuse", raw)
		}
	}
}

func TestMeSendsTokenAndDecodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/me" || r.Header.Get("Authorization") != "Bearer sprout_pat_abc" || r.Header.Get("User-Agent") != "sprout-test" {
			w.WriteHeader(http.StatusTeapot)
			return
		}
		_, _ = w.Write([]byte(`{"id":"u1","email":"a@example.com","displayName":"A","workspace":{"id":"w1","slug":"team","name":"Team"},"later":true}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "sprout_pat_abc", "sprout-test")
	if err != nil {
		t.Fatal(err)
	}
	me, err := client.Me(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if me.Email != "a@example.com" || me.Workspace.Slug != "team" {
		t.Fatalf("unexpected identity %+v", me)
	}
}

func TestErrorEnvelopeBecomesTypedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"authentication_required","message":"The access token is invalid, expired or revoked.","requestId":"r1"}}`))
	}))
	defer server.Close()

	client, _ := New(server.URL, "sprout_pat_bad", "sprout-test")
	_, err := client.Me(context.Background())
	if !HasCode(err, "authentication_required") {
		t.Fatalf("expected authentication_required, got %v", err)
	}
	if apiErr := err.(*Error); apiErr.Status != http.StatusUnauthorized || apiErr.RequestID != "r1" {
		t.Fatalf("unexpected error fields %+v", apiErr)
	}
}

func TestUnreadableResponseIsUnexpected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>proxy error</html>"))
	}))
	defer server.Close()

	client, _ := New(server.URL, "sprout_pat_abc", "sprout-test")
	if _, err := client.Me(context.Background()); !HasCode(err, "unexpected_response") {
		t.Fatalf("expected unexpected_response, got %v", err)
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	var leaked bool
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = true
	}))
	defer elsewhere.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/api/v1/me", http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	client, _ := New(server.URL, "sprout_pat_abc", "sprout-test")
	if _, err := client.Me(context.Background()); err == nil {
		t.Fatal("a redirect must not be treated as success")
	}
	if leaked {
		t.Fatal("the client followed a redirect and sent the request elsewhere")
	}
}
