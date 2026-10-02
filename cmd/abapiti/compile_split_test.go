package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestWasmSplitCLI(t *testing.T) {
	for _, tc := range []struct {
		name   string
		force  bool
		budget int
		files  int
	}{{"single", false, 20000, 1}, {"forced", true, 20000, 3}, {"automatic", false, 100, 3}} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			addWasmFlags(cmd)
			dir := t.TempDir()
			_ = cmd.Flags().Set("output", dir)
			_ = cmd.Flags().Set("class", "zcl_cli_test")
			if tc.force {
				_ = cmd.Flags().Set("split", "true")
			}
			if tc.budget == 100 {
				_ = cmd.Flags().Set("class-lines", "100")
			}
			if err := runCompileWasm(cmd, []string{"../../wasm/testdata/add.wasm"}); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != tc.files {
				t.Fatalf("got %d files, want %d", len(entries), tc.files)
			}
			if _, err := os.Stat(filepath.Join(dir, "zcl_cli_test.clas.abap")); err != nil {
				t.Fatal(err)
			}
		})
	}
}
