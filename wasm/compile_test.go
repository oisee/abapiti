package wasm

import (
	"os"
	"testing"
)

func TestCompileMultiClassQuickJS(t *testing.T) {
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
