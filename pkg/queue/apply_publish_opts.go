//ff:func feature=pkg-queue type=util control=iteration dimension=1
//ff:what Publish 옵션을 적용하여 설정을 반환한다
package queue

func applyPublishOpts(opts []PublishOption) publishConfig {
	cfg := publishConfig{priority: "normal"}
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}
