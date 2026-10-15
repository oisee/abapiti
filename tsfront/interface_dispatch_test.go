package tsfront

import "testing"

// A method missing from the receiver's interface (or union view) but
// defined by classes behind it is dispatched by a runtime class test, most
// derived first; no matching class throws a TypeError, an optional call
// yields undefined; differing parameter types convert pure arguments per
// class.
func TestInterfaceMethodDispatchNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface IObject { getName(): string }
`, `/** @typedef {import("./types.ts").IObject} IObject */
export class Table { /** @returns {string} */ getName() { return "T"; } /** @param {number} n @returns {string} */ parseType(n) { return "table" + n; } /** @param {number} n @returns {string[]} */ listKeys(n) { return ["k" + n]; } }
export class SubTable extends Table { /** @param {number} n @returns {string} */ parseType(n) { return "sub" + n; } }
export class View { /** @returns {string} */ getName() { return "V"; } }
export class IntfDef { /** @param {string} n */ constructor(n) { this.n = n; } }
export class ClassDef { /** @param {string} n */ constructor(n) { this.n = n; } }
export class Intf { constructor() { /** @type {IntfDef | undefined} */ this.def = new IntfDef("i"); } /** @param {IntfDef | undefined} d @returns {void} */ setDefinition(d) { this.def = d; } }
export class Klass { constructor() { /** @type {ClassDef | undefined} */ this.def = new ClassDef("c"); } /** @param {ClassDef | undefined} d @returns {void} */ setDefinition(d) { this.def = d; } }
/** @param {IObject} o @returns {string} */
const parse = (o) => { try { return o.parseType(2); } catch (e) { return e instanceof TypeError ? "TypeError" : "other"; } };
/** @param {IObject} o @returns {string} */
const keys = (o) => (o.listKeys?.(3) ?? ["none"]).join(",");
/** @param {Intf | Klass} obj @returns {string} */
const reset = (obj) => { obj.setDefinition(undefined); return obj.def === undefined ? "undefined" : "set"; };
export class Probe {
  /** @returns {string} */ static run() {
    return [parse(new Table()), parse(new SubTable()), parse(new View()), keys(new Table()), keys(new View()), reset(new Intf()), reset(new Klass())].join(" ");
  }
}
`, false, true)
}
