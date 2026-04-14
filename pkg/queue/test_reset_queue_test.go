//ff:func feature=pkg-queue type=test control=sequence
//ff:what 테스트용 큐 글로벌 상태를 초기화하는 헬퍼 함수
package queue

func resetQueue() {
	mu.Lock()
	inited = false
	handlers = nil
	backend = ""
	db = nil
	cancel = nil
	done = nil
	mu.Unlock()
}
