package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oisee/abapiti/abapsize"
	"github.com/oisee/abapiti/llvm"
	"github.com/oisee/abapiti/ts"
	"github.com/oisee/abapiti/wasm"
	"github.com/spf13/cobra"
)

// --- compile wasm command ---

var compileCmd = &cobra.Command{
	Use:   "compile",
	Short: "Compile and transpile source code",
	Long:  "Compile WebAssembly, LLVM IR / C, or TypeScript into ABAP. Fully offline.",
}

var compileWasmCmd = &cobra.Command{
	Use:   "wasm <input.wasm> [--class <name>] [--output <dir>]",
	Short: "Compile WebAssembly to ABAP",
	Long: `Compile a .wasm binary to native ABAP source code.
Fully offline — no SAP connection required.

The output is an ABAP class with one method per exported function.
Deploy with vsp: vsp deploy <output.clas.abap> '$TMP'

Examples:
  abapiti compile wasm program.wasm
  abapiti compile wasm program.wasm --class ZCL_MY_WASM
  abapiti compile wasm program.wasm --output ./src/
  abapiti program.wasm                  # shortcut for "compile wasm"`,
	Args: cobra.ExactArgs(1),
	RunE: runCompileWasm,
}

// --- compile ts command ---

var compileTsCmd = &cobra.Command{
	Use:   "ts <input.ts> [--prefix <zcl_>]",
	Short: "Transpile TypeScript to ABAP",
	Long: `Transpile TypeScript classes to ABAP source code.
Requires Node.js with TypeScript (for AST parsing).
Fully offline — no SAP connection required.

Each TS class becomes an ABAP class with proper types, methods, and OO structure.

Examples:
  abapiti compile ts lexer.ts
  abapiti compile ts lexer.ts --prefix zcl_
  abapiti compile ts lexer.ts --output ./src/

ts_ast.js is looked up in ./ts/, ./ts2go/, $ABAPITI_TS_AST_PATH and next to the
executable, so this works from a source checkout (run "npm install" first).`,
	Args: cobra.ExactArgs(1),
	RunE: runCompileTs,
}

// --- compile llvm command ---

var compileLLVMCmd = &cobra.Command{
	Use:   "llvm <file.ll|file.c>",
	Short: "Compile LLVM IR or C source to typed ABAP",
	Long: `Compile LLVM IR (.ll) or C source (.c) to typed ABAP CLASS-METHODS.
For .c files, clang is invoked automatically.

Examples:
  abapiti compile llvm mycode.c
  abapiti compile llvm mycode.c --class zcl_mycode -o mycode.abap
  abapiti compile llvm mycode.c --class zcl_mycode --zip -o mycode.zip
  abapiti compile llvm quickjs.c --class zcl_quickjs --zip`,
	Args: cobra.ExactArgs(1),
	RunE: runCompileLLVM,
}

var errLongLines = errors.New("generated ABAP has lines over 255 characters")

func reportGenerated(cmd *cobra.Command, files map[string]string) error {
	report := abapsize.Report(files)
	names := make([]string, 0, len(report.Files))
	for name := range report.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f := report.Files[name]
		fmt.Fprintf(os.Stderr, "size %s: lines=%d max-line=%d longest-METHOD/FORM=%d routines=%d bytes=%d\n", name, f.Lines, f.MaxLineLength, f.LongestRoutine, f.Routines, f.Bytes)
	}
	for _, warning := range report.Warnings(abapsize.DefaultThresholds) {
		fmt.Fprintln(os.Stderr, "warning:", warning)
	}
	allow, _ := cmd.Flags().GetBool("allow-long-lines")
	if !allow && len(report.LongLines) > 0 {
		for _, issue := range report.Errors() {
			fmt.Fprintln(os.Stderr, issue)
		}
		return errLongLines
	}
	return nil
}

func init() {
	// Compile subcommands
	compileCmd.AddCommand(compileWasmCmd)
	compileCmd.AddCommand(compileTsCmd)
	compileCmd.AddCommand(compileLLVMCmd)

	// Compile wasm flags
	addWasmFlags(compileWasmCmd)
	compileTsCmd.Flags().Bool("allow-long-lines", false, "Allow output lines over 255 characters")
	compileLLVMCmd.Flags().Bool("allow-long-lines", false, "Allow output lines over 255 characters")

	// Compile ts flags
	compileTsCmd.Flags().String("prefix", "zcl_", "ABAP class name prefix")
	compileTsCmd.Flags().StringP("output", "o", "", "Output directory (default: stdout)")

	// Compile llvm flags
	compileLLVMCmd.Flags().String("class", "zcl_compiled", "ABAP class name")
	compileLLVMCmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")
	compileLLVMCmd.Flags().Bool("zip", false, "Output as abapGit ZIP")
	compileLLVMCmd.Flags().String("package", "$TMP", "SAP package (for --zip)")
	compileLLVMCmd.Flags().String("desc", "Compiled via abapiti compile llvm", "Description")
	compileLLVMCmd.Flags().String("opt", "O1", "Clang optimization level (O0/O1/O2)")
	compileLLVMCmd.Flags().String("cflags", "", "Extra clang flags (e.g. \"-DCONFIG_VERSION=\\\"v1\\\" -D_GNU_SOURCE\")")
	compileLLVMCmd.Flags().Int("split", 0, "Split into multiple classes with N functions each (for transpiler)")

	rootCmd.AddCommand(compileCmd)
}

func runCompileWasm(cmd *cobra.Command, args []string) error {
	inputFile := args[0]
	className, _ := cmd.Flags().GetString("class")
	outputDir, _ := cmd.Flags().GetString("output")

	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", inputFile, err)
	}

	mod, err := wasm.Parse(data)
	if err != nil {
		return fmt.Errorf("failed to parse WASM: %w", err)
	}

	if className == "" {
		base := strings.TrimSuffix(filepath.Base(inputFile), ".wasm")
		className = "zcl_wasm_" + strings.ToLower(strings.ReplaceAll(base, "-", "_"))
	}

	fmt.Fprintf(os.Stderr, "WASM: %d bytes, %d functions, %d instructions\n",
		len(data), len(mod.Functions), countInstructions(mod))

	abapSrc := wasm.Compile(mod, className)

	lines := strings.Count(abapSrc, "\n")
	fmt.Fprintf(os.Stderr, "ABAP: %d lines, class %s\n", lines, className)

	if outputDir != "" {
		outFile := filepath.Join(outputDir, strings.ToLower(className)+".clas.abap")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(outFile, []byte(abapSrc), 0644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Written to %s\n", outFile)
		return reportAndHint(cmd, classFiles(className, abapSrc), fmt.Sprintf("Deploy with vsp: vsp deploy %s '$TMP'", outFile))
	}
	fmt.Print(abapSrc)
	return reportGenerated(cmd, classFiles(className, abapSrc))
}

func classFiles(className, src string) map[string]string {
	return map[string]string{strings.ToLower(className) + ".clas.abap": src}
}

// reportAndHint reports the generated sources and prints the deploy hint only
// when they passed, so a rejected output never ends with "Deploy with vsp".
func reportAndHint(cmd *cobra.Command, files map[string]string, hint string) error {
	if err := reportGenerated(cmd, files); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, hint)
	return nil
}

func runCompileTs(cmd *cobra.Command, args []string) error {
	inputFile := args[0]
	prefix, _ := cmd.Flags().GetString("prefix")
	outputDir, _ := cmd.Flags().GetString("output")

	// Check if Node.js is available
	if _, err := exec.LookPath("node"); err != nil {
		return fmt.Errorf("Node.js is required for TypeScript transpilation.\nInstall from https://nodejs.org/ or use: nvm install node")
	}

	// Find ts_ast.js
	tsAstScript := findTsAstScript()
	if tsAstScript == "" {
		return fmt.Errorf("ts_ast.js not found. Run from the abapiti source directory or set ABAPITI_TS_AST_PATH")
	}

	// Parse TS → JSON AST
	astCmd := exec.Command("node", tsAstScript, inputFile)
	astJSON, err := astCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to parse TypeScript: %w\nMake sure TypeScript is installed: npm install typescript", err)
	}

	// Transpile JSON AST → ABAP
	result, err := ts.Transpile(astJSON, prefix)
	if err != nil {
		return fmt.Errorf("transpilation failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Transpiled %d classes with prefix '%s'\n", len(result.Classes), prefix)

	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}
		for name, src := range result.Classes {
			outFile := filepath.Join(outputDir, name+".clas.abap")
			if err := os.WriteFile(outFile, []byte(src), 0644); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "  %s → %s (%d lines)\n", name, outFile, strings.Count(src, "\n"))
		}
	} else {
		for name, src := range result.Classes {
			fmt.Printf("* === %s ===\n%s\n", name, src)
		}
	}
	return reportGenerated(cmd, result.Classes)
}

// --- helpers ---

func countInstructions(mod *wasm.Module) int {
	total := 0
	for _, f := range mod.Functions {
		total += len(f.Code)
	}
	return total
}

func findTsAstScript() string {
	candidates := []string{
		"ts/ts_ast.js",
		"ts2go/ts_ast.js",
	}
	if p := os.Getenv("ABAPITI_TS_AST_PATH"); p != "" {
		candidates = append(candidates, filepath.Join(p, "ts_ast.js"))
	}
	// Also check relative to executable
	ex, _ := os.Executable()
	if ex != "" {
		dir := filepath.Dir(ex)
		candidates = append(candidates, filepath.Join(dir, "ts_ast.js"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// --- compile llvm implementation ---

func runCompileLLVM(cmd *cobra.Command, args []string) error {
	inputFile := args[0]
	ext := strings.ToLower(filepath.Ext(inputFile))
	className, _ := cmd.Flags().GetString("class")
	output, _ := cmd.Flags().GetString("output")
	asZip, _ := cmd.Flags().GetBool("zip")
	pkg, _ := cmd.Flags().GetString("package")
	desc, _ := cmd.Flags().GetString("desc")
	optLevel, _ := cmd.Flags().GetString("opt")

	var llSource string

	switch ext {
	case ".c", ".h":
		tmpLL := inputFile + ".ll"
		clangArgs := []string{"-S", "-emit-llvm", "-" + optLevel, inputFile, "-o", tmpLL}
		cflags, _ := cmd.Flags().GetString("cflags")
		if cflags != "" {
			clangArgs = append(strings.Fields(cflags), clangArgs...)
		}
		clangCmd := exec.Command("clang", clangArgs...)
		clangCmd.Stderr = os.Stderr
		if err := clangCmd.Run(); err != nil {
			return fmt.Errorf("clang failed: %w (is clang installed?)", err)
		}
		defer os.Remove(tmpLL)
		data, err := os.ReadFile(tmpLL)
		if err != nil {
			return err
		}
		llSource = string(data)
		fmt.Fprintf(os.Stderr, "clang: %s → %d lines LLVM IR\n", inputFile, strings.Count(llSource, "\n"))

	case ".ll":
		data, err := os.ReadFile(inputFile)
		if err != nil {
			return err
		}
		llSource = string(data)

	default:
		return fmt.Errorf("unsupported: %s (use .c or .ll)", ext)
	}

	mod, err := llvm.Parse(llSource)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	nonExt := 0
	for _, fn := range mod.Functions {
		if !fn.IsExternal && !strings.HasPrefix(fn.Name, "llvm.") {
			nonExt++
		}
	}
	fmt.Fprintf(os.Stderr, "parsed: %d functions, %d structs\n", nonExt, len(mod.Types))

	splitN, _ := cmd.Flags().GetInt("split")

	if splitN > 0 {
		// Multi-class split mode
		files := llvm.CompileMultiClass(mod, className, splitN)
		outDir := output
		if outDir == "" {
			outDir = "."
		}
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return err
		}

		totalLines := 0
		sources := make(map[string]string)
		for _, f := range files {
			lines := strings.Count(f.Source, "\n")
			totalLines += lines

			abapFile := filepath.Join(outDir, f.FileName+".clas.abap")
			sources[f.FileName+".clas.abap"] = f.Source
			if err := os.WriteFile(abapFile, []byte(f.Source), 0644); err != nil {
				return err
			}

			// Write .clas.xml metadata
			xmlFile := filepath.Join(outDir, f.FileName+".clas.xml")
			xml := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<abapGit version="v1.0.0" serializer="LCL_OBJECT_CLAS" serializer_version="v1.0.0">
 <asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0">
  <asx:values>
   <VSEOCLASS>
    <CLSNAME>%s</CLSNAME>
    <VERSION>1</VERSION>
    <LANGU>E</LANGU>
    <DESCRIPT>%s</DESCRIPT>
    <STATE>1</STATE>
    <CLSCCINCL>X</CLSCCINCL>
    <FIXPT>X</FIXPT>
    <UNICODE>X</UNICODE>
   </VSEOCLASS>
  </asx:values>
 </asx:abap>
</abapGit>
`, f.ClassName, desc)
			if err := os.WriteFile(xmlFile, []byte(xml), 0644); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "  %s: %d lines\n", f.FileName, lines)
		}
		fmt.Fprintf(os.Stderr, "compiled: %d files, %d total lines ABAP\n", len(files), totalLines)
		return reportGenerated(cmd, sources)
	}

	abap := llvm.Compile(mod, className)
	lines := strings.Count(abap, "\n")
	fmt.Fprintf(os.Stderr, "compiled: %d lines ABAP\n", lines)

	if asZip {
		outFile := output
		if outFile == "" {
			outFile = strings.TrimSuffix(filepath.Base(inputFile), ext) + ".zip"
		}
		if err := writeLLVMZip(outFile, strings.ToUpper(className), abap, desc, pkg); err != nil {
			return err
		}
		return reportGenerated(cmd, map[string]string{strings.ToLower(className) + ".clas.abap": abap})
	}

	if output == "" {
		fmt.Print(abap)
	} else {
		if err := os.WriteFile(output, []byte(abap), 0644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "written: %s\n", output)
	}
	return reportGenerated(cmd, map[string]string{strings.ToLower(className) + ".clas.abap": abap})
}

func writeLLVMZip(outFile, objName, source, desc, pkg string) error {
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)

	zf, _ := w.Create(".abapgit.xml")
	zf.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0">
 <asx:values><DATA>
  <MASTER_LANGUAGE>E</MASTER_LANGUAGE>
  <STARTING_FOLDER>/src/</STARTING_FOLDER>
  <FOLDER_LOGIC>PREFIX</FOLDER_LOGIC>
 </DATA></asx:values>
</asx:abap>
`))

	lower := strings.ToLower(objName)
	zf, _ = w.Create("src/" + lower + ".prog.abap")
	zf.Write([]byte(source))

	descLen := len(desc)
	if descLen > 70 {
		descLen = 70
	}
	zf, _ = w.Create("src/" + lower + ".prog.xml")
	fmt.Fprintf(zf, `<?xml version="1.0" encoding="utf-8"?>
<abapGit version="v1.0.0" serializer="LCL_OBJECT_PROG" serializer_version="v1.0.0">
 <asx:abap xmlns:asx="http://www.sap.com/abapxml" version="1.0">
  <asx:values>
   <PROGDIR><NAME>%s</NAME><SUBC>1</SUBC><FIXPT>X</FIXPT><UCCHECK>X</UCCHECK></PROGDIR>
   <TPOOL><item><ID>R</ID><ENTRY>%s</ENTRY><LENGTH>%d</LENGTH></item></TPOOL>
  </asx:values>
 </asx:abap>
</abapGit>
`, objName, desc, descLen)

	w.Close()
	fmt.Fprintf(os.Stderr, "zip: %s (%s)\n", outFile, objName)
	return nil
}
