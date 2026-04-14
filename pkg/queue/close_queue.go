//ff:func feature=pkg-queue type=util control=sequence
//ff:what 폴링 루프를 중지하고 정리한다
package queue

// Close stops the polling loop and waits for it to finish.
func Close() error {
	mu.Lock()
	c := cancel
	d := done
	mu.Unlock()

	if c != nil {
		c()
	}
	if d != nil {
		<-d
	}

	mu.Lock()
	inited = false
	handlers = nil
	backend = ""
	db = nil
	cancel = nil
	done = nil
	mu.Unlock()

	return nil
}
