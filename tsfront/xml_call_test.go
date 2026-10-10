package tsfront

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImmediateXMLSubsetCall(t *testing.T) {
	for _, accepted := range []bool{true, false} {
		t.Run(fmt.Sprint(accepted), func(t *testing.T) {
			dir := t.TempDir()
			pkg := filepath.Join(dir, "node_modules", "fast-xml-parser")
			if err := os.MkdirAll(pkg, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(pkg, "index.d.ts"), []byte(`export declare class XMLParser { constructor(options: any); parse(input: string): any; }`), 0600); err != nil {
				t.Fatal(err)
			}
			opts := `parseTagValue: false, ignoreAttributes: true, trimValues: false`
			if !accepted {
				opts = `parseTagValue: true, ignoreAttributes: true, trimValues: false`
			}
			source := `import {XMLParser as Parser} from "fast-xml-parser"; export class Probe { static run(): string { const v = new Parser({` + opts + `}).parse("<a><b>x&amp;y</b></a>"); return v.a.b; } }`
			for f, s := range map[string]string{"probe.ts": source, "tsconfig.json": `{"compilerOptions":{"strict":true,"moduleResolution":"node"},"files":["probe.ts"]}`} {
				if err := os.WriteFile(filepath.Join(dir, f), []byte(s), 0600); err != nil {
					t.Fatal(err)
				}
			}
			p, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			prog, diags, err := p.Lower([]string{"probe.ts"})
			if err != nil {
				t.Fatal(err)
			}
			if !accepted {
				for _, d := range diags {
					if d.Category == "unsupported-xml-parser" {
						return
					}
				}
				t.Fatal(diags)
			}
			for _, d := range diags {
				if !strings.HasPrefix(d.Category, "note-") {
					t.Fatal(d)
				}
			}
			if errors := hir.Verify(prog); len(errors) != 0 {
				t.Fatal(errors)
			}
			if _, err := abap.Emit(prog); err != nil {
				t.Fatal(err)
			}
			got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s().String())", goEntry("probe.ts.Probe", "run")))
			if strings.TrimSpace(got) != "x&y" {
				t.Fatal(got)
			}
		})
	}
}
