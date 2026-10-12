package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestJSON5CheckedPackageCall(t *testing.T) {
	dir := t.TempDir()
	for _, pkg := range RegistryNodePackages() {
		if pkg.Name == "json5" {
			if err := pkg.WriteEmbedded(filepath.Join(dir, "node_modules", pkg.Name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	source := `import * as J from "json5";import {parse as read} from "json5";
 export class Probe {
 static run(): string {return JSON.stringify(J.parse("{a:'x',b:[0x10,+.5,],}"))+JSON.stringify(read("[1.,]"));}
 static bad(): string {try {return JSON.stringify(J.parse("{a:}"));}catch(e){return "swallowed";}}
 }`
	for name, text := range map[string]string{"main.ts": source, "tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022","module":"node16","moduleResolution":"node16"},"files":["main.ts"]}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, ds, err := p.Lower([]string{"main.ts"})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if es := hir.Verify(prog); len(es) != 0 {
		t.Fatal(es)
	}
	if got := runGoHIR(t, prog, "fmt.Print("+goEntry("main.ts.Probe", "run")+"().String())"); got != `{"a":"x","b":[16,0.5]}[1]` {
		t.Fatal(got)
	}
	got := runGoHIR(t, prog, "defer func(){if x:=recover();x!=nil {fmt.Print(x)}}();fmt.Print("+goEntry("main.ts.Probe", "bad")+"().String())")
	if !strings.Contains(got, "JSON5 SyntaxError payload ABI is unsupported: JSON5: invalid character '}' at 1:4") {
		t.Fatal(got)
	}
	for _, input := range []string{`J.parse("{}", (_k,v)=>v)`, `J.parse(1)`, `J.stringify({a:1})`} {
		if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(`import * as J from "json5"; export class Probe {static run(): string {return JSON.stringify(`+input+`);}}`), 0600); err != nil {
			t.Fatal(err)
		}
		p, err := Load(filepath.Join(dir, "tsconfig.json"))
		if err != nil {
			t.Fatal(err)
		}
		_, ds, err := p.Lower([]string{"main.ts"})
		if err != nil || !hasBlocking(ds) {
			t.Fatal("accepted unsupported JSON5 operation", input, err, ds)
		}
	}
}
