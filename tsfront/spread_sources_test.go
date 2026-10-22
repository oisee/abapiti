package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// Spread sources consumed at once, exactly as JavaScript iterates them: Map
// entries, Map keys/values through an alias or union receiver, strings by
// code point (a surrogate pair stays one element), matchAll (index, empty
// matches advancing lastIndex), optional and Set | array operands.
func TestSpreadSourcesNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @type {Map<string, Item>} */ const DESCS = new Map([["a", {name: "A", size: 1}], ["b", {name: "B", size: 2}]]);
/** @typedef {import("./types.ts").Item} Item */
class Holder { constructor() { /** @type {string[]} */ this.items = ["x", "y"]; } /** @returns {string[]} */ getAll() { return this.items; } }
/** @typedef {Map<string, number>} NumMap */
export class Probe {
  /** @returns {string} */ static entries() { const out = []; for (const [key, d] of [...DESCS]) out.push(key + "=" + d.name); return out.join(","); }
  /** @param {NumMap | undefined} m @returns {string} */ static aliasValues(m) { if (m === undefined) return "/"; return [...m.values()].join("+") + "/" + [...m.keys()].join("+"); }
  /** @param {{objects: NumMap | Map<string, number>}} inputs @returns {string} */ static unionValues(inputs) { return [...inputs.objects.values()].join("+"); }
  /** @param {string} v @returns {string} */ static chars(v) { return [...v].slice(0, 6).join("|") + ":" + [...v].length; }
  /** @param {string} s @returns {string} */ static all(s) { return [...s.matchAll(/(\w)(\d)?/g)].map((m) => m[1] + (m[2] ?? "-") + "@" + m.index).join(",") + ";" + [...s.matchAll(/x*/g)].length; }
  /** @param {Holder | undefined} c @returns {string} */ static opt(c) { return [...(c?.getAll() ?? [])].join(","); }
  /** @param {Map<string, Set<string> | string[]>} index @param {string} key @returns {string} */ static providers(index, key) { return [...index.get(key) ?? []].join(","); }
  /** @param {readonly string[]} path @returns {string} */ static ro(path) { return [...path].join("/"); }
  /** @param {{methods: string[], stubs?: string[]}} cls @returns {string} */ static cat(cls) { return [...cls.methods, ...(cls.stubs ?? [])].join(","); }
  /** @param {Map<string, Set<string> | string[]>} index @param {string} k @param {Set<string> | string[]} v */ static put(index, k, v) { index.set(k, v); }
  /** @param {{builders?: Map<string, string>}} ctx @returns {string[]} */ static builders(ctx) { return [...[...(ctx.builders ?? new Map())].map(([n, sb]) => n + "=" + sb), "end"]; }
  /** @returns {string} */ static run() {
    /** @type {Map<string, Set<string> | string[]>} */ const index = new Map(); Probe.put(index, "s", new Set(["p", "q"])); Probe.put(index, "a", ["r"]);
    return [Probe.entries(), Probe.aliasValues(new Map([["k", 1], ["j", 2]])), Probe.aliasValues(undefined), Probe.unionValues({objects: new Map([["q", 5]])}), Probe.chars("a\u{1F600}bc"), Probe.all("a1b c2"), Probe.opt(new Holder()), Probe.opt(undefined), Probe.providers(index, "s"), Probe.providers(index, "a"), Probe.providers(index, "z"), Probe.ro(["u", "v"]), Probe.cat({methods: ["m"]}), Probe.cat({methods: ["m"], stubs: ["s"]}), Probe.builders({}).join(","), Probe.builders({builders: new Map([["b", "c"]])}).join(",")].join(" | ");
  }
}
`, false)
}

// The statically typed sources lower to existing operations; ABAP emits them.
const spreadSourcesTS = `class Probe {
 static entries(m:Map<string,number>):string {const out:string[]=[];for (const [k,v] of [...m]) out.push(k+'='+v);return out.join(',');}
 static same(m:Map<string,string>):string {return [...m].map((e) => e[0]+e[1]).join(',');}
 static chars(v:string):string {return [...v].join('|')+':'+[...v].length;}
 static all(s:string):string {return [...s.matchAll(/(a)(b)?/g)].map((m) => m[1]+(m[2] ?? '-')+m.index).join(',')+';'+[...s.matchAll(/q*/g)].length;}
 static run():string {const m=new Map<string,number>();m.set('x',1);m.set('y',2);const n=new Map<string,string>();n.set('k','v');
  return [Probe.entries(m),Probe.same(n),Probe.chars('a\u{1F600}z'),Probe.all('ab a xab')].join(' ');}
}`

func TestSpreadSourcesTypedNodeOracle(t *testing.T) {
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, spreadSourcesTS).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": spreadSourcesTS}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q\nNode %q", got, want)
	}
	_, abapErr := emitABAP(t, p)
	t.Logf("ABAP: %v", abapErr)
	if err := abapErr; err != nil && !strings.Contains(err.Error(), "probe.ts:") {
		t.Fatalf("ABAP refusal without a location: %v", err)
	}
}

// Non-global regexes now throw the same catchable TypeError as JavaScript.
func TestSpreadMatchAllNonGlobalTypeError(t *testing.T) {
	source := `class Probe {static run():string {try {const a=[..."a".matchAll(/a/)];return String(a.length);}catch(e){return e instanceof TypeError?"TypeError":"other";}}}`
	gogenOracle(t, source, "TypeError", "regexp.matchAllNext")
}
