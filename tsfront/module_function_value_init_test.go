package tsfront

import "testing"

// gogen's `export const compileProgram = withProvenance(function
// compileProgram(...) {... abaplint.Config.getDefault() ...})`: class reads
// inside a nested function happen when it is called, not when the module
// initializer creates the function value.
func TestModuleWrappedFunctionValueNodeOracle(t *testing.T) {
	runMjsOracle(t, `class Config {
  static base = 40;
  /** @returns {number} */ static getDefault() { return Config.base + 2; }
}
/** @param {(n: number) => number} f @returns {(n: number) => number} */ function wrap(f) {
  return (n) => f(n) * 10;
}
export const compile = wrap(function compile(n) { return Config.getDefault() + n; });
const twice = wrap((n) => Config.base + n);
export class Probe {
  /** @returns {string} */ static run() {
    Config.base = 1;
    return compile(1) + ":" + twice(2);
  }
}
`, true)
}
