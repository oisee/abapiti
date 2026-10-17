package tsfront

import "testing"

// frontend-local-constants.mjs hoistLocalConstants (OSG 573ad621): a local
// arrow add(name, type, init) with a JSDoc object-shape init, spread into a
// FrontendConstant ({go, type, ...init}), called from the nested arrow walk
// whose own local is also named type: the shorthand reads the parameter.
func TestStep4LocalAddCallableNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export type Type = {readonly k: "i"} | {readonly k: "c"; readonly len: number} | {readonly k: "struct"; readonly go: string; readonly of?: Type};
export type ConstantInitializer = {readonly e: "int"; readonly value: number; readonly type: Type} | {readonly e: "str"; readonly value: string; readonly type: Type};
export interface FrontendConstant { go: string; type: Type; value?: string | Record<string, string | undefined>; init?: ConstantInitializer; fieldInits?: Record<string, ConstantInitializer | undefined> }
export interface Ctx { consts: Map<string, FrontendConstant>; locals: Map<string, Type>; names: Set<string> }
export class Node { readonly name: string; readonly kids: Node[]; readonly value?: number; constructor(name: string, kids: Node[], value?: number) { this.name = name; this.kids = kids; this.value = value; } }
`, `import {Node} from "./types.ts";
/** @param {Node | undefined} body @param {import("./types.ts").Ctx} ctx @returns {void} */
function hoist(body, ctx) {
  /** @param {string} name @param {import("./types.ts").Type} type @param {{value?: string | Record<string, string | undefined>, init?: import("./types.ts").ConstantInitializer, fieldInits?: Record<string, import("./types.ts").ConstantInitializer | undefined>}} init */
  const add = (name, type, init) => {
    const go = "L_" + name;
    if (ctx.consts.has(go)) throw new Error("collision " + go);
    ctx.consts.set(go, {go, type, ...init});
    ctx.locals.delete(name);
  };
  /** @param {Node | undefined} node */ const walk = (node) => {
    if (!node) return;
    if (node.name.startsWith("S")) {
      const type = ctx.locals.get(node.name);
      if (type?.k !== "struct") throw new Error("not a struct " + node.name);
      /** @type {Record<string, import("./types.ts").ConstantInitializer | undefined>} */ const folded = {};
      for (const k of node.kids) folded[k.name] = {e: "int", value: k.value ?? 0, type: {k: "i"}};
      add(node.name, type, {fieldInits: folded});
      return;
    }
    if (node.value !== undefined) {
      ctx.names.add(node.name);
      add(node.name, {k: "i"}, {init: {e: "int", value: node.value, type: {k: "i"}}});
      return;
    }
    for (const child of node.kids) walk(child);
  };
  walk(body);
}
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {import("./types.ts").Ctx} */ const ctx = {consts: new Map(), locals: new Map([["S1", {k: "struct", go: "S1"}]]), names: new Set()};
    hoist(new Node("root", [new Node("A", [], 3), new Node("S1", [new Node("X", [], 1), new Node("Y", [], 2)]), new Node("b", [new Node("C", [], 7)])]), ctx);
    return [...ctx.consts.values()].map((c) => c.go + ":" + c.type.k + ":" + JSON.stringify(c.init ?? c.fieldInits)).join(",") + "|" + ctx.locals.size;
  }
}
`, false, true)
}
