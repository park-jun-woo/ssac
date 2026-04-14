//ff:func feature=pkg-queue type=util control=iteration dimension=1
//ff:what 토픽 핸들러에게 메시지를 전달하고 결과 상태를 반환한다
package queue

import "context"

func dispatchMessage(ctx context.Context, hs map[string][]func(ctx context.Context, msg []byte) error, topic string, payload []byte) string {
	topicHandlers, ok := hs[topic]
	if !ok {
		return "done"
	}
	for _, h := range topicHandlers {
		if err := h(ctx, payload); err != nil {
			return "failed"
		}
	}
	return "done"
}
