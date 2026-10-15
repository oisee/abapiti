package tsfront

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// gogen is typed in place with JSDoc (allowJs, no checkJs): same-line
// annotations, @typedef and inline import("./x.ts") types, JS properties
// created by this.x = …, and module-level let. Node runs the same .mjs.
func TestJSDocMjsNodeOracle(t *testing.T) {
	types := "export type Item = {name: string, size: number};\n"
	mjs := `/** @typedef {import("./types.ts").Item} Item */
/** @type {number} */ let counter = 0;
let label = "n";
export class Box {
  /** @param {Item[]} items */ constructor(items) { /** @type {Item[]} */ this.items = items; this.tag = label + items.length; }
  /** @returns {number} */ total() { let s = 0; for (const i of this.items) s += i.size; return s; }
  /** @param {string} t */ retag(t) { this.tag = t; label = t; }
}
export class Probe {
  /** @param {import("./types.ts").Item[]} items @returns {string} */ static describe(items) {
    counter++;
    const b = new Box(items);
    const before = b.tag;
    b.retag("x" + counter);
    return items.map((i) => i.name).join(",") + ":" + b.total() + ":" + counter + ":" + before + ":" + b.tag + ":" + label;
  }
  /** @returns {string} */ static run() {
    return Probe.describe([{name: "a", size: 1}, {name: "b", size: 2}]) + "|" + Probe.describe([]);
  }
}
`
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "types.ts"), []byte(types), 0600)
	os.WriteFile(filepath.Join(dir, "probe.mjs"), []byte(mjs), 0600)
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022","module":"nodenext","allowImportingTsExtensions":true,"noEmit":true,"allowJs":true},"include":["*.mjs","*.ts"]}`), 0600)
	want, err := exec.Command("node", "--input-type=module", "-e", `import {Probe} from `+fmt.Sprintf("%q", filepath.Join(dir, "probe.mjs"))+`;console.log(Probe.run())`).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.Lower([]string{"probe.mjs"})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Errorf("lower: %s", d)
		}
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs[0])
	}
	got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.mjs.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if _, err := emitABAP(t, prog); err != nil {
		t.Fatal(err)
	}
}
