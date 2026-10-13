package abap

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func chaiFixture() *hir.Program {
	d := hir.T(hir.Dynamic)
	methods := []*hir.Method{}
	add := func(name string, result hir.Type, params []hir.Param, expr *hir.Expr) {
		m := method(name, result, hir.B(ret(expr)))
		m.Static, m.Params = true, params
		methods = append(methods, m)
	}
	for _, op := range []string{"chai.deepEqual", "chai.deepInclude", "chai.arrayInclude"} {
		result := boolean
		if op == "chai.deepInclude" {
			result = i32
		}
		add(strings.TrimPrefix(op, "chai."), result, []hir.Param{{Name: "a", Type: d}, {Name: "b", Type: d}}, rt(op, local("a", d), result, local("b", d)))
	}
	add("members", boolean, []hir.Param{{Name: "a", Type: d}, {Name: "b", Type: d}, {Name: "deep", Type: boolean}}, rt("chai.members", local("a", d), boolean, local("b", d), local("deep", boolean)))
	add("throwMatch", boolean, []hir.Param{{Name: "a", Type: d}, {Name: "b", Type: str}}, rt("chai.throwMatch", local("a", d), boolean, local("b", str)))
	for _, t := range []hir.Type{str, i32, boolean, hir.T(hir.Number), hir.T(hir.Array, str), hir.T(hir.Array, d)} {
		add("box"+t.String(), d, []hir.Param{{Name: "a", Type: t}}, rt("chai.box", local("a", t), d))
	}
	record := hir.T(hir.OrderedMap, str, d)
	for _, name := range []string{"record", "nativeMap"} {
		obj := newObj(record)
		if name == "record" {
			keys := []string{}
			obj.OwnKeys = &keys
		}
		m := method(name, d, hir.B(decl("map", record, obj), run(rt("map.set", local("map", record), record, hir.L(str, "x"), local("a", d))), ret(rt("chai.box", local("map", record), d))))
		m.Static, m.Params = true, []hir.Param{{Name: "a", Type: d}}
		methods = append(methods, m)
	}
	add("positiveZero", d, nil, rt("chai.box", hir.L(hir.T(hir.Number), float64(0)), d))
	add("negativeZero", d, nil, rt("chai.box", hir.L(hir.T(hir.Number), math.Copysign(0, -1)), d))
	add("parse", d, []hir.Param{{Name: "a", Type: str}}, rt("json.parseSubset", local("a", str), d))
	thunk := hir.Type{Kind: hir.InterfaceRef, Name: "closure.thunk"}
	settled := hir.T(hir.Promise, hir.T(hir.Void))
	add("typedThrowMatch", boolean, []hir.Param{{Name: "a", Type: thunk}, {Name: "b", Type: str}}, rt("chai.throwMatch", local("a", thunk), boolean, local("b", str)))
	thrownThunk := method("call", settled, hir.B(ret(&hir.Expr{Kind: hir.PromiseReject, Type: settled, X: hir.L(str, "prefix BOOM suffix")})))
	thrownThunk.Virtual = true
	quietThunk := method("call", settled, hir.B(ret(&hir.Expr{Kind: hir.PromiseFulfill, Type: settled})))
	quietThunk.Virtual = true
	callable := "closure.function.chai"
	action := method("call", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Throw, X: hir.L(str, "prefix BOOM suffix")}))
	action.Virtual = true
	errorValue := newObj(hir.Ref("builtin.Error"))
	errorValue.Args = []*hir.Expr{hir.L(str, "prefix BOOM suffix")}
	errorAction := method("call", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Throw, X: errorValue}))
	errorAction.Virtual = true
	quiet := method("call", hir.T(hir.Void), hir.B(ret(nil)))
	quiet.Virtual = true
	refusal := method("call", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Trap, Name: "chai action refusal probe"}))
	refusal.Virtual = true
	nominal := hir.Ref("ChaiNominal")
	add("boxNominal", d, []hir.Param{{Name: "a", Type: nominal}}, rt("chai.box", local("a", nominal), d))
	dataCtor := func(name string) *hir.Method {
		ctor := method("constructor", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(name)}, Name: "x", Type: str}, Y: local("x", str)}))
		ctor.Params = []hir.Param{{Name: "x", Type: str}}
		return ctor
	}
	for _, name := range []string{"ChaiData", "ChaiData2"} {
		add("box"+name, d, []hir.Param{{Name: "a", Type: hir.Ref(name)}}, rt("chai.box", local("a", hir.Ref(name)), d))
	}
	errorCtor := method("constructor", hir.T(hir.Void), hir.B())
	errorCtor.Params = []hir.Param{{Name: "message", Type: hir.T(hir.Optional, str)}}
	return &hir.Program{Interfaces: []*hir.Interface{{Name: callable, Methods: []*hir.Method{{Name: "call", Result: hir.T(hir.Void)}}}, {Name: "closure.thunk", Methods: []*hir.Method{{Name: "call", Result: settled}}}}, Classes: []*hir.Class{
		{Name: "ChaiProbe", Methods: methods},
		{Name: "builtin.Error", Fields: []hir.Field{{Name: "message", Type: str}}, Ctor: errorCtor},
		{Name: "ChaiNominal", Fields: []hir.Field{{Name: "x", Type: str}}},
		{Name: "ChaiData", Fields: []hir.Field{{Name: "x", Type: str}}, Ctor: dataCtor("ChaiData")},
		{Name: "ChaiData2", Fields: []hir.Field{{Name: "x", Type: str}}, Ctor: dataCtor("ChaiData2")},
		{Name: "closure.chaiAction", Implements: []string{callable}, Methods: []*hir.Method{action}},
		{Name: "closure.chaiError", Implements: []string{callable}, Methods: []*hir.Method{errorAction}},
		{Name: "closure.chaiQuiet", Implements: []string{callable}, Methods: []*hir.Method{quiet}},
		{Name: "closure.chaiRefusal", Implements: []string{callable}, Methods: []*hir.Method{refusal}},
		{Name: "ChaiTypedThrow", Implements: []string{"closure.thunk"}, Methods: []*hir.Method{thrownThunk}},
		{Name: "ChaiTypedQuiet", Implements: []string{"closure.thunk"}, Methods: []*hir.Method{quietThunk}},
	}}
}

// Every answer below is obtained from real Chai 4, not from an implementation
// of deep-eql in the test. The same vector is asserted by the emitted ABAP unit.
func TestChaiFixtures(t *testing.T) {
	root := os.Getenv("ABAPITI_TESTS_UPSTREAM")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		root = filepath.Join(home, "dev", "abaplint")
	}
	chai := filepath.Join(root, "node_modules", "chai")
	if _, err := os.Stat(chai); err != nil {
		chai = filepath.Join(root, "packages", "core", "node_modules", "chai")
	}
	if _, err := os.Stat(chai); err != nil {
		t.Skip("real Chai oracle not installed")
	}
	script := `const {expect}=require(process.argv[1]);if(!require(process.argv[1]+"/package.json").version.startsWith("4."))throw Error("Chai 4 required");
 const ok=f=>{try{f();return true}catch(e){if(e.name!=="AssertionError")throw e;return false}};
 const u=undefined,a={x:u},b={x:u},cycle={},cycle2={};cycle.self=cycle;cycle2.self=cycle2;
 console.log(JSON.stringify([
 ok(()=>expect(1).eql(1)),ok(()=>expect(1).eql("1")),ok(()=>expect(u).eql(u)),ok(()=>expect(null).eql(u)),
 ok(()=>expect(a).eql(b)),ok(()=>expect(a).eql({})),ok(()=>expect(["x","y"]).eql(["y","x"])),ok(()=>expect(cycle).eql(cycle2)),
 ok(()=>expect([a]).include(a)),ok(()=>expect([a]).include(b)),ok(()=>expect([a]).deep.include(b)),
 ok(()=>expect(["x","x","y"]).members(["y","x","x"])),ok(()=>expect(["x","x","y"]).members(["y","y","x"])),
 ok(()=>expect([a]).members([b])),ok(()=>expect([a]).deep.members([b])),
 ok(()=>expect("BOOM").deep.include("boom")),ok(()=>expect("BOOM").deep.include("")),
 ok(()=>expect(a).deep.include(b)),ok(()=>expect({}).deep.include(b)),
 ok(()=>expect(()=>{throw "prefix BOOM suffix"}).throw("BOOM")),ok(()=>expect(()=>{throw "prefix BOOM suffix"}).throw("boom")),ok(()=>expect(()=>{}).throw("")),ok(()=>expect(()=>{throw "prefix BOOM suffix"}).throw("") ),
 ok(()=>expect(()=>{throw new Error("prefix BOOM suffix")}).throw("BOOM")),ok(()=>expect(()=>{throw new Error("prefix BOOM suffix")}).throw("boom")),ok(()=>expect(()=>{throw new Error("prefix BOOM suffix")}).throw(""))]));`
	oracle, err := exec.Command("node", "-e", script, chai).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(oracle))
	}
	var wants []bool
	if err = json.Unmarshal(oracle, &wants); err != nil {
		t.Fatal(err, string(oracle))
	}
	files, names, err := emitABAPNamed(t, chaiFixture())
	if err != nil {
		t.Fatal(err)
	}
	cls := names.Get("ChaiProbe")
	d := names.Get("runtime.dynamic")
	runtime := names.Get("runtime.chai")
	var unit strings.Builder
	fmt.Fprintf(&unit, "CLASS ltcl_test DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_test IMPLEMENTATION.\nMETHOD check.\nDATA one TYPE REF TO %s.\nDATA text TYPE REF TO %s.\nDATA undef TYPE REF TO %s.\nDATA null TYPE REF TO %s.\nDATA a TYPE REF TO %s.\nDATA b TYPE REF TO %s.\nDATA empty TYPE REF TO %s.\nDATA x TYPE REF TO %s.\nDATA y TYPE REF TO %s.\nDATA ax TYPE REF TO %s.\nDATA bx TYPE REF TO %s.\nDATA words TYPE REF TO %s.\nDATA shuffled TYPE REF TO %s.\nDATA duplicates TYPE REF TO %s.\nDATA action TYPE REF TO %s.\nDATA quiet TYPE REF TO %s.\nDATA blocked TYPE REF TO %s.\nDATA cycle TYPE REF TO %s.\nDATA cycle2 TYPE REF TO %s.\n", d, d, d, d, d, d, d, d, d, d, d, d, d, d, d, d, d, d, d)
	fmt.Fprintf(&unit, "one = %s=>%s( 1 ).\ntext = %s=>%s( `1` ).\nnull = NEW #( ).\nnull->tag = %s=>tag_null.\na = NEW #( ).\na->tag = %s=>tag_object.\na->put( p0 = `x` p1 = undef ).\nb = NEW #( ).\nb->tag = %s=>tag_object.\nb->put( p0 = `x` p1 = undef ).\nempty = NEW #( ).\nempty->tag = %s=>tag_object.\nx = %s=>%s( `x` ).\ny = %s=>%s( `y` ).\n", cls, names.Get("member.boxi32"), cls, names.Get("member.boxstring"), d, d, d, d, cls, names.Get("member.boxstring"), cls, names.Get("member.boxstring"))
	for _, arr := range []struct {
		name  string
		items []string
	}{{"ax", []string{"a"}}, {"bx", []string{"b"}}, {"words", []string{"x", "x", "y"}}, {"shuffled", []string{"y", "x", "x"}}, {"duplicates", []string{"y", "y", "x"}}} {
		fmt.Fprintf(&unit, "%s = NEW #( ).\n%s->tag = %s=>tag_array.\n", arr.name, arr.name, d)
		for _, item := range arr.items {
			fmt.Fprintf(&unit, "%s->append( %s ).\n", arr.name, item)
		}
	}
	fmt.Fprintf(&unit, "cycle = NEW #( ).\ncycle->tag = %s=>tag_object.\ncycle->put( p0 = `self` p1 = cycle ).\ncycle2 = NEW #( ).\ncycle2->tag = %s=>tag_object.\ncycle2->put( p0 = `self` p1 = cycle2 ).\naction = %s=>box_ref( NEW %s( ) ).\nquiet = %s=>box_ref( NEW %s( ) ).\nblocked = %s=>box_ref( NEW %s( ) ).\n", d, d, runtime, names.Get("closure.chaiAction"), runtime, names.Get("closure.chaiQuiet"), runtime, names.Get("closure.chaiRefusal"))
	op := func(name, a, b string) string {
		return cls + "=>" + names.Get("member."+name) + "( " + names.Get("param.a") + " = " + a + " " + names.Get("param.b") + " = " + b + " )"
	}
	member := func(a, b, deep string) string {
		return cls + "=>" + names.Get("member.members") + "( " + names.Get("param.a") + " = " + a + " " + names.Get("param.b") + " = " + b + " " + names.Get("param.deep") + " = " + deep + " )"
	}
	fmt.Fprintf(&unit, "DATA forward TYPE REF TO %s.\nDATA reverse TYPE REF TO %s.\nforward = NEW #( ).\nforward->tag = %s=>tag_array.\nforward->append( x ).\nforward->append( y ).\nreverse = NEW #( ).\nreverse->tag = %s=>tag_array.\nreverse->append( y ).\nreverse->append( x ).\nDATA boom TYPE REF TO %s.\nDATA lower TYPE REF TO %s.\nDATA blank TYPE REF TO %s.\nboom = %s=>%s( `BOOM` ).\nlower = %s=>%s( `boom` ).\nblank = %s=>%s( `` ).\n", d, d, d, d, d, d, d, cls, names.Get("member.boxstring"), cls, names.Get("member.boxstring"), cls, names.Get("member.boxstring"))
	fmt.Fprintf(&unit, "DATA error_action TYPE REF TO %s.\nerror_action = %s=>box_ref( NEW %s( ) ).\n", d, runtime, names.Get("closure.chaiError"))
	expressions := []string{op("deepEqual", "one", "one"), op("deepEqual", "one", "text"), op("deepEqual", "undef", "undef"), op("deepEqual", "null", "undef"), op("deepEqual", "a", "b"), op("deepEqual", "a", "empty"), op("deepEqual", "forward", "reverse"), op("deepEqual", "cycle", "cycle2"), op("arrayInclude", "ax", "a"), op("arrayInclude", "ax", "b"), "xsdbool( " + op("deepInclude", "ax", "b") + " = 1 )", member("words", "shuffled", "abap_false"), member("words", "duplicates", "abap_false"), member("ax", "bx", "abap_false"), member("ax", "bx", "abap_true"), "xsdbool( " + op("deepInclude", "boom", "lower") + " = 1 )", "xsdbool( " + op("deepInclude", "boom", "blank") + " = 1 )", "xsdbool( " + op("deepInclude", "a", "b") + " = 1 )", "xsdbool( " + op("deepInclude", "empty", "b") + " = 1 )", op("throwMatch", "action", "`BOOM`"), op("throwMatch", "action", "`boom`"), op("throwMatch", "quiet", "``"), op("throwMatch", "action", "``"), op("throwMatch", "error_action", "`BOOM`"), op("throwMatch", "error_action", "`boom`"), op("throwMatch", "error_action", "``")}
	if len(wants) != len(expressions) {
		t.Fatal("oracle vector length")
	}
	for i, expr := range expressions {
		want := "abap_false"
		if wants[i] {
			want = "abap_true"
		}
		fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( act = %s exp = %s msg = 'Chai case %d' ).\n", expr, want, i)
	}
	fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( act = %s exp = 2 ).\n", op("deepInclude", "a", "empty"))
	// Refusals remain observable: unsupported class enumeration, signed zero,
	// and a translated coverage trap must never become assertion success.
	fmt.Fprintf(&unit, "DATA zero TYPE REF TO %s.\nzero = %s=>%s( 0 ).\nDATA nominal TYPE REF TO %s.\nDATA nominal2 TYPE REF TO %s.\nnominal = %s=>box_ref( NEW %s( ) ).\nnominal2 = %s=>box_ref( NEW %s( ) ).\nDATA answer TYPE abap_bool.\nDATA trapped TYPE abap_bool.\n", d, cls, names.Get("member.boxnumber"), d, d, runtime, names.Get("ChaiNominal"), runtime, names.Get("ChaiNominal"))
	for _, expr := range []string{op("deepEqual", "zero", "zero"), op("deepEqual", "nominal", "nominal2"), op("throwMatch", "blocked", "``")} {
		unit.WriteString("CLEAR trapped.\nTRY.\nanswer = " + expr + ".\nCATCH " + names.Get("exception.chaiRefusal") + " " + names.Get("exception.unexecuted") + ".\ntrapped = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( trapped ).\n")
	}
	fmt.Fprintf(&unit, "DATA typed_record TYPE REF TO %s.\nDATA native_map TYPE REF TO %s.\ntyped_record = %s=>%s( undef ).\nnative_map = %s=>%s( undef ).\ncl_abap_unit_assert=>assert_true( %s ).\nCLEAR trapped.\nTRY.\nanswer = %s.\nCATCH %s.\ntrapped = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( trapped ).\n", d, d, cls, names.Get("member.record"), cls, names.Get("member.nativeMap"), op("deepEqual", "typed_record", "a"), op("deepEqual", "native_map", "a"), names.Get("exception.chaiRefusal"))
	// The zero sign is proven at source-literal/integral/JSON boundaries only.
	zeroOracle, err := exec.Command("node", "-e", `const {expect}=require(process.argv[1]);const ok=f=>{try{f();return true}catch(e){if(e.name!=="AssertionError")throw e;return false}};console.log(JSON.stringify([ok(()=>expect(0).eql(-0)),ok(()=>expect(-0).eql(JSON.parse("-0"))),ok(()=>expect(0).eql(JSON.parse("0")))]))`, chai).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(zeroOracle))
	}
	var zeroWants []bool
	if err := json.Unmarshal(zeroOracle, &zeroWants); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&unit, "DATA positive TYPE REF TO %s.\nDATA negative TYPE REF TO %s.\nDATA parsed_negative TYPE REF TO %s.\nDATA parsed_positive TYPE REF TO %s.\npositive = %s=>%s( ).\nnegative = %s=>%s( ).\nparsed_negative = %s=>%s( `-0` ).\nparsed_positive = %s=>%s( `0` ).\n", d, d, d, d, cls, names.Get("member.positiveZero"), cls, names.Get("member.negativeZero"), cls, names.Get("member.parse"), cls, names.Get("member.parse"))
	for i, expr := range []string{op("deepEqual", "positive", "negative"), op("deepEqual", "negative", "parsed_negative"), op("deepEqual", "positive", "parsed_positive")} {
		want := "abap_false"
		if zeroWants[i] {
			want = "abap_true"
		}
		fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( act = %s exp = %s msg = 'Chai zero %d' ).\n", expr, want, i)
	}
	// Exercise emitted typed array boxing, not only preboxed graph operations.
	arr := names.Get("runtime." + hir.T(hir.Array, str).String())
	fmt.Fprintf(&unit, "DATA typed TYPE REF TO %s.\nDATA typed2 TYPE REF TO %s.\nDATA boxed TYPE REF TO %s.\nDATA boxed2 TYPE REF TO %s.\ntyped = NEW #( ).\nAPPEND `x` TO typed->items.\ntyped2 = NEW #( ).\nAPPEND `x` TO typed2->items.\nboxed = %s=>%s( typed ).\nboxed2 = %s=>%s( typed2 ).\ncl_abap_unit_assert=>assert_true( %s ).\nAPPEND `y` TO typed2->items.\ncl_abap_unit_assert=>assert_false( %s ).\n", arr, arr, d, d, cls, names.Get("member.box"+hir.T(hir.Array, str).String()), cls, names.Get("member.box"+hir.T(hir.Array, str).String()), op("deepEqual", "boxed", "boxed2"), op("deepEqual", "boxed", "boxed2"))
	fmt.Fprintf(&unit, "DATA typed_action TYPE REF TO %s.\nDATA typed_quiet TYPE REF TO %s.\ntyped_action = NEW %s( ).\ntyped_quiet = NEW %s( ).\ncl_abap_unit_assert=>assert_true( %s ).\ncl_abap_unit_assert=>assert_false( %s ).\ncl_abap_unit_assert=>assert_false( %s ).\n", names.Get("closure.thunk"), names.Get("closure.thunk"), names.Get("ChaiTypedThrow"), names.Get("ChaiTypedQuiet"), op("typedThrowMatch", "typed_action", "`BOOM`"), op("typedThrowMatch", "typed_action", "`boom`"), op("typedThrowMatch", "typed_quiet", "``"))
	// Required-field data shapes certify own enumerable keys. Distinct nominal
	// instances above remain unproven and refuse instead of using these slots.
	shapeOracle, err := exec.Command("node", "-e", `const {expect}=require(process.argv[1]);class Data{constructor(x){this.x=x}}class Data2{constructor(x){this.x=x}}expect(new Data("x")).eql(new Data2("x"));expect(new Data("x")).eql({x:"x"});expect(new Data("y")).not.eql({x:"x"});console.log("ok")`, chai).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(shapeOracle))
	}
	fmt.Fprintf(&unit, "DATA shape TYPE REF TO %s.\nDATA shape2 TYPE REF TO %s.\nDATA shape_box TYPE REF TO %s.\nDATA shape_box2 TYPE REF TO %s.\nDATA record TYPE REF TO %s.\nshape = NEW #( %s = `x` ).\nshape2 = NEW #( %s = `x` ).\nshape_box = %s=>%s( shape ).\nshape_box2 = %s=>%s( shape2 ).\nrecord = %s=>%s( `{\"x\":\"x\"}` ).\ncl_abap_unit_assert=>assert_true( %s ).\ncl_abap_unit_assert=>assert_true( %s ).\nshape->%s = `y`.\ncl_abap_unit_assert=>assert_false( %s ).\n", names.Get("ChaiData"), names.Get("ChaiData2"), d, d, d, names.Get("param.x"), names.Get("param.x"), cls, names.Get("member.boxChaiData"), cls, names.Get("member.boxChaiData2"), cls, names.Get("member.parse"), op("deepEqual", "shape_box", "shape_box2"), op("deepEqual", "shape_box", "record"), names.Get("member.x"), op("deepEqual", "shape_box", "record"))
	unit.WriteString("ENDMETHOD.\nENDCLASS.\n")
	files[cls+".clas.testclasses.abap"] = unit.String()
	for file, src := range files {
		formatted, err := Format(src)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		files[file] = formatted
	}
	all := files[runtime+".clas.abap"]
	for _, part := range []string{"READ TABLE seen WITH KEY", "FIND FIRST OCCURRENCE OF p1", "RAISE EXCEPTION error", "p0->nval = 0 AND p1->nval = 0"} {
		if !strings.Contains(all, part) {
			t.Fatal("missing semantic boundary", part)
		}
	}
	if base := os.Getenv("ABAPITI_TEST_OUT"); base != "" {
		out := filepath.Join(base, t.Name())
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for file, src := range files {
			if err := os.WriteFile(filepath.Join(out, file), []byte(src), 0644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(out, "oracle.json"), oracle, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestChaiSparseRefusalEmission(t *testing.T) {
	arr := hir.T(hir.Array, str)
	d := hir.T(hir.Dynamic)
	a := local("a", arr)
	m := method("run", d, hir.B(decl("a", arr, newObj(arr)), &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.IndexGet, X: a, Y: lit(2), Type: str}, Y: hir.L(str, "x")}, ret(rt("chai.box", a, d))))
	m.Static = true
	files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{{Name: "SparseChai", Methods: []*hir.Method{m}}}})
	if err != nil {
		t.Fatal(err)
	}
	src := files[names.Get("SparseChai")+".clas.abap"]
	if !strings.Contains(src, "->put( p0 = ") {
		t.Fatal("sparse writes must use presence-aware storage")
	}
	adapter := files[names.Get("runtime.chai")+".clas.abap"]
	gate := strings.Index(adapter, "->missing IS NOT INITIAL.")
	read := strings.Index(adapter, "LOOP AT a0->items")
	if gate < 0 || read < 0 || gate > read {
		t.Fatal("Chai must refuse sparse arrays before reading dense items")
	}
}
