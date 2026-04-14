//ff:func feature=pkg-authz type=test control=sequence
//ff:what CheckRequest 구조체 필드가 올바르게 설정되는지 검증한다
package authz

import "testing"

func TestCheckRequestFields(t *testing.T) {
	req := CheckRequest{
		Action:     "AcceptProposal",
		Resource:   "gig",
		UserID:     42,
		Role:       "client",
		ResourceID: 99,
	}
	if req.Action != "AcceptProposal" {
		t.Fatal("Action mismatch")
	}
	if req.Resource != "gig" {
		t.Fatal("Resource mismatch")
	}
	if req.UserID != 42 {
		t.Fatal("UserID mismatch")
	}
	if req.Role != "client" {
		t.Fatal("Role mismatch")
	}
	if req.ResourceID != 99 {
		t.Fatal("ResourceID mismatch")
	}
}
