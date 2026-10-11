package tsfront

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func TestDirectDataConversionNodeOracle(t *testing.T) {
	source := `class Conf {value=1; optional?:string;}
 class Probe {
 static assign(input:any):Conf {const conf:Conf=input;return conf;}
 static assert(input:any):Conf {return input as Conf;}
 static inferred(input:any):number {const conf:Conf=input;return conf.value;}
 static run():string {
 const input=JSON.parse('{"value":1}');const a=Probe.assign(input);const b=Probe.assert(input);
 input["value"]=2;const seen=a.value;a.value=3;
 const native=new Conf();const same=Probe.assign(native)===native;
 return seen+":"+b.value+":"+input.value+":"+(a===b)+":"+same+":"+(a instanceof Conf)+":"+(a.optional===undefined)+":"+(Probe.assign(undefined)?.value===undefined)+":"+Probe.inferred(input);
 }
 static invalid():void {Probe.assign(42);}
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok{panic("invalid conversion escaped")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))
	if got := runGoHIR(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
}

func TestCommonClassAssertionNodeOracle(t *testing.T) {
	source := `class Base {value=1;read():number{return this.value;}}
 class Child extends Base {extra=2;}
 class Sibling extends Base {read():number{return this.value+10;}}
 class Probe {
 static calls=0;
 static once(input:Base):Base {Probe.calls=Probe.calls+1;return input;}
 static common(input:Base):number {const view=Probe.once(input) as Child;return view.read();}
 static specific(input:Base):number {const view=input as Child;return view.extra;}
 static escaping(input:Base):Child {const view=input as Child;return view;}
 static run():string {const a=new Child();const b=new Sibling();return Probe.common(a)+":"+Probe.common(b)+":"+Probe.calls+":"+(b instanceof Child);}
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	for _, c := range p.Classes {
		if c.Name != "probe.ts.Probe" {
			continue
		}
		for _, m := range c.Methods {
			if m.Name == "specific" || m.Name == "escaping" {
				if !strings.Contains(hir.Dump(&hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{m}}}}), "object.checkedClassView") {
					t.Fatalf("%s lost its nominal boundary", m.Name)
				}
				m.Body = hir.B(&hir.Stmt{Kind: hir.Trap, Name: "outside common view probe"})
			}
		}
	}
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
}

func TestCommonInterfaceClassAssertionNodeOracle(t *testing.T) {
	source := `interface ObjectLike {name():string;}
 abstract class Base implements ObjectLike {value=1;name():string{return "base";}read():number{return this.value;}}
 class Child extends Base {extra=2;}
 class Sibling extends Base {read():number{return this.value+10;}}
 class Other implements ObjectLike {name():string{return "other";}}
 class Probe {
 static first(input:ObjectLike):ObjectLike|undefined {return input;}
 static common(input:ObjectLike):number {const view=Probe.first(input) as Child;return view.read();}
 static absent(input?:ObjectLike):number|undefined {const view=input as Child;return view?.read();}
 static invalid():void {Probe.common(new Other());}
 static run():string {const a=new Child();const b=new Sibling();return Probe.common(a)+":"+Probe.common(b)+":"+(b instanceof Child)+":"+(Probe.absent()===undefined);}
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	main := fmt.Sprintf(`defer func(){if _,ok:=recover().(trap);!ok{panic("incompatible object escaped")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))
	if got := runGoHIR(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
}

func conversionOracle(t *testing.T, source string) string {
	t.Helper()
	out, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	return string(out)
}

func TestDynamicPrimitiveMethodConversionNodeOracle(t *testing.T) {
	source := `class Probe {
 static count=0;
 static once(value:any):any {Probe.count=Probe.count+1;return value;}
 static convert(value:any):string|undefined {return Probe.once(value)?.toString();}
 static invalid():void {const value:any=undefined;value.toString();}
 static run():string {return Probe.convert(10)+":"+Probe.convert(false)+":"+Probe.convert("text")+":"+Probe.convert(undefined)+":"+Probe.convert(null)+":"+Probe.count;}
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	main := fmt.Sprintf(`defer func(){if _,ok:=recover().(trap);!ok{panic("nullish method call escaped")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))
	if got := runGoHIR(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
}
