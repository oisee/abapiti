package tsfront

import "testing"

// OSG G batch 10 (eb38612e) declared `interface FrontendSelectColumns extends
// Array<FrontendSelectColumn> { grouped?: boolean; groupBy?: string[] }`. It
// was lowered as a data class whose fields adopted Array's own member
// symbols, so every other `xs.length` read and write became a field access
// ("field on non-class"). The interface is the array ABI; length stays the
// array length everywhere.
func TestStep4ArrayInterfaceNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Column { readonly name: string; readonly sql?: string }
export interface Columns extends Array<Column> { grouped?: boolean; groupBy?: string[] }
`, `/** @param {string[]} names @returns {import("./types.ts").Columns} */
function columns(names) {
  /** @type {import("./types.ts").Columns} */ const cols = [];
  for (const n of names) cols.push({name: n});
  return cols;
}
/** @param {number[]} xs @returns {number} */
function shrink(xs) { let n = 0; while (n < xs.length && xs[n] > 0) n++; xs.length = n; return xs.length; }
export class Probe {
  /** @returns {string} */ static run() {
    const a = columns(["A", "B"]), b = columns([]);
    return [String(a.length), a.map((c) => c.name).join(""), String(b.length), String(shrink([3, 2, 0, 5])), String("xyz".length)].join(",");
  }
}
`, false, true)
}
