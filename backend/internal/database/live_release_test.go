package database

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
)

// Live is a property of the whole application: the database, not Go code, must
// refuse a second live release, and promotion must be a demote-then-promote.
func TestLiveReleaseRulesAgainstPostgreSQL(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := Open(ctx, url)
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
	var user, workspace, app pgtype.UUID
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(tx.QueryRow(ctx, `INSERT INTO users(auth_provider,auth_subject,email,display_name) VALUES('test',$1::text,$1::text||'@example.test','t') RETURNING id`, "live-"+suffix).Scan(&user))
	must(tx.QueryRow(ctx, `INSERT INTO workspaces(slug,name,created_by) VALUES($1,'W',$2) RETURNING id`, "live-"+suffix, user).Scan(&workspace))
	must(tx.QueryRow(ctx, `INSERT INTO applications(workspace_id,name,slug,default_hostname,created_by) VALUES($1,'A','shop',$2,$3) RETURNING id`, workspace, "shop.live-"+suffix+".sprout.test", user).Scan(&app))

	deploy := func(artifact string) pgtype.UUID {
		var id pgtype.UUID
		must(tx.QueryRow(ctx, `INSERT INTO deployments(application_id,status,artifact_id,artifact_sha256,artifact_bytes)
			VALUES($1,'succeeded',$2,$3,10) RETURNING id`, app, strings.Repeat(artifact, 32), strings.Repeat("a", 64)).Scan(&id))
		return id
	}
	first, second := deploy("1"), deploy("2")
	q := dbgen.New(tx)

	promote := func(id pgtype.UUID) error {
		_, err := tx.Exec(ctx, `UPDATE deployments SET status='live' WHERE id=$1`, id)
		return err
	}
	// A failed statement aborts the transaction, so test the refusal in a savepoint.
	must(promote(first))
	sp, err := tx.Begin(ctx)
	must(err)
	if err := func() error {
		_, err := sp.Exec(ctx, `UPDATE deployments SET status='live' WHERE id=$1`, second)
		return err
	}(); err == nil {
		t.Fatal("a second live deployment for one application was accepted")
	}
	must(sp.Rollback(ctx))

	demoted, err := q.DemoteLiveDeployments(ctx, dbgen.DemoteLiveDeploymentsParams{ApplicationID: app, ID: second})
	must(err)
	if len(demoted) != 1 || demoted[0].ID != first {
		t.Fatalf("demoted = %#v, want the first release", demoted)
	}
	must(promote(second))

	row, err := q.GetLiveRelease(ctx, dbgen.GetLiveReleaseParams{WorkspaceSlug: "live-" + suffix, ApplicationSlug: "shop"})
	must(err)
	if row.DeploymentID != second || row.Lifecycle != dbgen.ApplicationLifecycleActive || row.ArtifactID.String != strings.Repeat("2", 32) {
		t.Fatalf("live release = %#v, want the second release", row)
	}
	live, err := q.ListLiveReleases(ctx)
	must(err)
	found := false
	for _, release := range live {
		found = found || release.DeploymentID == second
	}
	if !found {
		t.Fatal("ListLiveReleases omitted the live release")
	}
}
