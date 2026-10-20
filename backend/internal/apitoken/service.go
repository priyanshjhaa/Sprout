// Package apitoken issues and verifies personal access tokens: long random
// secrets that let the CLI, CI and coding agents call the deploy API as a user.
// A token is shown once, when it is created. Only its SHA-256 hash is stored, so
// a database leak does not reveal usable tokens.
package apitoken

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// Prefix makes tokens recognisable by secret scanners and by the API, which
	// uses it to decide between a token and a browser session.
	Prefix = "sprout_pat_"

	DefaultTTL = 90 * 24 * time.Hour
	MinTTL     = time.Hour
	MaxTTL     = 365 * 24 * time.Hour
	// MaxActive bounds how many unrevoked, unexpired tokens one user may hold.
	MaxActive = 20
	maxName   = 80

	secretBytes = 32 // 256 random bits
)

var (
	ErrInvalid         = errors.New("token request is invalid")
	ErrLimit           = errors.New("too many active tokens")
	ErrNotFound        = errors.New("token not found")
	ErrUnauthenticated = errors.New("token is not valid")
)

// Token is what the API may show about a token. It never includes the secret
// or its hash.
type Token struct {
	ID         string
	Name       string
	Hint       string // last characters of the secret, to tell tokens apart
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastUsedAt *time.Time
}

type Repository interface {
	Insert(ctx context.Context, userID, name, hash, hint string, expiresAt time.Time) (Token, error)
	List(ctx context.Context, userID string) ([]Token, error)
	CountActive(ctx context.Context, userID string) (int, error)
	// Revoke reports whether an active token owned by userID was revoked.
	Revoke(ctx context.Context, userID, id string) (bool, error)
	// FindByHash returns the owner of an unrevoked, unexpired token.
	FindByHash(ctx context.Context, hash string) (tokenID, userID string, err error)
	Touch(ctx context.Context, tokenID string) error
}

type Service struct {
	repository Repository
	random     io.Reader
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, random: rand.Reader, now: time.Now}
}

// Create makes a token for userID and returns it with its secret. This is the
// only time the secret exists outside the caller: it is not recoverable later.
func (s *Service) Create(ctx context.Context, userID, name string, ttl time.Duration) (Token, string, error) {
	name = strings.TrimSpace(name)
	if userID == "" || name == "" || utf8.RuneCountInString(name) > maxName || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return Token{}, "", ErrInvalid
	}
	if ttl == 0 {
		ttl = DefaultTTL
	}
	if ttl < MinTTL || ttl > MaxTTL {
		return Token{}, "", ErrInvalid
	}
	active, err := s.repository.CountActive(ctx, userID)
	if err != nil {
		return Token{}, "", err
	}
	if active >= MaxActive {
		return Token{}, "", ErrLimit
	}
	secret, err := s.newSecret()
	if err != nil {
		return Token{}, "", err
	}
	token, err := s.repository.Insert(ctx, userID, name, hash(secret), secret[len(secret)-4:], s.now().Add(ttl))
	if err != nil {
		return Token{}, "", err
	}
	return token, secret, nil
}

func (s *Service) List(ctx context.Context, userID string) ([]Token, error) {
	return s.repository.List(ctx, userID)
}

func (s *Service) Revoke(ctx context.Context, userID, id string) error {
	revoked, err := s.repository.Revoke(ctx, userID, id)
	if err != nil {
		return err
	}
	if !revoked {
		return ErrNotFound
	}
	return nil
}

// Authenticate returns the user a presented secret belongs to. Anything that is
// not shaped like a Sprout token is rejected without touching the database.
func (s *Service) Authenticate(ctx context.Context, presented string) (string, error) {
	if !wellFormed(presented) {
		return "", ErrUnauthenticated
	}
	tokenID, userID, err := s.repository.FindByHash(ctx, hash(presented))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrUnauthenticated
		}
		return "", err
	}
	// "Last used" is informational: failing to record it must not fail the request.
	_ = s.repository.Touch(ctx, tokenID)
	return userID, nil
}

// IsToken reports whether a bearer credential is a Sprout token (as opposed to
// a browser session), by its prefix alone.
func IsToken(credential string) bool { return strings.HasPrefix(credential, Prefix) }

var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func (s *Service) newSecret() (string, error) {
	raw := make([]byte, secretBytes)
	if _, err := io.ReadFull(s.random, raw); err != nil {
		return "", err
	}
	return Prefix + strings.ToLower(encoding.EncodeToString(raw)), nil
}

// 32 bytes encode to 52 base32 characters.
func wellFormed(secret string) bool {
	body, ok := strings.CutPrefix(secret, Prefix)
	if !ok || len(body) != encoding.EncodedLen(secretBytes) {
		return false
	}
	for _, r := range body {
		if !(r >= 'a' && r <= 'z' || r >= '2' && r <= '7') {
			return false
		}
	}
	return true
}

func hash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
