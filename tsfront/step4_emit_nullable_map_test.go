package tsfront

import (
	"strings"
	"testing"
)

func TestStep4EmitNullableStringMapViewNodeOracle(t *testing.T) {
	gogenTSImplementedABAPOracle(t, `interface Item {readonly name:string;readonly parent?:string}
 class Probe {
 static view(m:ReadonlyMap<string,Item>):ReadonlyMap<string|null|undefined,Item>{return m;}
 static chain(n:Item|undefined,m:ReadonlyMap<string|null|undefined,Item>):string {let out="";for(let c=n;c;c=m.get(c.parent))out+=c.name;return out;}
 static run():string{const root:Item={name:"r"},a:Item={name:"a",parent:"r"};const m=new Map<string,Item>([["r",root],["a",a]]);const view=Probe.view(m);const before=Probe.chain(a,view);const replacement:Item={name:"new"};m.set("a",replacement);return before+":"+view.get("a")!.name+":"+(view.get(null)===undefined)+":"+(view.get(undefined)===undefined)+":"+view.has(null)+":"+(view===m)+":"+(view.get("a")===replacement);}}
`)
}

func TestStep4EmitBoxedMapABAPRefusal(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": `class Probe {static run():number{const m=new Map<string|null|undefined,number>();m.set(null,7);return m.get(null)??0;}}`}, []string{"probe.ts"})
	_, err := emitABAP(t, p)
	if err == nil || !strings.Contains(err.Error(), "probe.ts:") || !strings.Contains(err.Error(), "value-based SameValueZero") {
		t.Fatalf("wanted located ABAP boxed Map refusal, got %v", err)
	}
}
