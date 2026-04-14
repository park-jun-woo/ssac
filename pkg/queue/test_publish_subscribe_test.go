//ff:func feature=pkg-queue type=test control=sequence
//ff:what Publish한 메시지가 Subscribe 핸들러로 전달되는지 검증한다
package queue

import (
	"context"
	"testing"
)

func TestPublishSubscribe(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	if err := Init(ctx, "memory", nil); err != nil {
		t.Fatal(err)
	}
	defer Close()

	var got string
	Subscribe("test.topic", func(_ context.Context, msg []byte) error {
		got = string(msg)
		return nil
	})

	if err := Publish(ctx, "test.topic", map[string]string{"hello": "world"}); err != nil {
		t.Fatal(err)
	}

	want := `{"hello":"world"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
