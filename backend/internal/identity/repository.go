package identity

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
)

type SQLRepository struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
}

func NewSQLRepository(pool *pgxpool.Pool) *SQLRepository {
	return &SQLRepository{pool: pool, queries: dbgen.New(pool)}
}

func (repository *SQLRepository) Find(ctx context.Context, subject string) (Principal, error) {
	user, err := repository.queries.GetUserByIdentity(ctx, subject)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrNotFound
	}
	if err != nil {
		return Principal{}, err
	}
	workspace, err := repository.queries.GetHomeWorkspace(ctx, user.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrNoWorkspace
	}
	if err != nil {
		return Principal{}, err
	}
	return principalFromRows(user.ID, user.Email, user.DisplayName, workspace.ID, workspace.Slug, workspace.Name), nil
}

func (repository *SQLRepository) Create(ctx context.Context, subject string, profile Profile) (Principal, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Principal{}, err
	}
	defer tx.Rollback(ctx)
	queries := repository.queries.WithTx(tx)

	user, err := queries.InsertClerkUser(ctx, dbgen.InsertClerkUserParams{
		AuthSubject: subject,
		Email:       profile.Email,
		DisplayName: profile.DisplayName,
	})
	if err != nil {
		return Principal{}, err
	}

	workspace, err := queries.GetHomeWorkspace(ctx, user.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		workspace, err = repository.createPersonalWorkspace(ctx, queries, user.ID)
	}
	if err != nil {
		return Principal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Principal{}, err
	}
	return principalFromRows(user.ID, user.Email, user.DisplayName, workspace.ID, workspace.Slug, workspace.Name), nil
}

func (repository *SQLRepository) createPersonalWorkspace(
	ctx context.Context, queries *dbgen.Queries, userID pgtype.UUID,
) (dbgen.GetHomeWorkspaceRow, error) {
	slug := "u-" + strings.ReplaceAll(userID.String(), "-", "")
	workspace, err := queries.InsertPersonalWorkspace(ctx, dbgen.InsertPersonalWorkspaceParams{
		Slug: slug, UserID: userID,
	})
	if err != nil {
		return dbgen.GetHomeWorkspaceRow{}, err
	}
	if err := queries.InsertOwnerMembership(ctx, dbgen.InsertOwnerMembershipParams{
		WorkspaceID: workspace.ID, UserID: userID,
	}); err != nil {
		return dbgen.GetHomeWorkspaceRow{}, err
	}
	return dbgen.GetHomeWorkspaceRow(workspace), nil
}

func (repository *SQLRepository) GetWorkspace(ctx context.Context, slug, userID string) (Workspace, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(userID); err != nil || !parsed.Valid {
		return Workspace{}, ErrNotFound
	}
	row, err := repository.queries.GetWorkspaceForMember(ctx, dbgen.GetWorkspaceForMemberParams{
		WorkspaceSlug: slug, UserID: parsed,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Workspace{}, ErrNotFound
	}
	if err != nil {
		return Workspace{}, err
	}
	return Workspace{ID: row.ID.String(), Slug: row.Slug, Name: row.Name}, nil
}

func principalFromRows(
	userID pgtype.UUID, email, displayName string,
	workspaceID pgtype.UUID, workspaceSlug, workspaceName string,
) Principal {
	return Principal{
		ID: userID.String(), Email: email, DisplayName: displayName,
		Workspace: Workspace{ID: workspaceID.String(), Slug: workspaceSlug, Name: workspaceName},
	}
}
