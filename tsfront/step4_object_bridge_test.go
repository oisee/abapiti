package tsfront

import (
	"fmt"
	"github.com/oisee/abapiti/hir"
	"os/exec"
	"strings"
	"testing"
)

const objectBridgeSource = `interface Statement {readonly s:string;value:string}
interface Expression {readonly e:string;name:string}
interface Wrapper {readonly child?:Value;readonly nested?:readonly Value[]}
type GraphNode=Statement|Expression|Wrapper;
type Value=GraphNode|readonly Value[]|string|number|boolean|null|undefined;
function isStmt(node:object):node is Statement {Probe.calls++;return "s" in node && typeof (node as any).s==="string";}
function isExpr(node:object):node is Expression {Probe.calls++;return "e" in node && typeof (node as any).e==="string";}
function children(node:object):readonly Value[] {const out:Value[]=[];for(const key of Object.keys(node))if(key!=="type"&&key!=="pos")out.push((node as any)[key]);return out;}
function isArray(value:Value):value is readonly Value[]{return Array.isArray(value);}
function walk(value:Value):string {
 if(isArray(value)){let out="";for(const child of value)out+=walk(child);return out;}
 if(!value||typeof value!=="object")return "";
 if(isStmt(value))return value.value+";";
 if(isExpr(value))return value.name+";";
 let out="";for(const child of children(value))out+=walk(child);return out;
}
function identity(value:object):object{return value;}
class Probe {static calls=0;static inputs=0;static input():any{Probe.inputs++;return JSON.parse('{"z":{"e":"name","name":"first"},"a":[{"s":"write","value":"second"},null,false,0,"",{"child":{"e":"name","name":"third"}}],"type":{"s":"skip","value":"bad"},"pos":17}');}
 static run():string {const raw=Probe.input();const object=identity(raw);const same=object===raw;raw.z.name="changed";const result=walk(object as Value);return same+"|"+result+"|"+Object.keys(object).join(",")+"|"+Probe.calls+":"+Probe.inputs;}
}`

func TestStep4ObjectBridgeNodeOracle(t *testing.T) {
	want, err := exec.Command("node", "--disable-warning=ExperimentalWarning", "-e", `eval(require("node:module").stripTypeScriptTypes(process.argv[1])+"\nconsole.log(Probe.run())")`, objectBridgeSource).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(want))
	}
	p := structuralUnionProgram(t, objectBridgeSource)
	t.Run("Go", func(t *testing.T) {
		if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != string(want) {
			t.Fatalf("Go %q Node %q", got, want)
		}
	})
	t.Run("ABAP", func(t *testing.T) { storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(string(want)), "") })
}

const objectIdentitySource = `function keep(value:object):object{return value;}
class Probe {static run():string {const raw=JSON.parse('{"x":1}');const value:unknown=raw;if(value===null||typeof value!=="object")return "bad";const kept=keep(value);return (kept===raw)+":"+(kept===keep(value));}}`

func TestStep4ObjectBridgeIdentityWithoutProperties(t *testing.T) {
	p := structuralUnionProgram(t, objectIdentitySource)
	t.Run("Go", func(t *testing.T) {
		if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != "true:true\n" {
			t.Fatal(got)
		}
	})
	t.Run("ABAP", func(t *testing.T) { storageABAPProbe(t, p, "probe.ts.Probe", "true:true", "") })
}

const objectArraySource = `function keep(value:object):object{return value;}
function inspect(value:object):string{return ("length" in value)+":"+("map" in value)+":"+("0" in value)+":"+Object.keys(value).join(",");}
class Probe {static run():string {const raw=JSON.parse('["a","b"]');const root=keep(raw);raw[0]="changed";raw[4]=undefined;const boxed:unknown=root;return (root===raw)+"|"+inspect(root)+"|"+(boxed as any)[0]+":"+((boxed as any)[4]===undefined);}}`

func TestStep4ObjectBridgeArrays(t *testing.T) {
	for _, native := range []bool{false, true} {
		name := "parsed"
		source := objectArraySource
		if native {
			name = "native"
			source = strings.Replace(source, `const raw=JSON.parse('["a","b"]');`, `const values:any[]=["a","b"];const tagged:unknown=values;const raw:any=tagged;`, 1)
		}
		t.Run(name, func(t *testing.T) {
			want := conversionOracle(t, source)
			p := structuralUnionProgram(t, source)
			t.Run("Go", func(t *testing.T) {
				if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
					t.Fatalf("Go %q Node %q", got, want)
				}
			})
			t.Run("ABAP", func(t *testing.T) { storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "") })
		})
	}
}

const objectConstructorSource = `class Constructor {}
function keep(value:object):object{return value;}
class Probe {static run():string {const first=keep(Constructor as unknown as object);const second=keep(Constructor as unknown as object);const boxed:unknown=first;return (first===second)+":"+(first===keep(Constructor as unknown as object))+":"+typeof boxed;}}`

func TestStep4ObjectBridgeConstructorIdentity(t *testing.T) {
	want := conversionOracle(t, objectConstructorSource)
	p := structuralUnionProgram(t, objectConstructorSource)
	t.Run("Go", func(t *testing.T) {
		if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
			t.Fatalf("Go %q Node %q", got, want)
		}
	})
	t.Run("ABAP", func(t *testing.T) { storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "") })
}

func TestStep4ObjectBridgeRejectsNonobjects(t *testing.T) {
	for _, tc := range []struct{ name, value string }{{"undefined", "undefined"}, {"null", "null"}, {"number", "17"}, {"boolean", "false"}, {"string", `"text"`}} {
		t.Run(tc.name, func(t *testing.T) {
			source := `function keep(value:object):object{return value;} function erase(value:any):object{return keep(value);} class Probe {static run():string{return "ok";} static invalid():void {const value:any=` + tc.value + `;erase(value);}}`
			p := structuralUnionProgram(t, source)
			main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok{panic("nonobject must refuse")};fmt.Println("refused")}();%s()`, goEntry("probe.ts.Probe", "invalid"))
			if got := runStructuralUnionGo(t, p, main); got != "refused\n" {
				t.Fatal(got)
			}
			storageABAPProbe(t, p, "probe.ts.Probe", "ok", "invalid", "cx_sy_move_cast_error")
		})
	}
}

func TestStep4ObjectBridgePropertyABI(t *testing.T) {
	dyn, str, boolean, root := hir.T(hir.Dynamic), hir.T(hir.String), hir.T(hir.Bool), hir.Ref(hir.RootObject)
	rt := func(op string, x *hir.Expr, typ hir.Type, args ...*hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.RuntimeOp, Op: op, X: x, Type: typ, Args: args}
	}
	lit := func(s string) *hir.Expr { return hir.L(str, s) }
	local := func(n string, t hir.Type) *hir.Expr { return hir.V(n, t) }
	decl := func(n string, t hir.Type, x *hir.Expr) *hir.Stmt {
		return &hir.Stmt{Kind: hir.VarDecl, Name: n, Type: t, X: x}
	}
	ret := func(s string) *hir.Stmt { return &hir.Stmt{Kind: hir.Return, X: lit(s)} }
	raw, obj := local("raw", dyn), local("obj", root)
	array, arr := local("array", dyn), local("arr", root)
	get := func(x *hir.Expr, key string) *hir.Expr { return rt("object.get", x, dyn, lit(key)) }
	has := func(x *hir.Expr, key string) *hir.Expr { return rt("object.hasOwn", x, boolean, lit(key)) }
	eq := func(x, y *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Op: "==", Type: boolean, X: x, Y: y}
	}
	body := hir.B(decl("raw", dyn, rt("json.parseSubset", lit(`{"value":"before","nil":null}`), dyn)), decl("obj", root, rt("dynamic.asObject", raw, root)))
	check := func(x *hir.Expr) {
		body.List = append(body.List, &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: boolean, X: x}, Body: ret("bad")})
	}
	check(eq(rt("dynamic.of", obj, dyn), raw))
	body.List = append(body.List, &hir.Stmt{Kind: hir.ExprStmt, X: rt("dynamic.put", raw, hir.T(hir.Void), lit("value"), rt("dynamic.of", lit("after"), dyn))}, &hir.Stmt{Kind: hir.ExprStmt, X: rt("dynamic.put", raw, hir.T(hir.Void), lit("undef"), rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), dyn))})
	check(has(obj, "undef"))
	check(eq(get(obj, "undef"), rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), dyn)))
	check(eq(get(obj, "nil"), rt("dynamic.null", hir.L(hir.T(hir.I32), 0), dyn)))
	body.List = append(body.List, decl("array", dyn, rt("json.parseSubset", lit(`["a","b"]`), dyn)), decl("arr", root, rt("dynamic.asObject", array, root)), &hir.Stmt{Kind: hir.ExprStmt, X: rt("dynamic.put", array, hir.T(hir.Void), lit("4"), rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), dyn))})
	check(has(arr, "length"))
	check(has(arr, "4"))
	check(eq(get(arr, "4"), rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), dyn)))
	check(eq(get(arr, "missing"), rt("dynamic.of", hir.L(hir.T(hir.Optional, str), nil), dyn)))
	check(&hir.Expr{Kind: hir.Unary, Op: "!", Type: boolean, X: has(arr, "2")})
	check(eq(rt("dynamic.asNumber", get(arr, "length"), hir.T(hir.Number)), hir.L(hir.T(hir.Number), 5)))
	body.List = append(body.List, &hir.Stmt{Kind: hir.Return, X: rt("dynamic.asString", get(obj, "value"), str)})
	p := &hir.Program{Classes: []*hir.Class{{Name: "Probe", Methods: []*hir.Method{{Name: "run", Static: true, Result: str, Body: body}}}}}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("Probe", "run"))); got != "after\n" {
		t.Fatal(got)
	}
	storageABAPProbe(t, p, "Probe", "after", "")
}

func TestStep4ObjectBridgeNominalBoundary(t *testing.T) {
	dyn, str, root, nom := hir.T(hir.Dynamic), hir.T(hir.String), hir.Ref(hir.RootObject), hir.Ref("Nominal")
	rt := func(op string, x *hir.Expr, typ hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.RuntimeOp, Op: op, X: x, Type: typ}
	}
	bridge := func(d *hir.Expr) *hir.Expr {
		return rt("dynamic.asRef", rt("dynamic.of", rt("dynamic.asObject", d, root), dyn), nom)
	}
	run := &hir.Method{Name: "run", Static: true, Result: str, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.VirtualCall, Name: "value", Type: str, X: bridge(rt("dynamic.of", &hir.Expr{Kind: hir.New, Type: nom}, dyn))}})}
	invalid := &hir.Method{Name: "invalid", Static: true, Result: str, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.VirtualCall, Name: "value", Type: str, X: bridge(rt("json.parseSubset", hir.L(str, `{"brand":1}`), dyn))}})}
	value := &hir.Method{Name: "value", Virtual: true, Result: str, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: hir.L(str, "native")})}
	p := &hir.Program{Classes: []*hir.Class{{Name: "Nominal", Fields: []hir.Field{{Name: "brand", Type: hir.T(hir.Number), Private: true}}, Methods: []*hir.Method{value}}, {Name: "Probe", Methods: []*hir.Method{run, invalid}}}}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("Probe", "run"))); got != "native\n" {
		t.Fatal(got)
	}
	main := fmt.Sprintf(`defer func(){if _,ok:=recover().(rangeFault);!ok {panic("nominal boundary must refuse parsed record")};fmt.Println("refused")}();%s()`, goEntry("Probe", "invalid"))
	if got := runStructuralUnionGo(t, p, main); got != "refused\n" {
		t.Fatal(got)
	}
	storageABAPProbe(t, p, "Probe", "native", "invalid", "cx_sy_move_cast_error")
}

// Matches the independently retained CLI oracle: a broad object predicate
// asserts an anonymous public shape before reading its property.
const objectShapePredicateSource = `interface Stmt {s:string}
function isStmt(n:object):n is Stmt {Probe.calls++;return "s" in n && typeof (n as {s:unknown}).s==="string";}
function read(n:unknown):string {if(!n || typeof n!=="object") return "primitive";if(isStmt(n))return n.s;return Object.keys(n).join(",");}
class Probe {static calls=0;static run():string{const raw=JSON.parse('{"s":"parsed"}');const before=read(raw);raw.s="live";return before+":"+read(raw)+":"+read({s:"native"})+":"+read(null)+":"+read(17)+":"+read({wrapper:true})+":"+Probe.calls;}}`

func TestStep4ObjectBridgeShapePredicate(t *testing.T) {
	for _, flow := range []bool{false, true} {
		name := "asserted"
		source := objectShapePredicateSource
		if flow {
			name = "flow"
			source = strings.Replace(source, `(n as {s:unknown}).s`, `n.s`, 1)
		}
		t.Run(name, func(t *testing.T) {
			want := conversionOracle(t, source)
			p := structuralUnionProgram(t, source)
			t.Run("Go", func(t *testing.T) {
				if got := runStructuralUnionGo(t, p, fmt.Sprintf("fmt.Println(%s())", goEntry("probe.ts.Probe", "run"))); got != want {
					t.Fatalf("Go %q Node %q", got, want)
				}
			})
			t.Run("ABAP", func(t *testing.T) { storageABAPProbe(t, p, "probe.ts.Probe", strings.TrimSpace(want), "") })
		})
	}
}
