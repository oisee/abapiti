package tsfront

import "testing"

func TestStep4OwnershipReadonlySetNodeOracle(t *testing.T) {
	a20Oracle(t, `interface Field{readonly name:string}
 interface Local extends Field{readonly local?:boolean}
 interface Attribute extends Field{readonly private?:boolean}
 interface Ownership{readonly declarations:ReadonlySet<Field>}
 function ownership(declarations:Set<Local|Attribute>):Ownership{return {declarations};}
 class Probe{static run():string{const a:Local={name:"a",local:true},b:Attribute={name:"b",private:true};const declarations=new Set<Local|Attribute>([a,b]);const own=ownership(declarations);const first=[...own.declarations][0];declarations.delete(a);declarations.add({name:"c",local:true});return [...own.declarations].map(x=>x.name).join(",")+":"+own.declarations.has(a)+":"+own.declarations.has(b)+":"+(first===a)+":"+(own.declarations===declarations);}}`)
}
