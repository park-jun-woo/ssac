//ff:type feature=pkg-queue type=model
//ff:what Backend interface — queue 포트 (Publish / PublishTx 구현체 외부 주입)
package queue

import "context"

// Backend is the interface implemented by queue backends. The package-level
// Publish/PublishTx functions delegate to the currently installed Backend.
//
// memory backend ships in ssac for tests and zero-config dev. A postgres (or
// other durable) backend is provided by yongol codegen from the user's sqlc
// Queries via pkg/queue/interface.yaml ports (QueuePublish / QueuePoll /
// QueueAck). ssac itself never imports database/sql or pgx.
//
// PublishTx accepts tx as `any` so both database/sql (*sql.Tx) and
// jackc/pgx (pgx.Tx) implementations are representable without ssac binding
// to a specific driver. The concrete Backend asserts the expected type.
type Backend interface {
	// Publish enqueues a serialized payload on topic with the supplied
	// delivery config. The memory backend dispatches handlers synchronously;
	// durable backends persist the row and return.
	Publish(ctx context.Context, topic string, data []byte, cfg PublishConfig) error

	// PublishTx enqueues inside the caller's transaction. tx is driver-
	// specific; backends that do not support transactional publishing
	// (e.g. memory) return ErrTxUnsupported.
	PublishTx(ctx context.Context, tx any, topic string, data []byte, cfg PublishConfig) error
}
