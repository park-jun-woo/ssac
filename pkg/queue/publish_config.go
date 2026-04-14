//ff:type feature=pkg-queue type=model
//ff:what Publish 호출 옵션 구조체
package queue

// publishConfig holds options for a single Publish call.
type publishConfig struct {
	delay    int    // seconds
	priority string // "high", "normal", "low"
}
