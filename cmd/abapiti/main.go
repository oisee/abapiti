// Command abapiti compiles WebAssembly, LLVM IR / C and TypeScript into ABAP.
//
//	abapiti compile wasm <in.wasm> [--class zcl_x] [-o dir]
//	abapiti compile llvm <in.ll|in.c> [--class zcl_x] [-o file] [--zip]
//	abapiti compile ts   <in.ts> [--prefix zcl_] [-o dir]
//	abapiti abaplint [abaplint-checkout] -o dir [--target all|a4h|osg|native]
//	abapiti <in.wasm>    shortcut for "compile wasm"
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oisee/abapiti/wasm"
	"github.com/spf13/cobra"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "abapiti [file.wasm]",
	Short: "ABAPiti: because we can. Compile WASM, LLVM IR and TypeScript into ABAP.",
	Long: `ABAPiti: because we can.
a new identity for your code.

Compiles WebAssembly, LLVM IR / C and TypeScript into ABAP source.
Fully offline: no SAP connection is needed. Deploy the output with vsp
(github.com/oisee/vibing-steampunk) or abapGit.

"abapiti <file.wasm>" is a shortcut for "abapiti compile wasm <file.wasm>".`,
	Version:       version,
	Args:          cobra.MaximumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		if !strings.EqualFold(filepath.Ext(args[0]), ".wasm") {
			return fmt.Errorf("unknown command or input %q (the shortcut takes a .wasm file; see \"abapiti compile --help\")", args[0])
		}
		return runCompileWasm(cmd, args)
	},
}

func init() {
	addWasmFlags(rootCmd)
}

// addWasmFlags registers the flags shared by "compile wasm" and the root shortcut.
func addWasmFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("split", false, "Force independently activatable WASM classes")
	cmd.Flags().Int("class-lines", wasm.DefaultClassLines, "Generated line budget per WASM chunk class")
	cmd.Flags().Bool("allow-long-lines", false, "Allow output lines over 255 characters")
	cmd.Flags().String("class", "", "ABAP class name (default: derived from filename)")
	cmd.Flags().StringP("output", "o", "", "Output directory (default: stdout)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "abapiti:", err)
		os.Exit(exitCode(err))
	}
}

func exitCode(err error) int {
	if errors.Is(err, errLongLines) {
		return 3
	}
	return 1
}
