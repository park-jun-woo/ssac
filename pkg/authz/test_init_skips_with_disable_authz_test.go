//ff:func feature=pkg-authz type=test control=sequence
//ff:what DISABLE_AUTHZ=1일 때 Init이 에러 없이 건너뛰는지 검증한다
package authz

import (
	"os"
	"testing"
)

func TestInitSkipsWithDisableAuthz(t *testing.T) {
	os.Setenv("DISABLE_AUTHZ", "1")
	defer os.Unsetenv("DISABLE_AUTHZ")

	err := Init(nil, nil)
	if err != nil {
		t.Fatalf("expected no error with DISABLE_AUTHZ=1, got: %v", err)
	}
}
