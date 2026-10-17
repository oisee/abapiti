package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

// frontend.mjs table inventory (OSG a8887bec): `/** @type {Table | View} */
// const o = obj; withArtifact(..., () => { /** @type {Table | undefined} */
// const table = o instanceof Table ? o : undefined; ... })`. obj is an
// ArtifactObject, whose public parsedType slot meets Table's private one, so
// the checker narrows o and the true branch to never. The guard held at run
// time: the branch is o, checked as a Table. (Other reads of o still see the
// checker's View; OSG's G fix casts obj through unknown.)
func TestStep4InstanceofNeverConditionalNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface ArtifactObject { getName(): string; parsedType?: unknown }
`, `class Table { constructor() { /** @private */ this.parsedType = 1; } /** @returns {string} */ getName() { return "T"; } /** @returns {string[]} */ listKeys() { return ["K1", "K2"]; } }
class View { /** @returns {string} */ getName() { return "V"; } }
/** @param {() => string} f */ const withArtifact = (f) => f();
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {import("./types.ts").ArtifactObject[]} */ const objs = [new Table(), new View()];
    const out = [];
    for (const obj of objs) {
      const view = obj instanceof View;
      if (!(obj instanceof Table) && !view) continue;
      /** @type {Table | View} */ const o = obj;
      out.push(withArtifact(() => {
        /** @type {Table | undefined} */ const table = o instanceof Table ? o : undefined;
        return table === undefined ? "none" : table.listKeys().join("+");
      }));
    }
    return out.join(",");
  }
}
`, false, true)
}

// The true branch is a checked view of o as Table, not o's declared union.
func TestStep4InstanceofNeverBranchIsTable(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "types.ts"), []byte("export interface ArtifactObject { getName(): string; parsedType?: unknown }\nexport type Expr = {readonly k: \"lit\"; readonly v: string} | {readonly k: \"neg\"; readonly inner: Expr};\n"), 0600)
	os.WriteFile(filepath.Join(dir, "probe.mjs"), []byte(`class Table { constructor() { /** @private */ this.parsedType = 1; } /** @returns {string[]} */ listKeys() { return ["K"]; } /** @returns {string} */ getName() { return "T"; } }
class View { /** @returns {string} */ getName() { return "V"; } }
/** @param {import("./types.ts").ArtifactObject} obj @returns {string} */
export function keys(obj) {
  /** @type {import("./types.ts").Expr} */ const e = {k: "neg", inner: {k: "lit", v: obj.getName()}};
  if (e.k === "neg" && e.inner.k === "lit" && e.inner.v === "") return "";
  /** @type {Table | undefined} */ const table = obj instanceof Table ? obj : undefined;
  return table?.listKeys().join("") ?? "";
}
`), 0600)
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022","module":"nodenext","allowImportingTsExtensions":true,"noEmit":true,"allowJs":true},"include":["*.mjs","*.ts"]}`), 0600)
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := p.Lower([]string{"probe.mjs", "types.ts"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	var expr func(*hir.Expr)
	var stmt func(*hir.Stmt)
	expr = func(x *hir.Expr) {
		if x == nil {
			return
		}
		if x.Kind == hir.Conditional && x.Y != nil && x.Y.Type.Kind == hir.ClassRef && strings.HasSuffix(x.Y.Type.Name, ".Table") {
			found = true
		}
		expr(x.X)
		expr(x.Y)
		expr(x.Z)
		for _, a := range x.Args {
			expr(a)
		}
		stmt(x.Stmt)
	}
	stmt = func(s *hir.Stmt) {
		if s == nil {
			return
		}
		expr(s.X)
		expr(s.Y)
		stmt(s.Body)
		stmt(s.Else)
		for _, c := range s.List {
			stmt(c)
		}
	}
	for _, c := range prog.Classes {
		for _, m := range c.Methods {
			stmt(m.Body)
		}
	}
	if !found {
		t.Fatal("o instanceof Table ? o : undefined: true branch is not a Table")
	}
}
