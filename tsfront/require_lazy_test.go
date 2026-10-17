package tsfront

import "testing"

// gogen's frontend.mjs requires lazily through the requirer exported by
// frontend-base.mjs (`import {require} from "./frontend-base.mjs"`):
// `/** @type {typeof import("./m.mjs")} */ (require("./m.mjs")).f(...)` and
// a typed local binding resolve statically, like the same-file forms.
func TestRequireLazyImportedRequirerNodeOracle(t *testing.T) {
	runRequireOracle(t, `import {require} from "./base.mjs";
export class Probe {
  /** @returns {string} */
  static run() {
    const a = /** @type {typeof import("./lazy.mjs")} */ (require("./lazy.mjs")).twice(3);
    /** @type {typeof import("./lazy.mjs")} */ const lazy = require("./lazy.mjs");
    return a + "|" + lazy.twice(4) + "|" + lazy.label;
  }
}
`, "6|8|L\n", map[string]string{
		"base.mjs": `import {createRequire} from "node:module";
const require = createRequire(import.meta.url);
export {require};
`,
		"lazy.mjs": `/** @param {number} n @returns {number} */ export function twice(n) { return n * 2; }
export const label = "L";
`,
	})
}
