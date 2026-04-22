//ff:type feature=pkg-auth type=store topic=auth-refresh
//ff:what sha256 해시 기반 refresh token DB 저장소 (JSONB claims + rotation + revoke)
package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrRefreshTokenNotFound — token absent or expired.
var ErrRefreshTokenNotFound = errors.New("refresh token not found or revoked")

// ErrRefreshTokenReused — token previously revoked but presented again.
// Indicates potential token theft; caller may choose to revoke the entire
// claim-family session (see RefreshStore.DetectReuseLogoutAll).
var ErrRefreshTokenReused = errors.New("refresh token reuse detected")

// RefreshStore persists refresh-token hashes for one-time-use rotation.
// The plaintext refresh token is never stored; only its sha256 hex digest.
// The associated claims are persisted as JSONB so rotation and bulk-revoke
// operate without any knowledge of the claim schema.
type RefreshStore struct {
	DB *sql.DB
	// DetectReuseLogoutAll controls whether a revoked-token reuse attempt
	// triggers revocation of every active token sharing the same claim set.
	// OWASP-recommended breach response; off by default.
	DetectReuseLogoutAll bool
}

// ClaimMatcher selects refresh-token rows whose JSONB claims contain every
// key/value in the matcher. Passed verbatim to Postgres `claims @> $1`.
type ClaimMatcher map[string]any

// hashRefreshToken returns the sha256 hex digest of a refresh-token string.
// Shared by Create / Consume / Revoke so the DB never stores plaintext.
func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Create persists a new refresh token. `token` is the plaintext JWT
// returned to the client; only its hash is stored. `claims` is marshaled
// to JSON and written to the JSONB column unchanged.
func (s *RefreshStore) Create(ctx context.Context, token string, claims any, expiresAt time.Time) error {
	if s == nil || s.DB == nil {
		return errors.New("refresh store: DB not configured")
	}
	raw, err := marshalClaimsJSON(claims)
	if err != nil {
		return fmt.Errorf("refresh store: marshal claims: %w", err)
	}
	_, err = s.DB.ExecContext(ctx,
		`INSERT INTO refresh_tokens (token_hash, claims, expires_at) VALUES ($1, $2, $3)`,
		hashRefreshToken(token), raw, expiresAt,
	)
	return err
}

// Consume implements one-time-use rotation: it looks up the token by hash,
// verifies it is active, marks it revoked, and returns the associated
// claims as raw JSON. A previously revoked token surfaces as
// ErrRefreshTokenReused together with the revoked-row claims so the caller
// can enforce DetectReuseLogoutAll.
func (s *RefreshStore) Consume(ctx context.Context, token string) (json.RawMessage, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("refresh store: DB not configured")
	}
	hash := hashRefreshToken(token)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var claims json.RawMessage
	var expiresAt time.Time
	var revokedAt sql.NullTime
	row := tx.QueryRowContext(ctx,
		`SELECT claims, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1`,
		hash,
	)
	if err := row.Scan(&claims, &expiresAt, &revokedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRefreshTokenNotFound
		}
		return nil, err
	}
	if revokedAt.Valid {
		return claims, ErrRefreshTokenReused
	}
	if time.Now().After(expiresAt) {
		return nil, ErrRefreshTokenNotFound
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`,
		hash,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return claims, nil
}

// Revoke marks a single refresh token as revoked (idempotent).
func (s *RefreshStore) Revoke(ctx context.Context, token string) error {
	if s == nil || s.DB == nil {
		return errors.New("refresh store: DB not configured")
	}
	_, err := s.DB.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`,
		hashRefreshToken(token),
	)
	return err
}

// RevokeAll revokes every active refresh token whose JSONB claims contain
// every key/value in `matcher`. Called when a revoked-token reuse is
// detected and DetectReuseLogoutAll is true. Empty matcher is rejected to
// prevent accidental full-table revocation.
func (s *RefreshStore) RevokeAll(ctx context.Context, matcher ClaimMatcher) error {
	if s == nil || s.DB == nil {
		return errors.New("refresh store: DB not configured")
	}
	if len(matcher) == 0 {
		return errors.New("refresh store: empty matcher rejected")
	}
	raw, err := json.Marshal(matcher)
	if err != nil {
		return fmt.Errorf("refresh store: marshal matcher: %w", err)
	}
	_, err = s.DB.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE revoked_at IS NULL AND claims @> $1`,
		raw,
	)
	return err
}

// marshalClaimsJSON normalizes the passthrough claims to a JSON blob.
// nil claims becomes "{}" so the JSONB column never receives NULL.
func marshalClaimsJSON(in any) ([]byte, error) {
	if in == nil {
		return []byte(`{}`), nil
	}
	if raw, ok := in.(json.RawMessage); ok {
		return raw, nil
	}
	return json.Marshal(in)
}
