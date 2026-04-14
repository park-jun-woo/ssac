//ff:func feature=pkg-config type=test control=sequence
//ff:what MustGet이 설정된 환경변수 값을 올바르게 반환하는지 검증한다
package config

import (
	"os"
	"testing"
)

func TestMustGet(t *testing.T) {
	os.Setenv("TEST_CONFIG_MUST", "value")
	defer os.Unsetenv("TEST_CONFIG_MUST")

	if v := MustGet("TEST_CONFIG_MUST"); v != "value" {
		t.Fatalf("expected 'value', got %q", v)
	}
}
