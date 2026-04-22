//ff:type feature=pkg-auth type=model topic=auth-refresh
//ff:what refresh_tokens 테이블 DDL (Postgres JSONB 스키마)
package auth

// RefreshTokensDDL is the required schema for refresh-token rotation
// storage. Run once at deploy time (or via a migration tool).
//
// Claim-agnostic design: the full claim set is stored as JSONB so this
// package does not bind to any particular user_id column type. Bulk
// revoke-by-claim uses the Postgres `@>` containment operator backed by a
// GIN index on `claims`.
//
// Postgres-only (JSONB). MySQL/SQLite are out of scope for the refresh
// rotation runtime.
const RefreshTokensDDL = `CREATE TABLE IF NOT EXISTS refresh_tokens (
    token_hash  TEXT        PRIMARY KEY,
    claims      JSONB       NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS refresh_tokens_claims_idx
    ON refresh_tokens USING GIN (claims);`
