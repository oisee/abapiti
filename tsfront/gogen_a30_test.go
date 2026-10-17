package tsfront

import "testing"

// frontend.mjs addSignatures: a for-of over a generator whose body holds a
// try/catch that continues, breaks or returns out of the catch. continue keeps
// the iterator alive; break/return close it once (the generator's finally
// runs); a throw escaping the catch closes it as well.
func TestGogenA30GeneratorRegionExitsNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export {};
`, `/** @type {string[]} */ const log = [];
/** @param {number} n @returns {Generator<number, void, undefined>} */ function* gen(n) {
  for (let i = 0; i < n; i++) yield i;
}
/** @param {number} i @returns {number} */ function check(i) { if (i % 2 === 1) throw new Error("odd" + i); return i; }
/** @param {number} stop @returns {string} */ function run(stop) {
  let out = "";
  const g = gen(6);
  try { return loop(g, stop, out); } finally { log.push(String(g.next().done)); }
}
/** @param {Generator<number, void, undefined>} g @param {number} stop @param {string} out @returns {string} */ function loop(g, stop, out) {
  for (const i of g) {
    let v;
    try { v = check(i); } catch (e) {
      if (!(e instanceof Error)) throw e;
      out += "c" + i;
      if (i === stop) break;
      if (i === stop + 2) return out + "R";
      if (i === 99) throw e;
      continue;
    }
    out += "v" + v;
    for (let j = 0; j < 2; j++) { try { if (j === 0) continue; out += "j"; } catch (e) { break; } }
  }
  return out;
}
/** @returns {string} */ function thrower() {
  let out = "";
  const g = gen(6);
  try {
    for (const i of g) {
      try { check(i); out += i; } catch (e) { if (i === 3) throw e; continue; }
    }
  } catch (e) { out += "T"; }
  return out + g.next().done;
}
export class Probe {
  /** @returns {string} */ static run() {
    const a = run(3), b = run(1), c = run(-1), e = run(100), d = thrower();
    return [a, b, c, e, d, log.join(",")].join("|");
  }
}
`, true, true)
}

// String.prototype.replace with a callback (frontend.mjs templateText,
// sink-locals.ts generatedBindings): literal RegExp, global or not, empty
// matches, captures that always participate, side effects in match order,
// and a callback result taken literally ("$&" is not a substitution).
func TestGogenA30CallableReplaceNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export {};
`, `/** @param {string} raw @returns {string} */ function templateText(raw) {
  const t = raw.replace(/^[|}]/, "").replace(/[{|]$/, "");
  /** @type {Readonly<Record<string, string | undefined>>} */ const escapes = {n: "\n", r: "\r", t: "\t"}; return t.replace(/\\([{}|\\nrt])/g, (m, c) => escapes[c] ?? c);
}
/** @param {string} source @returns {string} */ function blank(source) {
  return source.replace(/"[^"]*"/g, (text) => text.replace(/[a-z ]/g, "_"));
}
export class Probe {
  /** @returns {string} */ static run() {
    let n = 0;
    const order = "a1b22c333".replace(/[0-9]+/g, (m) => { n++; return "<" + m.length + ":" + n + ">"; });
    const empty = "abc".replace(/x*/g, (m) => "[" + m + "]");
    const first = "aXbXc".replace(/X/, (m) => m.toLowerCase());
    const literal = "ab".replace(/b/g, () => "$&$1");
    const pairs = "k1=v1;k2=v2".replace(/(\w+)=(\w+)/g, (all, k, v) => v + "=" + k);
    const parts = [templateText("|a\\nb\\{c\\|d\\\\e\\tf{"), blank('x = "q s" // c\ny'), order, empty, first, literal, pairs, "none".replace(/z/g, (m) => "!" + m)];
    return JSON.stringify(parts);
  }
}
`, true, true)
}
