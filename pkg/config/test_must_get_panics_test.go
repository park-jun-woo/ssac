//ff:func feature=pkg-config type=test control=sequence
//ff:what 미설정 환경변수에 대해 MustGet이 panic하는지 검증한다
package config

import (
	"os"
	"testing"
)

func TestMustGetPanics(t *testing.T) {
	os.Unsetenv("TEST_CONFIG_PANIC")
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	MustGet("TEST_CONFIG_PANIC")
}
