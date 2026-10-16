package tsfront

import (
	"strings"
	"testing"
)

func TestGenericContainersMapNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @template T @param {WeakMap<object, Map<string, T>>} map @param {object} owner @returns {Map<string, T>} */
const memo = (map, owner) => map.get(owner) ?? map.set(owner, new Map()).get(owner);
export class Probe {
/** @returns {string} */ static run() {
 const owner = {};
 /** @type {WeakMap<object, Map<string, number>>} */ const ns = new WeakMap();
 /** @type {WeakMap<object, Map<string, {n: number}>>} */ const os = new WeakMap();
 const a = memo(ns, owner), b = memo(ns, owner), c = memo(os, owner), obj = {n: 7};
 a.set("b", 2); b.set("a", 1); a.set("b", 3); c.set("x", obj); obj.n = 9;
 return [String(a === b), [...b.keys()].join("/"), String(b.get("b")), String(c.get("x") === obj), String(c.get("x").n)].join(",");
}}
`, false)
}

func TestGenericContainersRecordNodeOracle(t *testing.T) {
	runMjsOracle(t, `/** @template T @param {() => T} run @returns {T} */
function wrapped(run) { return run(); }
/** @template T @param {() => {names: string[], value: T}} run @returns {T} */
export function discover(run) { return wrapped(() => { const value = run(); value.names.push("seen"); return value.value; }); }
export class Probe {
/** @returns {string} */ static run() {
 const names = ["a"], obj = {n: 3};
 const n = discover(() => ({names, value: 7}));
 const s = discover(() => ({names, value: "x"}));
 const o = discover(() => ({names, value: obj})); o.n = 8;
 const xs = [1], ys = discover(() => ({names, value: xs})); ys.push(2);
 return [String(n), s, String(obj.n), String(o === obj), String(xs.length), names.join("/")].join(",");
}}
`, true)
}

func TestGenericContainersWeakMapABAPRefusalNodeOracle(t *testing.T) {
	gogenTSOracle(t, `function memo<T>(map: WeakMap<object, Map<string,T>>, owner: object): Map<string,T> | undefined { return map.get(owner) ?? map.set(owner,new Map()).get(owner); }
 class Probe { static run(): string { const owner={}; const m=new WeakMap<object,Map<string,number>>(); memo(m,owner)!.set("a",1); return String(memo(m,owner)!.get("a")); } }`, "weak.")
}

func TestGenericContainersNativeMapNodeOracle(t *testing.T) {
	gogenOracle(t, `function put<K,V>(map:Map<K,V>, key:K, value:V):Map<K,V> {map.set(key,value);return map;}
 function forward<T>(map:Map<string,T>, value:T):Map<string,T> {return put(map,"b",value);}
 class Probe {static run():string {const ns=new Map<string,number>();const os=new Map<string,{n:number}>();const obj={n:3};
 const a=forward(ns,1); put(ns,"a",2); put(ns,"b",4);const b=put(os,"x",obj);obj.n=9;
 return String(a===ns)+":"+[...ns.keys()].join("/")+":"+ns.get("b")+":"+String(b.get("x")===obj)+":"+b.get("x")!.n;}}`, "true:b/a:4:true:9")
}

func TestGenericContainersImportedReuseNodeOracle(t *testing.T) {
	helper := `function put<T>(map:Map<string,T>,value:T):Map<string,T>{map.set("x",value);return map;}`
	probe := `class Probe {static run():string {const ns=new Map<string,number>();const ss=new Map<string,string>();put(ns,1);put(ss,"a");put(ns,2);return ns.get("x")+":"+ss.get("x");}}`
	p := lowerStatementsProbe(t, map[string]string{"helper.ts": "export " + helper, "probe.ts": `import {put} from "./helper";` + probe}, []string{"probe.ts", "helper.ts"})
	names := map[string]bool{}
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			if strings.HasPrefix(m.Name, "put_specialized_") {
				names[m.Name] = true
			}
		}
	}
	if len(names) != 2 {
		t.Fatalf("want two concrete methods, got %v", names)
	}
	gogenCheck(t, p, "probe.ts", helper+probe, "2:a")
}
