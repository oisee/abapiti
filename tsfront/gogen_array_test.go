package tsfront

import "testing"

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
