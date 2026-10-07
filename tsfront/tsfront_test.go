package tsfront

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// lineOf extracts the 1-based line number from a "file:line:col" location.
func lineOf(t *testing.T, loc string) int {
	t.Helper()
	parts := strings.Split(loc, ":")
	if len(parts) != 3 {
		t.Fatalf("bad location %q", loc)
	}
	line, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatalf("bad location %q: %v", loc, err)
	}
	return line
}

func loadFixture(t *testing.T) *Program {
	t.Helper()
	program, err := Load(filepath.Join("testdata", "project", "tsconfig.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return program
}

func fixtureFiles(t *testing.T, program *Program, names ...string) []string {
	t.Helper()
	var files []string
	for _, name := range names {
		f, ok := program.File(filepath.Join("testdata", "project", name))
		if !ok {
			t.Fatalf("file %s not in program (have %v)", name, program.SourceFiles())
		}
		files = append(files, f.FileName())
	}
	return files
}

func TestLoadAndCheckFixture(t *testing.T) {
	program := loadFixture(t)
	// Three project files plus at least one bundled lib file.
	if len(program.SourceFiles()) < 4 {
		t.Errorf("program has only %d files", len(program.SourceFiles()))
	}
	diags := program.CheckerDiagnostics(context.Background())
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %+v", d)
	}
}

func TestDumpClasses(t *testing.T) {
	program := loadFixture(t)
	files := fixtureFiles(t, program, "base.ts", "derived.ts")
	dumps, err := program.Dump(files)
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if len(dumps) != 2 {
		t.Fatalf("got %d file dumps, want 2", len(dumps))
	}

	var circle *ClassDump
	var shape *ClassDump
	for i := range dumps {
		for j := range dumps[i].Classes {
			switch dumps[i].Classes[j].Name {
			case "Circle":
				circle = &dumps[i].Classes[j]
			case "Shape":
				shape = &dumps[i].Classes[j]
			}
		}
	}
	if shape == nil || circle == nil {
		t.Fatalf("classes not found: %+v", dumps)
	}

	if !shape.Abstract {
		t.Error("Shape should be abstract")
	}
	if shape.Extends != nil {
		t.Errorf("Shape extends: %+v", shape.Extends)
	}
	if len(shape.Members) != 4 {
		t.Errorf("Shape members: %+v", shape.Members)
	}
	// area(): number
	for _, m := range shape.Members {
		if m.Name == "area" {
			if m.Kind != "method" || m.ReturnType != "number" || m.Abstract != true {
				t.Errorf("area member: %+v", m)
			}
		}
		if m.Name == "name" && (m.DeclaredType != "string" || m.Visibility != "protected") {
			t.Errorf("name member: %+v", m)
		}
	}

	if circle.Abstract {
		t.Error("Circle should not be abstract")
	}
	if len(circle.Extends) != 1 || circle.Extends[0].Name != "Shape" {
		t.Fatalf("Circle extends: %+v", circle.Extends)
	}
	if !strings.Contains(circle.Extends[0].Decl, "base.ts:") {
		t.Errorf("extends not resolved to declaration: %q", circle.Extends[0].Decl)
	}
	member := map[string]MemberDump{}
	for _, m := range circle.Members {
		member[m.Name] = m
	}
	if m, ok := member["radius"]; !ok || m.Visibility != "private" || m.DeclaredType != "number" {
		t.Errorf("radius member: %+v (ok=%v)", m, ok)
	}
	if m, ok := member["count"]; !ok || !m.Static || m.DeclaredType != "number" {
		t.Errorf("count member: %+v (ok=%v)", m, ok)
	}
	if m, ok := member["area"]; !ok || m.ReturnType != "number" {
		t.Errorf("area member: %+v (ok=%v)", m, ok)
	}
	if m, ok := member["constructor"]; !ok || len(m.Parameters) != 1 || m.Parameters[0].Type != "number" {
		t.Errorf("constructor member: %+v (ok=%v)", m, ok)
	}
}

func TestDumpFunctions(t *testing.T) {
	program := loadFixture(t)
	files := fixtureFiles(t, program, "base.ts")
	dumps, err := program.Dump(files)
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if len(dumps[0].Functions) != 1 {
		t.Fatalf("functions: %+v", dumps[0].Functions)
	}
	fn := dumps[0].Functions[0]
	if fn.Name != "label" || fn.ReturnType != "string | undefined" {
		t.Errorf("label function: %+v", fn)
	}
	if len(fn.Parameters) != 1 || fn.Parameters[0].Name != "shape" || fn.Parameters[0].Type != "Shape" {
		t.Errorf("label parameters: %+v", fn.Parameters)
	}
}

func TestDumpExpressions(t *testing.T) {
	program := loadFixture(t)
	files := fixtureFiles(t, program, "base.ts", "derived.ts")
	dumps, err := program.Dump(files)
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if len(dumps[0].Expressions) == 0 {
		t.Fatal("no expressions dumped for base.ts")
	}

	// Property accesses in derived.ts resolve to the declaring class:
	// this.radius is declared by Circle itself (constructor assignment and
	// the two reads in area), this.describe()/this.name are Shape's.
	radius, own := 0, true
	describe, inherited := 0, true
	for _, e := range dumps[1].Expressions {
		if e.Kind != "PropertyAccessExpression" {
			continue
		}
		switch e.Symbol {
		case "radius":
			radius++
			own = own && e.DeclaringClass == "Circle" && !e.Inherited
		case "describe", "name":
			describe++
			inherited = inherited && e.DeclaringClass == "Shape" && e.Inherited
		}
	}
	if radius != 3 || !own {
		t.Errorf("this.radius accesses: got %d, all own in Circle = %v", radius, own)
	}
	if describe != 2 || !inherited {
		t.Errorf("this.describe()/this.name accesses: got %d, all inherited from Shape = %v", describe, inherited)
	}

	// Identifier resolution: `shape` in label() resolves to the parameter
	// declared in base.ts, and `a` (line 17) to the `const a` on line 16.
	// Symbol declarations are located at the declaration's first token:
	// `const a` puts `a` at 16:9 (raw node.Pos() would point at the
	// preceding trivia, 16:8).
	var shapeParam *ExprDump
	for i, e := range dumps[0].Expressions {
		if e.Kind == "Identifier" && e.Symbol == "shape" && strings.HasSuffix(e.SymDecl, "base.ts:15:23") {
			shapeParam = &dumps[0].Expressions[i]
		}
		if e.Kind == "Identifier" && e.Symbol == "a" && e.SymDecl != "" {
			if !strings.HasSuffix(e.SymDecl, "base.ts:16:9") {
				t.Errorf("identifier `a` symbolDecl %q, want ...base.ts:16:9 (token start)", e.SymDecl)
			}
		}
	}
	if shapeParam == nil {
		t.Error("no identifier `shape` resolved to its parameter declaration in base.ts")
	} else if shapeParam.Type != "Shape" {
		t.Errorf("identifier `shape` type: got %q, want Shape", shapeParam.Type)
	}

	// Type flags of specific expressions in base.ts's label():
	//   shape.area() -> number, a > 1 -> boolean, the conditional
	//   a > 1 ? shape.describe() : undefined -> "string | undefined" (union),
	//   " " -> string, undefined -> undefined.
	flagsForType := map[string][]string{}
	for _, e := range dumps[0].Expressions {
		if len(e.Flags) > 0 {
			flagsForType[e.Type] = e.Flags
		}
	}
	for typ, want := range map[string]string{
		"number":             "number",
		"boolean":            "boolean",
		"string | undefined": "union",
		"undefined":          "undefined",
		"string":             "string",
	} {
		if got := flagsForType[typ]; len(got) == 0 || got[0] != want {
			t.Errorf("flags for type %q: got %v, want [%s]", typ, got, want)
		}
	}
}

func TestDumpBoxMembers(t *testing.T) {
	program := loadFixture(t)
	files := fixtureFiles(t, program, "box.ts")
	dumps, err := program.Dump(files)
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if len(dumps[0].Classes) != 1 || dumps[0].Classes[0].Name != "Box" {
		t.Fatalf("classes: %+v", dumps[0].Classes)
	}
	member := map[string][]MemberDump{}
	for _, m := range dumps[0].Classes[0].Members {
		member[m.Name] = append(member[m.Name], m)
	}

	// Constructor parameter properties appear as class properties.
	for _, m := range member["value"] {
		if m.Kind != "property" {
			t.Errorf("param property value: kind %q, want property", m.Kind)
		}
		if m.Visibility != "public" || m.DeclaredType != "number" {
			t.Errorf("param property value: vis=%q type=%q, want public number", m.Visibility, m.DeclaredType)
		}
	}
	if len(member["value"]) == 0 {
		t.Error("no `value` param property member; constructor(public value: number) lost")
	}
	if len(member["secret"]) == 0 {
		t.Fatal("no `secret` param property member; constructor(private secret: string = \"s\") lost")
	}
	secret := member["secret"][0]
	if secret.Kind != "property" || secret.Visibility != "private" {
		t.Errorf("param property secret: %+v", secret)
	}
	if secret.Optional {
		t.Error("param property secret with default value must stay a required property")
	}
	if !strings.HasPrefix(secret.DeclaredType, "string") {
		t.Errorf("param property secret type: %q", secret.DeclaredType)
	}
	// The constructor *parameter* for the same member is optional (it has
	// a default), even though the property it declares is required.
	ctor := member["constructor"][0]
	if len(ctor.Parameters) != 2 || !ctor.Parameters[1].Optional || ctor.Parameters[1].Name != "secret" {
		t.Errorf("constructor parameters: %+v, want secret optional", ctor.Parameters)
	}

	// #private fields are private.
	hidden := member["#hidden"]
	if len(hidden) != 1 {
		t.Fatalf("#hidden members: %+v", hidden)
	}
	if hidden[0].Kind != "property" || hidden[0].Visibility != "private" {
		t.Errorf("#hidden member: %+v, want private property", hidden[0])
	}

	// The explicit `this` parameter keeps its own type; defaulted
	// parameters are optional and checker-typed.
	offset := member["offset"]
	if len(offset) != 1 || len(offset[0].Parameters) != 2 {
		t.Fatalf("offset member: %+v", offset)
	}
	thisParam, byParam := offset[0].Parameters[0], offset[0].Parameters[1]
	if thisParam.Name != "this" || thisParam.Type != "Box" {
		t.Errorf("this parameter: %+v, want this: Box", thisParam)
	}
	if byParam.Name != "by" || !byParam.Optional || byParam.Type != "number" {
		t.Errorf("by parameter: %+v, want optional number", byParam)
	}

	// Template-literal-typed expressions carry the string flag (the
	// `as BoxTag` assertion is typed `box${string}`).
	found := false
	for _, e := range dumps[0].Expressions {
		if e.Kind == "AsExpression" && e.Type == "`box${string}`" {
			found = true
			if len(e.Flags) != 1 || e.Flags[0] != "string" {
				t.Errorf("AsExpression `box${string}` flags: %v, want [string]", e.Flags)
			}
		}
	}
	if !found {
		t.Error("no AsExpression typed `box${string}` found")
	}
}

func TestDumpLocationsAreTokenStarts(t *testing.T) {
	program := loadFixture(t)
	files := fixtureFiles(t, program, "box.ts")
	dumps, err := program.Dump(files)
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	d := dumps[0]
	if !strings.HasSuffix(d.Classes[0].Loc, "box.ts:1:1") {
		t.Errorf("class Box loc: %q", d.Classes[0].Loc)
	}
	// Every member location must be on the member's own line, not at the
	// end of the preceding node's trivia (box.ts: constructor line 2,
	// #hidden line 4, offset line 6).
	for _, m := range d.Classes[0].Members {
		var want int
		switch m.Name {
		case "constructor", "value", "secret":
			want = 2
		case "#hidden":
			want = 4
		case "offset":
			want = 6
		default:
			t.Errorf("unexpected member %q", m.Name)
		}
		if got := lineOf(t, m.Loc); got != want {
			t.Errorf("member %q loc %q is line %d, want line %d", m.Name, m.Loc, got, want)
		}
	}
	// The named function on line 13 must be located on line 13 (not at the
	// preceding type alias's end).
	if len(d.Functions) != 1 || d.Functions[0].Name != "tag" {
		t.Fatalf("functions: %+v", d.Functions)
	}
	if got := lineOf(t, d.Functions[0].Loc); got != 13 {
		t.Errorf("function tag loc %q is line %d, want line 13", d.Functions[0].Loc, got)
	}
	// Expression spans are token starts too: the string literal "s" in
	// line 2 and the identifier `by` in line 7.
	for _, e := range d.Expressions {
		if e.Kind == "StringLiteral" && e.Type == `"s"` {
			if got := lineOf(t, e.Span); got != 2 {
				t.Errorf("string literal \"s\" span %q is line %d, want line 2", e.Span, got)
			}
		}
		if e.Symbol == "by" {
			if got := lineOf(t, e.Span); got != 7 {
				t.Errorf("identifier `by` span %q is line %d, want line 7", e.Span, got)
			}
		}
	}
}

func TestDumpDeterministic(t *testing.T) {
	program := loadFixture(t)
	files := fixtureFiles(t, program, "base.ts", "derived.ts", "box.ts")
	first, err := program.Dump(files)
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	a, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	second, err := program.Dump(files)
	if err != nil {
		t.Fatalf("Dump (2nd): %v", err)
	}
	b, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal (2nd): %v", err)
	}
	if string(a) != string(b) {
		t.Error("dump is not deterministic")
	}
}

// TestDumpDeterministicFreshLoad re-Loads the program from scratch before
// each dump: repeated loads must not leak state (memoized maps, symbol order)
// into the JSON.
func TestDumpDeterministicFreshLoad(t *testing.T) {
	files := []string{"base.ts", "derived.ts", "box.ts"}
	var first, second []byte
	for i, out := range []*[]byte{&first, &second} {
		program := loadFixture(t)
		dumps, err := program.Dump(fixtureFiles(t, program, files...))
		if err != nil {
			t.Fatalf("Dump (load %d): %v", i, err)
		}
		data, err := json.Marshal(dumps)
		if err != nil {
			t.Fatalf("marshal (load %d): %v", i, err)
		}
		*out = data
	}
	if string(first) != string(second) {
		t.Error("dump of two freshly loaded programs is not deterministic")
	}
}

// TestLoadConfigIncludeExcludeExtends checks that Load honours the tsconfig's
// extends/include/exclude: skip.ts and skipdir/** are excluded, everything
// else under testdata/config is in, and the compiler options inherited from
// tsconfig.base.json (strict) are applied.
func TestLoadConfigIncludeExcludeExtends(t *testing.T) {
	program, err := Load(filepath.Join("testdata", "config", "tsconfig.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	have := map[string]bool{}
	for _, name := range program.SourceFiles() {
		have[filepath.Base(name)] = true
		if strings.HasSuffix(name, filepath.Join("testdata", "config", "skip.ts")) {
			t.Errorf("excluded skip.ts is in the program")
		}
		if strings.HasSuffix(name, filepath.Join("testdata", "config", "skipdir", "gone.ts")) {
			t.Errorf("excluded skipdir/gone.ts is in the program")
		}
	}
	for _, want := range []string{"keep.ts", "keep2.ts"} {
		if !have[want] {
			t.Errorf("%s missing from the program", want)
		}
	}
	// strict (noImplicitAny) comes from the extended base config; keep.ts
	// has an implicit-any parameter that must be diagnosed.
	diags := program.CheckerDiagnostics(context.Background())
	found := false
	for _, d := range diags {
		if d.Code == 7006 && strings.Contains(d.Loc, "keep.ts:2:") {
			found = true
		}
	}
	if !found {
		t.Errorf("no TS7006 (implicit any) from extended strict config; diags: %+v", diags)
	}
}

// TestFileRelativeToConfigDir checks File's documented lookups from a
// working directory that is not the project directory.
func TestFileRelativeToConfigDir(t *testing.T) {
	program, err := Load(filepath.Join("testdata", "config", "tsconfig.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// cwd (tsfront/) has no keep.ts; this must resolve relative to the
	// tsconfig's directory.
	if _, ok := program.File("keep.ts"); !ok {
		t.Error("File(keep.ts) not found via tsconfig-relative lookup")
	}
	if _, ok := program.File(filepath.Join("keepdir", "keep2.ts")); !ok {
		t.Error("File(keepdir/keep2.ts) not found via tsconfig-relative lookup")
	}
	// Absolute paths keep working.
	abs, err := filepath.Abs(filepath.Join("testdata", "config", "keep.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := program.File(abs); !ok {
		t.Error("File(absolute keep.ts) not found")
	}
	// Excluded files are not part of the program.
	if _, ok := program.File("skip.ts"); ok {
		t.Error("File(skip.ts) found although excluded")
	}
}

// TestCheckerDiagnosticsMissingFile: a tsconfig listing a file that does
// not exist must produce a diagnostic (TS6053), not silence.
func TestCheckerDiagnosticsMissingFile(t *testing.T) {
	program, err := Load(filepath.Join("testdata", "missing", "tsconfig.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := program.CheckerDiagnostics(context.Background())
	if len(diags) == 0 {
		t.Fatal("no diagnostics for a missing file listed in the tsconfig")
	}
	found := false
	for _, d := range diags {
		if d.Code == 6053 && strings.Contains(d.Message, "missing.ts") {
			found = true
		}
		if d.Loc == "" {
			t.Errorf("diagnostic without location: %+v", d)
		}
	}
	if !found {
		t.Errorf("no TS6053 for missing.ts; diagnostics: %+v", diags)
	}
}

// TestCheckerDiagnosticsDeclarationFiles: declaration files of the project
// are checked like any other file, gated by skipLibCheck (as tsc does). With
// skipLibCheck false, the missing types in bad.d.ts and also.d.mts are
// reported (TS2304); with skipLibCheck true, the same project is clean.
func TestCheckerDiagnosticsDeclarationFiles(t *testing.T) {
	program, err := Load(filepath.Join("testdata", "libcheck", "tsconfig.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := program.File("bad.d.ts"); !ok {
		t.Error("bad.d.ts not part of the program")
	}
	diags := program.CheckerDiagnostics(context.Background())
	for _, want := range []struct{ file, name string }{
		{"bad.d.ts", "MissingType"},
		{"also.d.mts", "MissingToo"},
	} {
		found := false
		for _, d := range diags {
			if d.Code == 2304 && strings.Contains(d.Loc, want.file+":1:") && strings.Contains(d.Message, want.name) {
				found = true
			}
		}
		if !found {
			t.Errorf("no TS2304 for %s in %s; diagnostics: %+v", want.name, want.file, diags)
		}
	}

	program, err = Load(filepath.Join("testdata", "libcheck", "tsconfig.skiplib.json"))
	if err != nil {
		t.Fatalf("Load (skipLibCheck): %v", err)
	}
	for _, d := range program.CheckerDiagnostics(context.Background()) {
		t.Errorf("skipLibCheck=true, unexpected diagnostic: %+v", d)
	}
}

// TestCheckerDiagnosticsGlobalNoLib: with noLib, the checker reports the
// missing global types as *global* diagnostics (Loc "<global>"), which only
// appear while global diagnostics are collected.
func TestCheckerDiagnosticsGlobalNoLib(t *testing.T) {
	program, err := Load(filepath.Join("testdata", "nolib", "tsconfig.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := program.CheckerDiagnostics(context.Background())
	found := false
	for _, d := range diags {
		if d.Code == 2318 && strings.Contains(d.Message, "global type 'Array'") {
			found = true
			if d.Loc != "<global>" {
				t.Errorf("global diagnostic loc %q, want <global>", d.Loc)
			}
		}
	}
	if !found {
		t.Errorf("no TS2318 for the missing global Array type; diagnostics: %+v", diags)
	}
}

// TestCheckerDiagnosticsDeclarationEmit: with declaration + noEmit, the
// un-exported type leaking into main's declarations is reported (TS4023)
// by declaration diagnostics only.
func TestCheckerDiagnosticsDeclarationEmit(t *testing.T) {
	program, err := Load(filepath.Join("testdata", "decdiag", "tsconfig.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	diags := program.CheckerDiagnostics(context.Background())
	found := false
	for _, d := range diags {
		if d.Code == 4023 && strings.Contains(d.Loc, "main.ts:6:") {
			found = true
		}
	}
	if !found {
		t.Errorf("no TS4023 declaration diagnostic; diagnostics: %+v", diags)
	}
}

// loadSnippet writes a one-file project (main.ts plus a tsconfig with the
// given compilerOptions JSON object) into a temp directory and Loads it.
// For regression tests whose fixture does not fit the committed testdata
// projects (non-ASCII sources, unusual option combinations).
func loadSnippet(t *testing.T, source, options string) *Program {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	tsconfig := `{"compilerOptions": ` + options + `, "files": ["main.ts"]}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	program, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return program
}

// TestDumpOptionalMembers: the question token of a property or method
// declaration (`x?: number`, `method?(): void`) makes the dumped member
// optional; members without `?` stay required.
func TestDumpOptionalMembers(t *testing.T) {
	program := loadSnippet(t, "export class C { x?: number; method?(): void; req = 0 }\n", `{"noEmit": true}`)
	if diags := program.CheckerDiagnostics(context.Background()); len(diags) != 0 {
		t.Fatalf("invalid fixture: %+v", diags)
	}
	dumps, err := program.Dump([]string{"main.ts"})
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if len(dumps[0].Classes) != 1 || dumps[0].Classes[0].Name != "C" {
		t.Fatalf("classes: %+v", dumps[0].Classes)
	}
	for _, m := range dumps[0].Classes[0].Members {
		want := m.Name == "x" || m.Name == "method" // `req` has no `?`
		if m.Optional != want {
			t.Errorf("member %q optional = %v, want %v", m.Name, m.Optional, want)
		}
	}
}

// TestCheckerDiagnosticsNoEmitFiltering: `export const __esModule` under
// CommonJS reports TS1216 (a marker reserved by the module transform), but
// only when something is emitted: with noEmit, upstream tsc filters the
// diagnostic out and so must tsfront.
func TestCheckerDiagnosticsNoEmitFiltering(t *testing.T) {
	const source = "export const __esModule = 1;\n"
	const options = `{"strict": true, "target": "es2020", "module": "commonjs", "noEmit": `

	// Without noEmit the fixture really produces TS1216 (so the noEmit run
	// below is not vacuously clean).
	program := loadSnippet(t, source, options+`false}`)
	found := false
	for _, d := range program.CheckerDiagnostics(context.Background()) {
		if d.Code == 1216 {
			found = true
		}
	}
	if !found {
		t.Error("no TS1216 without noEmit; fixture does not trigger the emit-only diagnostic")
	}

	// With noEmit the same project is clean, as tsc reports it.
	program = loadSnippet(t, source, options+`true}`)
	for _, d := range program.CheckerDiagnostics(context.Background()) {
		t.Errorf("noEmit should suppress emit-only diagnostics: %+v", d)
	}
}

// TestDumpUTF16Columns: locations count columns in UTF-16 code units (as
// tsc does), not UTF-8 bytes. In "/*\u00e9\U0001f600*/ export class C {}"
// the `export` token starts at byte 12 but at UTF-16 column 9 (\u00e9 is
// one code unit, the emoji two); likewise for diagnostic positions.
func TestDumpUTF16Columns(t *testing.T) {
	program := loadSnippet(t, "/*\u00e9\U0001f600*/ export class C {}\n", `{"noEmit": true}`)
	dumps, err := program.Dump([]string{"main.ts"})
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if len(dumps[0].Classes) != 1 || dumps[0].Classes[0].Name != "C" {
		t.Fatalf("classes: %+v", dumps[0].Classes)
	}
	if loc := dumps[0].Classes[0].Loc; !strings.HasSuffix(loc, "main.ts:1:9") {
		t.Errorf("class C loc %q, want UTF-16 column 9 (main.ts:1:9)", loc)
	}

	// Diagnostic locations use the same UTF-16 columns: the unresolved name
	// sits at UTF-16 column 9 (byte column 12).
	program = loadSnippet(t, "/*\u00e9\U0001f600*/ missingName;\n", `{"noEmit": true}`)
	found := false
	for _, d := range program.CheckerDiagnostics(context.Background()) {
		if d.Code == 2304 {
			found = true
			if !strings.HasSuffix(d.Loc, "main.ts:1:9") {
				t.Errorf("TS2304 loc %q, want UTF-16 column 9 (main.ts:1:9)", d.Loc)
			}
		}
	}
	if !found {
		t.Error("no TS2304 in fixture; the diagnostic position is not covered")
	}
}

// TestLoadFailsOnBadConfig: configuration errors are reported by Load.
func TestLoadFailsOnBadConfig(t *testing.T) {
	_, err := Load(filepath.Join("testdata", "bad", "tsconfig.json"))
	if err == nil {
		t.Fatal("Load accepted an invalid target es999")
	}
	if !strings.Contains(err.Error(), "TS6046") {
		t.Errorf("error does not mention TS6046: %v", err)
	}
}

// logDiagnosticsBounded logs at most the first 10 diagnostics without
// failing the test; the caller reports the failure. (Bounding keeps a very
// broken project from flooding the log, and must not slice past the end —
// the original diags[:10] panicked for 1-9 diagnostics.)
func logDiagnosticsBounded(t *testing.T, diags []Diagnostic) {
	t.Helper()
	n := len(diags)
	if n > 10 {
		n = 10
	}
	for _, d := range diags[:n] {
		t.Logf("diagnostic: %+v", d)
	}
}

// TestLogDiagnosticsBounded is the default-running regression test for the
// bounded diagnostic log the abaplint integration uses: any count below,
// at, or above the bound must be safe (a plain diags[:10] panicked for
// 1-9 diagnostics).
func TestLogDiagnosticsBounded(t *testing.T) {
	for _, n := range []int{0, 1, 9, 10, 11, 37} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			diags := make([]Diagnostic, n)
			for i := range diags {
				diags[i] = Diagnostic{Category: "error", Code: 1, Loc: "<global>"}
			}
			logDiagnosticsBounded(t, diags) // must not panic
		})
	}
}

// TestAbaplintCore runs against the real abaplint core checkout. Set
// TSFRONT_ABAPLINT to ~/dev/abaplint/packages/core to enable it. No npm
// install is needed.
func TestAbaplintCore(t *testing.T) {
	root := os.Getenv("TSFRONT_ABAPLINT")
	if root == "" {
		t.Skip("set TSFRONT_ABAPLINT=<abaplint/packages/core> to run")
	}
	program, err := Load(filepath.Join(root, "tsconfig.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if diags := program.CheckerDiagnostics(context.Background()); len(diags) != 0 {
		logDiagnosticsBounded(t, diags)
		t.Fatalf("%d checker diagnostics, want 0", len(diags))
	}

	var files []string
	dir := filepath.Join(root, "src", "abap", "1_lexer")
	if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".ts") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no lexer files under %s", dir)
	}
	dumps, err := program.Dump(files)
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	exprs := 0
	classes := 0
	for _, d := range dumps {
		exprs += len(d.Expressions)
		classes += len(d.Classes)
	}
	t.Logf("dumped %d files, %d classes, %d expressions", len(dumps), classes, exprs)
	if classes == 0 || exprs == 0 {
		t.Error("empty dump")
	}
}
