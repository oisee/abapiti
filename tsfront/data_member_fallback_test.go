package tsfront

import "testing"

// Data interface members the general mapping could not type: always-absent
// members (x?: never) and optional methods (isDirty?(): boolean).
func TestDataInterfaceOptionalMembersNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface LiteralValue { readonly quote: string; readonly value: string; readonly refusal?: never }
export interface LiteralRefusal { readonly refusal: ""; readonly quote?: never; readonly value?: never }
export interface Artifact { getName(): string; isDirty?(): boolean; label?: string }
`, `/** @typedef {import("./types.ts").LiteralValue} LiteralValue */
/** @typedef {import("./types.ts").LiteralRefusal} LiteralRefusal */
/** @typedef {import("./types.ts").Artifact} Artifact */
/** @param {LiteralValue | LiteralRefusal} l @returns {string} */
const show = (l) => l.refusal === undefined ? "v:" + String(l.value) : "r:" + String(l.value) + String(l.quote);
/** @param {Artifact} a @returns {string} */
const dirty = (a) => a.getName() + ":" + String(a.isDirty?.() ?? "unknown");
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {LiteralValue} */ const v = {quote: "'", value: "x"};
    /** @type {LiteralRefusal} */ const r = {refusal: ""};
    /** @type {Artifact} */ const a = {getName: () => "A", isDirty: () => false};
    /** @type {Artifact} */ const b = {getName: () => "B"};
    return [show(v), show(r), dirty(a), dirty(b)].join(" ");
  }
}
`, false, true)
}
