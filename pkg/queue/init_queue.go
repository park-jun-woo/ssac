//ff:func feature=pkg-queue type=loader control=selection
//ff:what 큐 백엔드를 초기화한다
package queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrNotInitialized = errors.New("queue: not initialized, call Init first")
	ErrUnknownBackend = errors.New("queue: unknown backend")
)

// singleton state
var (
	mu       sync.RWMutex
	handlers map[string][]func(ctx context.Context, msg []byte) error
	backend  string
	db       *sql.DB
	cancel   context.CancelFunc
	done     chan struct{}
	inited   bool
)

// Init initializes the queue with the given backend ("postgres" or "memory").
// For "postgres", db must be non-nil; the fullend_queue table is auto-created.
func Init(ctx context.Context, b string, d *sql.DB) error {
	mu.Lock()
	defer mu.Unlock()

	switch b {
	case "postgres":
		if err := initPostgres(ctx, d); err != nil {
			return err
		}
		db = d
	case "memory":
		// no setup needed
	default:
		return fmt.Errorf("%w: %s", ErrUnknownBackend, b)
	}

	backend = b
	handlers = make(map[string][]func(ctx context.Context, msg []byte) error)
	inited = true
	return nil
}
