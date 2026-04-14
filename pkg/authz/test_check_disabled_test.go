//ff:func feature=pkg-authz type=test control=sequence
//ff:what DISABLE_AUTHZ=1일 때 Check가 에러 없이 통과하는지 검증한다
package authz

import (
	"os"
	"testing"
)

func TestCheckDisabled(t *testing.T) {
	os.Setenv("DISABLE_AUTHZ", "1")
	defer os.Unsetenv("DISABLE_AUTHZ")

	resp, err := Check(CheckRequest{
		Action:   "read",
		Resource: "gig",
		UserID:   1,
		Role:     "client",
	})
	if err != nil {
		t.Fatalf("expected no error with DISABLE_AUTHZ=1, got: %v", err)
	}
	_ = resp
}
