package generator

import (
	"os"
	"strings"
	"testing"

	"github.com/park-jun-woo/ssac/parser"
	"github.com/park-jun-woo/ssac/validator"
)

func mustGenerate(t *testing.T, sf parser.ServiceFunc, st *validator.SymbolTable) string {
	t.Helper()
	code, err := GenerateFunc(sf, st)
	if err != nil {
		t.Fatalf("GenerateFunc failed: %v", err)
	}
	return string(code)
}

func assertContains(t *testing.T, code, substr string) {
	t.Helper()
	if !strings.Contains(code, substr) {
		t.Errorf("expected code to contain %q\n--- code ---\n%s", substr, code)
	}
}

func assertNotContains(t *testing.T, code, substr string) {
	t.Helper()
	if strings.Contains(code, substr) {
		t.Errorf("expected code NOT to contain %q\n--- code ---\n%s", substr, code)
	}
}

func readFile(t *testing.T, path string) (string, error) {
	t.Helper()
	data, err := os.ReadFile(path)
	return string(data), err
}
