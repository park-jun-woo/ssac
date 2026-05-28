//ff:type feature=pkg-auth type=store topic=auth-refresh
//ff:what RefreshStore interface — refresh token 저장소 계약 (memory + yongol postgres 구현체)
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrRefreshTokenNotFound — token absent or expired.
var ErrRefreshTokenNotFound = errors.New("refresh token not found or revoked")

// ErrRefreshTokenReused — token previously revoked but presented again.
// Indicates potential token theft; callers wire DetectReuseLogoutAll via
// the store implementation's options if bulk-revoke is desired.
var ErrRefreshTokenReused = errors.New("refresh token reuse detected")

// ClaimMatcher selects refresh-token rows whose stored JSONB claims contain
// every key/value in the matcher. Postgres implementations use the `@>`
// containment operator; memory implementations walk the map.
type ClaimMatcher map[string]any

// RefreshStore persists sha256-hashed refresh tokens for one-time-use
// rotation. The plaintext refresh token is never stored. Implementations are
// injected by yongol codegen from the user's sqlc Queries (interface.yaml
// ports RefreshTokenInsert / RefreshTokenConsume / RefreshTokenCheckReuse /
// RefreshTokenRevoke / RefreshTokenRevokeAll) or provided via
// NewMemoryRefreshStore for tests.
type RefreshStore interface {
	// Create persists a new refresh token. token is the plaintext JWT
	// returned to the client; only its hash is stored. claims is marshaled
	// to JSON (raw) so the store stays claim-schema-agnostic.
	Create(ctx context.Context, token string, claims any, expiresAt time.Time) error

	// Consume implements one-time-use rotation: look up by hash, verify
	// active, mark revoked, return the claims as raw JSON. A previously
	// revoked token surfaces as ErrRefreshTokenReused together with the
	// revoked-row claims so callers can enforce reuse-detection lockout.
	Consume(ctx context.Context, token string) (json.RawMessage, error)

	// Revoke marks a single refresh token as revoked (idempotent).
	Revoke(ctx context.Context, token string) error

	// RevokeAll revokes every active refresh token whose claims contain
	// every key/value in matcher. Empty matcher must be rejected by the
	// implementation to prevent accidental full-table revocation.
	RevokeAll(ctx context.Context, matcher ClaimMatcher) error
}
