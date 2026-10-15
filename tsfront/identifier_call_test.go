package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

// Keep these as Node differentials: the callee binding and every argument
// must be read in source order, including discarded arguments and rest packs.
func TestIdentifierCallNodeOracle(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"optional rest closure", `class Probe {static run():string {let trace="";const f=(x:number=4,...rest:number[]):number=>{trace+="c";return x+rest.length;};function arg(x:number):number{trace+=""+x;return x;} const a=f();const b=f(undefined,arg(2),arg(3));return a+":"+b+":"+trace;}}`, "4:6:c23c"},
		{"parameter and discarded argument", `function invoke(f:(x:number)=>number):number{return f(2);} class Probe {static run():string{let value=1;let f=(x:number):number=>value+x;function replace():number{value=10;f=(x:number):number=>100+x;return 0;}const result=f(value,replace());return result+":"+invoke(f);}}`, "11:102"},
		{"guarded optional parameter", `function invoke(f:((x:number)=>number)|undefined):number {if(f){return f(3);}return 0;} class Probe {static run():number{return invoke((x)=>x+4);}}`, "7"},
		{"optional parameter invocation", `function invoke(f:((x:number)=>number)|undefined):number {let n=0;function arg():number{n++;return 2;}const value=f?.(arg());return n;} class Probe {static run():string{return invoke(undefined)+":"+invoke((x)=>x+1);}}`, "0:1"},
		{"optional module invocation", `let f:((x:number)=>void)|undefined; class Probe {static trace="";static arg():number {Probe.trace+="a";return 2;}static run():string {f?.(Probe.arg());f=(x)=>{Probe.trace+=""+x;};f?.(Probe.arg());return Probe.trace;}}`, "a2"},
		{"nullable module invocation", `let f:((x:number)=>void)|null|undefined; function install():void {f=(x)=>{Probe.trace+=""+x;};} class Probe {static trace="";static arg():number {Probe.trace+="a";return 2;}static run():string {f?.(Probe.arg());f=null;f?.(Probe.arg());install();f?.(Probe.arg());return Probe.trace;}}`, "a2"},
		{"module installed elsewhere", `let f:((x:number)=>number)|undefined; function install():void {f=(x)=>x+4;} function invoke():number{return f(3);} class Probe {static run():number{install();return invoke();}}`, "7"},
		{"unguarded initialized module", `let f:((x:number)=>number)|undefined; class Probe {static run():number {f=(x)=>x+4;return f(3);}}`, "7"},
		{"captured helper methods", `interface Helpers {tag:string;upper(x:string):string;isTok(x:unknown,s?:string):boolean;} function invoke(h:Helpers):string {const {upper,isTok}=h;const read=(x:string):string=>isTok(x)?upper(x):"no";return read("a");} class Probe {static run():string {return invoke({tag:"h",upper:(x)=>x+"!",isTok:(x,s)=>true});}}`, "a!"},
		{"module function value", `let f:(x:number)=>number=(x)=>x+1; class Probe {static run():number{f=(x)=>x+2;return f(3);}}`, "5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := conversionOracle(t, `"use strict";`+tc.source); got != tc.want+"\n" {
				t.Fatalf("Node %q want %q", got, tc.want)
			}
			gogenOracle(t, tc.source, tc.want)
		})
	}
}

func TestIdentifierImportedCallNodeOracle(t *testing.T) {
	library := `export function add(x:number=10,...tail:number[]):number{return x+tail.length;}`
	source := `import {add as imported} from "./library";
class Probe {static run():string {const alias=imported;const invoke=(f:(x?:number,...tail:number[])=>number):number=>f(undefined,2,3);return alias(4,5)+":"+invoke(imported);}}`
	want := conversionOracle(t, strings.Replace(library, "export ", "", 1)+strings.Replace(source, `import {add as imported} from "./library";`, `const imported=add;`, 1))
	p := lowerStatementsProbe(t, map[string]string{"library.ts": library, "probe.ts": source}, []string{"library.ts", "probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	if _, err := emitABAP(t, p); err != nil {
		t.Fatal(err)
	}
}
