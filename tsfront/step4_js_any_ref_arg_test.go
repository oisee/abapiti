package tsfront

import "testing"

// frontend.mjs ownInterfaceAttributes: program.reg.getObject("INTF", x)
// returns IObject, so obj?.getDefinition() and obj.getMainABAPFile() are
// any in unchecked JS; the any value top reaches walk(top), whose JSDoc
// declares a StructureNode. JavaScript passes the object on.
func TestStep4JSAnyReferenceArgumentNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface IObject { getName(): string }
export class Struct { readonly name: string; readonly kids: Struct[]; constructor(name: string, kids: Struct[]) { this.name = name; this.kids = kids; } getChildren(): Struct[] { return this.kids; } }
export class File { getStructure(): Struct | undefined { return new Struct("a", [new Struct("b", [new Struct("c", [])]), new Struct("d", [])]); } }
export class Intf implements IObject { getName(): string { return "I"; } getMainFile(): File | undefined { return new File(); } }
export class Registry { getObject(name: string): IObject | undefined { return name === "I" ? new Intf() : undefined; } }
`, `import {Registry, Struct} from "./types.ts";
/** @param {Registry} reg @param {string} name @returns {string} */
function attributes(reg, name) {
  const obj = reg.getObject(name);
  /** @type {string[]} */ const out = [];
  /** @param {Struct} node @returns {void} */ const walk = (node) => {
    out.push(node.name);
    for (const c of node.getChildren()) walk(c);
  };
  const top = obj?.getMainFile()?.getStructure();
  if (top) walk(top);
  return out.join(",");
}
export class Probe {
  /** @returns {string} */ static run() { return attributes(new Registry(), "I") + "|" + attributes(new Registry(), "X"); }
}
`, false, true)
}
