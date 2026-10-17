package tsfront

import "testing"

// frontend.mjs table inventory: /** @type {Table | undefined} */ const
// baseTable = reg.getObject("TABL", b) stores the general overload's
// IObject | undefined. Unchecked JS keeps the object (or undefined).
func TestStep4JSOptionalDowncastNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface IObject { getName(): string }
export class Table implements IObject { readonly name: string; constructor(name: string) { this.name = name; } getName(): string { return this.name; } fields(): string { return this.name + "-f"; } }
export class Registry { getObject(type: string, name: string): IObject | undefined { return type === "TABL" && name !== "none" ? new Table(name) : undefined; } }
`, `import {Registry} from "./types.ts";
/** @param {Registry} reg @param {string} b @returns {string} */
function base(reg, b) {
  /** @type {import("./types.ts").Table | undefined} */ const baseTable = reg.getObject("TABL", b);
  return baseTable?.fields() ?? "missing";
}
export class Probe {
  /** @returns {string} */ static run() { return base(new Registry(), "t1") + "," + base(new Registry(), "none"); }
}
`, false, true)
}
