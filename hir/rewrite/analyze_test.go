package rewrite

import (
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

var integer = hir.T(hir.I32)

func method(name string, ss ...*hir.Stmt) *hir.Method {
	return &hir.Method{Name: name, Static: true, Result: hir.T(hir.Void), Body: hir.B(ss...)}
}
func exp(e *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.ExprStmt, X: e} }
func call(owner, name string, args ...*hir.Expr) *hir.Expr {
	return &hir.Expr{Kind: hir.DirectCall, Owner: owner, Name: name, Type: hir.T(hir.Void), Args: args}
}
func get(owner, name string, t hir.Type) *hir.Expr {
	return &hir.Expr{Kind: hir.StaticGet, Owner: owner, Name: name, Type: t}
}
func runtimeExpr(op string, t hir.Type, recv *hir.Expr, args ...*hir.Expr) *hir.Expr {
	return &hir.Expr{Kind: hir.RuntimeOp, Op: op, Type: t, X: recv, Args: args}
}
func store(a, b *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Assign, X: a, Y: b} }
func ret(e *hir.Expr) *hir.Stmt      { return &hir.Stmt{Kind: hir.Return, X: e} }
func variable(n string, t hir.Type, e *hir.Expr) *hir.Stmt {
	return &hir.Stmt{Kind: hir.VarDecl, Name: n, Type: t, X: e}
}
func analyze(t *testing.T, cs ...*hir.Class) *DB {
	t.Helper()
	d, e := Analyze(&hir.Program{Classes: cs})
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func assertFact(t *testing.T, d *DB, want bool, p string, a ...string) {
	t.Helper()
	if d.Has(p, a...) != want {
		t.Errorf("%s%v = %v, want %v", p, a, !want, want)
	}
}

func TestCallCycleEffects(t *testing.T) {
	c := &hir.Class{Name: "C", Fields: []hir.Field{{Name: "n", Type: integer, Static: true}}}
	c.Methods = []*hir.Method{method("a", exp(call("C", "b"))), method("b", exp(call("C", "a")), store(get("C", "n", integer), hir.L(integer, 1))), method("cleanA", exp(call("C", "cleanB"))), method("cleanB", exp(call("C", "cleanA"))), method("trap", &hir.Stmt{Name: "fixture", Kind: hir.Trap}), method("raise", &hir.Stmt{Kind: hir.Throw, Type: integer, X: hir.L(integer, 1)}), method("caught", &hir.Stmt{Kind: hir.Try, Name: "err", Type: integer, Body: hir.B(exp(call("C", "raise"))), Else: hir.B()})}
	d := analyze(t, c)
	for _, m := range []string{"a", "b"} {
		assertFact(t, d, false, "pure", "C::"+m)
		assertFact(t, d, true, "writes_static_transitive", "C::"+m, "C", "n")
		assertFact(t, d, false, "may_throw", "C::"+m)
	}
	for _, m := range []string{"cleanA", "cleanB"} {
		assertFact(t, d, true, "pure", "C::"+m)
		assertFact(t, d, false, "writes_static_transitive", "C::"+m, "C", "n")
	}
	for _, m := range []string{"trap", "raise", "caught"} {
		assertFact(t, d, true, "may_throw", "C::"+m)
	}
}
func TestVirtualReceivers(t *testing.T) {
	base := &hir.Class{Name: "Base", Abstract: true, Methods: []*hir.Method{{Name: "f", Virtual: true, Abstract: true, Result: hir.T(hir.Void)}}}
	a := &hir.Class{Name: "A", Super: "Base", Methods: []*hir.Method{{Name: "f", Virtual: true, Result: hir.T(hir.Void), Body: hir.B()}}}
	b := &hir.Class{Name: "B", Super: "Base", Methods: []*hir.Method{{Name: "f", Virtual: true, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Name: "fixture", Kind: hir.Trap})}}}
	vc := func(r *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.VirtualCall, Name: "f", X: r, Type: hir.T(hir.Void)}
	}
	broad := method("broad", exp(vc(hir.V("r", hir.Ref("Base")))))
	broad.Params = []hir.Param{{Name: "r", Type: hir.Ref("Base")}}
	narrow := method("narrow", variable("a", hir.Ref("Base"), &hir.Expr{Kind: hir.New, Type: hir.Ref("A")}), variable("alias", hir.Ref("Base"), hir.V("a", hir.Ref("Base"))), exp(vc(hir.V("alias", hir.Ref("Base")))))
	reassigned := method("reassigned", variable("a", hir.Ref("Base"), &hir.Expr{Kind: hir.New, Type: hir.Ref("A")}), store(hir.V("a", hir.Ref("Base")), &hir.Expr{Kind: hir.New, Type: hir.Ref("B")}), exp(vc(hir.V("a", hir.Ref("Base")))))
	d := analyze(t, base, a, b, &hir.Class{Name: "Caller", Methods: []*hir.Method{broad, narrow, reassigned}})
	sites := map[string]int{}
	for _, r := range d.Facts("receivers") {
		sites[r[0]]++
	}
	for _, r := range d.Facts("virtual_call") {
		want := 2
		if r[0] == "Caller::narrow" {
			want = 1
			assertFact(t, d, true, "receivers", r[3], "A")
			assertFact(t, d, false, "receivers", r[3], "B")
		}
		if sites[r[3]] != want {
			t.Errorf("%s receivers %d", r[0], sites[r[3]])
		}
	}
	assertFact(t, d, true, "may_throw", "Caller::broad")
	assertFact(t, d, false, "may_throw", "Caller::narrow")
	assertFact(t, d, true, "pure", "Caller::narrow")
}
func TestEscapeAndLocalMutation(t *testing.T) {
	arr := hir.T(hir.Array, integer)
	newArr := func() *hir.Expr { return &hir.Expr{Kind: hir.New, Type: arr} }
	v := func(n string) *hir.Expr { return hir.V(n, arr) }
	push := func(n string) *hir.Stmt { return exp(runtimeExpr("array.push", integer, v(n), hir.L(integer, 1))) }
	c := &hir.Class{Name: "C", Fields: []hir.Field{{Name: "saved", Static: true, Type: arr}}}
	save := method("save", store(get("C", "saved", arr), v("p")))
	save.Params = []hir.Param{{Name: "p", Type: arr}}
	forward := method("forward", exp(call("C", "save", v("p"))))
	forward.Params = save.Params
	local := method("local", variable("a", arr, newArr()), push("a"))
	escape := method("escape", variable("a", arr, newArr()), variable("alias", arr, v("a")), push("a"), exp(call("C", "forward", v("alias"))))
	param := method("param", push("p"))
	param.Params = save.Params
	returned := method("returned", variable("a", arr, newArr()), ret(v("a")))
	returned.Result = arr
	c.Methods = []*hir.Method{save, forward, local, escape, param, returned}
	d := analyze(t, c)
	assertFact(t, d, true, "pure", "C::local")
	assertFact(t, d, false, "pure", "C::escape")
	assertFact(t, d, false, "pure", "C::param")
	for _, m := range []string{"save", "forward"} {
		assertFact(t, d, true, "escapes", "C::"+m, "C::"+m+"/param/p")
	}
	for _, m := range []string{"escape", "returned"} {
		assertFact(t, d, true, "escapes", "C::"+m, "C::"+m+"/body/s0/local/a")
	}
	assertFact(t, d, false, "escapes", "C::local", "C::local/body/s0/local/a")
}
func TestRuntimeAndInitialization(t *testing.T) {
	arr := hir.T(hir.Array, integer)
	c := &hir.Class{Name: "C", Fields: []hir.Field{{Name: "values", Static: true, Type: arr}, {Name: "ready", Static: true, Type: hir.T(hir.Bool)}}}
	init := method("class_constructor", store(get("C", "values", arr), &hir.Expr{Kind: hir.New, Type: arr}))
	use := method("use", exp(runtimeExpr("array.push", integer, get("C", "values", arr), hir.L(integer, 1))))
	bounds := method("bounds", exp(runtimeExpr("string.charCodeAt", integer, hir.L(hir.T(hir.String), "s"), hir.L(integer, 1))))
	length := method("length", exp(runtimeExpr("string.length", integer, hir.L(hir.T(hir.String), "s"))))
	c.Methods = []*hir.Method{init, use, bounds, length}
	d := analyze(t, c)
	assertFact(t, d, true, "lazy_init", "C")
	assertFact(t, d, true, "writes_static", "C::use", "C", "values")
	assertFact(t, d, true, "writes_static_transitive", "C::use", "C", "values")
	assertFact(t, d, true, "may_throw", "C::bounds")
	assertFact(t, d, false, "may_throw", "C::length")
	assertFact(t, d, true, "pure", "C::length")
	assertFact(t, d, false, "pure", "C::use")
	flag := get("Flag", "ready", hir.T(hir.Bool))
	f := &hir.Class{Name: "Flag", Fields: []hir.Field{{Name: "ready", Static: true, Type: hir.T(hir.Bool)}}, Methods: []*hir.Method{method("init", &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: flag}, Body: hir.B(store(flag, hir.L(hir.T(hir.Bool), true)))})}}
	d = analyze(t, f, &hir.Class{Name: "Plain"})
	assertFact(t, d, true, "lazy_init", "Flag")
	assertFact(t, d, false, "lazy_init", "Plain")
}
func memoMethod(name, compute string) *hir.Method {
	mt := hir.T(hir.OrderedMap, integer, integer)
	cache := get("Cache", "data", mt)
	k := hir.V("k", integer)
	calc := &hir.Expr{Kind: hir.DirectCall, Owner: "Cache", Name: compute, Type: integer, Args: []*hir.Expr{k}}
	m := method(name, &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: runtimeExpr("map.has", hir.T(hir.Bool), cache, k)}, Body: hir.B(variable("v", integer, calc), exp(runtimeExpr("map.set", mt, cache, k, hir.V("v", integer))))}, ret(runtimeExpr("map.get", hir.T(hir.Optional, integer), cache, k)))
	m.Params = []hir.Param{{Name: "k", Type: integer}}
	m.Result = hir.T(hir.Optional, integer)
	return m
}
func TestMemoAndCounterShapes(t *testing.T) {
	compute := method("compute", ret(hir.V("k", integer)))
	compute.Result = integer
	compute.Params = []hir.Param{{Name: "k", Type: integer}}
	bad := method("bad", store(get("Cache", "n", integer), hir.L(integer, 0)), ret(hir.V("k", integer)))
	bad.Result = integer
	bad.Params = compute.Params
	memo := memoMethod("memo", "compute")
	effectful := memoMethod("effectful", "bad")
	wrong := memoMethod("wrong", "compute")
	wrong.Body.List[1].X.Args[0] = hir.L(integer, 2)
	n := get("Cache", "n", integer)
	inc := store(n, &hir.Expr{Kind: hir.Binary, Op: "+", Type: integer, X: n, Y: hir.L(integer, 1)})
	c := &hir.Class{Name: "Cache", Fields: []hir.Field{{Name: "data", Type: hir.T(hir.OrderedMap, integer, integer), Static: true}, {Name: "n", Type: integer, Static: true}}, Methods: []*hir.Method{compute, bad, memo, effectful, wrong, method("count", inc), method("mixed", inc, store(n, hir.L(integer, 0)))}}
	d := analyze(t, c)
	assertFact(t, d, true, "memo_store", "Cache::memo", "Cache", "data")
	for _, m := range []string{"effectful", "wrong"} {
		assertFact(t, d, false, "memo_store", "Cache::"+m, "Cache", "data")
	}
	assertFact(t, d, true, "counter_store", "Cache::count", "Cache", "n")
	assertFact(t, d, false, "counter_store", "Cache::mixed", "Cache", "n")
	report := Report(d)
	for _, s := range []string{"Cache::memo Cache.data memo", "Cache::count Cache.n counter", "Cache::mixed Cache.n other"} {
		if !strings.Contains(report, s) {
			t.Errorf("missing %s", s)
		}
	}
}
func TestRejectInvalidHIR(t *testing.T) {
	if _, e := Analyze(nil); e == nil {
		t.Fatal("accepted nil")
	}
}
