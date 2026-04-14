//ff:func feature=pkg-config type=test control=sequence
//ff:what 존재하지 않는 환경변수에 대해 Get이 빈 문자열을 반환하는지 검증한다
package config

import (
	"os"
	"testing"
)

func TestGetEmpty(t *testing.T) {
	os.Unsetenv("TEST_CONFIG_MISSING")
	if v := Get("TEST_CONFIG_MISSING"); v != "" {
		t.Fatalf("expected empty string, got %q", v)
	}
}
