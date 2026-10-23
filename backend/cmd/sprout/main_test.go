package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/priyanshjhaa/Sprout/backend/internal/apitoken"
)

// Fake tokens in the server's shape: the prefix and 52 base32 characters.
var (
	goodToken    = "sprout_pat_" + strings.Repeat("a", 52)
	revokedToken = "sprout_pat_" + strings.Repeat("b", 52)
)

func TestTokenPrefixMatchesServer(t *testing.T) {
	if tokenPrefix != apitoken.Prefix {
		t.Fatalf("tokenPrefix %q drifted from apitoken.Prefix %q", tokenPrefix, apitoken.Prefix)
	}
}

// fakeAPI answers /api/v1/me for goodToken and refuses anything else.
func fakeAPI(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer "+goodToken {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"authentication_required","message":"The access token is invalid, expired or revoked.","requestId":"r1"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"u1","email":"ada@example.com","displayName":"Ada","workspace":{"id":"w1","slug":"ada-team","name":"Ada's team"}}`))
	}))
	t.Cleanup(server.Close)
	return server
}

type harness struct {
	env            environment
	stdout, stderr *bytes.Buffer
	vars           map[string]string
}

func newHarness(t *testing.T, secret string) *harness {
	h := &harness{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}, vars: map[string]string{}}
	h.env = environment{
		stdout: h.stdout, stderr: h.stderr,
		getenv:    func(name string) string { return h.vars[name] },
		configDir: filepath.Join(t.TempDir(), "sprout"),
		readSecret: func(string) (string, error) {
			return readLine(strings.NewReader(secret))
		},
	}
	return h
}

func (h *harness) run(args ...string) int {
	h.stdout.Reset()
	h.stderr.Reset()
	return run(context.Background(), args, h.env)
}

func TestLoginVerifiesThenStoresPrivately(t *testing.T) {
	api := fakeAPI(t)
	h := newHarness(t, goodToken+"\n")
	if code := h.run("login", "--api", api.URL); code != 0 {
		t.Fatalf("login exited %d: %s", code, h.stderr)
	}
	if !strings.Contains(h.stdout.String(), "ada@example.com") || !strings.Contains(h.stdout.String(), "ada-team") {
		t.Fatalf("login output %q does not name the account", h.stdout)
	}
	info, err := os.Stat(filepath.Join(h.env.configDir, credentialsFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credentials mode %v, want 0600", info.Mode().Perm())
	}
	if code := h.run("whoami"); code != 0 {
		t.Fatalf("whoami after login exited %d: %s", code, h.stderr)
	}
	if !strings.Contains(h.stdout.String(), "Ada <ada@example.com>") {
		t.Fatalf("whoami output %q", h.stdout)
	}
	if strings.Contains(h.stdout.String()+h.stderr.String(), goodToken) {
		t.Fatal("the token secret must never be printed")
	}
}

func TestLoginRefusesBadTokensWithoutStoring(t *testing.T) {
	api := fakeAPI(t)
	for name, secret := range map[string]string{
		"revoked":    revokedToken + "\n",
		"wrong kind": "ghp_notours\n",
		"empty":      "",
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, secret)
			if code := h.run("login", "--api", api.URL); code != 2 {
				t.Fatalf("login exited %d, want 2: %s", code, h.stderr)
			}
			if _, err := os.Stat(filepath.Join(h.env.configDir, credentialsFile)); !os.IsNotExist(err) {
				t.Fatal("a refused token was stored")
			}
		})
	}
}

func TestLoginExplainsAMangledPaste(t *testing.T) {
	api := fakeAPI(t)
	for name, tc := range map[string]struct{ input, says string }{
		"pasted twice":    {goodToken + goodToken + "\n", "pasted twice"},
		"cut short":       {goodToken[:40] + "\n", "not a complete"},
		"stray character": {goodToken + "x\n", "not a complete"},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, tc.input)
			if code := h.run("login", "--api", api.URL); code != 2 || !strings.Contains(h.stderr.String(), tc.says) {
				t.Fatalf("exit %d, stderr %q", code, h.stderr)
			}
		})
	}
	// Terminals that use bracketed paste wrap the text in markers; they are not part of the token.
	h := newHarness(t, "\x1b[200~"+goodToken+"\x1b[201~\n")
	if code := h.run("login", "--api", api.URL); code != 0 {
		t.Fatalf("a token wrapped in paste markers was refused: %s", h.stderr)
	}
}

func TestLoginRefusesTokenAsArgument(t *testing.T) {
	h := newHarness(t, goodToken)
	if code := h.run("login", goodToken); code != 2 || !strings.Contains(h.stderr.String(), "shell history") {
		t.Fatalf("exit %d, stderr %q", code, h.stderr)
	}
}

func TestLoginRefusesPlainHTTPOffThisMachine(t *testing.T) {
	h := newHarness(t, goodToken)
	if code := h.run("login", "--api", "http://sprout.example.com"); code != 2 {
		t.Fatalf("exit %d, stderr %q", code, h.stderr)
	}
}

func TestEnvironmentTokenWins(t *testing.T) {
	api := fakeAPI(t)
	h := newHarness(t, "")
	h.vars["SPROUT_API_URL"] = api.URL
	h.vars["SPROUT_TOKEN"] = goodToken
	if code := h.run("whoami"); code != 0 {
		t.Fatalf("whoami exited %d: %s", code, h.stderr)
	}
	if !strings.Contains(h.stdout.String(), "from SPROUT_TOKEN") {
		t.Fatalf("whoami output %q does not name the token source", h.stdout)
	}
}

func TestStoredTokenStaysWithItsAPI(t *testing.T) {
	api := fakeAPI(t)
	other := fakeAPI(t)
	h := newHarness(t, goodToken)
	if code := h.run("login", "--api", api.URL); code != 0 {
		t.Fatalf("login exited %d: %s", code, h.stderr)
	}
	h.vars["SPROUT_API_URL"] = other.URL
	if code := h.run("whoami"); code == 0 || !strings.Contains(h.stderr.String(), "logged in to "+api.URL) {
		t.Fatalf("a stored token was sent to another API: exit %d, stderr %q", code, h.stderr)
	}
}

func TestReadableCredentialsAreRefused(t *testing.T) {
	api := fakeAPI(t)
	h := newHarness(t, goodToken)
	if code := h.run("login", "--api", api.URL); code != 0 {
		t.Fatalf("login exited %d: %s", code, h.stderr)
	}
	if err := os.Chmod(filepath.Join(h.env.configDir, credentialsFile), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := h.run("whoami"); code == 0 || !strings.Contains(h.stderr.String(), "chmod 600") {
		t.Fatalf("exit %d, stderr %q", code, h.stderr)
	}
}

func TestWhoamiWithoutLogin(t *testing.T) {
	h := newHarness(t, "")
	if code := h.run("whoami"); code != 2 || !strings.Contains(h.stderr.String(), "sprout login") {
		t.Fatalf("exit %d, stderr %q", code, h.stderr)
	}
}

func TestRevokedStoredTokenExplainsWhatToDo(t *testing.T) {
	api := fakeAPI(t)
	h := newHarness(t, "")
	if err := writeCredentials(h.env.configDir, credentials{APIURL: api.URL, Token: revokedToken}); err != nil {
		t.Fatal(err)
	}
	if code := h.run("whoami"); code != 2 || !strings.Contains(h.stderr.String(), "was refused") {
		t.Fatalf("exit %d, stderr %q", code, h.stderr)
	}
}

func TestLogout(t *testing.T) {
	api := fakeAPI(t)
	h := newHarness(t, goodToken)
	if code := h.run("login", "--api", api.URL); code != 0 {
		t.Fatalf("login exited %d: %s", code, h.stderr)
	}
	if code := h.run("logout"); code != 0 || !strings.Contains(h.stdout.String(), "revoke") {
		t.Fatalf("logout exit %d, stdout %q", code, h.stdout)
	}
	if code := h.run("logout"); code != 0 || !strings.Contains(h.stdout.String(), "Not logged in") {
		t.Fatalf("second logout exit %d, stdout %q", code, h.stdout)
	}
}

func TestUnknownCommandAndNoArguments(t *testing.T) {
	h := newHarness(t, "")
	if code := h.run(); code != 2 || !strings.Contains(h.stderr.String(), "Usage") {
		t.Fatalf("no args: exit %d", code)
	}
	if code := h.run("deploi"); code != 2 || !strings.Contains(h.stderr.String(), `unknown command "deploi"`) {
		t.Fatalf("unknown: exit %d, stderr %q", code, h.stderr)
	}
}
