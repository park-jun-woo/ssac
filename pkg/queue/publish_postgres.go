//ff:func feature=pkg-queue type=util control=sequence
//ff:what postgres 백엔드에 메시지를 저장한다
package queue

import (
	"context"
	"time"
)

// insertQueueSQL is shared by publishPostgres (db-bound) and PublishTx (tx-bound).
const insertQueueSQL = `
	INSERT INTO fullend_queue (topic, payload, priority, deliver_at)
	VALUES ($1, $2, $3, $4)`

// deliverAtFor computes the deliver_at timestamp from publishConfig.
func deliverAtFor(cfg publishConfig) time.Time {
	deliverAt := time.Now()
	if cfg.delay > 0 {
		deliverAt = deliverAt.Add(time.Duration(cfg.delay) * time.Second)
	}
	return deliverAt
}

func publishPostgres(ctx context.Context, topic string, data []byte, cfg publishConfig) error {
	_, err := db.ExecContext(ctx, insertQueueSQL,
		topic, data, cfg.priority, deliverAtFor(cfg))
	return err
}
