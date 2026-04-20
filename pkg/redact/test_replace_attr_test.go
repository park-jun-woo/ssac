//ff:func feature=pkg-redact type=test control=sequence
//ff:what ReplaceAttr 가 민감 키를 REDACTED 로 바꾸는지 검증한다
package redact

import (
	"log/slog"
	"testing"
)

func TestReplaceAttrMasksSensitiveKey(t *testing.T) {
	fn := ReplaceAttr(DefaultKeys)
	got := fn(nil, slog.String("password", "plaintext"))
	if got.Value.String() != Redacted {
		t.Fatalf("password not masked: %v", got.Value)
	}
}

func TestReplaceAttrMasksUppercaseKey(t *testing.T) {
	fn := ReplaceAttr(DefaultKeys)
	got := fn(nil, slog.String("Authorization", "Bearer xxx"))
	if got.Value.String() != Redacted {
		t.Fatalf("Authorization not masked: %v", got.Value)
	}
}

func TestReplaceAttrPreservesNonSensitive(t *testing.T) {
	fn := ReplaceAttr(DefaultKeys)
	got := fn(nil, slog.String("op", "Login"))
	if got.Value.String() != "Login" {
		t.Fatalf("op mangled: %v", got.Value)
	}
}

func TestReplaceAttrCustomKey(t *testing.T) {
	keys := map[string]bool{"national_id": true}
	fn := ReplaceAttr(keys)
	got := fn(nil, slog.String("national_id", "123-45-6789"))
	if got.Value.String() != Redacted {
		t.Fatalf("national_id not masked: %v", got.Value)
	}
	got2 := fn(nil, slog.String("password", "x")) // not in custom map
	if got2.Value.String() != "x" {
		t.Fatalf("password unexpectedly masked with custom map: %v", got2.Value)
	}
}
