package apitoken

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type storedToken struct {
	Token
	userID, hash string
	revoked      bool
}

// memoryRepository mirrors the SQL repository's rules: unrevoked and unexpired
// tokens authenticate, ownership guards revocation.
type memoryRepository struct {
	tokens      []*storedToken
	now         func() time.Time
	hashLookups int
	touched     []string
	findErr     error
}

func (r *memoryRepository) Insert(_ context.Context, userID, name, hash, hint string, expiresAt time.Time) (Token, error) {
	token := &storedToken{Token: Token{ID: "t" + string(rune('a'+len(r.tokens))), Name: name, Hint: hint, CreatedAt: r.now(), ExpiresAt: expiresAt}, userID: userID, hash: hash}
	r.tokens = append(r.tokens, token)
	return token.Token, nil
}
func (r *memoryRepository) List(_ context.Context, userID string) ([]Token, error) {
	var out []Token
	for _, t := range r.tokens {
		if t.userID == userID && !t.revoked {
			out = append(out, t.Token)
		}
	}
	return out, nil
}
func (r *memoryRepository) CountActive(_ context.Context, userID string) (int, error) {
	count := 0
	for _, t := range r.tokens {
		if t.userID == userID && !t.revoked && t.ExpiresAt.After(r.now()) {
			count++
		}
	}
	return count, nil
}
func (r *memoryRepository) Revoke(_ context.Context, userID, id string) (bool, error) {
	for _, t := range r.tokens {
		if t.ID == id && t.userID == userID && !t.revoked {
			t.revoked = true
			return true, nil
		}
	}
	return false, nil
}
func (r *memoryRepository) FindByHash(_ context.Context, hash string) (string, string, error) {
	r.hashLookups++
	if r.findErr != nil {
		return "", "", r.findErr
	}
	for _, t := range r.tokens {
		if t.hash == hash && !t.revoked && t.ExpiresAt.After(r.now()) {
			return t.ID, t.userID, nil
		}
	}
	return "", "", ErrNotFound
}
func (r *memoryRepository) Touch(_ context.Context, id string) error {
	r.touched = append(r.touched, id)
	return nil
}

func newTestService() (*Service, *memoryRepository, *time.Time) {
	clock := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	repo := &memoryRepository{now: func() time.Time { return clock }}
	service := NewService(repo)
	service.now = func() time.Time { return clock }
	return service, repo, &clock
}

func TestCreateReturnsASecretOnceAndStoresOnlyItsHash(t *testing.T) {
	service, repo, _ := newTestService()
	token, secret, err := service.Create(context.Background(), "user-1", "  laptop  ", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(secret, Prefix) || len(secret) != len(Prefix)+52 || !wellFormed(secret) {
		t.Fatalf("secret shape: %q", secret)
	}
	if token.Name != "laptop" || token.Hint != secret[len(secret)-4:] || token.ExpiresAt.Sub(token.CreatedAt) != DefaultTTL {
		t.Fatalf("token = %+v", token)
	}
	stored := repo.tokens[0]
	if stored.hash == secret || len(stored.hash) != 64 || strings.Contains(stored.hash, secret) {
		t.Fatal("the secret must not be stored")
	}
	listed, _ := service.List(context.Background(), "user-1")
	if len(listed) != 1 || strings.Contains(listed[0].Name+listed[0].Hint+listed[0].ID, secret) {
		t.Fatalf("listing leaks the secret: %+v", listed)
	}
	// Every token is different.
	_, second, _ := service.Create(context.Background(), "user-1", "ci", time.Hour)
	if second == secret {
		t.Fatal("two tokens share a secret")
	}
}

func TestCreateValidation(t *testing.T) {
	service, _, _ := newTestService()
	for name, tc := range map[string]struct {
		name string
		ttl  time.Duration
	}{
		"empty name":    {"", 0},
		"blank name":    {"   ", 0},
		"long name":     {strings.Repeat("a", 81), 0},
		"control char":  {"bad\x00name", 0},
		"newline":       {"two\nlines", 0},
		"ttl too short": {"ok", time.Minute},
		"ttl too long":  {"ok", MaxTTL + time.Hour},
		"negative ttl":  {"ok", -time.Hour},
	} {
		if _, _, err := service.Create(context.Background(), "user-1", tc.name, tc.ttl); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, _, err := service.Create(context.Background(), "", "ok", 0); !errors.Is(err, ErrInvalid) {
		t.Errorf("no user accepted: %v", err)
	}
	if _, _, err := service.Create(context.Background(), "u", strings.Repeat("é", 80), MaxTTL); err != nil {
		t.Errorf("80 multi-byte characters and the maximum ttl are allowed: %v", err)
	}
}

func TestActiveTokenCap(t *testing.T) {
	service, _, clock := newTestService()
	ctx := context.Background()
	var first Token
	for i := 0; i < MaxActive; i++ {
		token, _, err := service.Create(ctx, "user-1", "t", time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = token
		}
	}
	if _, _, err := service.Create(ctx, "user-1", "one too many", 0); !errors.Is(err, ErrLimit) {
		t.Fatalf("err = %v, want limit", err)
	}
	if _, _, err := service.Create(ctx, "user-2", "other user", 0); err != nil {
		t.Fatalf("the cap is per user: %v", err)
	}
	// Revoking frees a slot, and so does expiry.
	if err := service.Revoke(ctx, "user-1", first.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Create(ctx, "user-1", "after revoke", 0); err != nil {
		t.Fatalf("revoke did not free a slot: %v", err)
	}
	*clock = clock.Add(2 * time.Hour)
	if _, _, err := service.Create(ctx, "user-1", "after expiry", 0); err != nil {
		t.Fatalf("expiry did not free slots: %v", err)
	}
}

func TestAuthenticate(t *testing.T) {
	service, repo, clock := newTestService()
	ctx := context.Background()
	token, secret, _ := service.Create(ctx, "user-1", "laptop", time.Hour)

	if user, err := service.Authenticate(ctx, secret); err != nil || user != "user-1" {
		t.Fatalf("valid token: %q %v", user, err)
	}
	if len(repo.touched) != 1 || repo.touched[0] != token.ID {
		t.Fatalf("last-used not recorded: %v", repo.touched)
	}

	// Malformed credentials are refused without a database lookup.
	lookups := repo.hashLookups
	for _, bad := range []string{"", "sprout_pat_", "sprout_pat_short", Prefix + strings.Repeat("A", 52), Prefix + strings.Repeat("1", 52), "bearer-from-clerk.jwt.token", secret + "x", secret[:len(secret)-1]} {
		if _, err := service.Authenticate(ctx, bad); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
	if repo.hashLookups != lookups {
		t.Fatal("malformed credentials reached the database")
	}

	// A well-formed but unknown secret is just unauthenticated.
	unknown := Prefix + strings.Repeat("a", 52)
	if _, err := service.Authenticate(ctx, unknown); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("unknown: %v", err)
	}

	// Revoked and expired tokens stop working.
	if err := service.Revoke(ctx, "user-1", token.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, secret); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("revoked token still works: %v", err)
	}
	_, secret2, _ := service.Create(ctx, "user-1", "short", time.Hour)
	*clock = clock.Add(61 * time.Minute)
	if _, err := service.Authenticate(ctx, secret2); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expired token still works: %v", err)
	}

	// An infrastructure failure is not "bad credentials": the caller must be able to tell.
	repo.findErr = errors.New("database down")
	_, secret3, _ := service.Create(ctx, "user-1", "third", time.Hour)
	if _, err := service.Authenticate(ctx, secret3); err == nil || errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("storage failure was reported as bad credentials: %v", err)
	}
}

func TestRevokeIsOwnerOnly(t *testing.T) {
	service, _, _ := newTestService()
	ctx := context.Background()
	token, secret, _ := service.Create(ctx, "owner", "mine", time.Hour)
	if err := service.Revoke(ctx, "someone-else", token.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another user revoked a token: %v", err)
	}
	if _, err := service.Authenticate(ctx, secret); err != nil {
		t.Fatalf("token should still work: %v", err)
	}
	if err := service.Revoke(ctx, "owner", token.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Revoke(ctx, "owner", token.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second revoke: %v", err)
	}
}

func TestRandomnessFailureIsNotSwallowed(t *testing.T) {
	service, _, _ := newTestService()
	service.random = bytes.NewReader([]byte("short"))
	if _, _, err := service.Create(context.Background(), "user-1", "x", 0); err == nil {
		t.Fatal("a failed random source produced a token")
	}
}

func TestIsToken(t *testing.T) {
	if !IsToken("sprout_pat_abc") || IsToken("eyJhbGciOi") || IsToken("") {
		t.Fatal("IsToken misclassifies credentials")
	}
}
