//ff:func feature=pkg-queue type=test control=sequence
//ff:what 초기화 전 Publish 호출 시 ErrNotInitialized를 반환하는지 검증한다
package queue

import (
	"context"
	"testing"
)

func TestPublishBeforeInit(t *testing.T) {
	resetQueue()

	err := Publish(context.Background(), "topic", "data")
	if err != ErrNotInitialized {
		t.Errorf("got %v, want %v", err, ErrNotInitialized)
	}
}
