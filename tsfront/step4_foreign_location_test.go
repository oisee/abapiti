package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Utility discovery revisits a foreign type alias while lowering a short
// caller. Its nodes still belong to the imported file, including when an
// intersection alias adds the named actual payload to a recursive family.
func TestStep4ForeignFamilyDiagnosticLocation(t *testing.T) {
	dir := t.TempDir()
	types := "// " + strings.Repeat("padding", 2400) + "\n" + `export type Expr = {k:'lit'; value:symbol} | {k:'call'; args:Argument[]};
/*é😀*/ export type Argument = Extract<Expr, {k:'call'}>;
export type EventArgument = Argument & {readonly name:string; readonly value:Expr};
`
	input := `import type {Expr,Argument,EventArgument} from './types';
export class Probe {
 first(x:Expr):void {}
 argument(x:Argument):void {}
 event(x:EventArgument):void {}
}`
	for name, source := range map[string]string{
		"types.ts": types, "input.ts": input,
		"tsconfig.json": `{"compilerOptions":{"target":"ES2022","strict":true},"files":["input.ts","types.ts"]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, ds, err := p.Lower([]string{"input.ts", "types.ts"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range ds {
		if strings.Contains(d.Message, "recursive family") && strings.HasPrefix(d.Loc, "types.ts:3:40") {
			found = true
		}
		if strings.Contains(d.Loc, dir) {
			t.Fatalf("absolute location: %v", d)
		}
	}
	if !found {
		t.Fatalf("missing foreign alias diagnostic at types.ts:3:40: %v", ds)
	}
}

func TestStep4DiagnosticPositionFallback(t *testing.T) {
	p := loadSnippet(t, "/*é😀*/ export class C {}\n", `{"noEmit":true}`)
	f, ok := p.File("main.ts")
	if !ok {
		t.Fatal("missing source")
	}
	for _, pos := range []int{-1, len(f.Text()) + 1} {
		if got := p.locString(f, pos); got != "main.ts:1:1" {
			t.Fatalf("invalid %d: %s", pos, got)
		}
	}
	if got := p.locString(f, strings.Index(f.Text(), "export")); got != "main.ts:1:9" {
		t.Fatal(got)
	}
	if got := p.locString(f, len(f.Text())); got != "main.ts:2:1" {
		t.Fatal(got)
	}
	l := &lowerer{prog: p, file: f}
	if got := l.locOf(nil); got != "main.ts:1:1" {
		t.Fatal(got)
	}
	if got := p.locString(nil, 100); got != "<unknown>" {
		t.Fatal(got)
	}
}
