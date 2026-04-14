//ff:func feature=pkg-queue type=test control=sequence
//ff:what 구독자 없는 토픽에 Publish해도 에러가 발생하지 않는지 검증한다
package queue

import (
	"context"
	"testing"
)

func TestPublishNoSubscriber(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	if err := Init(ctx, "memory", nil); err != nil {
		t.Fatal(err)
	}
	defer Close()

	if err := Publish(ctx, "no.subscriber", "payload"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
