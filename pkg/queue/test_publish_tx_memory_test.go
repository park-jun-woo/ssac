//ff:func feature=pkg-queue type=test control=sequence
//ff:what memory 백엔드에서 PublishTx가 ErrTxUnsupported를 반환하는지 검증한다
package queue

import (
	"context"
	"errors"
	"testing"
)

func TestPublishTxMemoryUnsupported(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	if err := Init(ctx, "memory", nil); err != nil {
		t.Fatal(err)
	}
	defer Close()

	err := PublishTx(ctx, nil, "test.topic", map[string]string{"k": "v"})
	if !errors.Is(err, ErrTxUnsupported) {
		t.Errorf("expected ErrTxUnsupported, got %v", err)
	}
}

func TestPublishTxNotInitialized(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	err := PublishTx(ctx, nil, "test.topic", map[string]string{"k": "v"})
	if !errors.Is(err, ErrNotInitialized) {
		t.Errorf("expected ErrNotInitialized, got %v", err)
	}
}
