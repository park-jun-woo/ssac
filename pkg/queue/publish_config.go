//ff:type feature=pkg-queue type=model
//ff:what Publish 호출 옵션 구조체 — 외부 Backend 구현체 (yongol infra adapter) 도 참조
package queue

// PublishConfig holds options applied to a single Publish / PublishTx call.
// Exported because Backend implementations provided outside the queue package
// (e.g. yongol-generated postgres adapter) must be able to name the config
// type in their method signatures.
//
// Fields are exported for the same reason — external implementations need
// read access to Priority / Delay when translating into their own driver
// calls (e.g. priority ordering, deliver_at offset). Callers still configure
// via WithPriority / WithDelay options; direct struct literals are reserved
// for tests and in-package wiring.
type PublishConfig struct {
	Delay    int    // seconds
	Priority string // "high", "normal", "low"
}
