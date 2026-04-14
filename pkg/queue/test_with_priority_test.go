//ff:func feature=pkg-queue type=test control=sequence
//ff:what WithPriority 옵션으로 Publish해도 memory 백엔드에서 핸들러가 호출되는지 검증한다
package queue

import (
	"context"
	"testing"
)

func TestWithPriority(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	if err := Init(ctx, "memory", nil); err != nil {
		t.Fatal(err)
	}
	defer Close()

	called := false
	Subscribe("prio", func(_ context.Context, msg []byte) error {
		called = true
		return nil
	})

	if err := Publish(ctx, "prio", "data", WithPriority("high")); err != nil {
		t.Fatal(err)
	}

	if !called {
		t.Error("handler should be called with priority option on memory backend")
	}
}
