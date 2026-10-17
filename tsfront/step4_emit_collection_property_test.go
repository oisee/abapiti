package tsfront

import "testing"

// An interface inheriting Map's readonly size registers the library property
// symbol. A later Map read must still use the collection's size operation.
func TestStep4EmitCollectionPropertyNodeOracle(t *testing.T) {
	a20Oracle(t, `interface Entry{readonly name:string;readonly text:string}
 interface ProgramSnapshot extends Map<string,unknown>{get(key:string):unknown}
 interface QueryTrie extends Map<string,QueryTrie|true>{}
 const NAMED=new Map<string,Entry>();
 function named(key:string):string{if(!NAMED.has(key))NAMED.set(key,{name:"tn_"+NAMED.size,text:key});return NAMED.get(key)!.name;}
 class Probe{static run():string{return named("a")+":"+named("b")+":"+named("a");}}`)
}
