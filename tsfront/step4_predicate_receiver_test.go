package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

const step4PredicateReceiver = `class ABase {private brandA=0;kind=1;state=10;name(s:string):string {this.state++;return "baseA"+s+this.state;}}
class A extends ABase {name(s:string):string {this.state+=2;return "A"+s+this.state;}onlyA():string{return "kind"+this.kind;}}
class BBase {private brandB=0;state=20;name(s:string):string {this.state++;return "baseB"+s+this.state;}}
class B extends BBase {name(s:string):string {this.state+=3;return "B"+s+this.state;}}
function isA(value:unknown,kind:number):value is A {return value instanceof A && value.kind===kind;}
function read(value:A|B):string {
 if(isA(value,0)) return "zero"+value.kind;
 if(isA(value,2)) return "two";
 return value.name("i");
}
function readArray(values:(A|B)[]):string {
 if(isA(values[0],0)) return "zero"+values[0].kind;
 if(isA(values[0],2)) return "two";
 return values[0].name(Probe.argument(values));
}
class Probe {
 static trace="";
 static argument(values:(A|B)[]):string {Probe.trace+="arg";values[0]=new B();return "e";}
 static positive(value:A|B):string {if(isA(value,1)) return value.onlyA();return "no";}
 static run():string {
 const a=new A();const b=new B();
 return read(a)+"|"+read(b)+"|"+readArray([a])+"|"+readArray([b])+"|"+a.state+"|"+b.state+"|"+Probe.trace+"|"+Probe.positive(a)+"|"+Probe.positive(b);
 }
}`

func TestStep4PredicateCommonReceiverNodeOracle(t *testing.T) {
	want := conversionOracle(t, step4PredicateReceiver)
	t.Logf("Node: %s", want)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": step4PredicateReceiver}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestStep4PredicateCommonReceiverJSDocNodeOracle(t *testing.T) {
	types, _, _ := strings.Cut(step4PredicateReceiver, "function isA")
	types = strings.ReplaceAll(types, "class ", "export class ")
	runMjsOracleTypes(t, types, step4PredicateReceiverJS, false, true)
}

const step4PredicateReceiverJS = `import {A,B} from "./types.ts";
/** @param {unknown} value @param {number} kind @returns {value is A} */
function isA(value,kind) {return value instanceof A && value.kind===kind;}
/** @param {A|B} value @returns {string} */
function read(value) {
 if(isA(value,0)) return "zero"+value.kind;
 if(isA(value,2)) return "two";
 return value.name("i");
}
/** @param {(A|B)[]} values @returns {string} */
function readArray(values) {
 if(isA(values[0],0)) return "zero"+values[0].kind;
 if(isA(values[0],2)) return "two";
 return values[0].name(Probe.argument(values));
}
export class Probe {
 static trace="";
 /** @param {(A|B)[]} values @returns {string} */
 static argument(values) {Probe.trace+="arg";values[0]=new B();return "e";}
 /** @param {A|B} value @returns {string} */
 static positive(value) {if(isA(value,1)) return value.onlyA();return "no";}
 /** @returns {string} */
 static run() {
 const a=new A();const b=new B();
 return read(a)+"|"+read(b)+"|"+readArray([a])+"|"+readArray([b])+"|"+a.state+"|"+b.state+"|"+Probe.trace+"|"+Probe.positive(a)+"|"+Probe.positive(b);
 }
}`

func TestStep4PredicateReceiverUnsupportedMemberNodeOracle(t *testing.T) {
	for _, tc := range []struct{ name, method, node string }{
		{"missing", "", "TypeError\n"},
		// JavaScript can execute this method, but its numeric ABI cannot
		// implement the selected string slot. Keep the checked refusal.
		{"incompatible", `name(n:number):string{return "number"+n;}`, "numberx\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `class A {private brandA=0;kind=1;` + tc.method + `}
class B {private brandB=0;name(s:string):string{return s;}}
function isA(value:unknown,kind:number):value is A {return value instanceof A && value.kind===kind;}
function read(value:A|B):string {if(isA(value,0)) return "zero";return value.name("x");}
class Probe {static run():string{return read(new A());}}`
			out, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\ntry{console.log(Probe.run())}catch(e){console.log(e.name)}")`, source).CombinedOutput()
			if err != nil || string(out) != tc.node {
				t.Fatalf("Node %v: %s", err, out)
			}
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
			got := runGoHIR(t, p, fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok{panic("unsupported member did not refuse at checked boundary")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "run")))
			if got != "refused\n" {
				t.Fatal(got)
			}
		})
	}
}

func TestStep4PredicateCommonReceiverAbsentNodeOracle(t *testing.T) {
	for _, absent := range []string{"undefined", "null"} {
		t.Run(absent, func(t *testing.T) {
			source := `class A {private brandA=0;kind=1;name():string{Probe.trace+="invokeA";return "a";}}
class B {private brandB=0;name():string{Probe.trace+="invokeB";return "b";}}
function isA(value:unknown,kind:number):value is A {Probe.trace+="predicate";return value!==null && value!==undefined && value instanceof A && value.kind===kind;}
function read(value:A|B|` + absent + `):string {if(isA(value,0)) return "zero";Probe.trace+="before";return value!.name();}
class Probe {static trace="";static snapshot():string{return Probe.trace;}static run():string {Probe.trace+="start";return read(` + absent + `);}}`
			out, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\ntry{Probe.run()}catch(e){console.log(e.name+'|'+Probe.snapshot())}")`, source).CombinedOutput()
			if err != nil || string(out) != "TypeError|startpredicatebefore\n" {
				t.Fatalf("Node %v: %s", err, out)
			}
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
			got := runGoHIR(t, p, fmt.Sprintf(`defer func(){switch recover().(type){case rangeFault,trap:default:panic("absent receiver did not refuse at checked boundary")};fmt.Println("refused|"+%s().String())}();%s()`, goEntry("probe.ts.Probe", "snapshot"), goEntry("probe.ts.Probe", "run")))
			if got != "refused|startpredicatebefore\n" {
				t.Fatal(got)
			}
		})
	}
}

func TestStep4PredicateCommonVariadicReceiverNodeOracle(t *testing.T) {
	source := `class A {private brandA=0;kind=1;state=0;name(...args:string[]):string{this.state++;return "A"+args.join(",")+this.state;}}
class B {private brandB=0;state=0;name(...args:string[]):string{this.state+=2;return "B"+args.join(",")+this.state;}}
function isA(value:unknown,kind:number):value is A{return value instanceof A && value.kind===kind;}
function read(value:A|B):string{if(isA(value,0))return "zero";return value.name("x","y");}
class Probe {static run():string{return read(new A())+"|"+read(new B());}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
	if got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

func TestStep4PredicateReceiverVariadicMismatchNodeOracle(t *testing.T) {
	source := `class A {private brandA=0;kind=1;name(...args:string[]):string{return "A"+args.length;}}
class B {private brandB=0;name(args:string[]):string{return "B"+args.length;}}
function isA(value:unknown,kind:number):value is A{return value instanceof A && value.kind===kind;}
function read(value:A|B):string{if(isA(value,0))return "zero";return value.name(["x","y"]);}
class Probe {static run():string{return read(new A())+"|"+read(new B());}}`
	// Node wraps the array as one rest argument for A. The shared HIR
	// array slot cannot substitute its selected nonvariadic convention.
	if got := conversionOracle(t, source); got != "A1|B2\n" {
		t.Fatal(got)
	}
	for _, trap := range []bool{false, true} {
		t.Run(fmt.Sprintf("trap=%t", trap), func(t *testing.T) {
			project := integerProbe(t, source)
			p, ds, err := project.LowerWithOptions([]string{"probe.ts"}, LowerOptions{TrapOnFailure: trap})
			if err != nil || hasBlocking(ds) {
				t.Fatal(err, ds)
			}
			if errs := hir.Verify(p); len(errs) != 0 {
				t.Fatal(errs)
			}
			got := runGoHIR(t, p, fmt.Sprintf(`defer func(){if x:=recover();x!=nil{if _,ok:=x.(rangeFault);!ok{panic(x)};fmt.Println("refused")}}();fmt.Println("returned:"+%s().String())`, goEntry("probe.ts.Probe", "run")))
			if got != "refused\n" {
				t.Fatalf("incompatible rest/array ABI must refuse; Go %q, Node would return A1|B2", got)
			}
		})
	}
}
