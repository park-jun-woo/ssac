//ff:func feature=pkg-queue type=util control=sequence topic=tracing
//ff:what postgres 백엔드에 메시지를 저장한다 (W3C traceparent 포함)
package queue

import (
	"context"
	"time"
)

// insertQueueSQL is shared by publishPostgres (db-bound) and PublishTx (tx-bound).
//
// The 5th column `traceparent` carries the W3C TraceContext header so
// downstream pollers can reconstruct the span context before invoking
// handlers. When tracing is disabled (no global tracer provider, no
// active span), the value is an empty string and consumers short-circuit.
const insertQueueSQL = `
	INSERT INTO fullend_queue (topic, payload, priority, deliver_at, traceparent)
	VALUES ($1, $2, $3, $4, $5)`

// deliverAtFor computes the deliver_at timestamp from publishConfig.
func deliverAtFor(cfg publishConfig) time.Time {
	deliverAt := time.Now()
	if cfg.delay > 0 {
		deliverAt = deliverAt.Add(time.Duration(cfg.delay) * time.Second)
	}
	return deliverAt
}

func publishPostgres(ctx context.Context, topic string, data []byte, cfg publishConfig) error {
	tp := extractTraceparent(ctx)
	_, err := db.ExecContext(ctx, insertQueueSQL,
		topic, data, cfg.priority, deliverAtFor(cfg), tp)
	return err
}
