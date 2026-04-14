//ff:func feature=pkg-authz type=test control=sequence
//ff:what OPA_POLICY_PATH 미설정 시 Init이 에러를 반환하는지 검증한다
package authz

import (
	"os"
	"testing"
)

func TestInitRequiresOPAPolicyPath(t *testing.T) {
	os.Unsetenv("DISABLE_AUTHZ")
	os.Unsetenv("OPA_POLICY_PATH")

	err := Init(nil, nil)
	if err == nil {
		t.Fatal("expected error when OPA_POLICY_PATH is not set")
	}
}
