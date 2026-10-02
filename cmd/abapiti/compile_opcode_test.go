package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompileWasmUnsupportedOpcode(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "unknown.wasm")
	bin := []byte{
		0, 0x61, 0x73, 0x6d, 1, 0, 0, 0,
		1, 4, 1, 0x60, 0, 0,
		3, 2, 1, 0,
		10, 5, 1, 3, 0, 0xff, 0x0b,
	}
	if err := os.WriteFile(input, bin, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.Flags().String("class", "zcl_bad", "")
	output := filepath.Join(dir, "output")
	cmd.Flags().String("output", output, "")
	err := runCompileWasm(cmd, []string{input})
	if err == nil || !strings.Contains(err.Error(), "failed to compile WASM: unsupported opcode 0xFF in function 0") || exitCode(err) == 0 {
		t.Fatalf("CLI must surface compile failure: %v", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("failed compilation wrote output: %v", err)
	}
}
