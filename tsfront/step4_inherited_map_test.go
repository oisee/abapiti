package tsfront

import "testing"

func TestStep4InheritedMapOverloadNodeOracle(t *testing.T) {
	a20Oracle(t, `interface Snapshot extends Map<string,unknown>{get(key:"sigs"):Map<string,string>;get(key:string):unknown}
 function snapshot():Snapshot{return new Map<string,unknown>([["sigs",new Map<string,string>([["a","A"]])],["n",3]]) as Snapshot;}
 class Probe{static run():string{const before=snapshot();return before.get("sigs").get("a")+":"+String(before.get("n"))+":"+before.size;}}`)
}
