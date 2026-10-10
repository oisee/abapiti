package hir

import "testing"

// run(r) { var out = new; for x of r { push(out, x+1) }; return out } is
// cloned; a method that also reads r, or breaks out of the loop, forwards.
func TestSingleton(t *testing.T) {
	i32 := T(I32)
	arr := T(Array, i32)
	iface := Type{Kind: InterfaceRef, Name: "Runner"}
	push := func(arrName string, v *Expr) *Stmt {
		return &Stmt{Kind: ExprStmt, X: &Expr{Kind: RuntimeOp, Op: "array.push", Type: i32, X: V(arrName, arr), Args: []*Expr{v}}}
	}
	run := func(extra ...*Stmt) *Method {
		body := []*Stmt{{Kind: VarDecl, Name: "out", Type: arr, X: &Expr{Kind: New, Type: arr}}}
		loop := &Stmt{Kind: ForEach, Name: "x0", Type: i32, X: V("r", arr), Body: B(append([]*Stmt{push("out", &Expr{Kind: Binary, Op: "+", Type: i32, X: V("x0", i32), Y: L(i32, 1)})}, extra...)...)}
		body = append(body, loop, &Stmt{Kind: Return, X: V("out", arr)})
		return &Method{Name: "run", Virtual: true, Params: []Param{{Name: "r", Type: arr}}, Result: arr, Body: B(body...)}
	}
	breaking := run(&Stmt{Kind: Break})
	reading := run()
	reading.Body.List = append([]*Stmt{{Kind: ExprStmt, X: &Expr{Kind: RuntimeOp, Op: "array.length", Type: i32, X: V("r", arr)}}}, reading.Body.List...)
	lit := &Expr{Kind: Seq, Type: arr, Y: V("a1", arr), Stmt: B(
		&Stmt{Kind: VarDecl, Name: "a1", Type: arr, X: &Expr{Kind: New, Type: arr}}, push("a1", V("v", i32)))}
	caller := &Method{Name: "go", Static: true, Params: []Param{{Name: "k", Type: iface}, {Name: "v", Type: i32}}, Result: arr,
		Body: B(&Stmt{Kind: Return, X: &Expr{Kind: VirtualCall, Name: "run", Type: arr, X: V("k", iface), Args: []*Expr{lit}}})}
	p := &Program{
		Interfaces: []*Interface{{Name: "Runner", Methods: []*Method{{Name: "run", Virtual: true, Abstract: true, Params: []Param{{Name: "r", Type: arr}}, Result: arr}}}},
		Classes: []*Class{
			{Name: "Plain", Implements: []string{"Runner"}, Methods: []*Method{run()}},
			{Name: "Breaks", Implements: []string{"Runner"}, Methods: []*Method{breaking}},
			{Name: "Reads", Implements: []string{"Runner"}, Methods: []*Method{reading}},
			{Name: "Caller", Methods: []*Method{caller}},
		}}
	st := Singleton(p)
	if st.Methods != 1 || st.Clones != 1 || st.Defaults != 2 || st.CallSites != 1 {
		t.Fatalf("stats %+v", st)
	}
	if errs := Verify(p); len(errs) > 0 {
		t.Fatal(errs[0])
	}
	call := caller.Body.List[0].X
	if call.Name != "run_one" || len(call.Args) != 1 || call.Args[0].Kind != Local || call.Args[0].Name != "v" {
		t.Fatalf("call not rewritten: %s", Dump(p))
	}
	one := func(c int) *Method { return p.Classes[c].Methods[len(p.Classes[c].Methods)-1] }
	if one(0).Name != "run_one" || one(0).Params[0].Name != "x" || containsCall(one(0).Body, "run") {
		t.Fatalf("Plain.run_one is not the loop body:\n%s", Dump(p))
	}
	for _, c := range []int{1, 2} {
		if !containsCall(one(c).Body, "run") {
			t.Fatalf("%s.run_one must forward to run([x]):\n%s", p.Classes[c].Name, Dump(p))
		}
	}
	if m := p.Interfaces[0].Methods; len(m) != 2 || m[1].Name != "run_one" || !m[1].Abstract {
		t.Fatal("interface lacks run_one")
	}
}

func containsCall(s *Stmt, name string) bool {
	found := false
	walk(s, func(*Stmt) {}, func(x *Expr) {
		if x.Kind == VirtualCall && x.Name == name {
			found = true
		}
	})
	return found
}
