package tsfront

import "testing"

// frontend.mjs interface signatures: all is any (getDefinition() through the
// general IObject overload), so Array.from(Array.isArray(all) ? all :
// all.getAll()) and its map callback parameter are any. The elements are
// the method definition objects, not strings.
func TestStep4JSAnyArrayFromNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface IObject { getName(): string }
export class MethodDef { readonly name: string; constructor(name: string) { this.name = name; } getName(): string { return this.name; } }
export class Defs { readonly list: MethodDef[]; constructor(list: MethodDef[]) { this.list = list; } getAll(): MethodDef[] { return this.list; } }
export class IntfDef { getMethodDefinitions(): Defs { return new Defs([new MethodDef("m1"), new MethodDef("m2")]); } }
export class Intf implements IObject { getName(): string { return "I"; } getDefinition(): IntfDef | undefined { return new IntfDef(); } }
export class Registry { getObject(type: string, name: string): IObject | undefined { return name === "I" ? new Intf() : undefined; } }
`, `import {Registry} from "./types.ts";
/** @param {unknown} s @returns {string} */ const upper = (s) => String(s).toUpperCase();
/** @param {Registry} reg @param {string[]} names @returns {string} */
function signatures(reg, names) {
  /** @type {string[]} */ const sigs = [];
  for (const i of names) {
    const all = reg.getObject("INTF", i)?.getDefinition()?.getMethodDefinitions();
    if (!all) continue;
    sigs.push(...Array.from(Array.isArray(all) ? all : all.getAll()).map((m) => i + "~" + upper(m.getName())));
  }
  return sigs.join(",");
}
export class Probe {
  /** @returns {string} */ static run() { return signatures(new Registry(), ["I", "X"]); }
}
`, false, true)
}
