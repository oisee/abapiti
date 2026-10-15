package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Partial fields: checkJs:false JavaScript (OSG gogen's JSDoc-typed .mjs)
// reads a field that only some members declare; the others yield undefined.
const partialFieldTypes = `export type T = {k:'leaf'; value:number; tag:string} | {k:'pair'; left:T; right:T} | {k:'opt'; value?:number; kid:T};
export type Part = {text:string} | {value:number; opts?:{w:number}};
`

func countPartialNotes(diags []LowerDiagnostic) int {
	notes := 0
	for _, d := range diags {
		if d.Category == "note-partial-field-read" {
			notes++
		}
	}
	return notes
}

func TestFamilyPartialFieldNodeOracle(t *testing.T) {
	diags := runMjsOracleTypes(t, partialFieldTypes, `/** @typedef {import("./types.ts").T} T */
export class Probe {
  /** @param {T} p @returns {string} */ static show(p) {
    const v = p.value;
    const t = p.tag ?? "-";
    const kid = p.kid;
    const len = p.tag?.length;
    const truthy = p.value ? "T" : "F";
    let s = p.k + ":" + String(v) + ":" + t + ":" + String(len) + ":" + truthy + ":" + (kid === undefined ? "none" : Probe.show(kid));
    if (p.k === "pair") s += "(" + Probe.show(p.left) + "," + Probe.show(p.right) + ")";
    return s;
  }
  /** @returns {string} */ static run() {
    /** @type {T} */ const leaf = {k: "leaf", value: 0, tag: "ab"};
    /** @type {T} */ const one = {k: "leaf", value: 7, tag: ""};
    /** @type {T} */ const opt = {k: "opt", kid: one};
    /** @type {T} */ const opt2 = {k: "opt", value: 3, kid: leaf};
    return Probe.show({k: "pair", left: opt, right: {k: "pair", left: opt2, right: leaf}});
  }
}
`, false)
	if n := countPartialNotes(diags); n != 5 {
		t.Fatalf("partial-field notes: %d, want 5", n)
	}
}

func TestUnionPartialFieldNodeOracle(t *testing.T) {
	diags := runMjsOracleTypes(t, partialFieldTypes, `/** @typedef {import("./types.ts").Part} Part */
export class Probe {
  /** @param {Part} p @returns {string} */ static part(p) {
    const text = p.text;
    const opts = p.opts ?? {w: 0};
    return (text !== undefined ? JSON.stringify(text) : "v" + String(p.value)) + ":" + opts.w + ":" + (text ? "T" : "F") + ":" + String(p.text?.length);
  }
  /** @returns {string} */ static run() {
    /** @type {Part[]} */ const parts = [{text: "a"}, {value: 2}, {text: ""}, {value: 3, opts: {w: 4}}];
    return parts.map((p) => Probe.part(p)).join(";");
  }
}
`, false)
	if n := countPartialNotes(diags); n < 3 {
		t.Fatalf("partial-field notes: %d", n)
	}
}

// A field no member declares stays an error, in JavaScript too.
func TestFamilyAbsentFieldRefusal(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"types.ts": partialFieldTypes,
		"probe.mjs": `/** @typedef {import("./types.ts").T} T */
export class Probe {
  /** @param {T} p */ static bad(p) { return p.nothing; }
}
`,
		"tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ES2022","module":"nodenext","allowImportingTsExtensions":true,"noEmit":true,"allowJs":true},"include":["*.mjs","*.ts"]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, diags, err := p.Lower([]string{"probe.mjs"})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") && strings.HasPrefix(d.Loc, "probe.mjs:3:") {
			return
		}
	}
	t.Fatalf("field absent from every member was not refused: %v", diags)
}
