package tsfront

import "testing"

// Object.entries of a string record yields [string, string] rows: a homogeneous
// tuple the checker maps to string[][] (emit-go.mjs exceptionSupers).
func TestStep4EmitStringRecordEntriesNodeOracle(t *testing.T) {
	a20Oracle(t, `interface P{readonly ex?:Readonly<Record<string,string>>}
 function sup(p:P):string[]{const m=Object.entries(p.ex??{}).sort(([a],[b])=>a.localeCompare(b));if(m.length===0)return [];const first=m[0];first[1]="Z";return m.map(([k,v])=>k+"="+v+":"+m.length);}
 class Probe {static run():string{const ex:Record<string,string>={ZB:"1",ZA:"2","3":"X"};const r=sup({ex}).join(",")+":"+sup({}).length;return r+":"+ex["ZA"]+":"+Object.entries(ex).length;}}`)
}
