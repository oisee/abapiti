package tsfront

import "testing"

// gogen's `withProvenance(function compileProgram({folders, objects,
// tolerant = false, skip = () => false, registry}) {...})`: an object
// binding parameter of a function value binds like a declared function's,
// each default evaluated left to right only when its property is undefined.
func TestFunctionValueObjectParameterNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Options {
  folders: string[];
  objects?: string[];
  tolerant?: boolean;
  skip?: (path: string) => boolean;
  registry?: {name: string};
}
`, `/** @type {string[]} */ const log = [];
/** @param {(o: import("./types.ts").Options) => string} f @returns {(o: import("./types.ts").Options) => string} */ function wrap(f) {
  return (o) => "[" + f(o) + "]";
}
export const compile = wrap(function compile({folders, objects, tolerant = (log.push("t"), false), skip = () => false, registry}) {
  return folders.filter((f) => !skip(f)).join("+") + ":" + (objects ?? ["-"]).join("") + ":" + tolerant + ":" + (registry ? registry.name : "none");
});
export class Probe {
  /** @returns {string} */ static run() {
    const a = compile({folders: ["a", "b"], skip: (p) => p === "b", tolerant: true, registry: {name: "r"}});
    const b = compile({folders: ["c"], objects: ["x", "y"]});
    return a + " " + b + " " + log.join("");
  }
}
`, false)
}
