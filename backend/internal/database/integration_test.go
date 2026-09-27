package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
)

func TestPoolAndGeneratedQueriesAgainstPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer pool.Close()

	assertPoolLimit(t, ctx, pool)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
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

	workspaceSlug := "integration-" + suffix
	var workspaceID pgtype.UUID
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO workspaces (slug, name, created_by)
		 VALUES ($1, 'Integration Workspace', $2)
		 RETURNING id`,
		workspaceSlug,
		userID,
	).Scan(&workspaceID); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	if _, err := tx.Exec(
		ctx,
		`INSERT INTO workspace_memberships (workspace_id, user_id, role)
		 VALUES ($1, $2, 'owner')`,
		workspaceID,
		userID,
	); err != nil {
		t.Fatalf("insert workspace membership: %v", err)
	}

	var applicationID pgtype.UUID
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO applications
		 (workspace_id, name, slug, description, default_hostname, created_by)
		 VALUES ($1, 'Integration App', 'integration-app', 'Temporary test application', $2, $3)
		 RETURNING id`,
		workspaceID,
		"integration-"+suffix+".sprout.test",
		userID,
	).Scan(&applicationID); err != nil {
		t.Fatalf("insert application: %v", err)
	}

	queries := dbgen.New(tx)

	access, err := queries.GetWorkspaceAccess(ctx, dbgen.GetWorkspaceAccessParams{
		WorkspaceSlug: workspaceSlug,
		UserID:        userID,
	})
	if err != nil {
		t.Fatalf("GetWorkspaceAccess() error = %v", err)
	}
	if access.WorkspaceID != workspaceID || access.Role != dbgen.WorkspaceRoleOwner {
		t.Fatalf("workspace access = %#v, want owner access", access)
	}

	applications, err := queries.ListApplicationsForMember(ctx, dbgen.ListApplicationsForMemberParams{
		WorkspaceSlug: workspaceSlug,
		UserID:        userID,
	})
	if err != nil {
		t.Fatalf("ListApplicationsForMember() error = %v", err)
	}
	if len(applications) != 1 || applications[0].ID != applicationID {
		t.Fatalf("applications = %#v, want inserted application", applications)
	}

	application, err := queries.GetApplicationForMember(
		ctx,
		dbgen.GetApplicationForMemberParams{
			WorkspaceSlug: workspaceSlug,
			ApplicationID: applicationID,
			UserID:        userID,
		},
	)
	if err != nil {
		t.Fatalf("GetApplicationForMember() error = %v", err)
	}
	if application.Name != "Integration App" || application.Lifecycle != dbgen.ApplicationLifecycleActive {
		t.Fatalf("application = %#v, want active Integration App", application)
	}

	_, err = queries.GetApplicationForMember(
		ctx,
		dbgen.GetApplicationForMemberParams{
			WorkspaceSlug: "different-workspace",
			ApplicationID: applicationID,
			UserID:        userID,
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-workspace query error = %v, want pgx.ErrNoRows", err)
	}
}

func assertPoolLimit(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	connections := make([]*pgxpool.Conn, 0, maxConnections)
	for range maxConnections {
		connection, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire bounded connection: %v", err)
		}
		connections = append(connections, connection)
	}
	defer func() {
		for _, connection := range connections {
			connection.Release()
		}
	}()

	waitCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	_, err := pool.Acquire(waitCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire() beyond pool limit error = %v, want context deadline", err)
	}
}
