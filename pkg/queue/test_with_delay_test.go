//ff:func feature=pkg-queue type=test control=sequence
//ff:what WithDelay 옵션으로 Publish해도 memory 백엔드에서 핸들러가 호출되는지 검증한다
package queue

import (
	"context"
	"testing"
)

func TestWithDelay(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	if err := Init(ctx, "memory", nil); err != nil {
		t.Fatal(err)
	}
	defer Close()

	called := false
	Subscribe("delayed", func(_ context.Context, msg []byte) error {
		called = true
		return nil
	})

	if err := Publish(ctx, "delayed", "data", WithDelay(30)); err != nil {
		t.Fatal(err)
	}

	if !called {
		t.Error("handler should be called even with delay on memory backend")
	}
}
