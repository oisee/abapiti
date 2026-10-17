package tsfront

import "testing"

// constructorFactories joins arguments from an asserted callback and [].
func TestStep4EmitConditionalArrayViewNodeOracle(t *testing.T) {
	a20Oracle(t, `interface Arg{readonly name?:string;readonly value?:string}
 interface RuntimeArg{readonly name?:string;readonly value?:string;readonly extra?:string}
 type Factory=(name:string)=>{readonly args:readonly RuntimeArg[]};
 const raw:Arg[]=[{name:"x",value:"v"}];
 function omitted(name:string):{readonly args:readonly Arg[]}{return {args:raw};}
 function factory(yes:boolean):readonly RuntimeArg[]{const args=yes?(omitted as Factory)("x").args:[];return args;}
 class Probe{static run():string{const a=factory(true);raw.push({name:"y"});return a.map(x=>x.name+":"+(x.extra??"none")).join(",")+":"+factory(false).length+":"+(a===raw)+":"+(a[0]===raw[0]);}}`)
}
