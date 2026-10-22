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

// A JSDoc non-null cast `/** @type {T} */ (x)` over `T | undefined` is an
// unchecked view: JS checks nothing at the cast. `??` after it must still
// test the runtime value, because the view can hold undefined.
const nullishCastTypes = `export interface Named { name(): string }
`

const nullishCastPrelude = `/** @typedef {import("./types.ts").Named} Named */
/** @typedef {{kw: string}} Kw */
export class Tok { /** @param {string} s */ constructor(s) { this.s = s; } /** @returns {string} */ name() { return "T" + this.s; } }
export class Ex { /** @param {string} s */ constructor(s) { this.s = s; } /** @returns {string} */ name() { return "E" + this.s; } }
let calls = 0;
/** @param {string} s @returns {Tok} */ const fb = (s) => { calls++; return new Tok(s); };
/** @param {string} s @returns {string} */ const fs = (s) => { calls++; return s; };
export class Probe {
  /** @param {string} k @returns {string} */ static one(k) {
    calls = 0;
    /** @type {Map<string, Tok>} */ const toks = new Map([["a", new Tok("a")]]);
    /** @type {Map<string, Named>} */ const named = new Map([["a", new Ex("n")]]);
    /** @type {Map<string, Tok | Ex>} */ const both = new Map([["a", new Ex("u")]]);
    /** @type {Map<string, Kw>} */ const kws = new Map([["a", {kw: "k"}]]);
    /** @type {Map<string, Set<string>>} */ const sets = new Map([["a", new Set(["s1", "s2"])]]);
    /** @type {Map<string, Map<string, number>>} */ const maps = new Map([["a", new Map([["m", 1]])]]);
    /** @type {Map<string, string[]>} */ const arrs = new Map([["a", ["x", "y"]]]);
    /** @type {Map<string, number>} */ const nums = new Map([["a", 3]]);
    /** @type {{name?: string}} */ const obj = k === "a" ? {name: "nm"} : {};
    /** @type {string} */ const v = %s;
    return v + ";" + calls;
  }
  /** @returns {string} */ static run() { return Probe.one("a") + "|" + Probe.one("zz"); }
}
`

var nullishCastCases = []struct{ name, expr string }{
	{"class", `(/** @type {Tok} */ (toks.get(k)) ?? fb("d")).name()`},
	{"interface", `(/** @type {Named} */ (named.get(k)) ?? fb("d")).name()`},
	{"class_union", `(/** @type {Tok | Ex} */ (both.get(k)) ?? fb("d")).name()`},
	{"shape", `(/** @type {Kw} */ (kws.get(k)) ?? {kw: fs("dflt")}).kw`},
	{"set", `String((/** @type {Set<string>} */ (sets.get(k)) ?? new Set([fs("d")])).size)`},
	{"map", `String((/** @type {Map<string, number>} */ (maps.get(k)) ?? new Map([[fs("d"), 1], ["e", 2]])).size)`},
	{"array", `String((/** @type {string[]} */ (arrs.get(k)) ?? [fs("d")]).length)`},
	{"string", `/** @type {string} */ (obj.name) ?? fs("dflt")`},
	{"string_unknown", `/** @type {string} */ (/** @type {unknown} */ (obj.name)) ?? fs("dflt")`},
	{"number", `String(/** @type {number} */ (nums.get(k)) ?? Number(fs("7")))`},
	{"class_bound", `(() => { const u = /** @type {Tok} */ (toks.get(k)); return (u ?? fb("d")).name(); })()`},
	{"shape_bound", `(() => { const u = /** @type {Kw} */ (kws.get(k)); return (u ?? {kw: fs("dflt")}).kw; })()`},
	{"control_or_class", `(/** @type {Tok} */ (toks.get(k)) || fb("o")).name()`},
	{"control_or_shape", `(/** @type {Kw} */ (kws.get(k)) || {kw: fs("o")}).kw`},
	{"control_optchain_class", `/** @type {Tok} */ (toks.get(k))?.name() ?? fs("q")`},
	{"control_optchain_shape", `/** @type {Kw} */ (kws.get(k))?.kw ?? fs("q")`},
}

func nullishCastSource(expr string) string { return fmt.Sprintf(nullishCastPrelude, expr) }

func TestNullishAfterNonNullCastNodeOracle(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node absent")
	}
	for _, c := range nullishCastCases {
		t.Run(c.name, func(t *testing.T) {
			runMjsOracleTypes(t, nullishCastTypes, nullishCastSource(c.expr), true, true)
		})
	}
}

// A fallback of another type than the asserted one: the checker types the
// `??` as Tok alone, so String() takes a Tok. The present value prints as in
// JS; the absent one reaches the string fallback and refuses at the Tok
// conversion instead of printing "undefined".
func TestNullishAfterNonNullCastOtherFallbackType(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node absent")
	}
	source := strings.Replace(nullishCastSource(`String(/** @type {Tok} */ (toks.get(k)) ?? fs("dflt"))`),
		`/** @returns {string} */ static run() { return Probe.one("a") + "|" + Probe.one("zz"); }`,
		`/** @returns {string} */ static run() { return Probe.one("a"); }
  /** @returns {string} */ static absent() { return Probe.one("zz"); }`, 1)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "types.ts"), []byte(nullishCastTypes), 0600)
	os.WriteFile(filepath.Join(dir, "probe.mjs"), []byte(source), 0600)
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022","module":"nodenext","allowImportingTsExtensions":true,"noEmit":true,"allowJs":true},"include":["*.mjs","*.ts"]}`), 0600)
	want, err := exec.Command("node", "--input-type=module", "-e", `import {Probe} from `+fmt.Sprintf("%q", filepath.Join(dir, "probe.mjs"))+`;console.log(Probe.run() + "|" + Probe.absent())`).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	if string(want) != "[object Object];0|dflt;1\n" {
		t.Fatalf("Node %q", want)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := p.Lower([]string{"probe.mjs", "types.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if errs := hir.Verify(prog); len(errs) != 0 {
		t.Fatal(errs[0])
	}
	main := fmt.Sprintf(`fmt.Println(%s());defer func(){r:=recover();if r==nil {panic("absent value escaped")};fmt.Println("refused")}();fmt.Println(%s())`, goEntry("probe.mjs.Probe", "run"), goEntry("probe.mjs.Probe", "absent"))
	if got := runGoHIR(t, prog, main); got != "[object Object];0\nrefused\n" {
		t.Fatalf("Go %q", got)
	}
}

// TypeScript's `as T` and `x!` erase absence the same way.
func TestNullishAfterNonNullAssertionTSNodeOracle(t *testing.T) {
	source := `class Tok { readonly s: string; constructor(s: string) { this.s = s; } name(): string { return "T" + this.s; } }
type Kw = {kw: string};
let calls = 0;
function fb(s: string): Tok { calls++; return new Tok(s); }
class Probe {
 static one(k: string): string {
  calls = 0;
  const toks = new Map<string, Tok>([["a", new Tok("a")]]);
  const kws = new Map<string, Kw>([["a", {kw: "k"}]]);
  const nums = new Map<string, number>([["a", 3]]);
  const a = (toks.get(k) as Tok ?? fb("d")).name();
  const b = (toks.get(k)! ?? fb("e")).name();
  const c = (kws.get(k) as Kw ?? {kw: "f"}).kw;
  const d = String(nums.get(k)! ?? 7);
  return a + ":" + b + ":" + c + ":" + d + ";" + calls;
 }
 static run(): string { return Probe.one("a") + "|" + Probe.one("zz"); }
}`
	want := conversionOracle(t, source)
	p := structuralUnionProgram(t, source)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

// The undefined test reaches ABAP through the same HIR; run it on OSG when
// ABAPITI_OSG_HOME is set (emit-only otherwise).
func TestNullishAfterNonNullCastABAP(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node absent")
	}
	for _, c := range nullishCastCases {
		// interface: the structural view of Ex as Named alone, without `??`,
		// already raises CX_SY_MOVE_CAST_ERROR on ABAP (?= to an interface Ex
		// does not implement nominally).
		if c.name == "interface" || strings.HasPrefix(c.name, "control_") {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			source := nullishCastSource(c.expr)
			os.WriteFile(filepath.Join(dir, "types.ts"), []byte(nullishCastTypes), 0600)
			os.WriteFile(filepath.Join(dir, "probe.mjs"), []byte(source), 0600)
			os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022","module":"nodenext","allowImportingTsExtensions":true,"noEmit":true,"allowJs":true},"include":["*.mjs","*.ts"]}`), 0600)
			want, err := exec.Command("node", "--input-type=module", "-e", `import {Probe} from `+fmt.Sprintf("%q", filepath.Join(dir, "probe.mjs"))+`;console.log(Probe.run())`).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(want))
			}
			p, err := Load(filepath.Join(dir, "tsconfig.json"))
			if err != nil {
				t.Fatal(err)
			}
			prog, _, err := p.Lower([]string{"probe.mjs", "types.ts"})
			if err != nil {
				t.Fatal(err)
			}
			if errs := hir.Verify(prog); len(errs) != 0 {
				t.Fatal(errs[0])
			}
			storageABAPProbe(t, prog, "probe.mjs.Probe", strings.TrimSpace(string(want)), "")
		})
	}
}
