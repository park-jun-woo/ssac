//ff:func feature=pkg-queue type=loader control=sequence
//ff:what PostgreSQL 큐 테이블과 인덱스를 생성한다 (traceparent 컬럼 포함)

package queue

import (
	"context"
	"database/sql"
)

// initPostgres creates the fullend_queue table and index for PostgreSQL backend.
//
// The `traceparent` column carries a W3C TraceContext (RFC 9110 §3)
// propagation header captured at Publish time so a poller can reconstruct
// the caller's span context before dispatching the handler. It stays TEXT
// (no JSON/array overhead) and is optional — empty string when tracing is
// disabled. The ALTER TABLE ADD COLUMN IF NOT EXISTS keeps existing
// deployments backward compatible: older rows simply carry NULL/empty and
// dispatch falls back to the caller's ambient ctx.
func initPostgres(ctx context.Context, d *sql.DB) error {
	_, err := d.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS fullend_queue (
			id           BIGSERIAL PRIMARY KEY,
			topic        TEXT NOT NULL,
			payload      JSONB NOT NULL,
			priority     TEXT NOT NULL DEFAULT 'normal',
			status       TEXT NOT NULL DEFAULT 'pending',
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deliver_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			processed_at TIMESTAMPTZ,
			traceparent  TEXT NOT NULL DEFAULT ''
		)`)
	if err != nil {
		return err
	}
	// Backward-compat for tables created before the traceparent column
	// existed. Idempotent; no-op on fresh installs.
	_, err = d.ExecContext(ctx, `
		ALTER TABLE fullend_queue ADD COLUMN IF NOT EXISTS traceparent TEXT NOT NULL DEFAULT ''`)
	if err != nil {
		return err
	}
	_, err = d.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_fullend_queue_pending
		ON fullend_queue (topic, status, deliver_at) WHERE status = 'pending'`)
	return err
}
