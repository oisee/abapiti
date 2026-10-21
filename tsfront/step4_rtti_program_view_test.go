package tsfront

import "testing"

func TestStep4RTTIProgramReadonlyClassesNodeOracle(t *testing.T) {
	a20Oracle(t, `interface FullClass{name:string;super:string|null;methods:string[]}
 interface Program{classes:FullClass[]}
 function rtti(program:{classes:readonly {name:string;super:string|null}[]}):string{return program.classes.map(c=>c.name+":"+(c.super??"-")).join(",");}
 class Probe{static run():string{const program:Program={classes:[{name:"A",super:null,methods:[]}]};return rtti(program);}}`)
}
