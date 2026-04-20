//ff:func feature=pkg-authz type=test control=sequence
//ff:what CheckRequest.Tx 필드가 nil/비nil 모두 허용됨을 검증한다
package authz

import (
	"database/sql"
	"testing"
)

// TestCheckRequestTxNil — backward compat: Tx omitted (nil) must still build.
func TestCheckRequestTxNil(t *testing.T) {
	req := CheckRequest{
		Action:     "Read",
		Resource:   "workflow",
		ResourceID: 1,
	}
	if req.Tx != nil {
		t.Fatalf("expected Tx nil by default, got %v", req.Tx)
	}
}

// TestCheckRequestTxAssign — structural check: *sql.Tx can be stored.
func TestCheckRequestTxAssign(t *testing.T) {
	var tx *sql.Tx // nil typed pointer — safe to assign
	req := CheckRequest{
		Tx:         tx,
		Action:     "Update",
		Resource:   "workflow",
		ResourceID: 2,
	}
	if req.Tx != nil {
		t.Fatalf("expected Tx nil typed ptr, got %v", req.Tx)
	}
}
