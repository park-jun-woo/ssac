//ff:func feature=pkg-authz type=test control=sequence
//ff:what 초기화되지 않은 상태에서 Check가 에러를 반환하는지 검증한다
package authz

import (
	"os"
	"testing"
)

func TestCheckNotInitialized(t *testing.T) {
	os.Unsetenv("DISABLE_AUTHZ")
	globalPolicy = ""

	_, err := Check(CheckRequest{
		Action:   "read",
		Resource: "gig",
		UserID:   1,
		Role:     "client",
	})
	if err == nil {
		t.Fatal("expected error when not initialized")
	}
	if err.Error() != "authz not initialized" {
		t.Fatalf("unexpected error: %v", err)
	}
}
