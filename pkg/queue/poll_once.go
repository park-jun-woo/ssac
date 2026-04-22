//ff:func feature=pkg-queue type=util control=iteration dimension=1
//ff:what DB에서 대기 중인 메시지를 한 배치 처리한다 (traceparent 복원 포함)
package queue

import "context"

// pollOnce processes one batch of pending messages from the database.
//
// Each row's `traceparent` column is used to reconstruct the W3C
// TraceContext span context before dispatching, so subscriber handlers
// automatically become children of the original Publish-time span. When
// the column is empty (publisher tracing disabled or row predates the
// column), dispatch runs under the poller's ambient ctx with no extra
// parent — observability falls back silently.
func pollOnce(ctx context.Context) error {
	mu.RLock()
	hs := handlers
	mu.RUnlock()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, topic, payload, traceparent FROM fullend_queue
		WHERE status = 'pending' AND deliver_at <= NOW()
		ORDER BY
			CASE priority WHEN 'high' THEN 0 WHEN 'normal' THEN 1 ELSE 2 END,
			id
		FOR UPDATE SKIP LOCKED
		LIMIT 100`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var topic string
		var payload []byte
		var traceparent string
		if err := rows.Scan(&id, &topic, &payload, &traceparent); err != nil {
			return err
		}

		msgCtx := injectTraceparent(ctx, traceparent)
		status := dispatchMessage(msgCtx, hs, topic, payload)

		_, err := tx.ExecContext(ctx, `
			UPDATE fullend_queue SET status = $1, processed_at = NOW() WHERE id = $2`,
			status, id)
		if err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	return tx.Commit()
}
