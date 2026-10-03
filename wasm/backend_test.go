package wasm

import (
	"os"
	"strings"
	"testing"
)

func TestBackendFUGR_QuickJS(t *testing.T) {
	data, err := os.ReadFile("testdata/quickjs_eval.wasm")
	if err != nil {
		t.Skipf("QuickJS WASM not found: %v", err)
	}

	mod, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	expectQuickJSCompileError(t, mod)
}

func TestBackendHybrid_QuickJS(t *testing.T) {
	data, err := os.ReadFile("testdata/quickjs_eval.wasm")
	if err != nil {
		t.Skipf("QuickJS WASM not found: %v", err)
	}

	mod, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	expectQuickJSCompileError(t, mod)
}

func containsStr(s, sub string) bool {
	return strings.Contains(s, sub)
}
