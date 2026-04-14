//ff:func feature=pkg-queue type=util control=selection
//ff:what 토픽에 메시지를 발행한다
package queue

import (
	"context"
	"encoding/json"
)

// Publish sends a message to the given topic.
func Publish(ctx context.Context, topic string, payload any, opts ...PublishOption) error {
	mu.RLock()
	if !inited {
		mu.RUnlock()
		return ErrNotInitialized
	}
	b := backend
	mu.RUnlock()

	cfg := applyPublishOpts(opts)

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	switch b {
	case "postgres":
		return publishPostgres(ctx, topic, data, cfg)
	case "memory":
		return publishMemory(ctx, topic, data)
	}

	return nil
}
