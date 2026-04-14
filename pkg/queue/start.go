//ff:func feature=pkg-queue type=util control=sequence
//ff:what 큐 메시지 처리를 시작한다
package queue

import "context"

// Start begins processing queued messages. It blocks until the context is
// cancelled. For the memory backend this is a no-op that blocks until cancel.
func Start(ctx context.Context) error {
	mu.RLock()
	b := backend
	mu.RUnlock()

	innerCtx, c := context.WithCancel(ctx)
	mu.Lock()
	cancel = c
	done = make(chan struct{})
	mu.Unlock()

	defer close(done)

	if b == "memory" {
		<-innerCtx.Done()
		return nil
	}

	return runPollingLoop(innerCtx)
}
