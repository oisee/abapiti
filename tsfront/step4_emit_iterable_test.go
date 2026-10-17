package tsfront

import "testing"

func TestStep4EmitSetIterableNodeOracle(t *testing.T) {
	gogenTSOracle(t, `class Probe {
 static record(xs:Iterable<string>):string {let out="";for(const x of xs)out+=x;return out;}
 static mutate(xs:Iterable<string>,s:Set<string>):string {let out="",first=true;for(const x of xs){out+=x;if(first){first=false;s.delete("b");s.add("c");s.delete("a");s.add("a");}else if(x==="c"){s.clear();s.add("z");}}return out;}
 static run():string{const xs=new Set<string>(["a","b"]);return Probe.record(xs)+":"+Probe.record(["c","d"])+":"+Probe.mutate(xs,xs)+":"+Probe.record(xs);}}`, "set.iterationValues")
}

func TestStep4EmitReferenceSetIterableNodeOracle(t *testing.T) {
	gogenTSOracle(t, `class Key {name:string;constructor(name:string){this.name=name;}}
 class Probe {static read(xs:Iterable<Key>,s:Set<Key>,a:Key,b:Key):string{let out="";for(const x of xs){out+=x.name;if(x===a){s.delete(b);s.add(new Key("c"));}}return out;}
 static run():string{const a=new Key("a"),b=new Key("b");const s=new Set<Key>([a,b]);return Probe.read(s,s,a,b);}}`, "set.iterationValues")
}
