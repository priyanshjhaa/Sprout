package team

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/priyanshjhaa/Sprout/backend/internal/database"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
	"github.com/priyanshjhaa/Sprout/backend/internal/identity"
)

type profiles map[string]string

type profileFunc func(context.Context, string) (identity.Profile, error)

func (f profileFunc) GetProfile(ctx context.Context, subject string) (identity.Profile, error) {
	return f(ctx, subject)
}

func (p profiles) GetProfile(_ context.Context, subject string) (identity.Profile, error) {
	email, ok := p[subject]
	if !ok {
		return identity.Profile{}, identity.ErrIncompleteProfile
	}
	return identity.Profile{Email: email}, nil
}

func TestTeamAgainstPostgreSQL(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL required for integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var users []pgtype.UUID
	var workspaces []pgtype.UUID
	// These tests need committed fixtures to exercise two real concurrent transactions.
	// Cleanup targets only IDs created by this test; never existing application data.
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, w := range workspaces {
			if _, err := pool.Exec(cleanup, "DELETE FROM workspaces WHERE id=$1", w); err != nil {
				t.Error(err)
			}
		}
		for _, u := range users {
			if _, err := pool.Exec(cleanup, "DELETE FROM users WHERE id=$1", u); err != nil {
				t.Error(err)
			}
		}
	}()
	addUser := func(name string) pgtype.UUID {
		var id pgtype.UUID
		err := pool.QueryRow(ctx, `INSERT INTO users(auth_provider,auth_subject,email,display_name) VALUES('clerk',$1,$2,$3) RETURNING id`, name+suffix, name+suffix+"@example.test", name).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		users = append(users, id)
		return id
	}
	ownerID := addUser("owner")
	editorID := addUser("editor")
	recipient := addUser("recipient")
	outsider := addUser("outsider")
	addWorkspace := func(name string) pgtype.UUID {
		var id pgtype.UUID
		if err := pool.QueryRow(ctx, `INSERT INTO workspaces(slug,name,created_by) VALUES($1,$2,$3) RETURNING id`, name+suffix, name, ownerID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		workspaces = append(workspaces, id)
		if _, err := pool.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,user_id,role) VALUES($1,$2,'owner')`, id, ownerID); err != nil {
			t.Fatal(err)
		}
		return id
	}
	workspace := addWorkspace("team-")
	other := addWorkspace("other-")
	_ = other
	slug := "team-" + suffix
	if _, err := pool.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,user_id,role) VALUES($1,$2,'editor')`, workspace, editorID); err != nil {
		t.Fatal(err)
	}
	provider := profiles{"recipient" + suffix: "new-email" + suffix + "@example.test", "outsider" + suffix: "outsider" + suffix + "@example.test"}
	service := NewService(pool, provider)
	email := provider["recipient"+suffix]
	expect := func(err, want error) {
		t.Helper()
		if !errors.Is(err, want) {
			t.Fatalf("got %v, want %v", err, want)
		}
	}
	create := func(email string) CreatedInvitation {
		t.Helper()
		invite, err := service.Create(ctx, slug, ownerID.String(), email, "viewer")
		if err != nil {
			t.Fatal(err)
		}
		return invite
	}

	t.Run("owner boundaries and validation", func(t *testing.T) {
		_, err := service.Create(ctx, slug, editorID.String(), email, "viewer")
		expect(err, ErrForbidden)
		_, err = service.List(ctx, slug, outsider.String())
		expect(err, ErrNotFound)
		_, err = service.Create(ctx, slug, ownerID.String(), email, "owner")
		expect(err, ErrInvalid)
		_, err = service.Create(ctx, slug, ownerID.String(), "A Person <person@example.test>", "viewer")
		expect(err, ErrInvalid)
		_, err = service.Create(ctx, slug, ownerID.String(), "editor"+suffix+"@example.test", "viewer")
		expect(err, ErrConflict)
		expect(service.ChangeRole(ctx, slug, ownerID.String(), ownerID.String(), "viewer"), ErrForbidden)
		expect(service.Remove(ctx, slug, ownerID.String(), ownerID.String()), ErrForbidden)
		expect(service.Remove(ctx, slug, editorID.String(), ownerID.String()), ErrForbidden)
		expect(service.ChangeRole(ctx, "other-"+suffix, ownerID.String(), editorID.String(), "viewer"), ErrNotFound)
	})

	invite := create(email)
	t.Run("hash only and duplicate invite", func(t *testing.T) {
		var hash string
		if err := pool.QueryRow(ctx, "SELECT token_hash FROM workspace_invitations WHERE id=$1", invite.ID).Scan(&hash); err != nil {
			t.Fatal(err)
		}
		if hash == invite.Token || hash != digest(invite.Token) {
			t.Fatal("incorrect token storage")
		}
		_, err := service.Create(ctx, slug, ownerID.String(), email, "editor")
		expect(err, ErrConflict)
		list, err := service.List(ctx, slug, ownerID.String())
		if err != nil || len(list) != 1 {
			t.Fatalf("list: %v, %v", list, err)
		}
	})
	t.Run("fresh verified email and tenant checks", func(t *testing.T) {
		// Cached recipient email differs from the freshly verified provider email.
		_, err := service.Accept(ctx, outsider.String(), "outsider"+suffix, invite.Token)
		expect(err, ErrInvitation)
		_, err = service.Accept(ctx, outsider.String(), "recipient"+suffix, invite.Token)
		expect(err, ErrInvitation)
		expect(service.Revoke(ctx, "other-"+suffix, ownerID.String(), invite.ID), ErrNotFound)
		_, err = service.Accept(ctx, recipient.String(), "recipient"+suffix, "invalid")
		expect(err, ErrInvitation)
	})
	t.Run("concurrent acceptance is single use", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := service.Accept(ctx, recipient.String(), "recipient"+suffix, invite.Token)
				results <- err
			}()
		}
		wg.Wait()
		close(results)
		successes := 0
		for err := range results {
			if err == nil {
				successes++
			} else {
				expect(err, ErrInvitation)
			}
		}
		if successes != 1 {
			t.Fatalf("successful acceptances = %d", successes)
		}
		role, err := dbgen.New(pool).GetTeamRole(ctx, dbgen.GetTeamRoleParams{WorkspaceID: workspace, UserID: recipient})
		if err != nil || role != "viewer" {
			t.Fatalf("membership: %s, %v", role, err)
		}
		list, err := service.Workspaces(ctx, recipient.String())
		if err != nil || len(list) != 1 || list[0].Slug != slug {
			t.Fatalf("workspaces: %v, %v", list, err)
		}
	})
	t.Run("removal clears grants and denies access", func(t *testing.T) {
		expect(service.ChangeRole(ctx, slug, ownerID.String(), recipient.String(), "editor"), nil)
		var app pgtype.UUID
		err := pool.QueryRow(ctx, `INSERT INTO applications(workspace_id,name,slug,default_hostname,created_by) VALUES($1,'Test','test',$2,$3) RETURNING id`, workspace, suffix+".team.test", ownerID).Scan(&app)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO application_access_grants(application_id,user_id,role,granted_by) VALUES($1,$2,'editor',$3)`, app, recipient, ownerID); err != nil {
			t.Fatal(err)
		}
		expect(service.Remove(ctx, slug, ownerID.String(), recipient.String()), nil)
		var count int
		if err = pool.QueryRow(ctx, "SELECT count(*) FROM application_access_grants WHERE application_id=$1 AND user_id=$2", app, recipient).Scan(&count); err != nil || count != 0 {
			t.Fatalf("grants remain: %d %v", count, err)
		}
		list, err := service.Workspaces(ctx, recipient.String())
		if err != nil || len(list) != 0 {
			t.Fatalf("removed user workspaces: %v %v", list, err)
		}
		_, err = service.Accept(ctx, recipient.String(), "recipient"+suffix, invite.Token)
		expect(err, ErrInvitation)
	})
	t.Run("revocation expiry and existing member role", func(t *testing.T) {
		revoked := create(email)
		expect(service.Revoke(ctx, slug, ownerID.String(), revoked.ID), nil)
		_, err := service.Accept(ctx, recipient.String(), "recipient"+suffix, revoked.Token)
		expect(err, ErrInvitation)
		expired := create(email)
		if _, err = pool.Exec(ctx, "UPDATE workspace_invitations SET expires_at=now()-interval '1 second' WHERE id=$1", expired.ID); err != nil {
			t.Fatal(err)
		}
		_, err = service.Accept(ctx, recipient.String(), "recipient"+suffix, expired.Token)
		expect(err, ErrInvitation)
		replacement := create(email)
		if _, err = pool.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,user_id,role) VALUES($1,$2,'editor')`, workspace, recipient); err != nil {
			t.Fatal(err)
		}
		_, err = service.Accept(ctx, recipient.String(), "recipient"+suffix, replacement.Token)
		expect(err, nil)
		role, err := dbgen.New(pool).GetTeamRole(ctx, dbgen.GetTeamRoleParams{WorkspaceID: workspace, UserID: recipient})
		if err != nil || role != "editor" {
			t.Fatalf("existing role changed: %s %v", role, err)
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		cancelled, stop := context.WithCancel(ctx)
		stop()
		if _, err := service.Create(cancelled, slug, ownerID.String(), "cancel@example.test", "viewer"); err == nil {
			t.Fatal("cancelled operation succeeded")
		}
	})

	t.Run("membership failure rolls back invitation consumption", func(t *testing.T) {
		deleted := addUser("deleted")
		invitation := create("deleted" + suffix + "@example.test")
		// Simulate an identity deletion after validation but before membership insertion.
		failing := NewService(pool, profileFunc(func(ctx context.Context, _ string) (identity.Profile, error) {
			_, err := pool.Exec(ctx, "DELETE FROM users WHERE id=$1", deleted)
			return identity.Profile{Email: invitation.Email}, err
		}))
		if _, err := failing.Accept(ctx, deleted.String(), "deleted"+suffix, invitation.Token); err == nil {
			t.Fatal("expected FK failure")
		}
		var accepted bool
		if err := pool.QueryRow(ctx, "SELECT accepted_at IS NOT NULL FROM workspace_invitations WHERE id=$1", invitation.ID).Scan(&accepted); err != nil {
			t.Fatal(err)
		}
		if accepted {
			t.Fatal("failed membership insertion consumed the invitation")
		}
	})
}
