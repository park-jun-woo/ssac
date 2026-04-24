//ff:type feature=pkg-queue type=model
//ff:what memoryBackend — 테스트/개발용 in-memory Backend 구현
package queue

import (
	"context"
	"errors"
)

// ErrTxUnsupported is returned by Backend.PublishTx when the active backend
// does not support transaction-bound publishing (memory backend).
var ErrTxUnsupported = errors.New("queue: tx-bound publish not supported by current backend")

// memoryBackend dispatches handlers synchronously — no durability, no delay,
// no priority ordering. Intended for tests and zero-config dev. Production
// deployments swap in a durable Backend via SetBackend.
type memoryBackend struct{}

func newMemoryBackend() Backend { return &memoryBackend{} }

func (m *memoryBackend) Publish(ctx context.Context, topic string, data []byte, _ PublishConfig) error {
	mu.RLock()
	hs := handlers[topic]
	mu.RUnlock()
	for _, h := range hs {
		if err := h(ctx, data); err != nil {
			return err
		}
	}
	return nil
}

func (m *memoryBackend) PublishTx(_ context.Context, _ any, _ string, _ []byte, _ PublishConfig) error {
	return ErrTxUnsupported
}
