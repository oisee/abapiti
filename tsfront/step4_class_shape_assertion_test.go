package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

// gogen's nativeRttiData asserts a looked-up class value (T | undefined) to a
// compatible object-literal shape and reads nested fields through it:
// `(attr(sd, "MT_REFS") as {type: {row: StructType}}).type.row.go`, then
// `(STRUCTDEFS.get(row) as {fields: readonly Field[]}).fields`. Strict tsc
// accepts both (TS2352 fires only for incompatible types) and JS erases them.
// The assertion is a structural view over the same storage, never a nominal
// downcast to the shape's projection layout.
const classShapeAssertionSource = `interface Field {readonly name:string; readonly type:string}
interface StructType {readonly k:"struct"; readonly go:string; readonly fields:readonly Field[]}
interface TableType {readonly k:"table"; readonly row:StructType}
class Attribute {
 label="a";
 readonly name:string; readonly type:TableType|StructType; readonly unsupported?:boolean;
 constructor(name:string, type:TableType|StructType, unsupported?:boolean) {this.name=name;this.type=type;this.unsupported=unsupported;}
}
class Holder {
 count=1;
 item:Attribute;
 constructor(item:Attribute) {this.item=item;}
}
function attr(list:Attribute[], n:string):Attribute|undefined {return list.find((a) => a.name===n && !a.unsupported);}
class Probe {
 static run():string {
  const row:StructType = {k:"struct", go:"RowT", fields:[{name:"ID", type:"i"}, {name:"TYPE", type:"ref"}]};
  const list = [new Attribute("MT_REFS", {k:"table", row}), new Attribute("X", row, true)];
  const defs = new Map<string, StructType>([["RowT", row]]);
  const go = (attr(list, "MT_REFS") as {type:{row:StructType}}).type.row.go;
  const kind = (attr(list, "MT_REFS") as {type:{row:StructType}}).type.row.k;
  const ftype = (defs.get(go) as {fields:readonly Field[]}).fields.find((f) => f.name==="TYPE")!.type;
  const a = list[0];
  const view = a as {name:string; label:string};
  const same = (view as unknown)===a;
  const h = new Holder(a);
  const hv = h as {count:number; item:{name:string}};
  hv.count = hv.count + 41;
  return go+":"+kind+":"+ftype+":"+view.name+":"+view.label+":"+same+":"+h.count+":"+hv.item.name;
 }
}`

func TestStep4ClassShapeAssertionNodeOracle(t *testing.T) {
	want := conversionOracle(t, classShapeAssertionSource)
	p := structuralUnionProgram(t, classShapeAssertionSource)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
}

// The same reads typed in place with JSDoc, exactly like emit-go.mjs.
func TestStep4ClassShapeAssertionJSDocMjsNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Field {readonly name:string; readonly type:string}
export interface StructType {readonly k:"struct"; readonly go:string; readonly fields:readonly Field[]}
export interface TableType {readonly k:"table"; readonly row:StructType}
`, `export class Attribute {
  /** @param {string} name @param {import("./types.ts").TableType | import("./types.ts").StructType} type */
  constructor(name, type) { this.name = name; this.type = type; this.label = "a"; }
}
/** @param {Attribute[]} list @param {string} n @returns {Attribute | undefined} */
const attr = (list, n) => list.find((a) => a.name === n);
export class Probe {
  static run() {
    /** @type {import("./types.ts").StructType} */ const row = {k: "struct", go: "RowT", fields: [{name: "ID", type: "i"}, {name: "TYPE", type: "ref"}]};
    const list = [new Attribute("MT_REFS", {k: "table", row})];
    /** @type {Map<string, import("./types.ts").StructType>} */ const defs = new Map([["RowT", row]]);
    const go = (/** @type {{type: {row: import("./types.ts").StructType}}} */ (attr(list, "MT_REFS"))).type.row.go;
    const ftype = (/** @type {import("./types.ts").Field} */ ((/** @type {{fields: readonly import("./types.ts").Field[]}} */ (defs.get(go))).fields.find((f) => f.name === "TYPE"))).type;
    return go + ":" + ftype;
  }
}
`, false)
}

// An incompatible assertion is still rejected by the checker, and an
// assertion whose runtime content is false keeps refusing at the read.
func TestStep4ClassShapeAssertionRefusesWrongContent(t *testing.T) {
	source := strings.Replace(classShapeAssertionSource, " static run():string {", ` static bad():string {
  const list = [new Attribute("S", {k:"struct", go:"G", fields:[]})];
  return (attr(list, "S") as {type:{row:StructType}}).type.row.go;
 }
 static run():string {`, 1)
	p := structuralUnionProgram(t, source)
	main := fmt.Sprintf(`defer func(){r:=recover();if r==nil {panic("false shape assertion escaped")};fmt.Println("refused")}();fmt.Println(%s())`, goEntry("probe.ts.Probe", "bad"))
	if got := strings.TrimSpace(runStructuralUnionGo(t, p, main)); got != "refused" {
		t.Fatalf("got %q", got)
	}
}

// ABAP has no property adapter on native class storage: the shape view of a
// class value raises the explicit view refusal instead of reading.
func TestStep4ClassShapeAssertionABAPRefuses(t *testing.T) {
	source := strings.Replace(classShapeAssertionSource, " static run():string {", " static run():string {return \"ok\";}\n static viewed():string {", 1)
	p := structuralUnionProgram(t, source)
	storageABAPProbe(t, p, "probe.ts.Probe", "ok", "viewed", "view.refusal")
}
