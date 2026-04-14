//ff:func feature=pkg-config type=test control=sequence
//ff:what Get이 환경변수 값을 올바르게 반환하는지 검증한다
package config

import (
	"os"
	"testing"
)

func TestGet(t *testing.T) {
	os.Setenv("TEST_CONFIG_KEY", "hello")
	defer os.Unsetenv("TEST_CONFIG_KEY")

	if v := Get("TEST_CONFIG_KEY"); v != "hello" {
		t.Fatalf("expected 'hello', got %q", v)
	}
}
