//ff:func feature=pkg-queue type=util control=sequence
//ff:what postgres 백엔드에 메시지를 저장한다
package queue

import (
	"context"
	"time"
)

func publishPostgres(ctx context.Context, topic string, data []byte, cfg publishConfig) error {
	deliverAt := time.Now()
	if cfg.delay > 0 {
		deliverAt = deliverAt.Add(time.Duration(cfg.delay) * time.Second)
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO fullend_queue (topic, payload, priority, deliver_at)
		VALUES ($1, $2, $3, $4)`,
		topic, data, cfg.priority, deliverAt)
	return err
}
