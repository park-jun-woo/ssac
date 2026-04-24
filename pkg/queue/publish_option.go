//ff:type feature=pkg-queue type=model
//ff:what Publish 옵션 함수 타입
package queue

// PublishOption configures a Publish call.
type PublishOption func(*PublishConfig)
