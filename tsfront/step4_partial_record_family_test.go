package tsfront

import "testing"

// frontend.mjs table inventory: `/** @type {Partial<Record<string,
// WriteType>>} */ const types = {C: {abap: "C", len}, P: {abap: "P", len,
// dec}, I: {abap: "I"}, ...}; const ir = types[kind] ?? types.g;`. Each value
// is a member of the WriteType union, typed by the record's value type.
func TestStep4PartialRecordFamilyNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type WriteType =
  | {readonly abap: 'C' | 'X'; readonly len: number}
  | {readonly abap: 'P'; readonly len: number; readonly dec: number}
  | {readonly abap: 'I' | 'INT8' | 'STRING' | 'D' | 'XSTRING'};
export interface WriteColumn { readonly name: string; readonly ir: WriteType | null }
export type Cell = {readonly k: 'leaf'; readonly w: WriteType} | {readonly k: 'pair'; readonly l: Cell; readonly r: Cell};
`, `/** @param {import("./types.ts").WriteType | null} w @returns {string} */
const show = (w) => w === null ? "null" : "set";
/** @param {string} kind @param {number} len @param {number} dec @returns {import("./types.ts").WriteColumn} */
function column(kind, len, dec) {
  /** @type {Partial<Record<string, import("./types.ts").WriteType>>} */ const types = {C: {abap: "C", len}, I: {abap: "I"}, "8": {abap: "INT8"}, P: {abap: "P", len: 2 * len - 1, dec},
    g: {abap: "STRING"}, X: {abap: "X", len}}; const ir = types[kind] ?? types.g;
  return {name: kind, ir};
}
/** @param {import("./types.ts").Cell} c @returns {number} */ const leaves = (c) => c.k === "leaf" ? 1 : leaves(c.l) + leaves(c.r);
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {import("./types.ts").Cell} */ const cell = {k: "pair", l: {k: "leaf", w: {abap: "I"}}, r: {k: "leaf", w: {abap: "C", len: 1}}};
    return leaves(cell) + ":" + ["C", "I", "8", "P", "g", "X", "h"].map((k) => show(column(k, 4, 2).ir)).join(",");
  }
}
`, false, true)
}
