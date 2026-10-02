package main

import (
	"github.com/oisee/abapiti/abapsize"
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
	}{{"single", false, 20000, 1}, {"forced", true, 20000, 4}, {"automatic", false, 100, 4}} {
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
			if tc.files > 1 {
				name := "zif_cli_test_c01.intf.abap"
				src, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				report := abapsize.Report(map[string]string{name: string(src)})
				if report.Files[name].Lines == 0 {
					t.Fatal("interface missing from size report")
				}
			}
		})
	}
}

// Run the real CLI pipeline for an external large-module regression fixture.
func TestWasmSplitExternalFixture(t *testing.T) {
	input := os.Getenv("ABAPITI_SPLIT_WASM")
	if input == "" {
		t.Skip("ABAPITI_SPLIT_WASM is unset")
	}
	cmd := &cobra.Command{}
	addWasmFlags(cmd)
	dir := os.Getenv("ABAPITI_SPLIT_OUTPUT")
	if dir == "" {
		dir = t.TempDir()
	}
	_ = cmd.Flags().Set("output", dir)
	_ = cmd.Flags().Set("class", "zcl_qjs")
	_ = cmd.Flags().Set("split", "true")
	if err := runCompileWasm(cmd, []string{input}); err != nil {
		t.Fatal(err)
	}
}
