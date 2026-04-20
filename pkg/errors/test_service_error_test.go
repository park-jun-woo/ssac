//ff:func feature=pkg-errors type=test control=sequence
//ff:what ServiceError 생성자 및 Unwrap 동작을 검증한다
package errors

import (
	stderrors "errors"
	"testing"
)

func TestNewPopulatesFields(t *testing.T) {
	e := New(402, "credit_insufficient", "Insufficient credits")
	if e.Status != 402 || e.Code != "credit_insufficient" || e.Message != "Insufficient credits" {
		t.Fatalf("fields mismatch: %+v", e)
	}
	if e.Cause != nil || e.Details != nil {
		t.Fatalf("expected zero Cause/Details, got %+v", e)
	}
	if e.Error() != "Insufficient credits" {
		t.Fatalf("Error() mismatch: %s", e.Error())
	}
}

func TestWrapPreservesCause(t *testing.T) {
	root := stderrors.New("root cause")
	e := Wrap(500, "internal_error", "Internal error", root)
	if !stderrors.Is(e, root) {
		t.Fatalf("errors.Is failed to unwrap to root")
	}
}

func TestWithDetailsAttaches(t *testing.T) {
	e := New(400, "bad_request", "Bad request")
	e = WithDetails(e, map[string]any{"email": []string{"required"}})
	if e.Details["email"] == nil {
		t.Fatalf("details not attached: %+v", e.Details)
	}
}

func TestWithDetailsNilSafe(t *testing.T) {
	if got := WithDetails(nil, map[string]any{"x": 1}); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}
