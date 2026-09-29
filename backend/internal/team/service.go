package team

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
	"github.com/priyanshjhaa/Sprout/backend/internal/identity"
)

var (
	ErrInvalid    = errors.New("invalid team request")
	ErrNotFound   = errors.New("team resource not found")
	ErrForbidden  = errors.New("workspace owner required")
	ErrConflict   = errors.New("member or pending invitation already exists, or invitation limit reached")
	ErrInvitation = errors.New("invitation unavailable or verified email does not match")
	ErrIdentity   = errors.New("verified identity unavailable")
)

type Invitation struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type CreatedInvitation struct {
	Invitation
	Token string `json:"token"`
}
type Service struct {
	pool     *pgxpool.Pool
	profiles identity.ProfileProvider
}

func NewService(pool *pgxpool.Pool, profiles identity.ProfileProvider) *Service {
	return &Service{pool: pool, profiles: profiles}
}

func uuid(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil || !id.Valid {
		return id, ErrInvalid
	}
	return id, nil
}
func validRole(role string) bool { return role == "editor" || role == "viewer" }
func digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Every team mutation locks the workspace first, so acceptance, revocation and
// removal cannot interleave. The caller owns commit; deferred rollback is safe
// after commit and releases the connection on all error paths.
func (s *Service) begin(ctx context.Context, slug string) (pgx.Tx, *dbgen.Queries, dbgen.LockTeamWorkspaceRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, dbgen.LockTeamWorkspaceRow{}, err
	}
	q := dbgen.New(tx)
	workspace, err := q.LockTeamWorkspace(ctx, slug)
	if err != nil {
		rollback(tx)
		if errors.Is(err, pgx.ErrNoRows) {
			err = ErrNotFound
		}
		return nil, nil, workspace, err
	}
	return tx, q, workspace, nil
}
func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
func owner(ctx context.Context, q *dbgen.Queries, workspace pgtype.UUID, actor string) (pgtype.UUID, error) {
	id, err := uuid(actor)
	if err != nil {
		return id, err
	}
	role, err := q.GetTeamRole(ctx, dbgen.GetTeamRoleParams{WorkspaceID: workspace, UserID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return id, ErrNotFound
	}
	if err != nil {
		return id, err
	}
	if role != dbgen.WorkspaceRoleOwner {
		return id, ErrForbidden
	}
	return id, nil
}

func (s *Service) Workspaces(ctx context.Context, actor string) ([]identity.Workspace, error) {
	id, err := uuid(actor)
	if err != nil {
		return nil, err
	}
	rows, err := dbgen.New(s.pool).ListMyWorkspaces(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]identity.Workspace, 0, len(rows))
	for _, r := range rows {
		result = append(result, identity.Workspace{ID: r.ID.String(), Slug: r.Slug, Name: r.Name})
	}
	return result, nil
}
func (s *Service) List(ctx context.Context, slug, actor string) ([]Invitation, error) {
	tx, q, w, err := s.begin(ctx, slug)
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	if _, err = owner(ctx, q, w.ID, actor); err != nil {
		return nil, err
	}
	rows, err := q.ListPendingInvitations(ctx, w.ID)
	if err != nil {
		return nil, err
	}
	result := make([]Invitation, 0, len(rows))
	for _, r := range rows {
		result = append(result, Invitation{r.ID.String(), r.Email, string(r.Role), r.ExpiresAt.Time})
	}
	return result, tx.Commit(ctx)
}
func (s *Service) Create(ctx context.Context, slug, actor, email, role string) (CreatedInvitation, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 320 || !validRole(role) {
		return CreatedInvitation{}, ErrInvalid
	}
	tx, q, w, err := s.begin(ctx, slug)
	if err != nil {
		return CreatedInvitation{}, err
	}
	defer rollback(tx)
	actorID, err := owner(ctx, q, w.ID, actor)
	if err != nil {
		return CreatedInvitation{}, err
	}
	member, err := q.TeamHasEmail(ctx, dbgen.TeamHasEmailParams{WorkspaceID: w.ID, Email: email})
	if err != nil {
		return CreatedInvitation{}, err
	}
	if member {
		return CreatedInvitation{}, ErrConflict
	}
	if err = q.ExpireTeamInvitations(ctx, w.ID); err != nil {
		return CreatedInvitation{}, err
	}
	pending, err := q.ListPendingInvitations(ctx, w.ID)
	if err != nil {
		return CreatedInvitation{}, err
	}
	if len(pending) >= 100 {
		return CreatedInvitation{}, ErrConflict
	}
	bytes := make([]byte, 32)
	if _, err = rand.Read(bytes); err != nil {
		return CreatedInvitation{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	row, err := q.CreateTeamInvitation(ctx, dbgen.CreateTeamInvitationParams{WorkspaceID: w.ID, Email: email, Role: dbgen.WorkspaceRole(role), TokenHash: digest(token), CreatedBy: actorID})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return CreatedInvitation{}, ErrConflict
	}
	if err != nil {
		return CreatedInvitation{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CreatedInvitation{}, err
	}
	return CreatedInvitation{Invitation{row.ID.String(), row.Email, string(row.Role), row.ExpiresAt.Time}, token}, nil
}
func (s *Service) Accept(ctx context.Context, actor, subject, token string) (identity.Workspace, error) {
	empty := identity.Workspace{}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 32 || len(token) != 43 {
		return empty, ErrInvitation
	}
	actorID, err := uuid(actor)
	if err != nil {
		return empty, err
	}
	q := dbgen.New(s.pool)
	matches, err := q.CheckInvitationIdentity(ctx, dbgen.CheckInvitationIdentityParams{ID: actorID, AuthSubject: subject})
	if err != nil {
		return empty, err
	}
	if !matches {
		return empty, ErrInvitation
	}
	// Do not trust cached users.email: Clerk must verify the current primary email.
	profile, err := s.profiles.GetProfile(ctx, subject)
	if err != nil {
		return empty, ErrIdentity
	}
	slug, err := q.FindInvitationWorkspace(ctx, digest(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, ErrInvitation
	}
	if err != nil {
		return empty, err
	}
	tx, q, w, err := s.begin(ctx, slug)
	if err != nil {
		return empty, err
	}
	defer rollback(tx)
	role, err := q.ConsumeTeamInvitation(ctx, dbgen.ConsumeTeamInvitationParams{WorkspaceID: w.ID, TokenHash: digest(token), Email: strings.ToLower(strings.TrimSpace(profile.Email))})
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, ErrInvitation
	}
	if err != nil {
		return empty, err
	}
	if err = q.JoinTeam(ctx, dbgen.JoinTeamParams{WorkspaceID: w.ID, UserID: actorID, Role: role}); err != nil {
		return empty, err
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, err
	}
	return identity.Workspace{ID: w.ID.String(), Slug: w.Slug, Name: w.Name}, nil
}
func (s *Service) Revoke(ctx context.Context, slug, actor, invitation string) error {
	id, err := uuid(invitation)
	if err != nil {
		return err
	}
	tx, q, w, err := s.begin(ctx, slug)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if _, err = owner(ctx, q, w.ID, actor); err != nil {
		return err
	}
	count, err := q.RevokeTeamInvitation(ctx, dbgen.RevokeTeamInvitationParams{WorkspaceID: w.ID, ID: id})
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}
func (s *Service) ChangeRole(ctx context.Context, slug, actor, target, role string) error {
	if !validRole(role) {
		return ErrInvalid
	}
	return s.changeMember(ctx, slug, actor, target, role, false)
}
func (s *Service) Remove(ctx context.Context, slug, actor, target string) error {
	return s.changeMember(ctx, slug, actor, target, "", true)
}
func (s *Service) changeMember(ctx context.Context, slug, actor, target, role string, remove bool) error {
	id, err := uuid(target)
	if err != nil {
		return err
	}
	tx, q, w, err := s.begin(ctx, slug)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if _, err = owner(ctx, q, w.ID, actor); err != nil {
		return err
	}
	current, err := q.GetTeamRole(ctx, dbgen.GetTeamRoleParams{WorkspaceID: w.ID, UserID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if current == dbgen.WorkspaceRoleOwner {
		return ErrForbidden
	}
	if remove {
		if _, err = q.RemoveTeamMember(ctx, dbgen.RemoveTeamMemberParams{WorkspaceID: w.ID, UserID: id}); err != nil {
			return err
		}
		if err = q.RemoveTeamMemberGrants(ctx, dbgen.RemoveTeamMemberGrantsParams{WorkspaceID: w.ID, UserID: id}); err != nil {
			return err
		}
		if err = q.RevokeMemberInvitations(ctx, dbgen.RevokeMemberInvitationsParams{WorkspaceID: w.ID, UserID: id}); err != nil {
			return err
		}
	} else {
		if _, err = q.ChangeTeamRole(ctx, dbgen.ChangeTeamRoleParams{WorkspaceID: w.ID, UserID: id, Role: dbgen.WorkspaceRole(role)}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
