//ff:func feature=pkg-queue type=util control=selection
//ff:what 트랜잭션 내에서 토픽에 메시지를 발행한다 (atomicity 보장)
package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// ErrTxUnsupported is returned by PublishTx when the active backend does not
// support transaction-bound publishing (e.g. the in-memory backend).
var ErrTxUnsupported = errors.New("queue: tx-bound publish not supported by current backend")

// PublishTx sends a message to the given topic, using the provided *sql.Tx so
// that the enqueue INSERT participates in the caller's transaction. On commit,
// the message becomes visible to pollers; on rollback, no trace remains.
//
// Only the "postgres" backend supports this. The "memory" backend returns
// ErrTxUnsupported because synchronous handler invocation has no transactional
// semantics.
func PublishTx(ctx context.Context, tx *sql.Tx, topic string, payload any, opts ...PublishOption) error {
	mu.RLock()
	if !inited {
		mu.RUnlock()
		return ErrNotInitialized
	}
	b := backend
	mu.RUnlock()

	cfg := applyPublishOpts(opts)

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	switch b {
	case "postgres":
		if tx == nil {
			return errors.New("queue: PublishTx requires a non-nil *sql.Tx")
		}
		tp := extractTraceparent(ctx)
		_, err := tx.ExecContext(ctx, insertQueueSQL,
			topic, data, cfg.priority, deliverAtFor(cfg), tp)
		return err
	case "memory":
		return ErrTxUnsupported
	}

	return ErrUnknownBackend
}
