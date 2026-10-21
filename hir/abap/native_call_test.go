package abap

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestNativeCall0Fixtures(t *testing.T) {
	out, err := exec.Command("node", "-e", `class B{value(){return 1}};class C extends B{value(){return 2}};console.log([new B,new C].reduce((n,v)=>n+v.value(),0)===3 && [1,undefined].map(v=>typeof v).join('')==='numberundefined');`).CombinedOutput()
	if err != nil || string(out) != "true\n" {
		t.Fatalf("Node: %s %v", out, err)
	}
	dyn := hir.T(hir.Dynamic)
	opt := hir.T(hir.Optional, i32)
	value := method("value", i32, ret(lit(1)))
	value.Virtual = true
	override := method("value", i32, ret(lit(2)))
	override.Virtual = true
	undef := method("undef", opt, ret(hir.L(opt, nil)))
	void := method("nothing", hir.T(hir.Void), hir.B())
	bridge := method("nothing_value", str, ret(hir.L(str, "bridge")))
	call0 := func(name string) *hir.Method {
		m := method(name, dyn, ret(rt("dynamic.call0", local("d", dyn), dyn, hir.L(str, name))))
		m.Static = true
		m.Params = []hir.Param{{Name: "d", Type: dyn}}
		return m
	}
	number := hir.T(hir.Number)
	base := hir.Ref("NativeBase")
	array := hir.T(hir.Array, base)
	loop := method("loop", boolean, hir.B(decl("a", array, newObj(array)), run(rt("array.push", local("a", array), i32, newObj(base))), run(rt("array.push", local("a", array), i32, newObj(hir.Ref("NativeChild")))), decl("sum", number, hir.L(number, 0.0)), &hir.Stmt{Kind: hir.ForEach, Name: "item", Type: base, X: local("a", array), Body: assign("sum", number, binary("+", local("sum", number), rt("dynamic.asNumber", rt("dynamic.call0", rt("dynamic.of", local("item", base), dyn), dyn, hir.L(str, "value")), number), number))}, ret(binary("==", local("sum", number), hir.L(number, 3.0), boolean))))
	loop.Static = true
	optional := hir.T(hir.Optional, i32)
	options := hir.T(hir.Array, optional)
	boxes := method("boxes", boolean, hir.B(decl("a", options, newObj(options)), run(rt("array.push", local("a", options), i32, lit(1))), run(rt("array.push", local("a", options), i32, hir.L(optional, nil))), decl("text", str, hir.L(str, "")), &hir.Stmt{Kind: hir.ForEach, Name: "item", Type: optional, X: local("a", options), Body: assign("text", str, rt("string.concat", local("text", str), str, rt("dynamic.typeofValue", rt("dynamic.of", local("item", optional), dyn), str)))}, ret(binary("==", local("text", str), hir.L(str, "numberundefined"), boolean))))
	boxes.Static = true
	p := &hir.Program{Classes: []*hir.Class{
		{Name: "NativeBase", Methods: []*hir.Method{value, undef, void, bridge}},
		{Name: "NativeChild", Super: "NativeBase", Methods: []*hir.Method{override}},
		{Name: "NativeUnsafe", Super: "NativeBase", PrototypeAccessors: true},
		{Name: "NativeCalls", Methods: []*hir.Method{call0("value"), call0("undef"), call0("nothing"), call0("missing"), loop, boxes}},
	}}
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	owner, d := names.Get("NativeCalls"), names.Get("runtime.dynamic")
	unit := fmt.Sprintf("CLASS ltcl_calls DEFINITION FOR TESTING DURATION SHORT RISK LEVEL HARMLESS.\nPRIVATE SECTION.\nMETHODS check FOR TESTING.\nENDCLASS.\nCLASS ltcl_calls IMPLEMENTATION.\nMETHOD check.\nDATA d TYPE REF TO %s.\nDATA v TYPE REF TO %s.\nDATA refused TYPE abap_bool.\nCREATE OBJECT d.\nd->tag = %s=>tag_ref.\n", d, d, d)
	unit += fmt.Sprintf("cl_abap_unit_assert=>assert_true( %s=>%s( ) ).\ncl_abap_unit_assert=>assert_true( %s=>%s( ) ).\n", owner, names.Get("member.loop"), owner, names.Get("member.boxes"))
	for _, c := range []string{"NativeBase", "NativeChild"} {
		want := 1
		if c == "NativeChild" {
			want = 2
		}
		unit += fmt.Sprintf("d->oval = NEW %s( ).\nv = %s=>%s( d ).\ncl_abap_unit_assert=>assert_equals( act = v->nval exp = %d ).\nv = %s=>%s( d ).\ncl_abap_unit_assert=>assert_true( xsdbool( v IS NOT BOUND ) ).\nv = %s=>%s( d ).\ncl_abap_unit_assert=>assert_equals( act = v->sval exp = `bridge` ).\n", names.Get(c), owner, names.Get("member.value"), want, owner, names.Get("member.undef"), owner, names.Get("member.nothing"))
	}
	for _, c := range []string{"NativeUnsafe"} {
		unit += fmt.Sprintf("d->oval = NEW %s( ).\nCLEAR refused.\nTRY.\nv = %s=>%s( d ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\n", names.Get(c), owner, names.Get("member.value"), names.Get("class.refusal"))
	}
	unit += fmt.Sprintf("d->oval = NEW %s( ).\nCLEAR refused.\nTRY.\nv = %s=>%s( d ).\nCATCH %s.\nrefused = abap_true.\nENDTRY.\ncl_abap_unit_assert=>assert_true( refused ).\nENDMETHOD.\nENDCLASS.\n", names.Get("NativeBase"), owner, names.Get("member.missing"), names.Get("class.refusal"))
	files[owner+".clas.testclasses.abap"] = unit
	storageHostFixture(t, files, owner)
}

// Keep the caller in its own class so the grid compares its complete source,
// including temporary declarations, rather than trimming away dispatch costs.
func nativeCall0Grid(t *testing.T, classes, sites int) (map[string]string, *hir.Names) {
	t.Helper()
	dyn := hir.T(hir.Dynamic)
	p := &hir.Program{}
	for i := 0; i < classes; i++ {
		p.Classes = append(p.Classes, &hir.Class{Name: fmt.Sprintf("GridNative%d", i), Methods: []*hir.Method{method("getName", str, ret(hir.L(str, "name")))}})
	}
	caller := &hir.Class{Name: "GridCaller"}
	for i := 0; i < sites; i++ {
		m := method(fmt.Sprintf("site%d", i), dyn, ret(rt("dynamic.call0", local("d", dyn), dyn, hir.L(str, "getName"))))
		m.Static = true
		m.Params = []hir.Param{{Name: "d", Type: dyn}}
		caller.Methods = append(caller.Methods, m)
	}
	p.Classes = append(p.Classes, caller)
	files, names, err := emitABAPNamed(t, p)
	if err != nil {
		t.Fatal(err)
	}
	return files, names
}

func TestNativeCall0Shared(t *testing.T) {
	files, names := nativeCall0Grid(t, 128, 2)
	helper := files[names.Get("runtime.native_call0")+".clas.abap"]
	member := names.Get("member.getName")
	if strings.Count(helper, "METHOD "+member+".") != 1 || strings.Count(helper, "CLASS-METHODS "+member+" ") != 1 {
		t.Fatal("expected one shared method declaration and implementation")
	}
	caller := files[names.Get("GridCaller")+".clas.abap"]
	if strings.Count(caller, names.Get("runtime.native_call0")+"=>"+member+"(") != 2 || strings.Contains(caller, "IS INSTANCE OF") {
		t.Fatal("expected two helper calls without inline probes")
	}
	if got := strings.Count(helper, "IS INSTANCE OF"); got != 128 {
		t.Fatalf("helper probes = %d, want 128", got)
	}
	for _, line := range strings.Split(helper, "\n") {
		if len(line) > 255 || strings.HasPrefix(strings.TrimSpace(line), "*") || strings.Contains(line, `"`) {
			t.Fatalf("invalid emitted helper line: %q", line)
		}
	}
}

func TestNativeCall0FixtureGrid(t *testing.T) {
	var callers [2][2]string
	var helpers [2][2]string
	for c, classes := range []int{0, 128} {
		for s, sites := range []int{1, 2} {
			files, names := nativeCall0Grid(t, classes, sites)
			callers[c][s] = files[names.Get("GridCaller")+".clas.abap"]
			helpers[c][s] = files[names.Get("runtime.native_call0")+".clas.abap"]
		}
	}
	for s := range callers[0] {
		if callers[0][s] != callers[1][s] {
			t.Fatalf("adding 128 classes changed %d-site caller", s+1)
		}
	}
	for c := range helpers {
		if helpers[c][0] != helpers[c][1] {
			t.Fatal("second site changed helper")
		}
		if strings.Count(callers[c][1], "=>")-strings.Count(callers[c][0], "=>") != 1 {
			t.Fatal("second site must add exactly one helper call")
		}
	}
	if len(helpers[1][1]) <= len(helpers[0][1]) {
		t.Fatal("additional classes must grow the helper")
	}
}

func TestNativeCall0GridLines(t *testing.T) {
	files, names := nativeCall0Grid(t, 128, 2)
	caller := files[names.Get("GridCaller")+".clas.abap"]
	helper := files[names.Get("runtime.native_call0")+".clas.abap"]
	total := 0
	for _, source := range files {
		total += strings.Count(source, "\n")
	}
	t.Logf("2-site x 128-class fixture: caller=%d helper=%d combined=%d total=%d emitted lines", strings.Count(caller, "\n"), strings.Count(helper, "\n"), strings.Count(caller+helper, "\n"), total)
}

func TestNativeCall0HelperUsageAndOrder(t *testing.T) {
	files, names := nativeCall0Grid(t, 128, 0)
	if _, emitted := files[names.Get("runtime.native_call0")+".clas.abap"]; emitted {
		t.Fatal("unused native dispatcher emitted")
	}
	var previous string
	for _, order := range [][]string{{"zeta", "alpha"}, {"alpha", "zeta"}} {
		c := &hir.Class{Name: "OrderedCalls"}
		for _, name := range order {
			dyn := hir.T(hir.Dynamic)
			m := method(name, dyn, ret(rt("dynamic.call0", local("d", dyn), dyn, hir.L(str, name))))
			m.Static = true
			m.Params = []hir.Param{{Name: "d", Type: dyn}}
			c.Methods = append(c.Methods, m)
		}
		files, names, err := emitABAPNamed(t, &hir.Program{Classes: []*hir.Class{c}})
		if err != nil {
			t.Fatal(err)
		}
		helper := files[names.Get("runtime.native_call0")+".clas.abap"]
		for _, prefix := range []string{"CLASS-METHODS ", "METHOD "} {
			a := strings.Index(helper, prefix+names.Get("member.alpha"))
			z := strings.Index(helper, prefix+names.Get("member.zeta"))
			if a < 0 || z < a {
				t.Fatal("helper methods must be sorted by source name")
			}
		}
		if previous != "" && helper != previous {
			t.Fatal("helper changed with call-site discovery order")
		}
		previous = helper
	}
}
