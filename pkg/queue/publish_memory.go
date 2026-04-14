//ff:func feature=pkg-queue type=util control=iteration dimension=1
//ff:what memory 백엔드에서 핸들러를 직접 호출한다
package queue

import "context"

func publishMemory(ctx context.Context, topic string, data []byte) error {
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
