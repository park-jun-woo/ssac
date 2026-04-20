//ff:func feature=pkg-redact type=util control=selection
//ff:what ReplaceAttr — slog handler 에 주입하는 민감 키 마스킹 함수
package redact

import (
	"log/slog"
	"strings"
)

// Redacted is the masked value written in place of a sensitive attribute value.
const Redacted = "[REDACTED]"

// ReplaceAttr returns a slog.HandlerOptions.ReplaceAttr callback that masks
// any attribute whose lowercased key is present in sensitiveKeys.
//
// The callback is safe for concurrent use — it reads the map without mutating
// it. Callers should build the map once at startup and never mutate it
// afterwards.
//
// This is a defensive net for ad-hoc slog calls such as
// `slog.Info("reset", "password", value)`. Struct-valued attrs are better
// handled via the slog.LogValuer interface (generated per-table LogValue
// methods), so both layers work together.
func ReplaceAttr(sensitiveKeys map[string]bool) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if sensitiveKeys[strings.ToLower(a.Key)] {
			return slog.String(a.Key, Redacted)
		}
		return a
	}
}
