package tsfront

import (
	"fmt"
	"strings"
	"testing"
)

// gogen's frontend-inventory reads `(obj as unknown as ArtifactObjectWithDef).def`
// where obj is an abaplint object: a class instance with a prototype. JS reads
// the property: a field the class declares (even privately) has its value, an
// optional field it does not declare is undefined. A structural view of such a
// source reads through the class layout and never traps on construction.
const prototypeViewSource = `interface Artifact {getName():string; syntaxResult?:unknown; definition?:unknown}
interface ArtifactWithDef extends Artifact {def?:unknown}
type ArtifactAndDef = Artifact & {def?:unknown};
interface DefView {def?:unknown; definition?:unknown; syntaxResult?:unknown}
abstract class AbstractObject {
 syntaxResult:{n:number}|undefined = undefined;
 protected name:string;
 constructor(name:string) {this.name=name;}
 getName():string {return this.name;}
}
class ClassObj extends AbstractObject {
 private def:{n:number}|undefined;
 constructor(name:string) {super(name);this.def={n:5};}
}
class IntfObj extends AbstractObject {
 definition:string|undefined = "idef";
}
function viaInterface(obj:Artifact):string {const d=(obj as unknown as ArtifactWithDef).def;return d===undefined ? "none" : "def";}
function viaIntersection(obj:Artifact):boolean {return Boolean(obj.syntaxResult || (obj as unknown as ArtifactAndDef).def || obj.definition);}
function viaData(obj:Artifact):string {const v=obj as unknown as DefView;return (v.def===undefined)+"/"+(v.definition===undefined)+"/"+(v.syntaxResult===undefined);}
class Probe {
 static run():string {
  const objs:Artifact[] = [new ClassObj("C"), new IntfObj("I")];
  let out = "";
  for (const o of objs) out += o.getName()+"="+viaInterface(o)+":"+viaIntersection(o)+":"+viaData(o)+";";
  const c = objs[0] as unknown as {def?:{n:number}};
  return out+(c.def===undefined ? -1 : c.def.n);
 }
 static writeUndeclared():void {const v=new IntfObj("W") as unknown as DefView;v.def=1;}
 static readMethod():boolean {const v=new IntfObj("M") as unknown as {getName?:unknown};return v.getName===undefined;}
 static viewed():string {const v=new IntfObj("V") as unknown as DefView;return String(v.definition);}
}`

func TestStep4PrototypeViewNodeOracle(t *testing.T) {
	want := conversionOracle(t, prototypeViewSource)
	p := structuralUnionProgram(t, prototypeViewSource)
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
		t.Fatalf("Go %q Node %q", got, want)
	}
	// JS would add an own property; a class layout cannot, so the write refuses.
	main := fmt.Sprintf(`defer func(){if _,ok:=recover().(trap);!ok {panic("undeclared write through a class view escaped")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "writeUndeclared"))
	if got := strings.TrimSpace(runStructuralUnionGo(t, p, main)); got != "refused" {
		t.Fatalf("got %q", got)
	}
	// JS reads the prototype function, which is not an absent property.
	main = fmt.Sprintf(`defer func(){if _,ok:=recover().(trap);!ok {panic("prototype function read as absent")};fmt.Println("refused")}();fmt.Println(%s())`, goEntry("probe.ts.Probe", "readMethod"))
	if got := strings.TrimSpace(runStructuralUnionGo(t, p, main)); got != "refused" {
		t.Fatalf("got %q", got)
	}
}

// ABAP has no property adapter on native class storage: a structural view of
// a class instance raises the explicit view refusal instead of reading.
func TestStep4PrototypeViewABAPRefuses(t *testing.T) {
	source := `interface DefView {def?:unknown; definition?:unknown}
class IntfObj {definition:string|undefined = "idef"; getName():string {return "I";}}
class Probe {
 static run():string {return "ok";}
 static viewed():string {const v=new IntfObj() as unknown as DefView;return String(v.definition);}
}`
	p := structuralUnionProgram(t, source)
	storageABAPProbe(t, p, "probe.ts.Probe", "ok", "viewed", "view.refusal")
}

// The same reads typed in place with JSDoc, like frontend-inventory.mjs.
func TestStep4PrototypeViewJSDocMjsNodeOracle(t *testing.T) {
	runMjsOracleTypes(t, `export interface Artifact {getName(): string; syntaxResult?: unknown; definition?: unknown}
export type ArtifactAndDef = Artifact & {def?: unknown};
export interface ArtifactWithDef extends Artifact {def?: unknown}
`, `class AbstractObject {
  /** @param {string} name */ constructor(name) { this.name = name; /** @type {unknown} */ this.syntaxResult = undefined; }
  /** @returns {string} */ getName() { return this.name; }
}
class ClassObj extends AbstractObject {
  /** @param {string} name */ constructor(name) { super(name); /** @type {{n: number} | undefined} */ this.def = {n: 5}; }
}
class IntfObj extends AbstractObject {
  /** @param {string} name */ constructor(name) { super(name); /** @type {string | undefined} */ this.definition = "idef"; }
}
/** @param {import("./types.ts").Artifact} obj @returns {boolean} */
const built = (obj) => Boolean(obj.syntaxResult || /** @type {import("./types.ts").ArtifactAndDef} */ (/** @type {unknown} */ (obj)).def || obj.definition);
/** @param {import("./types.ts").Artifact} obj @returns {boolean} */
const hasDef = (obj) => /** @type {import("./types.ts").ArtifactWithDef} */ (/** @type {unknown} */ (obj)).def !== undefined;
export class Probe {
  static run() {
    /** @type {import("./types.ts").Artifact[]} */ const objs = [new ClassObj("C"), new IntfObj("I")];
    return objs.map((o) => o.getName() + "=" + built(o) + ":" + hasDef(o)).join(";");
  }
}
`, false, true)
}
