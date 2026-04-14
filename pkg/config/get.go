//ff:func feature=pkg-config type=util control=sequence
//ff:what 환경 변수 값을 반환한다
package config

import "os"

// Get returns the environment variable value for the given key.
// Returns empty string if not set.
func Get(key string) string {
	return os.Getenv(key)
}
