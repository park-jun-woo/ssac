//ff:func feature=pkg-queue type=test control=sequence
//ff:what 서로 다른 토픽의 구독자가 각각 올바른 메시지를 수신하는지 검증한다
package queue

import (
	"context"
	"sync"
	"testing"
)

func TestMultipleSubscribers(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	if err := Init(ctx, "memory", nil); err != nil {
		t.Fatal(err)
	}
	defer Close()

	var muResult sync.Mutex
	results := make(map[string]string)

	Subscribe("topic.a", func(_ context.Context, msg []byte) error {
		muResult.Lock()
		results["a"] = string(msg)
		muResult.Unlock()
		return nil
	})

	Subscribe("topic.b", func(_ context.Context, msg []byte) error {
		muResult.Lock()
		results["b"] = string(msg)
		muResult.Unlock()
		return nil
	})

	if err := Publish(ctx, "topic.a", "alpha"); err != nil {
		t.Fatal(err)
	}
	if err := Publish(ctx, "topic.b", "beta"); err != nil {
		t.Fatal(err)
	}

	muResult.Lock()
	defer muResult.Unlock()

	if results["a"] != `"alpha"` {
		t.Errorf("topic.a: got %q, want %q", results["a"], `"alpha"`)
	}
	if results["b"] != `"beta"` {
		t.Errorf("topic.b: got %q, want %q", results["b"], `"beta"`)
	}
}
