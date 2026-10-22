package tsfront

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGogenA18LogicalNodeOracle(t *testing.T) {
	source := `class Probe {
 static effects=0;
 static choose(x:any):string {
 const a=x||[1,2].map(v=>{Probe.effects++;return v+1;});
 const b=x&&[3].map(v=>{Probe.effects++;return v;});
 const c=x??[4].map(v=>{Probe.effects++;return v;});
 return JSON.stringify([a,b,c]);
 }
 static run():string {const out:string[]=[];for(const x of [0,-0,parseFloat("NaN"),"",null,undefined,false,2,"yes",true,[],{}])out.push(Probe.choose(x));return out.join("|")+":"+Probe.effects;}
 }`
	want := conversionOracle(t, source)
	gogenOracle(t, source, want[:len(want)-1])
}

func TestGogenA18MixedNativeNodeOracle(t *testing.T) {
	source := `class Probe {
 static effects=0;
 static text(n:number,s:string):number|string{return n||s;}
 static number(s:string,n:number):number|string{return s&&n;}
 static fallback(n:number|undefined,s:string):number|string{return n??s;}
 static run():string {const a:number[]=[];const b:number[]=[];const c=a||b;c.push(7);return JSON.stringify([Probe.text(0,"x"),Probe.text(2,"x"),Probe.number("",4),Probe.number("x",4),Probe.fallback(0,"x"),Probe.fallback(undefined,"x"),a.length,b.length]);}
 }`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	gogenCheck(t, p, "probe.ts", source, want[:len(want)-1])
	if t.Failed() {
		return
	}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	class := names.Get("probe.ts.Probe")
	driver := "CLASS ltcl_logic DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS observations FOR TESTING.\nENDCLASS.\nCLASS ltcl_logic IMPLEMENTATION.\nMETHOD observations.\nDATA expected TYPE string.\nDATA ch TYPE c LENGTH 1.\n"
	for _, line := range abapStringBuild("expected", "ch", want[:len(want)-1]) {
		driver += line + "\n"
	}
	driver += fmt.Sprintf("cl_abap_unit_assert=>assert_equals( act = %s=>%s( ) exp = expected ).\nENDMETHOD.\nENDCLASS.\n", class, names.Get("member.run"))
	files[class+".clas.testclasses.abap"] = driver
	if out := fixtureOutputDir(t); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, text := range files {
			if err := os.WriteFile(filepath.Join(out, name), []byte(text), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestGogenA18ObjectValuesNodeOracle(t *testing.T) {
	source := `class Probe {static values(x:any):string{return JSON.stringify(Object.values(x));}static run():string {return Probe.values({b:2,"10":10,"2":2,a:false})+":"+Probe.values(["x","y"])+":"+Probe.values("ab");}}`
	want := conversionOracle(t, source)
	gogenImplementedABAPOracle(t, source, want[:len(want)-1])
}

func TestGogenA18NumericNodeOracle(t *testing.T) {
	source := `class Probe {static check(x:any):string{return [Number.isInteger(x),Number.isFinite(x),Number.isSafeInteger(x)].join(",");}
 static run():string {const out:string[]=[];for(const x of [0,-0,1.5,NaN,Infinity,-Infinity,Number.MAX_SAFE_INTEGER,9007199254740992,"2",null,undefined,false,[],{}])out.push(Probe.check(x));return out.join("|")+":"+String(Math.abs(-0))+":"+Math.abs(-3);}}`
	want := conversionOracle(t, source)
	gogenImplementedABAPOracle(t, source, want[:len(want)-1])
}

func TestGogenA18ArrayFromNodeOracle(t *testing.T) {
	source := `class Probe {static effects="";static run():string {const a=Array.from("a😀".padEnd(4,"😀"),(c,i)=>{Probe.effects+=String(i);return c+":"+c.length;});const b=Array.from({length:3,0:"x",2:"z"},(c,i)=>String(c)+i);return JSON.stringify(a)+":"+JSON.stringify(b)+":"+Probe.effects;}}`
	want := conversionOracle(t, source)
	gogenOracle(t, source, want[:len(want)-1], "arrayLike.length")
}

func TestGogenA18SegmentNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {return Array.from("a😀",c=>c.charCodeAt(0).toString(16).padStart(4,"0")).join("");}}`
	want := conversionOracle(t, source)
	gogenImplementedABAPOracle(t, source, want[:len(want)-1])
}

func TestGogenA18DynamicSegmentNodeOracle(t *testing.T) {
	source := `class Probe {static segment(s:any):string{return Array.from(s,c=>c.charCodeAt(0).toString(16).padStart(4,"0")).join("");}static run():string{return Probe.segment("a😀");}}`
	want := conversionOracle(t, source)
	gogenImplementedABAPOracle(t, source, strings.TrimSuffix(want, "\n"))
}

func TestGogenA18FromEntriesSpreadNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {const entries:Record<string,{field:string,n:number}>={base:{field:"base",n:0},...Object.fromEntries([1,2].map(i=>["key"+i,{field:"v"+i,n:i}]))};return JSON.stringify(entries);}}`
	want := conversionOracle(t, source)
	gogenOracle(t, source, strings.TrimSuffix(want, "\n"), "own-property order")
}

func TestGogenA18FromEntriesNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {const values=Object.fromEntries([1,2,3,4].map(i=>["key"+i,{field:"v"+i,n:i}]));return JSON.stringify(values)+":"+Object.keys(values).join(",");}}`
	want := conversionOracle(t, source)
	gogenOracle(t, source, want[:len(want)-1], "own-property order")
}

func TestGogenA18FromEntriesTaggedNodeOracle(t *testing.T) {
	source := `class Probe {static make():any {return {value:Object.fromEntries([1,2].map(i=>["k"+i,i===1?{field:"v"}:undefined]))};}static run():string {const x=Probe.make();return JSON.stringify(x)+":"+Object.keys(x.value).join(",");}}`
	want := conversionOracle(t, source)
	gogenOracle(t, source, want[:len(want)-1])
}

func TestGogenA18ArrayFromLiveNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {const a=[1,2];delete a[0];const b=Array.from(a,(v,i)=>{if(i===0)a.push(3);return String(v);});return JSON.stringify(b)+":"+a.length;}}`
	want := conversionOracle(t, source)
	gogenOracle(t, source, want[:len(want)-1])
}

func TestGogenA18EnvNodeOracle(t *testing.T) {
	t.Setenv("GOGEN_A18_PRESENT", "value")
	t.Setenv("GOGEN_A18_EMPTY", "")
	source := `declare const process: {env: {[key:string]:string|undefined}};
 class Probe {static run():string {const name="GOGEN_A18_PRESENT";return JSON.stringify([process.env[name],process.env.GOGEN_A18_EMPTY,process.env.GOGEN_A18_MISSING]);}}`
	want := conversionOracle(t, source)
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	gogenCheck(t, p, "probe.ts", source, strings.TrimSuffix(want, "\n"))
	// Without a declared env host capability, ABAP deliberately observes all
	// environment lookups as undefined. Keep the Go/Node environment oracle and
	// also pin this documented ABAP fallback rather than reading the host process.
	if !t.Failed() {
		storageABAPProbe(t, p, "probe.ts.Probe", "[null,null,null]", "")
	}
}

func TestGogenA18ShadowedBuiltinsNodeOracle(t *testing.T) {
	source := `class Probe {static run():string {const Object={values:(x:number)=>[x+1]};const Number={isInteger:(x:number)=>false};const Math={abs:(x:number)=>x};const Infinity=7;return JSON.stringify([Object.values(2),Number.isInteger(2),Math.abs(-3),Infinity]);}}`
	want := conversionOracle(t, source)
	gogenOracle(t, source, strings.TrimSuffix(want, "\n"))
}

func TestGogenA18LocatedHostAndPrototypeRefusals(t *testing.T) {
	for _, source := range []string{
		`declare const process:{argv:string[]};class Probe {static run(){return process.argv[1];}}`,
		`declare const process:{exitCode:number};class Probe {static run(){process.exitCode=1;}}`,
		`class Probe {static run(o:any){return Object.assign(Object.create(Object.getPrototypeOf(o)),o,{getName:()=>"changed"});}}`,
	} {
		_, ds := lowerProbe(t, sourceProbe(t, source))
		found := false
		for _, d := range ds {
			if (d.Category == "unsupported-process" || d.Category == "unsupported-prototype") && strings.Contains(d.Loc, "input.ts:") {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing located refusal: %v", ds)
		}
	}
}
