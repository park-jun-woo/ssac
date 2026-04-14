//ff:func feature=pkg-queue type=util control=sequence
//ff:what 토픽 핸들러를 등록한다
package queue

import "context"

// Subscribe registers a handler for the given topic.
func Subscribe(topic string, handler func(ctx context.Context, msg []byte) error) {
	mu.Lock()
	defer mu.Unlock()
	handlers[topic] = append(handlers[topic], handler)
}
