package tsfront

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func TestMinMaxGeneralAndSpread(t *testing.T) {
	t.Setenv("ABAPITI_ASSUME_INT", "")
	source := `export class Probe {
 static pair(a:number,b:number):number { return Math.min(a,b); }
 static maximum(a:number,b:number):number { return Math.max(a,b); }
 static spread(a:number[]):number { return Math.min(...a); }
 static empty():number { return Math.max(); }
 static one(n:number):number { return Math.min(n); }
 static mutate(a:number[]):number { a.push(-99); return 7; }
 static snapshot():boolean { const a=[1.25,2.5]; return Math.min(...a,Probe.mutate(a))===1.25; }
 static run(n:number):string {
  const xs=[n,2.5,-1.25];
  return (Math.min(9,...xs,4)===-1.25)+":"+(Math.max(...xs)===n)+":"+(Math.min(1,2,3)===1);
 }
}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	proven := 0
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			walkNumberStmt(m.Body, func(*hir.Stmt) {}, func(e *hir.Expr) {
				if e.Kind == hir.NumericMinMax && e.Type.Kind == hir.I32 {
					proven++
				}
			})
		}
	}
	if proven < 2 {
		t.Fatal("variadic integral operands lost their proofs", hir.Dump(p))
	}
	main := fmt.Sprintf(`
 check:=func(ok bool){if !ok {panic("minmax")}}
 min,max,spread:=%s,%s,%s
 nz:=negativeZero()
 check(1/min(0,nz)<0 && 1/min(nz,0)<0)
 check(1/max(0,nz)>0 && 1/max(nz,0)>0)
 nan:=numberMinArray([]float64{0.0/float64(len([]int{})),1})
 check(min(nan,1)!=min(nan,1) && max(1,nan)!=max(1,nan))
 check(spread(&array[float64]{Items:[]float64{2.5,-1.25,4}})==-1.25)
 check(spread(&array[float64]{})>1e308 && %s() < -1e308)
 check(%s(2.5)==2.5)
 check(%s())
 fmt.Println(%s(3.75))`, goEntry("probe.ts.Probe", "pair"), goEntry("probe.ts.Probe", "maximum"), goEntry("probe.ts.Probe", "spread"), goEntry("probe.ts.Probe", "empty"), goEntry("probe.ts.Probe", "one"), goEntry("probe.ts.Probe", "snapshot"), goEntry("probe.ts.Probe", "run"))
	if got := runGoHIR(t, p, main); got != "true:true:true\n" {
		t.Fatal(got)
	}
}

func TestMinMaxAssumedIntegerSpread(t *testing.T) {
	p := integerProbe(t, `export class Probe { static run(n:number, a:number[]):number { return Math.max(n,...a,1); } }`)
	prog, ds, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{AssumeOnlyIntegerCalculations: true})
	if err != nil || hasBlocking(ds) {
		t.Fatal(err, ds)
	}
	if errs := hir.Verify(prog); len(errs) > 0 {
		t.Fatal(errs)
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s(3,&array[int64]{Items:[]int64{5,2}}))", goEntry("probe.ts.Probe", "run")))
	if got != "5\n" {
		t.Fatal(got)
	}
}

func TestTypeofOptional(t *testing.T) {
	t.Setenv("ABAPITI_ASSUME_INT", "")
	source := `export class Item {}
 export class Probe {
 static calls=0;
 static value(s:string|undefined):string|undefined {this.calls++;return s;}
 static run(s:string|undefined,n:number|undefined,b:boolean|undefined,o:Item|undefined,a:number[]|undefined):string {
  return (typeof Probe.value(s)==="string")+":"+(typeof s!=="string")+":"+(typeof s==="undefined")+":"+(typeof s==="object")+":"+(typeof n==="number")+":"+(typeof b==="boolean")+":"+(typeof o==="object")+":"+(typeof a!=="undefined")+":"+Probe.calls;
 }
}`
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if _, err := abap.Emit(p); err != nil {
		t.Fatal(err)
	}
	// Primitive and reference optionals have different representations.
	main := fmt.Sprintf(`fmt.Println(%s(optional[jsString]{Has:true,Value:str("")},optional[float64]{Has:true,Value:1.5},optional[bool]{Has:true,Value:false},nil,nil));fmt.Println(%s(optional[jsString]{},optional[float64]{},optional[bool]{},nil,&array[float64]{}))`, goEntry("probe.ts.Probe", "run"), goEntry("probe.ts.Probe", "run"))
	got := runGoHIR(t, p, main)
	if strings.TrimSpace(got) != "true:false:false:false:true:true:false:false:1\nfalse:true:true:false:false:false:false:true:2" {
		t.Fatal(got)
	}
}
