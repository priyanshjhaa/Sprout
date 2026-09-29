package sharing

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/priyanshjhaa/Sprout/backend/internal/authorization"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
)

var (
	ErrNotFound  = errors.New("workspace, application, member, or grant not found")
	ErrForbidden = errors.New("not permitted to manage access")
	ErrInvalid   = errors.New("invalid access request")
)

type Member struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

type Grant struct {
	Member
}

type Access struct {
	Mode      string  `json:"accessMode"`
	CreatedBy string  `json:"createdBy"`
	Grants    []Grant `json:"grants"`
}

type Service struct {
	queries *dbgen.Queries
}

func NewService(queries *dbgen.Queries) *Service {
	return &Service{queries: queries}
}

func (service *Service) ListMembers(ctx context.Context, workspaceSlug, actorID string) ([]Member, error) {
	actor, err := parseUUID(actorID)
	if err != nil {
		return nil, ErrInvalid
	}
	if _, err := service.queries.GetWorkspaceAccess(ctx, dbgen.GetWorkspaceAccessParams{
		WorkspaceSlug: workspaceSlug, UserID: actor,
	}); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	rows, err := service.queries.ListWorkspaceMembers(ctx, dbgen.ListWorkspaceMembersParams{
		WorkspaceSlug: workspaceSlug, ActorID: actor,
	})
	if err != nil {
		return nil, err
	}
	members := make([]Member, 0, len(rows))
	for _, row := range rows {
		members = append(members, Member{
			ID: row.ID.String(), Email: row.Email, DisplayName: row.DisplayName, Role: string(row.Role),
		})
	}
	return members, nil
}

func (service *Service) GetAccess(ctx context.Context, workspaceSlug, applicationID, actorID string) (Access, error) {
	permission, application, actor, creatorID, err := service.manager(ctx, workspaceSlug, applicationID, actorID)
	if err != nil {
		return Access{}, err
	}
	rows, err := service.queries.ListApplicationGrants(ctx, dbgen.ListApplicationGrantsParams{
		WorkspaceSlug: workspaceSlug, ApplicationID: application, ActorID: actor,
	})
	if err != nil {
		return Access{}, err
	}
	grants := make([]Grant, 0, len(rows))
	for _, row := range rows {
		grants = append(grants, Grant{Member: Member{
			ID: row.ID.String(), Email: row.Email, DisplayName: row.DisplayName, Role: string(row.Role),
		}})
	}
	return Access{Mode: string(permission.Mode), CreatedBy: creatorID, Grants: grants}, nil
}

func (service *Service) SetMode(ctx context.Context, workspaceSlug, applicationID, actorID, mode string) (Access, error) {
	if mode != string(authorization.WorkspaceWide) && mode != string(authorization.Restricted) {
		return Access{}, ErrInvalid
	}
	_, application, actor, _, err := service.manager(ctx, workspaceSlug, applicationID, actorID)
	if err != nil {
		return Access{}, err
	}
	_, err = service.queries.SetApplicationAccessMode(ctx, dbgen.SetApplicationAccessModeParams{
		AccessMode: dbgen.ApplicationAccessMode(mode), ApplicationID: application,
		WorkspaceSlug: workspaceSlug, ActorID: actor,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Access{}, ErrNotFound
	}
	if err != nil {
		return Access{}, err
	}
	return service.GetAccess(ctx, workspaceSlug, applicationID, actorID)
}

func (service *Service) Grant(ctx context.Context, workspaceSlug, applicationID, actorID, targetID, role string) (Access, error) {
	if role != string(authorization.ApplicationEditor) && role != string(authorization.ApplicationViewer) {
		return Access{}, ErrInvalid
	}
	_, application, actor, _, err := service.manager(ctx, workspaceSlug, applicationID, actorID)
	if err != nil {
		return Access{}, err
	}
	target, err := parseUUID(targetID)
	if err != nil {
		return Access{}, ErrInvalid
	}
	_, err = service.queries.UpsertApplicationGrant(ctx, dbgen.UpsertApplicationGrantParams{
		Role: dbgen.ApplicationRole(role), ActorID: actor, TargetID: target,
		WorkspaceSlug: workspaceSlug, ApplicationID: application,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Access{}, ErrNotFound
	}
	if err != nil {
		return Access{}, err
	}
	return service.GetAccess(ctx, workspaceSlug, applicationID, actorID)
}

func (service *Service) Revoke(ctx context.Context, workspaceSlug, applicationID, actorID, targetID string) (Access, error) {
	_, application, actor, _, err := service.manager(ctx, workspaceSlug, applicationID, actorID)
	if err != nil {
		return Access{}, err
	}
	target, err := parseUUID(targetID)
	if err != nil {
		return Access{}, ErrInvalid
	}
	_, err = service.queries.RevokeApplicationGrant(ctx, dbgen.RevokeApplicationGrantParams{
		TargetID: target, ApplicationID: application, WorkspaceSlug: workspaceSlug, ActorID: actor,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Access{}, ErrNotFound
	}
	if err != nil {
		return Access{}, err
	}
	return service.GetAccess(ctx, workspaceSlug, applicationID, actorID)
}

func (service *Service) manager(
	ctx context.Context, workspaceSlug, applicationID, actorID string,
) (authorization.ApplicationAccess, pgtype.UUID, pgtype.UUID, string, error) {
	application, err := parseUUID(applicationID)
	if err != nil {
		return authorization.ApplicationAccess{}, pgtype.UUID{}, pgtype.UUID{}, "", ErrInvalid
	}
	actor, err := parseUUID(actorID)
	if err != nil {
		return authorization.ApplicationAccess{}, pgtype.UUID{}, pgtype.UUID{}, "", ErrInvalid
	}
	row, err := service.queries.GetApplicationAuthorization(ctx, dbgen.GetApplicationAuthorizationParams{
		UserID: actor, WorkspaceSlug: workspaceSlug, ApplicationID: application,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return authorization.ApplicationAccess{}, pgtype.UUID{}, pgtype.UUID{}, "", ErrNotFound
	}
	if err != nil {
		return authorization.ApplicationAccess{}, pgtype.UUID{}, pgtype.UUID{}, "", err
	}
	grant := authorization.ApplicationRole("")
	if row.GrantRole.Valid {
		grant = authorization.ApplicationRole(row.GrantRole.ApplicationRole)
	}
	permission := authorization.ApplicationAccess{
		WorkspaceRole: authorization.WorkspaceRole(row.WorkspaceRole),
		Grant:         grant, Mode: authorization.AccessMode(row.AccessMode), IsCreator: row.IsCreator,
	}
	if !permission.CanView() {
		return authorization.ApplicationAccess{}, pgtype.UUID{}, pgtype.UUID{}, "", ErrNotFound
	}
	if !permission.CanManageAccess() {
		return authorization.ApplicationAccess{}, pgtype.UUID{}, pgtype.UUID{}, "", ErrForbidden
	}
	return permission, application, actor, row.CreatedBy.String(), nil
}

func parseUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil || !parsed.Valid {
		return pgtype.UUID{}, ErrInvalid
	}
	return parsed, nil
}
