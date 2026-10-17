package tsfront

import "testing"

// Object.fromEntries filling an optional record slot (frontend.mjs
// registerConst: {go, type, value: Object.fromEntries(fields.map(...))} with
// value?: Record<string, string | undefined>) uses the record as pair context.
func TestStep4FromEntriesOptionalRecordNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Const { readonly go: string; readonly value?: Record<string, string | undefined> }
`, `/** @param {string} v @returns {string} */
const unquote = (v) => v.slice(1, -1);
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {Map<string, string>} */ const byName = new Map([["a", "'x'"], ["c", "'z'"]]);
    const fields = [{name: "a"}, {name: "b"}, {name: "c"}];
    /** @type {Map<string, import("./types.ts").Const>} */ const consts = new Map();
    consts.set("k", {go: "k", value: Object.fromEntries(fields.map((f) => [f.name, byName.has(f.name) ? unquote(byName.get(f.name)) : undefined]))});
    const v = consts.get("k").value ?? {};
    return Object.keys(v).join(",") + ":" + String(v.a) + String(v.b) + String(v.c) + ":" + String("b" in v);
  }
}
`, false, true)
}
