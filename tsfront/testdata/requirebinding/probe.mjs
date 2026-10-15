import {createRequire} from "node:module";
const require = createRequire(import.meta.url);
const transpilerRequire = createRequire(require.resolve("@fake/transpiler/package.json"));
/** @type {typeof import("@fake/core")} */
const core = transpilerRequire("@fake/core");
/** @type {typeof import("@fake/transpiler/build/src/rearranger")} */
const {Rearranger} = require("@fake/transpiler/build/src/rearranger");
const {Kinds, Token} = core;

export class Probe {
  /** @param {import("@fake/core").Kinds.Kind} k @returns {string} */
  static describe(k) {
    if (k instanceof core.Kinds.CharKind) {
      return "char:" + k.length;
    }
    if (k instanceof Kinds.IntKind) {
      return "int";
    }
    return "other";
  }
  /** @returns {string} */
  static run() {
    const a = Kinds.CharKind.of(3);
    const b = new core.Kinds.IntKind();
    const t = new Token("x");
    const r = new Rearranger();
    return Probe.describe(a) + "|" + Probe.describe(b) + "|" + a.name() + "|" + t.text() + "|" + r.run([a, b]);
  }
}
