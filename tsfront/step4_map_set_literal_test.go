package tsfront

import "testing"

// frontend.mjs localConstant: `program.consts.set(go, {go, type, value:
// Object.fromEntries(...)})` with consts a Map<string, FrontendConstant>.
// The literal value is built as the map's value type, like an assignment.
func TestStep4MapSetLiteralNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface FrontendConstant { go: string; type: string; value?: string | Record<string, string | undefined> }
export interface Program { consts: Map<string, FrontendConstant> }
`, `/** @param {string} s @returns {string} */ const unquote = (s) => s.slice(1, -1);
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {import("./types.ts").Program} */ const program = {consts: new Map()};
    const byName = new Map([["A", "'1'"], ["B", "'x'"]]);
    const fields = [{name: "A"}, {name: "B"}, {name: "C"}];
    const go = "c1", type = "T";
    program.consts.set(go, {go, type, value: Object.fromEntries(fields.map((f) => [f.name, byName.has(f.name) ? unquote(byName.get(f.name) ?? "") : undefined]))});
    program.consts.set("c2", ({go: "c2", type: "U", value: "v"}));
    program.consts.set("c3", {go: "c3", type});
    return [...program.consts.values()].map((c) => c.go + c.type + typeof c.value + (typeof c.value === "string" ? c.value : "")).join(",");
  }
}
`, false)
}
