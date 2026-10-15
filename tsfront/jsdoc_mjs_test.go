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

// runMjsOracle lowers probe.mjs (allowJs, no checkJs) next to a type-only
// types.ts and compares Probe.run() on Go with Node; abap also emits ABAP.
func runMjsOracle(t *testing.T, mjs string, abap bool) {
	t.Helper()
	runMjsOracleTypes(t, "export type Item = {name: string, size: number};\n", mjs, abap)
}

// runMjsOracleTypes is runMjsOracle with its own type-only types.ts.
func runMjsOracleTypes(t *testing.T, types, mjs string, abap bool) []LowerDiagnostic {
	t.Helper()
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
	if abap {
		if _, err := emitABAP(t, prog); err != nil {
			t.Fatal(err)
		}
	}
	return diags
}

// gogen is typed in place with JSDoc: same-line annotations, @typedef and
// inline import("./x.ts") types, JS properties created by this.x = …, and
// module-level let.
func TestJSDocMjsNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @typedef {import("./types.ts").Item} Item */
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
`, true)
}

// Presence of JS this-properties follows execution, as in JavaScript: x
// (written only in a method) and y (only on one constructor branch) are own
// keys only once written. Observed via JSON.stringify and Object.keys before
// and after. ABAP refuses this JSON (unproven own-key order), so Go only.
func TestJSThisPropertyPresenceNodeOracle(t *testing.T) {
	runMjsOracle(t, `export class Lazy {
  /** @param {boolean} flag */ constructor(flag) { this.a = 1; if (flag) { this.y = "set"; } }
  /** @param {number} v */ fill(v) { this.x = v; }
}
export class Probe {
  /** @param {Lazy} o @returns {string} */ static show(o) { return JSON.stringify(o) + "/" + Object.keys(o).join(","); }
  /** @returns {string} */ static run() {
    const p = new Lazy(false), q = new Lazy(true);
    const before = Probe.show(p) + ";" + Probe.show(q);
    p.fill(5);
    q.fill(6);
    return before + ";" + Probe.show(p) + ";" + Probe.show(q);
  }
}
`, false)
}
