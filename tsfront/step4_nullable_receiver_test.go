package tsfront

import "testing"

// A JSDoc receiver typed T | null | undefined is boxed as Dynamic. A method
// call on it (after a === null test, or plain in unchecked JS) views the box
// through T: JavaScript calls the method on the present value
// (frontend.mjs constructor(): c.body.getChildren()).
var step4NullableReceiver = [2]string{`export class Node { readonly name: string; readonly kids: Node[]; constructor(name: string, kids: Node[]) { this.name = name; this.kids = kids; } getChildren(): Node[] { return this.kids; } getName(): string { return this.name; } }
export interface Item { body: Node | null | undefined; text: string }
export class File { getRaw(): string { return "<x/>"; } }
export class Obj { readonly f: File | undefined; constructor(f: File | undefined) { this.f = f; } getXMLFile(): File | undefined { return this.f; } }
export class Reg { getObject(t: string): Obj | undefined { return t === "x" ? new Obj(new File()) : t === "y" ? new Obj(undefined) : undefined; } }
`, `import {Node, Reg} from "./types.ts";
/** @param {string} n @param {Node[]} kids @returns {Node} */
const node = (n, kids) => new Node(n, kids);
/** @param {import("./types.ts").Item} c @returns {number} */
function count(c) { if (c.body === null || c.body.getChildren().length !== 1) return -1; return c.body.getChildren().length; }
/** @param {Node | null} body @returns {string} */
function names(body) { let s = ""; for (const k of body?.getChildren() ?? []) s += k.getName(); return s; }
/** @param {import("./types.ts").Reg} r @param {string} t @returns {string} */
const xml = (r, t) => r.getObject(t)?.getXMLFile?.()?.getRaw?.() ?? "";
export class Probe {
  /** @returns {string} */ static run() {
    const a = node("a", []), b = node("b", [a]);
    const r = new Reg();
    return count({body: b, text: ""}) + ":" + count({body: null, text: ""}) + ":" + names(b) + names(null) + ":" + xml(r, "x") + xml(r, "y") + xml(r, "z");
  }
}
`}

func TestStep4NullableReceiverCallNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, step4NullableReceiver[0], step4NullableReceiver[1], false, true)
}
