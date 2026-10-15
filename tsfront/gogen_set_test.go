package tsfront

import "testing"

func TestGogenSetArrayExpression(t *testing.T) {
	gogenOracle(t, `class Item {name: string; constructor(n: string) {this.name=n;}}
 class Probe {static run(): string {
 const CLASSES = new Map<string, Item>([["x",new Item("b")],["y",new Item("")],["z",new Item("b")],["a",new Item("a")]]);
 const names = new Set([...CLASSES.values()].map(c => c.name).filter(Boolean));
 const nums = new Set([parseFloat("NaN"), parseFloat("NaN"), -0, 0, 2]);
 return [...names].join(",") + ":" + nums.size + ":" + nums.has(parseFloat("NaN")) + ":" + [...nums].join(",");
 }}`, "b,a:3:true:NaN,0,2")
}

func TestGogenSetInsertionOrder(t *testing.T) {
	gogenOracle(t, `class Item {name="x";}
 class Probe {static run(): string {
 const nan=parseFloat("NaN");
 const input:number[]=[nan,nan,0,-0];
 const nums=new Set(input);
 const before=[...nums].join(",");
 const removed=nums.delete(nan);nums.add(nan);
 const item=new Item();const refs=new Set([item,item,new Item()]);
 const blanks:string[]=[];const empty=new Set(blanks.map(s=>s));
 return nums.size+":"+before+":"+removed+":"+nums.has(nan)+":"+[...nums].join(",")+":"+refs.size+":"+empty.size;
 }}`, "2:NaN,0:true:true:0,NaN:2:0")
}
