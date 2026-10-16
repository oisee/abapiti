package tsfront

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestGenericPropertyTraversalNodeOracle(t *testing.T) {
	source := `type PropertyValue<T>=T extends object?T[keyof T]:never;
function children<T extends object>(node:T,excluded:readonly string[]=['type','pos']):readonly PropertyValue<T>[] {const keys=Object.keys(node) as (keyof T&string)[];return keys.filter((key)=>!excluded.includes(key)).map((key)=>node[key] as PropertyValue<T>);}
function entries<T extends object>(node:T):readonly (readonly [string,PropertyValue<T>])[] {const keys=Object.keys(node) as (keyof T&string)[];return keys.map((key)=>[key,node[key] as PropertyValue<T>] as const);}
class Probe {static run():string {
 const obj={b:'bee',type:'skip',a:7,pos:9,empty:undefined};
 const first=entries(obj)[0];const erased:unknown=first;
 return String(Array.isArray(first))+':'+String(Array.isArray(erased))+'|'+JSON.stringify(children(obj))+'|'+JSON.stringify(children(obj,['b']))+'|'+JSON.stringify(entries(obj));
}}`
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != string(want) {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}

func TestGenericPropertyConditionalSubsetRefused(t *testing.T) {
	source := `type Value<T>=T extends object?string:never;function wrong<T extends object>(node:T):Value<T> {return 'x' as Value<T>;}class Probe {static run():string {return wrong({a:1});}}`
	ds := lowerFamilyDiagnostics(t, source)
	for _, d := range ds {
		if d.Category == "unsupported-type" && d.Loc != "" {
			return
		}
	}
	t.Fatalf("unrelated conditional was accepted: %v", ds)
}
