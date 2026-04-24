//ff:func feature=pkg-queue type=util control=sequence
//ff:what 메시지 우선순위를 설정한다
package queue

// WithPriority sets the message priority ("high", "normal", "low").
func WithPriority(p string) PublishOption {
	return func(c *PublishConfig) { c.Priority = p }
}
