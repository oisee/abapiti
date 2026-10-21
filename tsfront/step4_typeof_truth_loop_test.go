package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

// typeofLoopSource keeps the retained CLI oracle shape: the loop variable's
// declared union mixes an object payload with boolean and nullish values, so
// `n && typeof n === "object"` must apply JavaScript truthiness (false and
// null never enter the object loop) and read the runtime typeof tag.
const typeofLoopSource = `interface Shape {x:number}
function inspect(value:Shape|boolean|null|undefined):string {
 let hits=0;
 for(let n:Shape|boolean|null|undefined=value;n && typeof n === "object";n=undefined) hits++;
 return typeof value+":"+hits;
}
class Probe {
 static run():string {
  return [inspect(false),inspect(true),inspect(null),inspect(undefined),inspect({x:1})].join("|");
 }
}`

func TestStep4TypeofLoopTruthinessNodeOracle(t *testing.T) {
	want := conversionOracle(t, typeofLoopSource)
	if want != "boolean:0|boolean:0|object:0|undefined:0|object:1\n" {
		t.Fatalf("node oracle drifted: %q", want)
	}
	p := structuralUnionProgram(t, typeofLoopSource)
	t.Run("Go", func(t *testing.T) {
		if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
			t.Fatalf("Go %q Node %q", got, want)
		}
	})
	t.Run("ABAP", func(t *testing.T) { storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "") })
}

// typeofUnionNullSource keeps the typed-union lane: the loop variable's
// declared union lowers to a checked structural family while the chase fields
// stay object-typed, and a legal null argument must leave the loop through
// truthiness. The narrowed family view must not run its checked admission
// before the typeof tag: typeof never converts.
const typeofUnionNullSource = `interface Expr {kind:"expr";e:string;base?:AstNode;x?:AstNode}
interface Type {kind:"type";k:string;base?:AstNode;x?:AstNode}
type AstNode=Expr|Type;
class Probe {
  static run():string {
    const count=(value:Expr|null|undefined):number=>{
      let hits=0;
      for(let n:AstNode|boolean|null|undefined=value;n && typeof n === "object";n=n.base??n.x) {hits++;}
      return hits;
    };
    const valid:Expr={kind:"expr",e:"plain"};
    return [count(valid),count(null),count(undefined)].join("|");
  }
}`

func TestStep4TypeofUnionNullCheckedViewNodeOracle(t *testing.T) {
	want := conversionOracle(t, typeofUnionNullSource)
	if want != "1|0|0\n" {
		t.Fatalf("node oracle drifted: %q", want)
	}
	p := structuralUnionProgram(t, typeofUnionNullSource)
	// Recursive structural families keep the Go oracle until the ABAP ABI
	// exists (hir/abap/ops.go); the ABAP truthiness and typeof lanes are
	// covered by the loop and chase sources below.
	t.Run("Go", func(t *testing.T) {
		if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
			t.Fatalf("Go %q Node %q", got, want)
		}
	})
}

// typeofBooleanChaseSource keeps the boolean chase whose field declares the
// boolean member: the loop condition must read the runtime tag (a boolean
// never narrows to the object family) and stay green.
const typeofBooleanChaseSource = `type Type = {k:"bool"};
type Expr = {e:"attr";name:string;x?:Expr|Type|boolean};
function chase(value:Expr|null|undefined):number {
  let count=0;
  for(let n:Expr|Type|boolean|null|undefined=value;n && typeof n === "object";n=("x" in n?n.x:undefined)) count++;
  return count;
}
class Probe {
  static run():string {
    return [chase({e:"attr",name:"a",x:true}),chase({e:"attr",name:"a",x:false}),chase({e:"attr",name:"a",x:{k:"bool"}}),chase(undefined)].join("|");
  }
}`

func TestStep4TypeofBooleanChaseNodeOracle(t *testing.T) {
	want := conversionOracle(t, typeofBooleanChaseSource)
	if want != "1|1|2|0\n" {
		t.Fatalf("node oracle drifted: %q", want)
	}
	p := structuralUnionProgram(t, typeofBooleanChaseSource)
	t.Run("Go", func(t *testing.T) {
		if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
			t.Fatalf("Go %q Node %q", got, want)
		}
	})
	t.Run("ABAP", func(t *testing.T) { storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "") })
}

// typeofShadowCaptureSource exercises the restored declared-storage read's
// binding bookkeeping: a block-shadowed erased-union local (the range pass
// renames one binding) and an arrow capturing an erased-union local must both
// read the runtime truthiness and typeof tag of the value they closed over.
const typeofShadowCaptureSource = `type Node1 = { x?: Node1 };
class Probe {
  static run(): string {
    const out: string[] = [];
    const shadow = (v: Node1|boolean|null|undefined): string => {
      let n: Node1|boolean|null|undefined = v;
      {
        let n: Node1|boolean|null|undefined = true;
        if (n && typeof n === "object") return "inner-object";
        return "inner-" + typeof n;
      }
    };
    let captured: Node1|boolean|null|undefined = null;
    const read = (): string => captured && typeof captured === "object" ? "obj" : typeof captured;
    out.push(shadow({x: undefined}));
    out.push(read());
    captured = {x: undefined};
    out.push(read());
    captured = false;
    out.push(read());
    return out.join("|");
  }
}`

func TestStep4TypeofShadowCaptureNodeOracle(t *testing.T) {
	want := conversionOracle(t, typeofShadowCaptureSource)
	if want != "inner-boolean|object|obj|boolean\n" {
		t.Fatalf("node oracle drifted: %q", want)
	}
	p := structuralUnionProgram(t, typeofShadowCaptureSource)
	t.Run("Go", func(t *testing.T) {
		if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
			t.Fatalf("Go %q Node %q", got, want)
		}
	})
	t.Run("ABAP", func(t *testing.T) { storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "") })
}

// typeofChaseSource mirrors tools/gogen/emit-table-move.ts keys(): a storage
// chase over `Expr | boolean | null | undefined` whose update yields absent
// optionals, plus typeof in call, if and return positions. Booleans, null and
// undefined must leave or exit the loop through truthiness and the runtime
// typeof tag without any checked object conversion.
const typeofChaseSource = `type Node1 = { x?: Node1; tag?: string };
class Probe {
  static run(): string {
    const out: string[] = [];
    const keys = (value: Node1 | boolean | null | undefined): string[] => {
      const result: string[] = [];
      for (let n: Node1 | boolean | null | undefined = value; n && typeof n === "object"; n = n.x) {
        if (n.tag) result.push(n.tag);
      }
      return result;
    };
    const walk = (value: Node1 | boolean | null | undefined): number => {
      let count = 0;
      let n: Node1 | boolean | null | undefined = value;
      while (n && typeof n === "object") {
        count += 1;
        n = n.x;
      }
      return count;
    };
    const tag = (v: Node1 | boolean | null | undefined): string => typeof v;
    const kind = (v: Node1 | boolean | null | undefined): string => {
      if (typeof v === "object") return v === null ? "null" : "obj";
      if (typeof v === "boolean") return "bool";
      return "other";
    };
    const native: Node1 = {tag: "a", x: {tag: "b", x: {tag: "c"}}};
    const parsed = JSON.parse('{"tag":"j","x":{"tag":"k"}}') as Node1;
    out.push(keys(native).join(","));
    out.push(keys(parsed).join(","));
    out.push(keys(true).join(","));
    out.push(keys(false).join(","));
    out.push(keys(null).join(","));
    out.push(keys(undefined).join(","));
    out.push(String(walk(native)));
    out.push(String(walk(parsed)));
    out.push(String(walk(true)) + String(walk(false)) + String(walk(null)) + String(walk(undefined)));
    out.push(tag(true) + "/" + tag(false) + "/" + tag(null) + "/" + tag(undefined) + "/" + tag(native));
    out.push(kind(native) + "/" + kind(true) + "/" + kind(false) + "/" + kind(null) + "/" + kind(undefined));
    return out.join(";");
  }
}`

func TestStep4TypeofChaseErasedMembersNodeOracle(t *testing.T) {
	want := conversionOracle(t, typeofChaseSource)
	if want != "a,b,c;j,k;;;;;3;2;0000;boolean/boolean/object/undefined/object;obj/bool/bool/null/other\n" {
		t.Fatalf("node oracle drifted: %q", want)
	}
	p := structuralUnionProgram(t, typeofChaseSource)
	t.Run("Go", func(t *testing.T) {
		if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
			t.Fatalf("Go %q Node %q", got, want)
		}
	})
	t.Run("ABAP", func(t *testing.T) { storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "") })
}
