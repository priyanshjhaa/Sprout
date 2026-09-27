package identity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/priyanshjhaa/Sprout/backend/internal/database"
)

func TestIdentityProvisioningAndWorkspaceRevocationAgainstPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer pool.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	subject := "user_test_" + suffix
	repository := NewSQLRepository(pool)
	provider := &stubProvider{profile: Profile{
		Email: "identity-" + suffix + "@example.test", DisplayName: "Test user",
	}}
	service := NewService(provider, repository)
	defer func() {
		user, err := repository.queries.GetUserByIdentity(context.Background(), subject)
		if err != nil {
			return
		}
		_, _ = pool.Exec(context.Background(), "DELETE FROM workspaces WHERE created_by = $1", user.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
	}()

	principal, err := service.Resolve(ctx, subject)
	if err != nil {
		t.Fatalf("first Resolve() error = %v", err)
	}
	if principal.Workspace.Slug == "" || principal.Workspace.Name != "My workspace" {
		t.Fatalf("created workspace = %#v", principal.Workspace)
	}
	provider.called = false
	second, err := service.Resolve(ctx, subject)
	if err != nil || second.ID != principal.ID || provider.called {
		t.Fatalf("second Resolve() = %#v, %v; fetched profile = %v", second, err, provider.called)
	}
	if _, err := service.GetWorkspace(ctx, principal.Workspace.Slug, principal.ID); err != nil {
		t.Fatalf("owner GetWorkspace() error = %v", err)
	}
	if _, err := service.GetWorkspace(ctx, "acme", principal.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-workspace access error = %v, want ErrNotFound", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM workspace_memberships WHERE user_id = $1", principal.ID); err != nil {
		t.Fatalf("remove membership: %v", err)
	}
	if _, err := service.Resolve(ctx, subject); !errors.Is(err, ErrNoWorkspace) {
		t.Fatalf("Resolve() after revocation error = %v, want ErrNoWorkspace", err)
	}
}
