//ff:func feature=pkg-queue type=util control=sequence
//ff:what 메시지 전달 지연 시간을 설정한다
package queue

// WithDelay sets the delivery delay in seconds.
func WithDelay(seconds int) PublishOption {
	return func(c *PublishConfig) { c.Delay = seconds }
}
