package tsfront

import "testing"

// frontend-base.mjs sameType(a, b): a.k === b.k && (a.k !== "table" ||
// sameSort(a, b)). Only a is narrowed to the table member; unchecked JS
// passes b, a Type, to sameSort's TableType parameter. b is a table there.
func TestStep4JSMemberProjectionNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type Type = {readonly k: "i"} | {readonly k: "c"; readonly len: number} | {readonly k: "table"; readonly row: Type; readonly sorted?: string[]; readonly unique?: boolean};
export type TableType = Extract<Type, {k: "table"}>;
`, `/** @param {import("./types.ts").TableType} a @param {import("./types.ts").TableType} b @returns {boolean} */
const sameSort = (a, b) => (a.sorted ?? null) === null ? (b.sorted ?? null) === null : (b.sorted !== undefined && a.sorted.join() === b.sorted.join() && a.unique === b.unique);
/** @param {import("./types.ts").Type} a @param {import("./types.ts").Type} b @returns {boolean} */
const sameType = (a, b) => a.k === b.k && (a.k !== "table" || (sameType(a.row, b.row) && sameSort(a, b)));
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {import("./types.ts").Type} */ const i = {k: "i"};
    /** @type {import("./types.ts").Type} */ const t1 = {k: "table", row: i, sorted: ["A"], unique: true};
    /** @type {import("./types.ts").Type} */ const t2 = {k: "table", row: i, sorted: ["A"], unique: true};
    /** @type {import("./types.ts").Type} */ const t3 = {k: "table", row: i};
    /** @type {import("./types.ts").Type} */ const c = {k: "c", len: 1};
    return [sameType(t1, t2), sameType(t1, t3), sameType(t3, t3), sameType(i, c), sameType(c, c), sameType(t1, i)].map((b) => String(b)).join(",");
  }
}
`, false, true)
}
