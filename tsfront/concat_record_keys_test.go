package tsfront

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func TestConcatAndRecordKeysNodeOracle(t *testing.T) {
	cases := map[string]string{
		"covariant concat": `class Base {text:string;constructor(text:string){this.text=text;}}
 class Child extends Base {}
 class Probe {static run():string {
 const a:Base[]=[new Base("a")];const b:Child[]=[new Child("b")];
 const out=a.concat(b,[new Child("c")]);b[0].text="changed";
 return out[0].text+":"+out[1].text+":"+out[2].text;
 }}`,
		"concat": `class Probe {
 static log:string="";
 static base(a:number[]):number[] {Probe.log+="r";return a;}
 static first(a:number[]):number[] {Probe.log+="a";return a;}
 static last(a:number[]):number[] {Probe.log+="b";a.push(3);return [4];}
 static scalar(a:number[]):number {a.push(5);return 6;}
 static run():string {
 const a=[1];const b=[2];
 const out=Probe.base(a).concat(Probe.first(b),Probe.last(b));
 const copy=a.concat();copy.push(9);
 const scalars=a.concat(7,8);
 const single=a.concat(Probe.scalar(a));
 return Probe.log+":"+out.join(",")+":"+a.join(",")+":"+copy.join(",")+":"+scalars.join(",")+":"+single.join(",");
 }}`,
		"record keys": `class Probe {
 static put(r:{[key:string]:string},k:any,v:string):void {r[k]=v;}
 static get(r:{[key:string]:string},k:any):string {return r[k];}
 static run():string {
 const r:{[key:string]:string}={};
 Probe.put(r,"", "empty");Probe.put(r,12,"number");Probe.put(r,false,"bool");
 Probe.put(r,undefined,"absent");Probe.put(r,null,"null");
 const optional:string|undefined=undefined;
 r[optional as any]="optional";
 return Probe.get(r,"")+":"+Probe.get(r,12)+":"+Probe.get(r,false)+":"+Probe.get(r,undefined)+":"+Probe.get(r,null);
 }}`,
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, source).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(want))
			}
			p := integerProbe(t, source)
			prog, ds, err := p.LowerWithOptions([]string{"probe.ts"}, LowerOptions{TrapOnFailure: true})
			if err != nil || hasBlocking(ds) {
				t.Fatal(err, ds)
			}
			if errors := hir.Verify(prog); len(errors) != 0 {
				t.Fatal(errors)
			}
			got := runGoHIR(t, prog, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run")))
			if got != string(want) {
				t.Fatalf("Go %q Node %q", got, want)
			}
			if _, err := abap.Emit(prog); err != nil {
				t.Fatal(err)
			}
		})
	}
}
