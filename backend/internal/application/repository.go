package application

import (
	"context"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
)

type SQLRepository struct {
	queries *dbgen.Queries
}

func NewSQLRepository(queries *dbgen.Queries) *SQLRepository {
	return &SQLRepository{queries: queries}
}

func (repository *SQLRepository) List(
	ctx context.Context,
	workspaceSlug, userID string,
) ([]Application, error) {
	parsedUserID, err := parseUUID(userID)
	if err != nil {
		return nil, ErrInvalidIdentity
	}
	if _, err := repository.authorize(ctx, workspaceSlug, parsedUserID, false); err != nil {
		return nil, err
	}

	rows, err := repository.queries.ListApplicationsForMember(
		ctx,
		dbgen.ListApplicationsForMemberParams{WorkspaceSlug: workspaceSlug, UserID: parsedUserID},
	)
	if err != nil {
		return nil, err
	}

	applications := make([]Application, 0, len(rows))
	for _, row := range rows {
		applications = append(applications, mapApplication(row))
	}
	return applications, nil
}

func (repository *SQLRepository) Get(
	ctx context.Context,
	workspaceSlug, applicationID, userID string,
) (Application, error) {
	parsedUserID, err := parseUUID(userID)
	if err != nil {
		return Application{}, ErrInvalidIdentity
	}
	parsedApplicationID, err := parseUUID(applicationID)
	if err != nil {
		return Application{}, ErrNotFound
	}
	if _, err := repository.authorize(ctx, workspaceSlug, parsedUserID, false); err != nil {
		return Application{}, err
	}

	row, err := repository.queries.GetApplicationForMember(
		ctx,
		dbgen.GetApplicationForMemberParams{
			WorkspaceSlug: workspaceSlug,
			ApplicationID: parsedApplicationID,
			UserID:        parsedUserID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Application{}, ErrNotFound
	}
	if err != nil {
		return Application{}, err
	}

	return mapApplication(row), nil
}

func (repository *SQLRepository) Create(ctx context.Context, input CreateInput) (Application, error) {
	parsedUserID, err := parseUUID(input.UserID)
	if err != nil {
		return Application{}, ErrInvalidIdentity
	}
	if _, err := repository.authorize(ctx, input.WorkspaceSlug, parsedUserID, true); err != nil {
		return Application{}, err
	}

	row, err := repository.queries.CreateApplicationForMember(
		ctx,
		dbgen.CreateApplicationForMemberParams{
			Name:            input.Name,
			Slug:            input.Slug,
			Description:     input.Description,
			DefaultHostname: input.Slug + "." + input.WorkspaceSlug + ".sprout.local",
			UserID:          parsedUserID,
			WorkspaceSlug:   input.WorkspaceSlug,
		},
	)
	if isUniqueViolation(err) {
		return Application{}, ErrConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Application{}, ErrForbidden
	}
	if err != nil {
		return Application{}, err
	}

	return mapApplication(row), nil
}

func (repository *SQLRepository) Update(ctx context.Context, input UpdateInput) (Application, error) {
	parsedUserID, err := parseUUID(input.UserID)
	if err != nil {
		return Application{}, ErrInvalidIdentity
	}
	parsedApplicationID, err := parseUUID(input.ApplicationID)
	if err != nil {
		return Application{}, ErrNotFound
	}
	if _, err := repository.authorize(ctx, input.WorkspaceSlug, parsedUserID, true); err != nil {
		return Application{}, err
	}

	params := dbgen.UpdateApplicationForMemberParams{
		ApplicationID: parsedApplicationID,
		WorkspaceSlug: input.WorkspaceSlug,
		UserID:        parsedUserID,
	}
	if input.Name != nil {
		params.Name = pgtype.Text{String: *input.Name, Valid: true}
	}
	if input.Description != nil {
		params.Description = pgtype.Text{String: *input.Description, Valid: true}
	}
	if input.Lifecycle != nil {
		params.Lifecycle = dbgen.NullApplicationLifecycle{
			ApplicationLifecycle: dbgen.ApplicationLifecycle(*input.Lifecycle),
			Valid:                true,
		}
	}

	row, err := repository.queries.UpdateApplicationForMember(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return Application{}, ErrNotFound
	}
	if err != nil {
		return Application{}, err
	}

	return mapApplication(row), nil
}

func (repository *SQLRepository) authorize(
	ctx context.Context,
	workspaceSlug string,
	userID pgtype.UUID,
	write bool,
) (dbgen.WorkspaceRole, error) {
	access, err := repository.queries.GetWorkspaceAccess(
		ctx,
		dbgen.GetWorkspaceAccessParams{WorkspaceSlug: workspaceSlug, UserID: userID},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if write && access.Role == dbgen.WorkspaceRoleViewer {
		return "", ErrForbidden
	}

	return access.Role, nil
}

func parseUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil || !parsed.Valid {
		return pgtype.UUID{}, ErrInvalidIdentity
	}
	return parsed, nil
}

func formatUUID(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}

	encoded := hex.EncodeToString(value.Bytes[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func mapApplication(row dbgen.Application) Application {
	return Application{
		ID:              formatUUID(row.ID),
		WorkspaceID:     formatUUID(row.WorkspaceID),
		Name:            row.Name,
		Slug:            row.Slug,
		Description:     row.Description,
		Lifecycle:       Lifecycle(row.Lifecycle),
		DefaultHostname: row.DefaultHostname,
		CreatedBy:       formatUUID(row.CreatedBy),
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}
}

func isUniqueViolation(err error) bool {
	var pgError *pgconn.PgError
	return errors.As(err, &pgError) && pgError.Code == "23505"
}
