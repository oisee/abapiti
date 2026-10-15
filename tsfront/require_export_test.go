package tsfront

import "testing"

// requireExportModules: base.mjs holds the require bindings and exports
// them (export list and export const destructuring); mid.mjs re-exports
// some, renamed.
var requireExportModules = map[string]string{
	"base.mjs": `import {createRequire} from "node:module";
const require = createRequire(import.meta.url);
const transpilerRequire = createRequire(require.resolve("@fake/transpiler/package.json"));
/** @type {typeof import("@fake/core")} */
const core = transpilerRequire("@fake/core");
/** @type {typeof import("@fake/transpiler/build/src/rearranger")} */
const {Rearranger} = require("@fake/transpiler/build/src/rearranger");
const {Kinds} = core;
export const {Token} = core;
export {core, Kinds, Rearranger};
`,
	"mid.mjs": `export {Kinds, Rearranger as Arranger, core} from "./base.mjs";
`,
}

// An exported require binding, and names destructured from it, resolve by
// import (named, renamed, namespace, re-export chains) to the same static
// module or export; a class binding passes as a class value.
func TestRequireBindingExportNodeOracle(t *testing.T) {
	runRequireOracle(t, `import {Kinds, Arranger, core} from "./mid.mjs";
import * as base from "./base.mjs";
import {Token as Tok} from "./base.mjs";
export class Probe {
  /** @param {import("@fake/core").Kinds.Kind} k @returns {string} */
  static describe(k) {
    if (k instanceof core.Kinds.CharKind) {
      return "char:" + k.length;
    }
    if (k instanceof base.Kinds.IntKind) {
      return "int";
    }
    return "other";
  }
  /** @param {typeof import("@fake/transpiler/build/src/rearranger").Rearranger} cls @param {import("@fake/core").Kinds.Kind[]} ks @returns {string} */
  static via(cls, ks) {
    return new cls().run(ks);
  }
  /** @returns {string} */
  static run() {
    const a = Kinds.CharKind.of(3);
    const b = new base.Kinds.IntKind();
    const t = new Tok("x");
    const r = new base.Rearranger();
    return Probe.describe(a) + "|" + Probe.describe(b) + "|" + a.name() + "|" + t.text() + "|" + r.run([a, b]) + "|" + Probe.via(Arranger, [b, a]);
  }
}
`, "char:3|int|C3|'x'|I>C3|C3>I\n", requireExportModules)
}

// The namespace object itself is not a value: passing it stays refused.
func TestRequireBindingExportNamespaceValueRefused(t *testing.T) {
	dir := copyRequireFixture(t, `import {Kinds} from "./mid.mjs";
export class Probe {
  /** @param {unknown} ns @returns {string} */
  static take(ns) {
    return typeof ns;
  }
  /** @returns {string} */
  static run() {
    return Probe.take(Kinds);
  }
}
`, requireExportModules)
	_, diags := lowerRequireFixture(t, dir)
	for _, d := range diags {
		if d.Category == "unsupported-require" && d.Loc == "probe.mjs:9:23" {
			return
		}
	}
	t.Fatalf("namespace value not refused at probe.mjs:9:23: %v", diags)
}
