package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

func TestGogenArrayCalls(t *testing.T) {
	gogenOracle(t, `class Probe {
 static values: number[] = [];
 static arg(n: number): number { Probe.values.push(n); return Probe.values.length; }
 static run(): string {
  const a = Probe.values;
  const size = a.push(Probe.arg(7), Probe.arg(8));
  return size + ":" + a.join(",") + ":" + a.at(-1) + ":" + a.at(-4) + ":" + (a.at(-5) === undefined) + ":" + (a.at(4) === undefined) + ":" + a.at(-1.9) + ":" + a.push();
 }
}`, "4:7,8,1,2:2:7:true:true:2:4")
}

func TestGogenArrayAtEdges(t *testing.T) {
	gogenOracle(t, `class Item {name="ok";}
 class Probe {static run(): string {
 const a=["first","last"];
 const optional: (number | undefined)[] = [undefined, 0];
 const refs=[new Item()];const empty:number[]=[];
 const nums=[1,2];const size=nums.push(0,...nums,9);
 return a.at()+":"+a.at(parseFloat("NaN"))+":"+a.at(-0.9)+":"+(a.at(parseFloat("Infinity"))===undefined)+":"+(a.at(-1e100)===undefined)+":"+(empty.at(-1)===undefined)+":"+refs.at(-1)!.name+":"+(refs.at(1)===undefined)+":"+(optional.at(0)===undefined)+":"+optional.at(-1)+":"+size+":"+nums.join(",");
 }}`, "first:first:first:true:true:true:ok:true:true:0:6:1,2,0,1,2,9")
}

func TestGogenSparsePushSpread(t *testing.T) {
	for _, args := range []string{"...xs", "2,...xs"} {
		t.Run(args, func(t *testing.T) {
			source := `class Probe {static run():string {const xs:(number|undefined)[]=[1];delete xs[0];const a:(number|undefined)[]=[];a.push(` + args + `);let visits=0;a.forEach(v=>{visits++;});return a.length+":"+(a[a.length-1]===undefined)+":"+visits;}}`
			p := lowerStatementsProbe(t, map[string]string{"probe.ts": source}, []string{"probe.ts"})
			want := "1:true:1\n"
			if args != "...xs" {
				want = "2:true:2\n"
			}
			if got := runGoHIR(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
				t.Fatalf("Go %q want %q", got, want)
			}
			if got := conversionOracle(t, source); got != want {
				t.Fatalf("Node %q want %q", got, want)
			}
			storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSuffix(want, "\n"), "")
		})
	}
}

func TestGogenPrimitivePushSpreadRefusal(t *testing.T) {
	for _, args := range []string{"...xs", "2,...xs"} {
		requireDiagnostic(t, sourceProbe(t, `class Probe {static run():void {const xs=[1];delete xs[0];const a:number[]=[];a.push(`+args+`);}}`), "unsupported-call")
	}
}

func TestGogenSparsePrimitiveSourceIntoOptionalPush(t *testing.T) {
	source := `class Probe {static run():string {const xs:number[]=[];xs[2]=3;const copy:(number|undefined)[]=[...xs];const a:(number|undefined)[]=[];a.push(2,...xs);let visits=0;a.forEach(v=>{visits++;});return (a[1]===undefined)+":"+(copy[0]===undefined)+":"+visits;}}`
	gogenImplementedABAPOracle(t, source, "true:true:4")
}

func TestGogenPushSpreadSnapshotsBeforeLaterArguments(t *testing.T) {
	source := `class Probe {static change(a:(number|undefined)[]):number {a.push(9);return 3;} static run():string {const a:(number|undefined)[]=[1];a.push(2,...a,Probe.change(a));return a.length+":"+a[0]+":"+a[1]+":"+a[2]+":"+a[3]+":"+a[4];}}`
	gogenImplementedABAPOracle(t, source, "5:1:9:2:1:3")
}

func TestGogenPushSetSpread(t *testing.T) {
	gogenOracle(t, `class Probe {static run():string {const s=new Set([1,2]);const a:number[]=[];return a.push(...s)+":"+a.push(3,...s,4)+":"+a.join(",");}}`, "2:6:1,2,3,1,2,4")
}
