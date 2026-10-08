package tsfront

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/abap"
)

func lowerStatementsProbe(t *testing.T, sources map[string]string, files []string) *hir.Program {
	t.Helper()
	dir := t.TempDir()
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"target":"ES2022"},"include":["*.ts"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	prog, diags, err := p.Lower(files)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if !strings.HasPrefix(d.Category, "note-") {
			t.Errorf("lower: %s", d)
		}
	}
	if errors := hir.Verify(prog); len(errors) != 0 {
		for _, err := range errors {
			t.Error(err)
		}
	}
	return prog
}

func TestStatementsOrdinaryConstructionIsNotRegExp(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Item { constructor(public a: string, public b: string) {} }
 export class Probe { run(): Item { return new Item("a", "b"); } }
 `}, []string{"probe.ts"})
	dump := hir.Dump(prog)
	if strings.Contains(dump, "regexp") {
		t.Fatal("ordinary class construction became RegExp")
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestStatementsNamespaceClassValue(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{
		"item.ts":  `export class Item {}`,
		"probe.ts": `import * as Items from "./item"; export class Probe { run(): typeof Items.Item { return Items.Item; } }`,
	}, []string{"item.ts", "probe.ts"})
	if !strings.Contains(hir.Dump(prog), "classof") {
		t.Fatal("missing namespace class descriptor")
	}
}

func TestStatementsForwardFunctionAndRest(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export function first(): number { return later("a", "b"); }
 function later(...args: string[]): number { return args.length; }
 `}, []string{"probe.ts"})
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	// Removing the temporary declaration must make the verifier reject the call.
	for _, c := range prog.Classes {
		for _, m := range c.Methods {
			if m.Name != "first" {
				continue
			}
			stmt := m.Body
			for stmt != nil && stmt.Kind == hir.Block {
				stmt = stmt.List[len(stmt.List)-1]
			}
			if stmt == nil {
				t.Fatal("missing function body")
			}
			x := stmt.X
			if x == nil || x.Kind != hir.Seq || len(x.Stmt.List) == 0 {
				t.Fatal("missing rest construction prelude")
			}
			x.Stmt.List = x.Stmt.List[1:]
			if len(hir.Verify(prog)) == 0 {
				t.Fatal("missing temporary declaration passed verification")
			}
		}
	}
}

func TestStatementsDefaultFunctionParameter(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export function first(): string { return later(); }
 function later(value: string = "default"): string { return value; }
 `}, []string{"probe.ts"})
	if !strings.Contains(hir.Dump(prog), "undefined") {
		t.Fatal("default parameter prologue disappeared")
	}
}

func TestStatementsNamespaceRegistry(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{
		"items.ts": `export class Second {} export class First {} class Private {}`,
		"probe.ts": `import * as Items from "./items"; export class Probe {
    keys(): string[] { return Object.keys(Items); }
    values(): (typeof Items.First)[] { return Object.values(Items); }
    count(): number { let count = 0; for (const key in Items) { count += key.length; } return count; }
  }`,
	}, []string{"items.ts", "probe.ts"})
	dump := hir.Dump(prog)
	if strings.Contains(dump, `"Private"`) {
		t.Fatal("included non-exported class")
	}
	if !strings.Contains(dump, "map.keys") || !strings.Contains(dump, "seq") {
		t.Fatal("missing registry iteration", dump)
	}
	for _, c := range prog.Classes {
		for _, m := range c.Methods {
			if m.Name == "class_constructor" && strings.HasSuffix(c.Name, "items.ts.module") {
				if m.Body.List[0].Kind != hir.Assign || m.Body.List[0].X.Kind != hir.StaticGet {
					t.Fatal("registry allocated into a local")
				}
			}
		}
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestStatementsErrorAndDescriptorConstruction(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Item {}
 export class Failure extends Error {}
 export class Probe {
  make(t: new () => Item): Item { return new t(); }
  fail(): void { try { throw new Error("message"); } catch (error) { throw error; } }
  empty(): void { throw new Failure(); }
 }`}, []string{"probe.ts"})
	if !strings.Contains(hir.Dump(prog), "classvalue.new") {
		t.Fatal("lost dynamic construction")
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}
func TestStatementsSpliceAndReverse(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe { run(a: string[]): string[] { a.splice(1); a.splice(0,1); a.splice(1,0,"x"); return a.reverse(); } }
 `}, []string{"probe.ts"})
	dump := hir.Dump(prog)
	for _, op := range []string{"array.splice1", "array.splice2", "array.splice3", "array.reverse"} {
		if !strings.Contains(dump, op) {
			t.Fatal("missing", op)
		}
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestStatementsNamespaceAliasConstruction(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{
		"items.ts": `export class Item {}`,
		"probe.ts": `import * as Items from "./items"; export class Probe { run(): number { const list: any = Items; let n=0; for (const key in Items) { if (typeof list[key] === "function") { const value = new list[key](); n += 1; } } return n; } }`,
	}, []string{"items.ts", "probe.ts"})
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestStatementsForwardDataInterface(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{
		"a.ts": `import {Data} from "./b"; export class Probe { make(): Data {return {name:"value"};} }`,
		"b.ts": `export interface Data { name: string; }`,
	}, []string{"a.ts", "b.ts"})
	for _, i := range prog.Interfaces {
		if strings.HasSuffix(i.Name, ".Data") {
			t.Fatal("data interface kept method-interface identity")
		}
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestStatementsSatisfiesTupleRegistry(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 interface Value { name: string; ordinal: number; }
 const rows = [["a", {ordinal:1}], ["b", {ordinal:2}]] as const satisfies readonly (readonly [string, {ordinal:number}])[];
 const values: {[name:string]:Value} = Object.fromEntries(rows.map(([name, row]) => [name, {name,ordinal:row.ordinal}]));
 export class Probe {run(): string {return values["b"].name;} }
 `}, []string{"probe.ts"})
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestStatementsShapeFieldTypeIdentity(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe {
 text(): {value:string} { return {value:"x"}; }
 number(): {value:number} { return {value:1}; }
 }`}, []string{"probe.ts"})
	var fields []hir.Type
	for _, c := range prog.Classes {
		if strings.HasPrefix(c.Name, "shape.") {
			fields = append(fields, c.Fields[0].Type)
		}
	}
	if len(fields) != 2 || fields[0].Equal(fields[1]) {
		t.Fatal("shapes with different field types share an identity")
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestStatementsTupleSpreadAndRestCopy(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export function take(first: string, second: number, ...rest: string[]): number {return second + rest.length;}
 export function run(): number {const args = ["x",2,"y"] as const; return take(...args);}
 export function copy(...rest: string[]): number {rest.push("y"); return rest.length;}
 export function spread(a: string[]): number {return copy(...a);}
 `}, []string{"probe.ts"})
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
	for _, c := range prog.Classes {
		for _, m := range c.Methods {
			if m.Name == "spread" {
				stmt := m.Body
				for stmt.Kind == hir.Block {
					stmt = stmt.List[len(stmt.List)-1]
				}
				if stmt.X.Kind != hir.Seq {
					t.Fatal("rest spread aliases the caller's array")
				}
			}
		}
	}
}

func TestStatementsArrayRestDestructuring(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe { run(values: string[]): string[] { const [first,second,...rest] = values; rest.push(first); return rest; } }
 `}, []string{"probe.ts"})
	if !strings.Contains(hir.Dump(prog), "array.slice1") {
		t.Fatal("rest binding must copy a slice")
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestStatementsDescriptorSet(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{
		"a.ts":     `export class Item {}`,
		"b.ts":     `export class Item {}`,
		"probe.ts": `import {Item as A} from "./a"; import {Item as B} from "./b"; export class Probe {static run(): number {const set=new Set<typeof A>();set.add(A);set.add(A);set.add(B);return set.size;} }`,
	}, []string{"a.ts", "b.ts", "probe.ts"})
	files, err := abap.Emit(prog)
	if err != nil {
		t.Fatal(err)
	}
	names := hir.NewNames()
	class := names.Get("probe.ts.Probe")
	files[class+".clas.testclasses.abap"] = "CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\nMETHOD check.\nDATA actual TYPE i.\nactual = " + class + "=>" + names.Get("member.run") + "( ).\ncl_abap_unit_assert=>assert_equals( act = actual exp = 2 ).\nENDMETHOD.\nENDCLASS.\n"
	if out := os.Getenv("STMTS_PROBE_OUT"); out != "" {
		dir := filepath.Join(out, t.Name())
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, source := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Scalar kinds without supported key semantics remain rejected.
	for _, c := range prog.Classes {
		for _, m := range c.Methods {
			if m.Name == "run" {
				m.Body.List[0].Type = hir.T(hir.OrderedSet, hir.T(hir.Number))
			}
		}
	}
	if len(hir.Verify(prog)) == 0 {
		t.Fatal("unsupported number key passed verification")
	}
}

func TestStatementsOptionalArgumentsAndStaticThis(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 interface Options {mode?:string;}
 export class Probe {
 static value: number = 0;
 static update(): number {const add=(n:number)=>n+1; this.value = add(1); return this.read();}
 static read(): number {return this.value;}
 take(opts: Options = {}): string {return opts.mode ?? "default";}
 run(): string {return this.take({mode:"x"});}
 }`}, []string{"probe.ts"})
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestStatementsReduceAccumulatorBinding(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
export class Probe {static run(): string[] {const values=["x","y"];return values.reduce((a,c)=>a.concat([c]), [] as string[]);}}
`}, []string{"probe.ts"})
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestStatementsGenericErasedBridges(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 interface Item { value(): string; }
 export class Leaf implements Item { value(): string { return "leaf"; } }
 export abstract class Base<T extends Item> {
   protected children: T[] = [];
   add(x: T): void { this.children.push(x); }
   getChildren(): T[] { return this.children; }
   abstract get(): object;
 }
 export class Node extends Base<Leaf> {
   add(x: Leaf): void { super.add(x); }
   get(): Leaf { return new Leaf(); }
   run(): string { return this.getChildren()[0].value(); }
 }
 `}, []string{"probe.ts"})
	files, err := abap.Emit(prog)
	if err != nil {
		t.Fatal(err)
	}
	assertSuperSameMethod(t, files)
	dump := hir.Dump(prog)
	if !strings.Contains(dump, "instantiated") || !strings.Contains(dump, "narrow") {
		t.Fatal("missing bridge or use-site narrowing", dump)
	}
}

func TestStatementsUnionReceiverInterface(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class A { common(): number { return 1; } onlyA(): string { return "a"; } private hidden(): number { return 3; } }
 export class B { common(): number { return 2; } onlyB(): string { return "b"; } private hidden(): number { return 4; } }
 export class Probe {
   run(x: A | B): number { return x.common(); }
   again(x: B | A): number { return x.common(); }
 }
 `}, []string{"probe.ts"})
	count := 0
	for _, i := range prog.Interfaces {
		if strings.HasPrefix(i.Name, "union.") {
			count++
			if len(i.Methods) != 1 || i.Methods[0].Name != "common" {
				t.Fatalf("wrong common members: %v", i.Methods)
			}
		}
	}
	if count != 1 {
		t.Fatalf("union not deduplicated: %d", count)
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestStatementsCapturedLiftedCallee(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Probe { run(): number {
  const value = 7;
  const first = (n: number): number => value + n;
  const second = (n: number): number => first(n) + first(n);
  return second(2);
 } }
 `}, []string{"probe.ts"})
	for _, c := range prog.Classes {
		for _, m := range c.Methods {
			if strings.HasPrefix(m.Name, "fn_") {
				for _, p := range m.Params {
					if p.Type.Kind == hir.Void || p.Name == "first" {
						t.Fatal("captured known function as a value")
					}
				}
			}
		}
	}
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestStatementsOptionalAccessAndTupleRest(t *testing.T) {
	prog := lowerStatementsProbe(t, map[string]string{"probe.ts": `
 export class Item { value(): string { return "value"; } }
 function join(...parts: (string | typeof Item)[]): number { return parts.length; }
 export class Probe {
  run(item?: Item): string { const text = item?.value(); if (text === undefined) { return "absent"; } return text; }
  count(): number { const parts = ["a", Item] as const; return join(...parts); }
 }
 `}, []string{"probe.ts"})
	if _, err := abap.Emit(prog); err != nil {
		t.Fatal(err)
	}
}

func TestStatementsEnumNamespaceInitializesStatic(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"enum.ts": `export enum E {One="one", Two="two"} export class Probe {static run():number {return Object.values(E).length;}}`}, []string{"enum.ts"})
	found := false
	for _, c := range p.Classes {
		for _, m := range c.Methods {
			if m.Name != "class_constructor" {
				continue
			}
			for _, s := range m.Body.List {
				if s.Kind == hir.Assign && s.X.Kind == hir.StaticGet && s.X.Name == "E_namespace" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("enum namespace was never assigned to its static field")
	}
}

func TestStatementsConstructorDefaults(t *testing.T) {
	p := lowerStatementsProbe(t, map[string]string{"ctor.ts": `export class Probe {private n:number; constructor(n:number=3){this.n=n;} run():number{return this.n;}}`}, []string{"ctor.ts"})
	found := false
	for _, c := range p.Classes {
		if c.Ctor != nil {
			found = true
			if c.Ctor.Body.List[0].Kind != hir.If {
				t.Fatal("constructor omitted its default prologue")
			}
		}
	}
	if !found {
		t.Fatal("missing constructor")
	}
	files, err := abap.Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	optional := false
	for _, source := range files {
		if strings.Contains(source, "METHODS constructor IMPORTING") && strings.Contains(source, " OPTIONAL.") {
			optional = true
		}
	}
	if !optional {
		t.Fatal("defaulted constructor parameter is mandatory in ABAP")
	}
}
