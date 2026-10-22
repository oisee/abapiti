package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

// Exercise the own-keys result through every consumer that checks presence.
// Dynamic parameters select the same runtime path as gogen's compileProgram
// closure. A fresh result must have every key present, even for sparse inputs.
func TestOwnKeysDenseABAPNodeOracle(t *testing.T) {
	shapes := []struct{ name, setup string }{
		{"literal", `const x:any={z:1,a:2};`},
		{"parsed", `const x:any=JSON.parse('{"z":1,"a":2}');`},
		{"instance", `const x:any=new Fields();`},
		{"readded", `const x:any={z:1,a:2};delete x.z;x.z=3;`},
		{"integerKeys", `const x:any=JSON.parse('{"10":1,"2":2,"01":3,"0":4,"length":5}');`},
		{"array", `const x:any=["z","a"];`},
		{"parsedArray", `const x:any=JSON.parse('["z","a"]');`},
		{"sparseArray", `const a=["z","a","b"];delete a[1];const x:any=a;`},
	}
	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			source := `class Fields {z=1;a=2;}
class Probe {
 static consume(x:any):string {
  let iter="";for(const k of Object.keys(x)){iter+=k+",";}
  const keys=Object.keys(x);let indexed="";
  for(let i=0;i<keys.length;i++){indexed+=keys[i]+",";}
  return iter+"/"+keys.length+"/"+indexed+"/"+keys.map(k=>"["+k+"]").join(",");
 }
 static run():string {` + shape.setup + `return Probe.consume(x);}
}`
			want := conversionOracle(t, source)
			t.Logf("Node: %s", strings.TrimSpace(want))
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
			if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
				t.Fatalf("Go %q Node %q", got, want)
			}
			storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSuffix(want, "\n"), "")
		})
	}
}

func TestOwnKeysDenseOptionsCopyABAPNodeOracle(t *testing.T) {
	source := `class Registry {name="registry";}
class Options {
 folders=["src"];objects=["ZCL_PROBE"];tolerant=true;
 registry:Registry|undefined=new Registry();session:any={};
 provenance:boolean|undefined=undefined;
}
function withProvenance(compile:(options:Options)=>string):(options:Options)=>string {
 return options=>{
  const run=()=>{
   if(options.registry) options={...options,registry:new Registry()};
   return compile(options);
  };
  return run();
 };
}
class Probe {
 static run():string {
  const compile=withProvenance(options=>{
   const x:any=options;let out="";
   const target:any={};
   for(const k of Object.keys(x)){target[k]=x[k];out+=k+",";}
   return out+"/"+Object.keys(target).map(k=>"["+k+"]").join(",");
  });
  return compile(new Options());
 }
}`
	want := conversionOracle(t, source)
	t.Logf("Node: %s", strings.TrimSpace(want))
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSuffix(want, "\n"), "")
}

// Other helpers also append directly into fresh array storage. Check that
// callers see present slots, including empty strings, through the same ABI.
func TestDenseArrayBuildersABAPNodeOracle(t *testing.T) {
	source := `class Probe {
 static show(xs:string[]):string {
  let out="";for(const value of xs){out+="["+value+"]";}
  for(let i=0;i<xs.length;i++){out+=xs[i]+",";}
  return out+"/"+xs.length+"/"+xs.map(x=>"("+x+")").join(",");
 }
 static run():string {
  const record:Record<string,string>={z:"Z",a:""};
  const map=new Map<string,string>();map.set("z","Z");map.set("a","");
  const set=new Set<string>(["Z",""]);
  return Probe.show(Object.values(record))+"|"+
   Probe.show(Object.entries(record).map(([k,v])=>k+"="+v))+"|"+
   Probe.show([...map.keys()])+"|"+Probe.show([...map.values()])+"|"+
   Probe.show([...set.values()])+"|"+Probe.show("z,,a,".split(","));
 }
}`
	want := conversionOracle(t, source)
	t.Logf("Node: %s", strings.TrimSpace(want))
	p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
	if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSuffix(want, "\n"), "")
}
