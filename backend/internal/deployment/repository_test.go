package deployment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/priyanshjhaa/Sprout/backend/internal/artifact"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
)

func TestSimulationRepositoryPostgreSQL(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	// Recovery changes all unfinished simulations, so tests use a private schema,
	// never the developer's real workspace or jobs. Types still come from Drizzle.
	schema := fmt.Sprintf("simulation_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	for _, table := range []string{"users", "workspaces", "workspace_memberships", "applications", "application_access_grants", "deployments", "deployment_stage_events"} {
		tableName := pgx.Identifier{table}.Sanitize()
		if _, err = admin.Exec(ctx, "CREATE TABLE "+quoted+"."+tableName+" (LIKE public."+tableName+" INCLUDING ALL)"); err != nil {
			t.Fatal(err)
		}
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repo := NewSQLRepository(pool)
	addUser := func(name string) pgtype.UUID {
		var id pgtype.UUID
		if err := pool.QueryRow(ctx, `INSERT INTO users(auth_provider,auth_subject,email,display_name) VALUES('test',$1::text,$1::text || '@example.test',$1::text) RETURNING id`, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	owner := addUser("owner")
	editor := addUser("editor")
	viewer := addUser("viewer")
	outsider := addUser("outsider")
	var workspace, app, otherApp pgtype.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO workspaces(slug,name,created_by) VALUES('simulation','Simulation',$1) RETURNING id`, owner).Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	for _, member := range []struct {
		id   pgtype.UUID
		role string
	}{{owner, "owner"}, {editor, "editor"}, {viewer, "viewer"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,user_id,role) VALUES($1,$2,$3)`, workspace, member.id, member.role); err != nil {
			t.Fatal(err)
		}
	}
	for i, dest := range []*pgtype.UUID{&app, &otherApp} {
		if err := pool.QueryRow(ctx, `INSERT INTO applications(workspace_id,name,slug,default_hostname,created_by) VALUES($1,'Simulation',$2::text,$2::text || '.test',$3) RETURNING id`, workspace, fmt.Sprintf("app-%d", i), owner).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	scope := Scope{WorkspaceSlug: "simulation", ApplicationID: app.String(), UserID: owner.String(), Simulated: true}
	finish := func(id, status, code string) error {
		_, err := repo.Finish(ctx, id, status, code, nil)
		return err
	}
	expect := func(err, want error) {
		t.Helper()
		if !errors.Is(err, want) {
			t.Fatalf("got %v want %v", err, want)
		}
	}
	for _, tc := range []struct {
		user pgtype.UUID
		want error
	}{{viewer, ErrForbidden}, {outsider, ErrNotFound}} {
		denied := scope
		denied.UserID = tc.user.String()
		_, err = repo.Create(ctx, denied)
		expect(err, tc.want)
	}
	job, err := repo.Create(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if !job.Simulated || job.Status != "queued" || len(job.Stages) != 3 {
		t.Fatalf("queued job: %+v", job)
	}
	_, err = repo.Create(ctx, scope)
	expect(err, ErrConflict)
	hidden := scope
	hidden.WorkspaceSlug = "elsewhere"
	_, err = repo.Get(ctx, hidden, job.ID)
	expect(err, ErrNotFound)
	hidden = scope
	hidden.ApplicationID = otherApp.String()
	_, err = repo.Get(ctx, hidden, job.ID)
	expect(err, ErrNotFound)
	readOnly := scope
	readOnly.UserID = viewer.String()
	if _, err = repo.Get(ctx, readOnly, job.ID); err != nil {
		t.Fatal(err)
	}
	_, err = repo.Cancel(ctx, readOnly, job.ID)
	expect(err, ErrForbidden)
	expect(repo.Start(ctx, scope, job.ID), nil)
	expect(repo.Advance(ctx, job.ID, "build", "running"), ErrConflict)
	expect(finish(job.ID, "succeeded", ""), ErrConflict)
	for _, stage := range stages {
		expect(repo.Advance(ctx, job.ID, stage, "running"), nil)
		expect(repo.Advance(ctx, job.ID, stage, "succeeded"), nil)
	}
	expect(finish(job.ID, "succeeded", ""), nil)
	done, err := repo.Get(ctx, scope, job.ID)
	if err != nil || done.Status != "succeeded" || done.FinishedAt == nil {
		t.Fatalf("completed: %+v %v", done, err)
	}
	_, err = repo.Cancel(ctx, scope, job.ID)
	expect(err, ErrConflict)
	// Simulations can never be mistaken for a live deployment, including direct SQL.
	if _, err = pool.Exec(ctx, "UPDATE deployments SET status='live' WHERE id=$1", job.ID); err == nil {
		t.Fatal("simulation marked live")
	}

	// A real build records its artifact reference; each kind is invisible to the other.
	realScope := scope
	realScope.Simulated = false
	built, err := repo.Create(ctx, realScope)
	if err != nil || built.Simulated {
		t.Fatalf("real build: %+v %v", built, err)
	}
	_, err = repo.Get(ctx, scope, built.ID)
	expect(err, ErrNotFound)
	expect(repo.Start(ctx, realScope, built.ID), nil)
	for _, stage := range stages {
		expect(repo.Advance(ctx, built.ID, stage, "running"), nil)
		expect(repo.Advance(ctx, built.ID, stage, "succeeded"), nil)
	}
	output := &artifact.Descriptor{ID: strings.Repeat("a", 32), SHA256: strings.Repeat("b", 64), Size: 42}
	_, err = repo.Finish(ctx, built.ID, "failed", "build_failed", output)
	expect(err, ErrInvalid)
	recorded, err := repo.Finish(ctx, built.ID, "succeeded", "", output)
	if err != nil || !recorded {
		t.Fatalf("artifact not recorded: %v %v", recorded, err)
	}
	referenced, err := repo.ReferencedArtifacts(ctx)
	if err != nil || !referenced[output.ID] {
		t.Fatalf("artifact reference missing: %v %v", referenced, err)
	}
	if recorded, err = repo.Finish(ctx, built.ID, "succeeded", "", output); err != nil || recorded {
		t.Fatalf("terminal job re-recorded: %v %v", recorded, err)
	}
	// A simulation can never claim an artifact.
	simulated, err := repo.Create(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	expect(repo.Start(ctx, scope, simulated.ID), nil)
	for _, stage := range stages {
		expect(repo.Advance(ctx, simulated.ID, stage, "running"), nil)
		expect(repo.Advance(ctx, simulated.ID, stage, "succeeded"), nil)
	}
	_, err = repo.Finish(ctx, simulated.ID, "succeeded", "", output)
	expect(err, ErrInvalid)
	expect(finish(simulated.ID, "succeeded", ""), nil)

	cancelled, err := repo.Create(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	expect(repo.Start(ctx, scope, cancelled.ID), nil)
	expect(repo.Advance(ctx, cancelled.ID, "source", "running"), nil)
	_, err = repo.Cancel(ctx, scope, cancelled.ID)
	expect(err, nil)
	expect(finish(cancelled.ID, "succeeded", ""), nil)
	done, err = repo.Get(ctx, scope, cancelled.ID)
	if err != nil || done.Status != "cancelled" {
		t.Fatalf("cancellation lost: %+v %v", done, err)
	}
	for _, stage := range done.Stages {
		want := "skipped"
		if stage.Name == "source" {
			want = "failed"
		}
		if stage.Status != want {
			t.Fatalf("unfinished stage: %+v", stage)
		}
	}

	// Restricted applications honour explicit grants; removed membership overrides grants.
	if _, err = pool.Exec(ctx, "UPDATE applications SET access_mode='restricted' WHERE id=$1", app); err != nil {
		t.Fatal(err)
	}
	editorScope := scope
	editorScope.UserID = editor.String()
	_, err = repo.List(ctx, editorScope)
	expect(err, ErrNotFound)
	if _, err = pool.Exec(ctx, `INSERT INTO application_access_grants(application_id,user_id,role,granted_by) VALUES($1,$2,'editor',$3)`, app, editor, owner); err != nil {
		t.Fatal(err)
	}
	revokedJob, err := repo.Create(ctx, editorScope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "DELETE FROM workspace_memberships WHERE workspace_id=$1 AND user_id=$2", workspace, editor); err != nil {
		t.Fatal(err)
	}
	expect(repo.Start(ctx, editorScope, revokedJob.ID), ErrNotFound)
	expect(finish(revokedJob.ID, "failed", "start_rejected"), nil)
	_, err = repo.List(ctx, editorScope)
	expect(err, ErrNotFound)

	if _, err = pool.Exec(ctx, "UPDATE applications SET lifecycle='paused' WHERE id=$1", app); err != nil {
		t.Fatal(err)
	}
	_, err = repo.Create(ctx, scope)
	expect(err, ErrConflict)
	if _, err = pool.Exec(ctx, "UPDATE applications SET lifecycle='active' WHERE id=$1", app); err != nil {
		t.Fatal(err)
	}

	interrupted, err := repo.Create(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	expect(repo.Start(ctx, scope, interrupted.ID), nil)
	expect(repo.Advance(ctx, interrupted.ID, "source", "running"), nil)
	// The worker owns real builds too, so an unfinished real row is also recovered.
	var realID pgtype.UUID
	if err = pool.QueryRow(ctx, "INSERT INTO deployments(application_id) VALUES($1) RETURNING id", otherApp).Scan(&realID); err != nil {
		t.Fatal(err)
	}
	release, err := repo.AcquireProcessLock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, err = repo.AcquireProcessLock(ctx)
	expect(err, ErrUnavailable)
	recovered, err := repo.Get(ctx, scope, interrupted.ID)
	if err != nil || recovered.Status != "failed" || recovered.FailureCode == nil || *recovered.FailureCode != "process_interrupted" {
		t.Fatalf("recovery: %+v %v", recovered, err)
	}
	var realStatus, realCode string
	if err = pool.QueryRow(ctx, "SELECT status, failure_code FROM deployments WHERE id=$1", realID).Scan(&realStatus, &realCode); err != nil || realStatus != "failed" || realCode != "process_interrupted" {
		t.Fatalf("real deployment not recovered: %s %s %v", realStatus, realCode, err)
	}
	// Recovery released the per-app slot.
	if _, err = repo.Create(ctx, scope); err != nil {
		t.Fatal(err)
	}
	// Generated query also rejects a viewer, independently of service checks.
	_, err = dbgen.New(pool).CreateDeployment(ctx, dbgen.CreateDeploymentParams{ApplicationID: app, WorkspaceSlug: "simulation", UserID: viewer, Simulated: true})
	expect(err, pgx.ErrNoRows)
	release()
	expect(repo.CheckOwnership(ctx), ErrUnavailable)
}
