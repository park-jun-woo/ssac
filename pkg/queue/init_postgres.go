//ff:func feature=pkg-queue type=loader control=sequence
//ff:what PostgreSQL 큐 테이블과 인덱스를 생성한다

package queue

import (
	"context"
	"database/sql"
)

// initPostgres creates the fullend_queue table and index for PostgreSQL backend.
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
			processed_at TIMESTAMPTZ
		)`)
	if err != nil {
		return err
	}
	_, err = d.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_fullend_queue_pending
		ON fullend_queue (topic, status, deliver_at) WHERE status = 'pending'`)
	return err
}
