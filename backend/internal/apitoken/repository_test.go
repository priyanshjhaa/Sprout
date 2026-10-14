package apitoken

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
)

// Everything runs in a transaction that is rolled back, so the developer's
// database is never changed.
func TestTokenLifecycleAgainstPostgreSQL(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var owner, other string
	for name, target := range map[string]*string{"owner": &owner, "other": &other} {
		if err := tx.QueryRow(ctx, `INSERT INTO users(auth_provider,auth_subject,email,display_name) VALUES('test',$1::text,$1::text||'@example.test',$2) RETURNING id::text`,
			name+"-"+suffix, name).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	service := NewService(NewSQLRepository(dbgen.New(tx)))

	token, secret, err := service.Create(ctx, owner, "laptop", 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if token.ExpiresAt.Before(time.Now().Add(time.Hour)) || token.LastUsedAt != nil {
		t.Fatalf("token = %+v", token)
	}

	// The database holds the hash, never the secret.
	var stored string
	if err := tx.QueryRow(ctx, `SELECT token_hash FROM api_tokens WHERE id = $1::uuid`, token.ID).Scan(&stored); err != nil || stored == secret || len(stored) != 64 {
		t.Fatalf("stored = %q (%v)", stored, err)
	}
	var leaked int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM api_tokens WHERE name = $1 OR token_hash = $1 OR hint = $1`, secret).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("secret found in the table: %d %v", leaked, err)
	}

	// Authentication works, stamps last-used, and the stamp is throttled.
	if user, err := service.Authenticate(ctx, secret); err != nil || user != owner {
		t.Fatalf("authenticate: %q %v", user, err)
	}
	listed, err := service.List(ctx, owner)
	if err != nil || len(listed) != 1 || listed[0].LastUsedAt == nil {
		t.Fatalf("list: %+v %v", listed, err)
	}
	first := *listed[0].LastUsedAt
	if _, err := service.Authenticate(ctx, secret); err != nil {
		t.Fatal(err)
	}
	if listed, _ = service.List(ctx, owner); !listed[0].LastUsedAt.Equal(first) {
		t.Fatal("last-used was rewritten within a minute")
	}

	// Another user sees and can do nothing with it.
	if others, _ := service.List(ctx, other); len(others) != 0 {
		t.Fatalf("another user lists %d tokens", len(others))
	}
	if err := service.Revoke(ctx, other, token.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another user revoked it: %v", err)
	}
	if err := service.Revoke(ctx, owner, "not-a-uuid"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("garbage id: %v", err)
	}

	// Expiry is enforced by the lookup itself.
	if _, err := tx.Exec(ctx, `UPDATE api_tokens SET expires_at = now() - interval '1 second', created_at = now() - interval '1 hour' WHERE id = $1::uuid`, token.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, secret); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expired token authenticated: %v", err)
	}
	if listed, _ = service.List(ctx, owner); len(listed) != 1 {
		t.Fatalf("an expired token should stay listed so it can be removed: %d", len(listed))
	}
	if active, _ := NewSQLRepository(dbgen.New(tx)).CountActive(ctx, owner); active != 0 {
		t.Fatalf("expired token counted as active: %d", active)
	}

	// Revocation removes it from the list and from authentication.
	_, secret2, err := service.Create(ctx, owner, "ci", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	all, _ := service.List(ctx, owner)
	var ciID string
	for _, candidate := range all {
		if candidate.Name == "ci" {
			ciID = candidate.ID
		}
	}
	if err := service.Revoke(ctx, owner, ciID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, secret2); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("revoked token authenticated: %v", err)
	}
	if remaining, _ := service.List(ctx, owner); len(remaining) != 1 {
		t.Fatalf("revoked token still listed: %d", len(remaining))
	}
}
