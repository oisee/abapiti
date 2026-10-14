package abap

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/abapfixture"
)

func liveConstructorFixture() *hir.Program {
	void, dyn := hir.T(hir.Void), hir.T(hir.Dynamic)
	counter := func(name string) *hir.Expr {
		return &hir.Expr{Kind: hir.StaticGet, Owner: "ConstructorProbe", Name: name, Type: i32}
	}
	inc := func(name string) *hir.Stmt {
		return &hir.Stmt{Kind: hir.Assign, X: counter(name), Y: binary("+", counter(name), lit(1), i32)}
	}
	ctor := method("constructor", void, hir.B(inc("baseCount"), &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("ViewBase")}, Name: "value", Type: i32}, Y: local("value", i32)}))
	ctor.Params = []hir.Param{{Name: "value", Type: i32}}
	bump := method("bump", i32, hir.B(inc("argCount"), ret(binary("+", local("seed", i32), lit(1), i32))))
	bump.Static, bump.Params = true, []hir.Param{{Name: "seed", Type: i32}}
	super := run(call(hir.SuperCall, nil, "", "constructor", void, call(hir.DirectCall, nil, "ConstructorProbe", "bump", i32, local("seed", i32))))
	childCtor := method("constructor", void, hir.B(&hir.Stmt{Kind: hir.If, X: binary("==", local("seed", i32), lit(0), boolean), Body: assign("seed", i32, lit(4))}, super, inc("childCount"), &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: i32, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("ViewChild")}, Name: "seedAfter"}, Y: local("seed", i32)}))
	childCtor.Params = []hir.Param{{Name: "seed", Type: i32}}
	activate := func(name, target string) *hir.Method {
		m := method(name, hir.Ref(target), ret(rt("dynamic.classView", local("input", dyn), hir.Ref(target))))
		m.Static, m.Params = true, []hir.Param{{Name: "input", Type: dyn}}
		return m
	}
	static := method("class_constructor", void, hir.B(inc("staticCount")))
	static.Static = true
	plain := method("constructor", void, hir.B(inc("plainCount")))
	outside := method("constructor", void, hir.B(run(call(hir.SuperCall, nil, "", "constructor", void, lit(7)))))
	return &hir.Program{Classes: []*hir.Class{
		{Name: "ViewBase", Abstract: true, Fields: []hir.Field{{Name: "value", Type: i32}}, Ctor: ctor},
		{Name: "ViewChild", Super: "ViewBase", Fields: []hir.Field{{Name: "seedAfter", Type: i32}}, Ctor: childCtor, Methods: []*hir.Method{static}},
		{Name: "PlainBase", Abstract: true, Ctor: plain},
		{Name: "PlainChild", Super: "PlainBase"},
		{Name: "OutsideView", Super: "ViewBase", Ctor: outside},
		{Name: "ConstructorProbe", Fields: []hir.Field{{Name: "baseCount", Type: i32, Static: true}, {Name: "childCount", Type: i32, Static: true}, {Name: "argCount", Type: i32, Static: true}, {Name: "plainCount", Type: i32, Static: true}, {Name: "staticCount", Type: i32, Static: true}}, Methods: []*hir.Method{bump, activate("view", "ViewChild"), activate("plain", "PlainChild")}},
	}}
}

func TestLiveConstructorFixtures(t *testing.T) {
	oracle := `let baseCount=0,childCount=0,argCount=0,plainCount=0;
class Base {constructor(value){baseCount++;this.value=value}}
class Child extends Base {constructor(seed){if(seed===0)seed=4;super((argCount++,seed+1));childCount++;this.seedAfter=seed}}
class PlainBase {constructor(){plainCount++}};class PlainChild extends PlainBase {};
class Outside extends Base {constructor(){super(7)}};
const native=new Child(0),view={value:9},plainView={};new PlainChild();const outside=new Outside();
console.log(native.value,native.seedAfter,view.value,outside.value,baseCount,childCount,argCount,plainCount);`
	out, err := exec.Command("node", "-e", oracle).CombinedOutput()
	if err != nil || string(out) != "5 4 9 7 2 1 1 1\n" {
		t.Fatalf("Node constructor oracle: %s %v", out, err)
	}
	p := liveConstructorFixture()
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	if issues := abapfixture.KernelViolations(files); len(issues) > 0 {
		t.Fatal(issues)
	}
	// ABAP emission must not strip the shared HIR's native super statement.
	if p.Classes[1].Ctor.Body.List[1].X.Kind != hir.SuperCall {
		t.Fatal("ABAP mutated the constructor HIR")
	}
	owner, d := names.Get("ConstructorProbe"), names.Get("runtime.dynamic")
	var unit strings.Builder
	unit.WriteString("CLASS ltcl_ctor DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_ctor IMPLEMENTATION.\nMETHOD check.\n")

	fmt.Fprintf(&unit, "DATA graph TYPE REF TO %s.\nCREATE OBJECT graph.\ngraph->tag = %s=>tag_object.\nDATA num TYPE REF TO %s.\nCREATE OBJECT num.\nnum->tag = %s=>tag_number.\nnum->nval = 9.\ngraph->put( p0 = `value` p1 = num ).\nDATA view TYPE REF TO %s.\nview = %s=>%s( graph ).\ncl_abap_unit_assert=>assert_equals( act = view->%s( ) exp = 9 ).\n", d, d, d, d, names.Get("ViewChild"), owner, names.Get("member.view"), names.Get("view.read.ViewBase.value"))
	fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( act = %s=>%s exp = 0 ).\n", owner, names.Get("member.staticCount"))
	fmt.Fprintf(&unit, "DATA native TYPE REF TO %s.\nnative = NEW #( %s = 0 ).\ncl_abap_unit_assert=>assert_equals( act = native->%s exp = 5 ).\n", names.Get("ViewChild"), names.Get("param.seed"), names.Get("member.value"))
	fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( act = native->%s exp = 4 ).\n", names.Get("member.seedAfter"))
	fmt.Fprintf(&unit, "DATA plain TYPE REF TO %s.\nplain = %s=>%s( graph ).\nplain = NEW #( ).\nDATA outside TYPE REF TO %s.\noutside = NEW #( ).\ncl_abap_unit_assert=>assert_equals( act = outside->%s exp = 7 ).\n", names.Get("PlainChild"), owner, names.Get("member.plain"), names.Get("OutsideView"), names.Get("member.value"))
	for name, want := range map[string]int{"baseCount": 2, "childCount": 1, "argCount": 1, "plainCount": 1, "staticCount": 1} {
		fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_equals( act = %s=>%s exp = %d ).\n", owner, names.Get("member."+name), want)
	}
	unit.WriteString("ENDMETHOD.\nENDCLASS.\n")
	files[owner+".clas.testclasses.abap"] = unit.String()
	storageHostFixture(t, files, owner)
}

func TestLiveConstructorUnsafeSuperRefused(t *testing.T) {
	for _, shape := range []string{"conditional", "early-return", "duplicate", "native-conditional", "this-argument"} {
		t.Run(shape, func(t *testing.T) {
			p := liveConstructorFixture()
			m := p.Classes[1].Ctor
			super := m.Body.List[1]
			switch shape {
			case "conditional", "native-conditional":
				if shape == "native-conditional" {
					p.Classes[5].Methods = p.Classes[5].Methods[:1]
				}
				m.Body = hir.B(&hir.Stmt{Kind: hir.If, X: hir.L(boolean, true), Body: super})
			case "early-return":
				m.Body = hir.B(&hir.Stmt{Kind: hir.Return}, super)
			case "duplicate":
				m.Body = hir.B(super, super)
			case "this-argument":
				super.X.Args[0] = &hir.Expr{Kind: hir.FieldGet, Type: i32, Name: "value", X: &hir.Expr{Kind: hir.This, Type: hir.Ref("ViewChild")}}
				m.Body = hir.B(super)
			}
			if _, err := emitABAP(t, p); err == nil || !strings.Contains(err.Error(), "single top-level super") {
				t.Fatalf("unproven constructor accepted: %v", err)
			}
		})
	}
}
