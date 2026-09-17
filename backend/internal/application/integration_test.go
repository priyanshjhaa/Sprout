package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/priyanshjhaa/Sprout/backend/internal/database"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
)

func TestApplicationServiceAgainstPostgreSQL(t *testing.T) {
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

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ownerID := insertTestUser(t, ctx, tx, suffix+"-owner")
	viewerID := insertTestUser(t, ctx, tx, suffix+"-viewer")
	outsiderID := insertTestUser(t, ctx, tx, suffix+"-outsider")
	workspaceSlug := "api-" + suffix

	var workspaceID pgtype.UUID
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO workspaces (slug, name, created_by)
		 VALUES ($1, 'API Test Workspace', $2)
		 RETURNING id`,
		workspaceSlug,
		ownerID,
	).Scan(&workspaceID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	for _, membership := range []struct {
		userID pgtype.UUID
		role   string
	}{
		{userID: ownerID, role: "owner"},
		{userID: viewerID, role: "viewer"},
	} {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO workspace_memberships (workspace_id, user_id, role) VALUES ($1, $2, $3)`,
			workspaceID,
			membership.userID,
			membership.role,
		); err != nil {
			t.Fatalf("insert membership: %v", err)
		}
	}

	repository := NewSQLRepository(dbgen.New(tx))
	service := NewService(repository)
	owner := formatUUID(ownerID)
	viewer := formatUUID(viewerID)
	outsider := formatUUID(outsiderID)

	created, err := service.Create(ctx, CreateInput{
		WorkspaceSlug: workspaceSlug,
		UserID:        owner,
		Name:          "Invoice approvals",
		Slug:          "invoice-approvals",
		Description:   "Review invoices",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Lifecycle != LifecycleActive || created.DefaultHostname != "invoice-approvals."+workspaceSlug+".sprout.local" {
		t.Fatalf("created application = %#v", created)
	}

	applications, err := service.List(ctx, workspaceSlug, viewer)
	if err != nil {
		t.Fatalf("viewer List() error = %v", err)
	}
	if len(applications) != 1 || applications[0].ID != created.ID {
		t.Fatalf("applications = %#v, want created application", applications)
	}

	if _, err := service.Get(ctx, workspaceSlug, created.ID, viewer); err != nil {
		t.Fatalf("viewer Get() error = %v", err)
	}

	_, err = service.Create(ctx, CreateInput{
		WorkspaceSlug: workspaceSlug,
		UserID:        viewer,
		Name:          "Forbidden",
		Slug:          "forbidden",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer Create() error = %v, want ErrForbidden", err)
	}

	if _, err := service.List(ctx, workspaceSlug, outsider); !errors.Is(err, ErrNotFound) {
		t.Fatalf("outsider List() error = %v, want ErrNotFound", err)
	}

	paused := LifecyclePaused
	updatedName := "Invoice review"
	updated, err := service.Update(ctx, UpdateInput{
		WorkspaceSlug: workspaceSlug,
		ApplicationID: created.ID,
		UserID:        owner,
		Name:          &updatedName,
		Lifecycle:     &paused,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != updatedName || updated.Lifecycle != LifecyclePaused {
		t.Fatalf("updated application = %#v", updated)
	}

	_, err = service.Update(ctx, UpdateInput{
		WorkspaceSlug: workspaceSlug,
		ApplicationID: created.ID,
		UserID:        viewer,
		Name:          &updatedName,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer Update() error = %v, want ErrForbidden", err)
	}

	_, err = service.Create(ctx, CreateInput{
		WorkspaceSlug: workspaceSlug,
		UserID:        owner,
		Name:          "Duplicate",
		Slug:          "invoice-approvals",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate Create() error = %v, want ErrConflict", err)
	}
}

func insertTestUser(t *testing.T, ctx context.Context, tx pgx.Tx, suffix string) pgtype.UUID {
	t.Helper()

	var userID pgtype.UUID
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO users (auth_provider, auth_subject, email, display_name)
		 VALUES ('integration-test', $1, $2, 'Integration Test')
		 RETURNING id`,
		"subject-"+suffix,
		"integration-"+suffix+"@example.test",
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return userID
}
