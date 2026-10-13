package abap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func nestedClassViewFixture() *hir.Program {
	dyn, child, parent := hir.T(hir.Dynamic), hir.Ref("NestedChild"), hir.Ref("NestedParent")
	field := func(x *hir.Expr, name string, ty hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.FieldGet, X: x, Name: name, Type: ty}
	}
	get := func(x *hir.Expr, key string) *hir.Expr { return rt("dynamic.get", x, dyn, hir.L(str, key)) }
	put := func(x *hir.Expr, key string, value *hir.Expr) *hir.Stmt {
		return run(rt("dynamic.put", x, hir.T(hir.Void), hir.L(str, key), value))
	}
	flag := func(name string) *hir.Expr { return field(local(name, child), "flag", boolean) }
	count := &hir.Expr{Kind: hir.StaticGet, Owner: "NestedChild", Name: "count", Type: i32}
	ctor := method("constructor", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Assign, X: count, Y: binary("+", count, lit(1), i32)}))
	body := hir.B(
		&hir.Stmt{Kind: hir.Assign, X: count, Y: lit(0)},
		decl("graph", dyn, rt("json.parseSubset", hir.L(str, `{"child":{"flag":true,"extra":"kept"}}`), dyn)),
		decl("parent", parent, rt("dynamic.classView", local("graph", dyn), parent)),
		decl("a", child, field(local("parent", parent), "child", child)),
		decl("b", child, field(local("parent", parent), "child", child)),
		decl("ok", boolean, binary("==", local("a", child), local("b", child), boolean)),
		&hir.Stmt{Kind: hir.Assign, X: flag("a"), Y: hir.L(boolean, false)},
	)
	check := func(x *hir.Expr) {
		body.List = append(body.List, assign("ok", boolean, binary("&&", local("ok", boolean), x, boolean)))
	}
	check(binary("==", flag("b"), hir.L(boolean, false), boolean))
	check(binary("==", rt("dynamic.asBoolean", get(get(local("graph", dyn), "child"), "flag"), boolean), hir.L(boolean, false), boolean))
	body.List = append(body.List, put(get(local("graph", dyn), "child"), "flag", rt("dynamic.of", hir.L(boolean, true), dyn)))
	check(flag("a"))
	body.List = append(body.List,
		decl("replacement", dyn, rt("json.parseSubset", hir.L(str, `{"flag":false,"extra":"new"}`), dyn)),
		put(local("graph", dyn), "child", local("replacement", dyn)),
		decl("fresh", child, field(local("parent", parent), "child", child)),
	)
	check(binary("!=", local("fresh", child), local("a", child), boolean))
	check(binary("==", flag("fresh"), hir.L(boolean, false), boolean))
	check(flag("a"))
	body.List = append(body.List, &hir.Stmt{Kind: hir.Assign, X: flag("fresh"), Y: hir.L(boolean, true)})
	check(rt("dynamic.asBoolean", get(local("replacement", dyn), "flag"), boolean))
	check(binary("==", rt("dynamic.asString", get(local("replacement", dyn), "extra"), str), hir.L(str, "new"), boolean))
	check(binary("==", count, lit(0), boolean))
	body.List = append(body.List, ret(local("ok", boolean)))
	m := method("aliases", boolean, body)
	m.Static = true
	read := method("read", boolean, hir.B(
		decl("graph", dyn, rt("json.parseSubset", local("input", str), dyn)),
		decl("parent", parent, rt("dynamic.classView", local("graph", dyn), parent)),
		ret(field(field(local("parent", parent), "child", child), "flag", boolean)),
	))
	read.Static, read.Params = true, []hir.Param{{Name: "input", Type: str}}
	optional := hir.T(hir.Optional, child)
	optParent := hir.Ref("OptionalParent")
	absent := method("absent", boolean, hir.B(
		decl("graph", dyn, rt("json.parseSubset", hir.L(str, `{}`), dyn)),
		decl("parent", optParent, rt("dynamic.classView", local("graph", dyn), optParent)),
		ret(binary("==", field(local("parent", optParent), "child", optional), hir.L(optional, nil), boolean)),
	))
	absent.Static = true
	nominal, nominalParent := hir.Ref("NominalChild"), hir.Ref("NominalParent")
	nominalRead := method("nominal", boolean, hir.B(
		decl("graph", dyn, rt("json.parseSubset", hir.L(str, `{"child":{"flag":true}}`), dyn)),
		decl("parent", nominalParent, rt("dynamic.classView", local("graph", dyn), nominalParent)),
		ret(field(field(local("parent", nominalParent), "child", nominal), "flag", boolean)),
	))
	nominalRead.Static = true
	native := method("native", boolean, hir.B(
		decl("child", nominal, newObj(nominal)),
		&hir.Stmt{Kind: hir.Assign, X: field(local("child", nominal), "flag", boolean), Y: hir.L(boolean, true)},
		decl("graph", dyn, rt("json.parseSubset", hir.L(str, `{}`), dyn)),
		put(local("graph", dyn), "child", rt("dynamic.of", local("child", nominal), dyn)),
		decl("parent", nominalParent, rt("dynamic.classView", local("graph", dyn), nominalParent)),
		decl("alias", nominal, field(local("parent", nominalParent), "child", nominal)),
		ret(binary("&&", binary("==", local("alias", nominal), local("child", nominal), boolean),
			call(hir.VirtualCall, local("alias", nominal), "", "behavior", boolean), boolean)),
	))
	native.Static = true
	behavior := method("behavior", boolean, ret(hir.L(boolean, true)))
	behavior.Virtual = true
	return &hir.Program{Classes: []*hir.Class{
		{Name: "NestedChild", Structural: true, Ctor: ctor, Fields: []hir.Field{{Name: "flag", Type: boolean}, {Name: "count", Type: i32, Static: true}}},
		{Name: "NestedParent", Structural: true, Fields: []hir.Field{{Name: "child", Type: child}}},
		{Name: "OptionalParent", Structural: true, Fields: []hir.Field{{Name: "child", Type: optional}}},
		{Name: "NominalChild", Fields: []hir.Field{{Name: "flag", Type: boolean}}, Methods: []*hir.Method{behavior}},
		{Name: "NominalParent", Structural: true, Fields: []hir.Field{{Name: "child", Type: nominal}}},
		{Name: "NestedViews", Methods: []*hir.Method{m, read, absent, nominalRead, native}},
	}}
}

func TestNestedClassViewFixtures(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node oracle unavailable")
	}
	const oracle = `class Child {static count=0;constructor(){Child.count++}}
const graph=JSON.parse('{"child":{"flag":true,"extra":"kept"}}');
const a=graph.child,b=graph.child;let ok=a===b;
a.flag=false;ok=ok&&b.flag===false&&graph.child.flag===false;
graph.child.flag=true;ok=ok&&a.flag;
graph.child={flag:false,extra:"new"};const fresh=graph.child;
ok=ok&&fresh!==a&&fresh.flag===false&&a.flag;
fresh.flag=true;ok=ok&&graph.child.flag&&graph.child.extra==="new";
ok=ok&&Child.count===0;
class Nominal {behavior(){return true}}
const native=new Nominal(),parent={child:native};const alias=parent.child;
console.log(ok,JSON.parse('{}').child===undefined,alias===native&&alias.behavior());`
	want, err := exec.Command("node", "-e", oracle).CombinedOutput()
	if err != nil || string(want) != "true true true\n" {
		t.Fatalf("Node alias oracle: %s %v", want, err)
	}
	p := nestedClassViewFixture()
	if errors := hir.Verify(p); len(errors) > 0 {
		t.Fatal(errors)
	}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner := names.Get("NestedViews")
	var unit strings.Builder
	unit.WriteString("CLASS ltcl_views DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_views IMPLEMENTATION.\nMETHOD check.\nDATA refused TYPE abap_bool.\nDATA actual TYPE abap_bool.\n")
	for _, name := range []string{"aliases", "absent", "native"} {
		fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_true( %s=>%s( ) ).\n", owner, names.Get("member."+name))
	}
	fmt.Fprintf(&unit, "cl_abap_unit_assert=>assert_true( %s=>%s( `{\"child\":{\"flag\":true}}` ) ).\n", owner, names.Get("member.read"))
	for _, input := range []string{`{"child":null}`, `{"child":42}`, `{"child":"text"}`} {
		fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\nactual = %s=>%s( `%s` ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", owner, names.Get("member.read"), input, names.Get("view.refusal"))
	}
	fmt.Fprintf(&unit, "CLEAR refused.\nTRY.\nactual = %s=>%s( ).\nCATCH cx_sy_move_cast_error.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", owner, names.Get("member.nominal"))
	unit.WriteString("ENDMETHOD.\nENDCLASS.\n")
	files[owner+".clas.testclasses.abap"] = unit.String()
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		out = filepath.Join(out, t.Name())
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		for name, source := range files {
			source, err = Format(source)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(out, name), []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestExplicitNestedClassLayoutFixtures(t *testing.T) {
	// A data-only class can be explicitly activated as a structural layout
	// even without the synthetic Structural marker. Its nested reads must
	// follow Go's activated-layout boundary as well.
	p := nestedClassViewFixture()
	p.Classes[0].Structural = false
	child, dyn := hir.Ref("NestedChild"), hir.T(hir.Dynamic)
	m := method("activate", child, ret(rt("dynamic.classView", local("input", dyn), child)))
	m.Static, m.Params = true, []hir.Param{{Name: "input", Type: dyn}}
	p.Classes[len(p.Classes)-1].Methods = append(p.Classes[len(p.Classes)-1].Methods, m)
	files, names, err := emitABAPNamed(t, p)
	if err != nil { t.Fatal(err) }
	owner := names.Get("NestedViews")
	files[owner+".clas.testclasses.abap"] = fmt.Sprintf("CLASS ltcl_explicit DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_explicit IMPLEMENTATION.\nMETHOD check.\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\nENDMETHOD.\nENDCLASS.\n", owner, names.Get("member.aliases"))
	if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
		out = filepath.Join(out, t.Name())
		if err := os.MkdirAll(out, 0755); err != nil { t.Fatal(err) }
		for name, source := range files {
			source, err = Format(source); if err != nil { t.Fatal(err) }
			if err := os.WriteFile(filepath.Join(out, name), []byte(source), 0644); err != nil { t.Fatal(err) }
		}
	}
}
