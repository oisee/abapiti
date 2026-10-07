// Package tsfront is the TypeScript front end of the TS→HIR→ABAP pipeline:
// it loads a tsgo (TypeScript 7 in Go, vendored under internal/tsgo) Program
// from a project's real tsconfig.json and dumps a deterministic JSON view of
// the checked syntax trees (classes, functions, and a checker type on every
// expression). Only this package sees tsgo types.
package tsfront

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/bundled"
	"github.com/oisee/abapiti/internal/tsgo/compiler"
	"github.com/oisee/abapiti/internal/tsgo/core"
	"github.com/oisee/abapiti/internal/tsgo/diagnostics"
	"github.com/oisee/abapiti/internal/tsgo/locale"
	"github.com/oisee/abapiti/internal/tsgo/tsoptions"
	"github.com/oisee/abapiti/internal/tsgo/vfs/osvfs"
)

// Program is a tsgo program built from a tsconfig.json. The checker runs on
// demand, one source file at a time (single-threaded).
type Program struct {
	prog      *compiler.Program
	configDir string                     // absolute directory of the tsconfig
	files     map[string]*ast.SourceFile // cleaned absolute path -> file
}

// Load builds a Program for the project described by tsconfigPath. The
// tsconfig's include/exclude/files and compiler options are honoured; this is
// the project's own configuration, not a synthesized one.
func Load(tsconfigPath string) (*Program, error) {
	abs, err := filepath.Abs(tsconfigPath)
	if err != nil {
		return nil, fmt.Errorf("resolve tsconfig path: %w", err)
	}
	currentDir := filepath.Dir(abs)
	fs := bundled.WrapFS(osvfs.FS())
	host := compiler.NewCachedFSCompilerHost(currentDir, fs, bundled.LibPath(), nil /*extendedConfigCache*/, nil /*trace*/)

	cfg, readErrs := tsoptions.GetParsedCommandLineOfConfigFile(abs, nil /*options*/, nil /*optionsRaw*/, host, nil /*extendedConfigCache*/)
	if cfg == nil {
		return nil, fmt.Errorf("parse tsconfig %s failed:\n%s", abs, FormatDiagnostics(readErrs))
	}
	if errs := errorDiagnostics(readErrs, cfg.Errors); len(errs) > 0 {
		return nil, fmt.Errorf("parse tsconfig %s failed:\n%s", abs, FormatDiagnostics(errs))
	}

	p := compiler.NewProgram(compiler.ProgramOptions{
		Config:         cfg,
		Host:           host,
		SingleThreaded: core.TSTrue,
	})
	files := make(map[string]*ast.SourceFile, len(p.SourceFiles()))
	for _, f := range p.SourceFiles() {
		files[filepath.Clean(f.FileName())] = f
	}
	return &Program{prog: p, configDir: currentDir, files: files}, nil
}

// SourceFiles returns the (sorted) file names of the program.
func (p *Program) SourceFiles() []string {
	out := make([]string, 0, len(p.files))
	for name := range p.files {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// File looks a source file up by (absolute, working-directory-relative, or
// tsconfig-directory-relative) path.
func (p *Program) File(name string) (*ast.SourceFile, bool) {
	if f, ok := p.fileByPath(name); ok {
		return f, true
	}
	if p.configDir != "" {
		return p.fileByPath(filepath.Join(p.configDir, name))
	}
	return nil, false
}

func (p *Program) fileByPath(name string) (*ast.SourceFile, bool) {
	abs, err := filepath.Abs(name)
	if err != nil {
		abs = name
	}
	f, ok := p.files[filepath.Clean(abs)]
	return f, ok
}

// CheckerDiagnostics runs the checker over the files of the program and
// returns all diagnostics of the program, mirroring what tsc reports for
// the same configuration: configuration parsing, program-level (compiler
// options and include resolution, e.g. files listed in the tsconfig that do
// not exist), per-file syntactic and include-resolution, global, and
// semantic diagnostics (emit-only ones are filtered out under noEmit,
// exactly as tsc filters them) — plus declaration diagnostics when the
// configuration asks for declaration checking. Declaration files (.d.ts)
// of the project are included; the checker itself decides whether they are
// actually type-checked (skipLibCheck / skipDefaultLibCheck gate it the
// same way tsc does). The result is sorted by location.
func (p *Program) CheckerDiagnostics(ctx context.Context) []Diagnostic {
	var out []Diagnostic
	add := func(ds []*ast.Diagnostic) {
		for _, d := range ds {
			out = append(out, NewDiagnostic(d))
		}
	}

	// Configuration parsing and program-level diagnostics (options,
	// include resolution).
	add(p.prog.GetConfigFileParsingDiagnostics())
	add(p.prog.GetProgramDiagnostics())

	var sources []*ast.SourceFile
	for _, f := range p.prog.SourceFiles() {
		sources = append(sources, f)
		add(p.prog.GetSyntacticDiagnostics(ctx, f))
		add(p.prog.GetIncludeProcessorDiagnostics(f))
	}

	// Global checker diagnostics; like upstream tsc, ask again after
	// semantic checking when the first call found nothing (checking may add
	// new globals, e.g. missing global types under noLib).
	globals := p.prog.GetGlobalDiagnostics(ctx)
	add(globals)
	// Semantic diagnostics; upstream filters out diagnostics that only
	// matter when emitting (e.g. TS1216 for `export const __esModule`
	// under CommonJS) once noEmit is set — apply the same filter.
	for _, ds := range p.prog.GetSemanticDiagnosticsWithoutNoEmitFiltering(ctx, sources) {
		add(compiler.FilterNoEmitSemanticDiagnostics(ds, p.prog.Options()))
	}
	if len(globals) == 0 {
		add(p.prog.GetGlobalDiagnostics(ctx))
	}

	// Declaration diagnostics, when declaration checking is enabled and
	// there is nothing else to report (as upstream gates it).
	if opts := p.prog.Options(); opts.GetEmitDeclarations() && opts.NoEmit.IsTrue() && len(out) == 0 {
		for _, f := range sources {
			add(p.prog.GetDeclarationDiagnostics(ctx, f))
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Loc < out[j].Loc || (out[i].Loc == out[j].Loc && out[i].Code < out[j].Code)
	})
	return out
}

// Diagnostic is a checker diagnostic in a compact, deterministic form.
type Diagnostic struct {
	Category string `json:"category"`
	Code     int32  `json:"code"`
	Message  string `json:"message,omitempty"`
	Loc      string `json:"loc"` // file:line:col
}

// NewDiagnostic converts a tsgo diagnostic.
func NewDiagnostic(d *ast.Diagnostic) Diagnostic {
	out := Diagnostic{
		Category: d.Category().Name(),
		Code:     d.Code(),
		Message:  MessageText(d),
	}
	file := d.File()
	if file != nil {
		out.Loc = locString(file, d.Pos())
	} else {
		out.Loc = "<global>"
	}
	return out
}

// MessageText formats a diagnostic's message (best effort; falls back to the
// message key).
func MessageText(d *ast.Diagnostic) (text string) {
	defer func() {
		if recover() != nil || text == "" {
			text = string(d.MessageKey())
		}
	}()
	return diagnostics.Localize(locale.Default, nil, d.MessageKey(), d.MessageArgs()...)
}

// FormatDiagnostics renders diagnostics, one per line.
func FormatDiagnostics(ds []*ast.Diagnostic) string {
	var b strings.Builder
	for _, d := range ds {
		fmt.Fprintf(&b, "%s TS%d: %s (%s)\n", d.Category().Name(), d.Code(), MessageText(d), NewDiagnostic(d).Loc)
	}
	return b.String()
}

func errorDiagnostics(lists ...[]*ast.Diagnostic) []*ast.Diagnostic {
	var out []*ast.Diagnostic
	for _, list := range lists {
		for _, d := range list {
			if d.Category() == diagnostics.CategoryError {
				out = append(out, d)
			}
		}
	}
	return out
}

// locString renders "file:line:col" (1-based) for a position in f.
func locString(f *ast.SourceFile, pos int) string {
	line, col := lineCol(f, pos)
	return fmt.Sprintf("%s:%d:%d", f.FileName(), line, col)
}

// lineCol returns the 1-based line and column of pos in f. Positions are
// byte offsets into the file text, but the column is counted in UTF-16
// code units — what tsc reports and editors display (é is one unit, an
// emoji outside the BMP two) — never in UTF-8 bytes.
func lineCol(f *ast.SourceFile, pos int) (line, col int) {
	starts := f.ECMALineMap() // sorted ascending, byte offsets
	idx := sort.Search(len(starts), func(i int) bool { return starts[i] > core.TextPos(pos) }) - 1
	if idx < 0 {
		idx = 0
	}
	return idx + 1, int(core.UTF16Len(f.Text()[int(starts[idx]):pos])) + 1
}
