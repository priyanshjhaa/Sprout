package apitoken

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/priyanshjhaa/Sprout/backend/internal/database/dbgen"
)

type SQLRepository struct{ queries *dbgen.Queries }

func NewSQLRepository(queries *dbgen.Queries) *SQLRepository { return &SQLRepository{queries: queries} }

func parseID(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil || !id.Valid {
		return id, ErrInvalid
	}
	return id, nil
}

func (r *SQLRepository) Insert(ctx context.Context, userID, name, hash, hint string, expiresAt time.Time) (Token, error) {
	user, err := parseID(userID)
	if err != nil {
		return Token{}, err
	}
	row, err := r.queries.InsertAPIToken(ctx, dbgen.InsertAPITokenParams{
		UserID: user, Name: name, TokenHash: hash, Hint: hint,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return Token{}, err
	}
	return mapToken(row.ID, row.Name, row.Hint, row.CreatedAt, row.ExpiresAt, row.LastUsedAt), nil
}

func (r *SQLRepository) List(ctx context.Context, userID string) ([]Token, error) {
	user, err := parseID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListAPITokens(ctx, user)
	if err != nil {
		return nil, err
	}
	tokens := make([]Token, 0, len(rows))
	for _, row := range rows {
		tokens = append(tokens, mapToken(row.ID, row.Name, row.Hint, row.CreatedAt, row.ExpiresAt, row.LastUsedAt))
	}
	return tokens, nil
}

func (r *SQLRepository) CountActive(ctx context.Context, userID string) (int, error) {
	user, err := parseID(userID)
	if err != nil {
		return 0, err
	}
	count, err := r.queries.CountActiveAPITokens(ctx, user)
	return int(count), err
}

func (r *SQLRepository) Revoke(ctx context.Context, userID, id string) (bool, error) {
	user, err := parseID(userID)
	if err != nil {
		return false, err
	}
	token, err := parseID(id)
	if err != nil {
		return false, nil // an unparseable ID names no token
	}
	count, err := r.queries.RevokeAPIToken(ctx, dbgen.RevokeAPITokenParams{ID: token, UserID: user})
	return count > 0, err
}

func (r *SQLRepository) FindByHash(ctx context.Context, hash string) (string, string, error) {
	row, err := r.queries.GetAPITokenByHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", err
	}
	return row.ID.String(), row.UserID.String(), nil
}

func (r *SQLRepository) Touch(ctx context.Context, tokenID string) error {
	id, err := parseID(tokenID)
	if err != nil {
		return err
	}
	return r.queries.TouchAPIToken(ctx, id)
}

func mapToken(id pgtype.UUID, name, hint string, created, expires, lastUsed pgtype.Timestamptz) Token {
	token := Token{ID: id.String(), Name: name, Hint: hint, CreatedAt: created.Time, ExpiresAt: expires.Time}
	if lastUsed.Valid {
		token.LastUsedAt = &lastUsed.Time
	}
	return token
}
