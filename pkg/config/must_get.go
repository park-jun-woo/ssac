//ff:func feature=pkg-config type=util control=sequence
//ff:what 환경 변수 값을 반환하되 없으면 panic한다
package config

import "os"

// MustGet returns the environment variable value, panics if empty.
func MustGet(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required env var not set: " + key)
	}
	return v
}
