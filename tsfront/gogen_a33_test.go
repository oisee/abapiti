package tsfront

import "testing"

// Map.prototype.clear (frontend.mjs session.classes.clear(),
// LOCAL_DEFS.clear()) empties the map in place: every alias observes it,
// size is 0, lookups miss, and later sets restart insertion order.
func TestGogenA33MapClearNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @type {Map<string, number>} */ const SHARED = new Map();
export class Probe {
  /** @returns {string} */ static run() {
    /** @type {Map<string, number>} */ const m = new Map([["b", 1], ["a", 2], ["c", 3]]);
    const alias = m;
    const before = m.size;
    m.clear();
    m.set("z", 9);
    m.set("b", 4);
    SHARED.set("x", 1);
    SHARED.set("y", 2);
    SHARED.clear();
    SHARED.clear();
    SHARED.set("w", 5);
    return before + ":" + alias.size + ":" + alias.has("a") + ":" + String(alias.get("c")) + ":" + [...alias.keys()].join(",") + ":" + [...alias.values()].join(",") + ":" + [...SHARED.keys()].join(",") + ":" + SHARED.size;
  }
}
`, true)
}

// frontend.mjs (OSG dd94a6a9) destructures a typed inline lazy require in a
// block and holds the functions in a Pick-typed optional record:
// `const {f, g} = /** @type {typeof import("./m.mjs")} */ (require("./m.mjs"));`
func TestGogenA33DestructuredInlineRequireNodeOracle(t *testing.T) {
	runRequireOracle(t, `import {require} from "./base.mjs";
/** @param {number} n @returns {number} */ function plain(n) { return n + 100; }
export class Probe {
  /** @param {boolean} on @returns {string} */
  static pick(on) {
    /** @type {Pick<typeof import("./lazy.mjs"), "twice" | "thrice"> | undefined} */ let lazy;
    if (on) {
      const {twice, thrice} = /** @type {typeof import("./lazy.mjs")} */ (require("./lazy.mjs"));
      lazy = {twice, thrice};
    }
    const a = lazy ? lazy.twice(3) : -1;
    const f = lazy ? lazy.thrice : plain;
    return a + "," + f(4);
  }
  /** @returns {string} */
  static run() {
    return Probe.pick(true) + "|" + Probe.pick(false);
  }
}
`, "6,12|-1,104\n", map[string]string{
		"base.mjs": `import {createRequire} from "node:module";
const require = createRequire(import.meta.url);
export {require};
`,
		"lazy.mjs": `/** @param {number} n @returns {number} */ export function twice(n) { return n * 2; }
/** @param {number} n @returns {number} */ export function thrice(n) { return n * 3; }
`,
	})
}
