package tsfront

import "testing"

// o.f?.(sideEffect()) evaluates no argument when the optional member is
// absent: void and value results, and the ?. result used in an expression.
func TestOptionalMemberCallArgumentsNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Hooks { name: string; log?(line: string): void; size?(n: number): number }
`, `/** @typedef {import("./types.ts").Hooks} Hooks */
let count = 0;
/** @type {string[]} */ const trail = [];
/** @param {string} s @returns {string} */ const mark = (s) => { count++; trail.push(s); return s; };
/** @param {Hooks} h @returns {string} */
const use = (h) => {
  h.log?.(mark("log:" + h.name));
  h.log?.([mark("arr:" + h.name), "!"].join(""));
  const sized = h.size?.([mark("len:" + h.name)].length) ?? 0;
  const n = h.size?.(mark("size:" + h.name).length);
  const total = (h.size?.(count++) ?? -1) + 1;
  return h.name + "=" + String(n) + "/" + total + "/" + sized;
};
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {string[]} */ const logged = [];
    /** @type {Hooks} */ const full = {name: "full", log: (l) => { logged.push(l); }, size: (n) => n * 2};
    /** @type {Hooks} */ const bare = {name: "bare"};
    return [use(bare), use(full), String(count), trail.join(","), logged.join(",")].join(" ");
  }
}
`, false, true)
}
