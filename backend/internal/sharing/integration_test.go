package sharing

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

func TestSharingAgainstPostgreSQL(t *testing.T) {
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
	owner := insertUser(t, ctx, tx, suffix+"-owner")
	creator := insertUser(t, ctx, tx, suffix+"-creator")
	viewer := insertUser(t, ctx, tx, suffix+"-viewer")
	outsider := insertUser(t, ctx, tx, suffix+"-outsider")
	workspaceSlug := "sharing-" + suffix
	var workspace pgtype.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO workspaces (slug, name, created_by) VALUES ($1, 'Sharing Test', $2) RETURNING id`,
		workspaceSlug, owner,
	).Scan(&workspace); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	for _, member := range []struct {
		id   pgtype.UUID
		role string
	}{{owner, "owner"}, {creator, "editor"}, {viewer, "viewer"}} {
		if _, err := tx.Exec(ctx,
			`INSERT INTO workspace_memberships (workspace_id, user_id, role) VALUES ($1, $2, $3)`,
			workspace, member.id, member.role,
		); err != nil {
			t.Fatalf("insert membership: %v", err)
		}
	}
	var application pgtype.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO applications (workspace_id, name, slug, default_hostname, created_by)
		 VALUES ($1, 'Invoices', 'invoices', 'invoices.sharing.test', $2) RETURNING id`,
		workspace, creator,
	).Scan(&application); err != nil {
		t.Fatalf("insert application: %v", err)
	}

	service := NewService(dbgen.New(tx))
	members, err := service.ListMembers(ctx, workspaceSlug, owner.String())
	if err != nil || len(members) != 3 {
		t.Fatalf("ListMembers() = %#v, %v, want three members", members, err)
	}
	if _, err := service.ListMembers(ctx, workspaceSlug, outsider.String()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("outsider ListMembers() error = %v, want ErrNotFound", err)
	}
	if _, err := service.GetAccess(ctx, workspaceSlug, application.String(), viewer.String()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer GetAccess() error = %v, want ErrForbidden", err)
	}
	if _, err := service.SetMode(ctx, workspaceSlug, application.String(), viewer.String(), "restricted"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("viewer SetMode() error = %v, want ErrForbidden", err)
	}
	access, err := service.SetMode(ctx, workspaceSlug, application.String(), creator.String(), "restricted")
	if err != nil || access.Mode != "restricted" {
		t.Fatalf("creator SetMode() = %#v, %v", access, err)
	}
	_, err = dbgen.New(tx).SetApplicationAccessMode(ctx, dbgen.SetApplicationAccessModeParams{
		AccessMode:    dbgen.ApplicationAccessModeWorkspace,
		ApplicationID: application, WorkspaceSlug: workspaceSlug, ActorID: viewer,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("viewer direct access-mode write error = %v, want no rows", err)
	}
	_, err = dbgen.New(tx).UpsertApplicationGrant(ctx, dbgen.UpsertApplicationGrantParams{
		Role: dbgen.ApplicationRoleEditor, ActorID: viewer, TargetID: creator,
		WorkspaceSlug: workspaceSlug, ApplicationID: application,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("viewer direct grant write error = %v, want no rows", err)
	}
	if _, err := service.GetAccess(ctx, workspaceSlug, application.String(), viewer.String()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("restricted viewer GetAccess() error = %v, want ErrNotFound", err)
	}
	if _, err := service.Grant(ctx, workspaceSlug, application.String(), owner.String(), outsider.String(), "viewer"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("outsider Grant() error = %v, want ErrNotFound", err)
	}
	if _, err := service.Grant(ctx, workspaceSlug, application.String(), owner.String(), viewer.String(), "owner"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid role Grant() error = %v, want ErrInvalid", err)
	}
	access, err = service.Grant(ctx, workspaceSlug, application.String(), creator.String(), viewer.String(), "viewer")
	if err != nil || len(access.Grants) != 1 || access.Grants[0].Role != "viewer" {
		t.Fatalf("creator Grant() = %#v, %v", access, err)
	}
	access, err = service.Grant(ctx, workspaceSlug, application.String(), owner.String(), viewer.String(), "editor")
	if err != nil || access.Grants[0].Role != "editor" {
		t.Fatalf("owner Grant() = %#v, %v", access, err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM workspace_memberships WHERE workspace_id = $1 AND user_id = $2`, workspace, viewer,
	); err != nil {
		t.Fatalf("remove membership: %v", err)
	}
	access, err = service.GetAccess(ctx, workspaceSlug, application.String(), owner.String())
	if err != nil || len(access.Grants) != 0 {
		t.Fatalf("access after membership removal = %#v, %v, want no active grants", access, err)
	}
	if _, err := service.GetAccess(ctx, workspaceSlug, application.String(), viewer.String()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("former member GetAccess() error = %v, want ErrNotFound", err)
	}
	if _, err := service.Revoke(ctx, workspaceSlug, application.String(), owner.String(), viewer.String()); err != nil {
		t.Fatalf("owner Revoke() error = %v", err)
	}
}

func insertUser(t *testing.T, ctx context.Context, tx pgx.Tx, suffix string) pgtype.UUID {
	t.Helper()
	var user pgtype.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO users (auth_provider, auth_subject, email, display_name)
		 VALUES ('integration-test', $1, $2, 'Sharing Test User') RETURNING id`,
		"subject-"+suffix, "integration-"+suffix+"@example.test",
	).Scan(&user); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return user
}
