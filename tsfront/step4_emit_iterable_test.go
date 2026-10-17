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

func TestStep4EmitNumberSetIterableNodeOracle(t *testing.T) {
	gogenTSOracle(t, `class Probe {static consume(xs:Iterable<number>,s:Set<number>):string {let out="",first=true;for(const n of xs){out+=n+",";if(first){first=false;s.delete(2);s.add(3);s.delete(1);s.add(1);}else if(n===3){s.clear();s.add(4);}}return out;}
 static run():string{const s=new Set<number>([1,2]);return Probe.consume(s,s);}}`, "set.iterationValues")
}
