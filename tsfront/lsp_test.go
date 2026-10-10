package tsfront

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func lspProbe(t *testing.T, source string) (*hir.Program, []LowerDiagnostic) {
	t.Helper()
	dir := t.TempDir()
	for _, pkg := range RegistryNodePackages() {
		if pkg.Name == "vscode-languageserver-types" {
			if err := pkg.WriteEmbedded(filepath.Join(dir, "node_modules", pkg.Name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	for name, text := range map[string]string{
		"main.ts":       source,
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022","module":"node16","moduleResolution":"node16","noEmit":true},"files":["main.ts"]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	if ds := p.CheckerDiagnostics(context.Background()); len(ds) != 0 {
		t.Fatal(ds)
	}
	prog, ds, err := p.Lower([]string{"main.ts"})
	if err != nil {
		t.Fatal(err)
	}
	return prog, ds
}

func TestLSPConstructors(t *testing.T) {
	const source = `import * as L from "vscode-languageserver-types";
import {Position as P, Range as R, TextEdit as E, TextDocumentEdit as D, RenameFile as F} from "vscode-languageserver-types";
export class Probe {
 static log: string = "";
 static tick(x: number): number { Probe.log = Probe.log + x; return x; }
 static order(): string { Probe.log = ""; const r = R.create(Probe.tick(1), Probe.tick(2), Probe.tick(3), Probe.tick(4)); return Probe.log + "|" + r.end.character; }
 static identity(): number { const p = P.create(1,2); const r = R.create(p,p); p.line = 9; return r.end.line; }
 static commandAbsent(): boolean { return L.Command.create("title", "command").arguments === undefined; }
 static position(): number { return P.create(2, 3).character; }
 static rangeNumbers(): number { return R.create(1, 2, 3, 4).end.character; }
 static rangePositions(): number { return L.Range.create(P.create(1, 2), P.create(3, 4)).start.line; }
 static replace(): string { return E.replace(R.create(1, 2, 3, 4), "hello").newText; }
 static document(): string { return D.create({uri: "file", version: 1}, [E.replace(R.create(1, 2, 3, 4), "edit")]).textDocument.uri; }
 static command(): string { return L.Command.create("title", "command").title; }
 static commandArgs(): number { return L.Command.create("title", "command", "x", 42).arguments!.length; }
 static rename(): string { return F.create("old", "new").newUri; }
 static renameOptions(): boolean { return F.create("old", "new", {overwrite: true}, "id").options!.overwrite!; }
 static symbol(): string { const s: L.DocumentSymbol = {name:"root", kind: L.SymbolKind.Class, range: R.create(1,2,3,4), selectionRange: R.create(1,2,3,4), children: []}; return s.name; }
}`
	prog, ds := lspProbe(t, source)
	for _, d := range ds {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Error(d)
		}
	}
	if t.Failed() {
		return
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs)
	}
	files, names, err := abap.EmitNamed(prog)
	if err != nil {
		t.Fatal(err)
	}
	body := files[names.Get("main.ts.Probe")+".clas.abap"]
	for _, shape := range []string{"Position", "Range", "TextEdit", "TextDocumentEdit", "Command", "RenameFile"} {
		owner := "node_modules/vscode-languageserver-types/lib/umd/main.d.ts." + shape
		if !strings.Contains(body, "NEW "+names.Get(owner)+"(") {
			t.Errorf("ABAP missing construction of %s", owner)
		}
	}
	tests := []struct{ name, want string }{
		{"order", "1234|4"}, {"identity", "9"}, {"commandAbsent", "true"}, {"position", "3"}, {"rangeNumbers", "4"}, {"rangePositions", "1"}, {"replace", "hello"}, {"document", "file"}, {"command", "title"}, {"commandArgs", "2"}, {"rename", "new"}, {"renameOptions", "true"}, {"symbol", "root"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Both backends consume the same ordinary named data-shape constructors.
			if _, err := abap.Emit(prog); err != nil {
				t.Fatal(err)
			}
			got := runGoHIR(t, prog, "fmt.Print("+goEntry("main.ts.Probe", tc.name)+"())")
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLSPUnsupportedAndShadowed(t *testing.T) {
	_, ds := lspProbe(t, `import {TextDocumentEdit} from "vscode-languageserver-types"; export class Probe { static run(): boolean { return TextDocumentEdit.is({}); } }`)
	found := false
	for _, d := range ds {
		if d.Category == "unsupported-lsp-call" && strings.Contains(d.Message, "TextDocumentEdit.is") {
			found = true
		}
	}
	if !found {
		t.Fatal(ds)
	}
	prog := lowerStatementsProbe(t, map[string]string{"main.ts": `class Range { static create(a: number, b: number): number { return a+b; } } export class Probe { static run(): number { return Range.create(2,3); } }`}, []string{"main.ts"})
	if got := runGoHIR(t, prog, "fmt.Print("+goEntry("main.ts.Probe", "run")+"())"); got != "5" {
		t.Fatal(got)
	}
}
