package tsfront

import "testing"

func TestStep4EmptySetFallbackNodeOracle(t *testing.T) {
	a20Oracle(t, `interface RTTI{readonly known:ReadonlySet<string>}
 interface Class{readonly name:string}
 interface Program{readonly classes:readonly Class[];readonly rtti?:RTTI}
 type ProgramInput=Program|(Class[]&Partial<Program>);
 function known(program:ProgramInput):string{return [...(program.rtti?.known??[])].filter(n=>!n.includes("=>")).join(",");}
 class Probe{static run():string{return known({classes:[]})+":"+known({classes:[],rtti:{known:new Set<string>(["a","b=>c","d"])}});}}`)
}
