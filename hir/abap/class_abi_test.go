package abap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func abiSource(t *testing.T, p *hir.Program) (map[string]string, *hir.Names) {
	t.Helper()
	files, names, err := EmitNamed(p)
	if err != nil {
		t.Fatal(err)
	}
	for name, src := range files {
		for _, line := range strings.Split(src, "\n") {
			if len(line) > 255 {
				t.Fatalf("%s: overlong ABAP line", name)
			}
		}
		if out := os.Getenv("ABAPITI_TEST_OUT"); out != "" {
			dir := filepath.Join(out, t.Name())
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return files, names
}

func requireABI(t *testing.T, source string, parts ...string) {
	t.Helper()
	normalize := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	for _, part := range parts {
		if !strings.Contains(normalize(source), normalize(part)) {
			t.Fatalf("missing %q in emitted ABAP:\n%s", part, source)
		}
	}
}

func TestCheckedClassViewABAP(t *testing.T) {
	p := &hir.Program{Classes: []*hir.Class{{Name: "Base"}, {Name: "Child", Super: "Base"}, {Name: "Sibling", Super: "Base"}}}
	for _, pair := range [][2]string{{"Base", "Child"}, {"Child", "Base"}, {"Sibling", "Child"}, {hir.RootObject, "Child"}} {
		m := method("cast"+pair[0], hir.Ref(pair[1]), ret(rt("object.checkedClassView", local("input", hir.Ref(pair[0])), hir.Ref(pair[1]))))
		m.Params = []hir.Param{{Name: "input", Type: hir.Ref(pair[0])}}
		p.Classes[0].Methods = append(p.Classes[0].Methods, m)
	}
	files, names := abiSource(t, p)
	src := files[names.Get("Base")+".clas.abap"]
	// ?= retains object identity, including an initial receiver. A mismatch
	// must escape as a cast fault; no catch, allocation or structural copy.
	if strings.Count(src, " ?= ") != 4 || strings.Contains(src, "CATCH cx_sy_move_cast_error") || strings.Contains(src, "view_source =") {
		t.Fatal("nominal view must emit four identity-preserving checked casts:\n" + src)
	}
	if !SupportsRuntimeOp("object.checkedClassView") {
		t.Fatal("checked nominal casts still blocked by catalogue")
	}
}

func TestBuiltinErrorClassOfABAP(t *testing.T) {
	cv := hir.T(hir.ClassValue)
	m := method("type", cv, ret(rt("object.classOf", newObj(hir.Ref("builtin.Error")), cv)))
	files, names := abiSource(t, &hir.Program{Classes: []*hir.Class{{Name: "builtin.Error"}, {Name: "Caller", Methods: []*hir.Method{m}}}})
	requireABI(t, files[names.Get("builtin.Error")+".clas.abap"], names.Get("runtime.described")+"~"+names.Get("builtin.nativeConstructor"), "result = me->"+names.Get("builtin.nativeConstructor")+"( ).")
}

func TestRootClassOfABAP(t *testing.T) {
	root := hir.Ref(hir.RootObject)
	cv := hir.T(hir.ClassValue)
	p := &hir.Program{Classes: []*hir.Class{{Name: "Native"}, {Name: "Child", Super: "Native"}, {Name: "Record", Structural: true}}}
	for i, ty := range []hir.Type{root, hir.T(hir.Optional, root), hir.Ref("Record")} {
		m := method([]string{"root", "optional", "record"}[i], cv, ret(rt("object.classOf", local("input", ty), cv)))
		m.Params = []hir.Param{{Name: "input", Type: ty}}
		p.Classes[0].Methods = append(p.Classes[0].Methods, m)
	}
	files, names := abiSource(t, p)
	src := files[names.Get("Native")+".clas.abap"]
	requireABI(t, src, "IS NOT BOUND.", "CATCH cx_sy_move_cast_error.", "->"+names.Get("builtin.nativeConstructor")+"( )", "IF result IS NOT BOUND.")
	requireABI(t, files[names.Get("Child")+".clas.abap"], names.Get("builtin.nativeConstructor")+" REDEFINITION.", names.Get("builtin.classOf")+" REDEFINITION.")
	record := files[names.Get("Record")+".clas.abap"]
	requireABI(t, record, "METHOD "+names.Get("builtin.nativeConstructor")+".\nRAISE EXCEPTION TYPE "+names.Get("class.refusal"))
	requireABI(t, files[names.Get("runtime.described")+".intf.abap"], names.Get("builtin.nativeConstructor"))
}

func TestClassArgsFactoryABAP(t *testing.T) {
	cv, dyn := hir.T(hir.ClassValue), hir.T(hir.Dynamic)
	arr := hir.T(hir.Array, dyn)
	types := []hir.Type{str, boolean, hir.T(hir.Number), dyn, cv, hir.Ref("Payload"), {Kind: hir.InterfaceRef, Name: "I"}, hir.T(hir.Optional, str), hir.T(hir.Optional, hir.Ref("Payload")), hir.T(hir.Optional, boolean), hir.T(hir.Optional, hir.T(hir.Number)), hir.T(hir.Optional, cv), hir.Ref(hir.RootObject)}
	ctor := method("constructor", hir.T(hir.Void), hir.B())
	for i, ty := range types {
		ctor.Params = append(ctor.Params, hir.Param{Name: []string{"s", "b", "n", "d", "c", "r", "i", "o", "or", "ob", "on", "oc", "root"}[i], Type: ty})
	}
	m := method("make", hir.Ref(hir.RootObject), ret(rt("classvalue.newArgs", local("ctor", cv), hir.Ref(hir.RootObject), local("args", arr))))
	m.Params = []hir.Param{{Name: "ctor", Type: cv}, {Name: "args", Type: arr}}
	init := method("class_constructor", hir.T(hir.Void), hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.StaticGet, Owner: "ImplicitInit", Name: "count", Type: i32}, Y: binary("+", &hir.Expr{Kind: hir.StaticGet, Owner: "ImplicitInit", Name: "count", Type: i32}, lit(1), i32)}))
	init.Static = true
	p := &hir.Program{Interfaces: []*hir.Interface{{Name: "I"}}, Classes: []*hir.Class{{Name: "Payload", Implements: []string{"I"}}, {Name: "Args", Ctor: ctor}, {Name: "InheritedArgs", Super: "Args"}, {Name: "ImplicitInit", Fields: []hir.Field{{Name: "count", Type: i32, Static: true}}, Methods: []*hir.Method{init}}, {Name: "Caller", Methods: []*hir.Method{m}}}}
	files, names := abiSource(t, p)
	factory := files[names.Get("runtime.classvalue.factory")+".clas.abap"]
	requireABI(t, factory, "METHOD new_args.", "->as_string( )", "->as_boolean( )", "->as_number( )", "->as_classvalue( )", "->as_ref( )", "->has = abap_true.", "CREATE OBJECT result TYPE "+names.Get("Args")+" EXPORTING", "WHEN OTHERS.\nRAISE EXCEPTION TYPE "+names.Get("class.refusal"))
	requireABI(t, factory, "CREATE OBJECT result TYPE "+names.Get("InheritedArgs")+" EXPORTING", names.Get("ImplicitInit")+"=>"+names.Get("builtin.initialize.ImplicitInit")+"( ).")
	if strings.Count(factory, "READ TABLE p1->items INDEX ") != 2*len(types) {
		t.Fatal("missing/extra argument reads (excess args must be ignored)")
	}
	requireABI(t, files[names.Get("Caller")+".clas.abap"], "=>new_args( p0 =", " p1 =", " ?= ")
}

func TestClassArgsFactoryRefusesUnsupportedStorageABAP(t *testing.T) {
	cv, dyn := hir.T(hir.ClassValue), hir.T(hir.Dynamic)
	p := &hir.Program{Classes: []*hir.Class{{Name: "Caller"}}}
	m := method("make", hir.Ref(hir.RootObject), ret(rt("classvalue.newArgs", local("ctor", cv), hir.Ref(hir.RootObject), local("args", hir.T(hir.Array, dyn)))))
	m.Params = []hir.Param{{Name: "ctor", Type: cv}, {Name: "args", Type: hir.T(hir.Array, dyn)}}
	p.Classes[0].Methods = []*hir.Method{m}
	for i, ty := range []hir.Type{i32, hir.T(hir.I64), hir.T(hir.Array, str), hir.T(hir.OrderedMap, str, str), hir.T(hir.OrderedSet, str), hir.T(hir.RegExp), hir.T(hir.Optional, i32)} {
		ctor := method("constructor", hir.T(hir.Void), hir.B())
		ctor.Params = []hir.Param{{Name: "value", Type: ty}}
		p.Classes = append(p.Classes, &hir.Class{Name: "Refused" + string(rune('A'+i)), Ctor: ctor})
	}
	variadic := method("constructor", hir.T(hir.Void), hir.B())
	variadic.Params = []hir.Param{{Name: "rest", Type: hir.T(hir.Array, dyn), Variadic: true}}
	large := method("constructor", hir.T(hir.Void), hir.B())
	for i := 0; i < 41; i++ {
		large.Params = append(large.Params, hir.Param{Name: "arg" + strings.Repeat("x", i), Type: str})
	}
	p.Classes = append(p.Classes, &hir.Class{Name: "Abstract", Abstract: true}, &hir.Class{Name: "Variadic", Ctor: variadic}, &hir.Class{Name: "LargeBehavioral", Ctor: large})
	files, names := abiSource(t, p)
	factory := files[names.Get("runtime.classvalue.factory")+".clas.abap"]
	args := factory[strings.Index(factory, "METHOD new_args."):]
	for _, c := range p.Classes[1:] {
		requireABI(t, args, "WHEN `"+strings.ToUpper(names.Get(c.Name))+"`.\nRAISE EXCEPTION TYPE "+names.Get("class.refusal")+".")
		if strings.Contains(args, "CREATE OBJECT result TYPE "+names.Get(c.Name)) {
			t.Fatalf("unsupported constructor %s supplied initial arguments", c.Name)
		}
	}
}

func TestFreshClassDescriptorABAP(t *testing.T) {
	cv := hir.T(hir.ClassValue)
	fresh := func() *hir.Expr { return &hir.Expr{Kind: hir.FreshClassOf, Owner: "Local", Type: cv} }
	m := method("make", cv, hir.B(decl("first", cv, fresh()), ret(fresh())))
	p := &hir.Program{Classes: []*hir.Class{{Name: "Local", Scoped: true}, {Name: "Caller", Methods: []*hir.Method{m}}}}
	files, names := abiSource(t, p)
	caller := files[names.Get("Caller")+".clas.abap"]
	if strings.Count(caller, "= NEW "+names.Get("runtime.classvalue")) != 2 || strings.Count(caller, "->lexical = abap_true.") != 2 {
		t.Fatal("fresh evaluations must allocate distinct lexical descriptors")
	}
	requireABI(t, files[names.Get("Local")+".clas.abap"], "result = me->"+names.Get("class.lexicalDescriptor")+".")
	requireABI(t, files[names.Get("runtime.classvalue.factory")+".clas.abap"], "IF p0->lexical = abap_true.", "->"+names.Get("class.lexicalDescriptor")+" = p0.")
}

func TestFreshClassDescriptorRefusesUnprovenABAP(t *testing.T) {
	for _, c := range []*hir.Class{{Name: "Local"}, {Name: "Local", Scoped: true, Abstract: true}, {Name: "Local", Scoped: true, Super: "Base"}, {Name: "Local", Scoped: true, Ctor: &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Params: []hir.Param{{Name: "x", Type: str}}, Body: hir.B()}}} {
		m := method("fresh", hir.T(hir.ClassValue), ret(&hir.Expr{Kind: hir.FreshClassOf, Owner: "Local", Type: hir.T(hir.ClassValue)}))
		p := &hir.Program{Classes: []*hir.Class{{Name: "Base"}, c, {Name: "Caller", Methods: []*hir.Method{m}}}}
		_, err := Emit(p)
		if err == nil || !(strings.Contains(err.Error(), "fresh lexical constructor descriptor requires") || strings.Contains(err.Error(), "freshclassof requires a scoped class")) {
			t.Fatalf("unproven lexical class accepted: %v", err)
		}
	}
}
