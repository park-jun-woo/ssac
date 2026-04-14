//ff:func feature=pkg-queue type=util control=iteration dimension=1
//ff:what postgres 폴링 루프를 실행한다
package queue

import (
	"context"
	"time"
)

func runPollingLoop(ctx context.Context) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_ = pollOnce(ctx)
		}
	}
}
