package rewrite

import (
	"reflect"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
)

func TestStoreGuards(t *testing.T) {
	source, err := ruleFiles.ReadFile("rules/stores.grace")
	if err != nil {
		t.Fatal(err)
	}
	// Preserve priority ordering but bypass the embedded-rule fast path.
	_, unfiltered, err := Parse(strings.Replace(string(source), "copy-local 20", "copy-local 21", 1))
	if err != nil {
		t.Fatal(err)
	}
	i, b := hir.T(hir.I32), hir.T(hir.Bool)
	decl := func(n string, x *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.VarDecl, Name: n, Type: i, X: x} }
	ret := func(x *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Return, X: x} }
	assign := func(n string, x *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Assign, X: hir.V(n, i), Y: x} }
	call := func() *hir.Expr { return &hir.Expr{Kind: hir.DirectCall, Owner: "C", Name: "effect", Type: i} }
	cases := []struct {
		name      string
		body      *hir.Stmt
		copy, dse int
	}{
		{"single-use", hir.B(decl("v", hir.L(i, 3)), ret(hir.V("v", i))), 1, 0},
		{"single-assign", hir.B(decl("v", nil), assign("v", hir.L(i, 3)), ret(hir.V("v", i))), 1, 0},
		{"intervening-unrelated-store", hir.B(decl("x", nil), decl("v", hir.V("x", i)), decl("unused", hir.L(i, 1)), ret(hir.V("v", i))), 1, 1},
		{"intervening-write", hir.B(decl("x", nil), decl("v", hir.V("x", i)), assign("x", hir.L(i, 2)), ret(hir.V("v", i))), 0, 1},
		{"effect-next", hir.B(decl("v", call()), ret(hir.V("v", i))), 1, 0},
		{"effect-reordering", hir.B(decl("v", call()), &hir.Stmt{Kind: hir.ExprStmt, X: call()}, ret(hir.V("v", i))), 0, 0},
		{"exception-between", hir.B(decl("v", call()), &hir.Stmt{Kind: hir.Trap, Name: "test:1:1", Node: hir.Node{Source: "test"}}, ret(hir.V("v", i))), 0, 0},
		{"loop-duplication", hir.B(decl("v", call()), &hir.Stmt{Kind: hir.While, X: hir.L(b, false), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: hir.V("v", i)})}, ret(hir.L(i, 0))), 0, 0},
		{"finally-read", hir.B(decl("v", hir.L(i, 1)), &hir.Stmt{Kind: hir.Finally, Body: hir.B(&hir.Stmt{Kind: hir.Trap, Name: "test:1:1", Node: hir.Node{Source: "test"}}), Else: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: hir.V("v", i)})}, ret(hir.L(i, 0))), 0, 0},
		{"try-boundary", hir.B(decl("v", call()), &hir.Stmt{Kind: hir.Try, Name: "err", Type: i, Body: hir.B(ret(hir.V("v", i))), Else: hir.B(ret(hir.L(i, 0)))}), 0, 0},
		{"unused-raising", hir.B(decl("v", call()), ret(hir.L(i, 0))), 0, 0},
		{"unused-pure", hir.B(decl("v", hir.L(i, 1)), ret(hir.L(i, 0))), 0, 1},
		{"killed-store", hir.B(decl("v", hir.L(i, 1)), assign("v", call()), ret(hir.V("v", i))), 1, 1},
		{"two-uses", hir.B(decl("v", hir.L(i, 1)), ret(&hir.Expr{Kind: hir.Binary, Op: "+", Type: i, X: hir.V("v", i), Y: hir.V("v", i)})), 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			makeP := func() *hir.Program {
				return &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Result: i, Body: cloneStoreStmt(tc.body)}, {Name: "effect", Static: true, Result: i, Body: hir.B(&hir.Stmt{Kind: hir.Throw, X: hir.L(i, 1)})}}}}}
			}
			p, q, z := makeP(), makeP(), makeP()
			stats, err := CopyProp(p)
			if err != nil {
				t.Fatal(err)
			}
			if stats.CopySites != tc.copy || stats.DeadStores != tc.dse {
				t.Fatalf("%s want copy=%d dse=%d", StoreReport(stats), tc.copy, tc.dse)
			}
			t.Setenv("ABAPITI_GRACE_RECOMPUTE", "full")
			other, err := CopyProp(q)
			if err != nil {
				t.Fatal(err)
			}
			if hir.Dump(p) != hir.Dump(q) || !reflect.DeepEqual(stats, other) {
				t.Fatal("nondeterministic")
			}
			generic, err := Rewrite(z, unfiltered, Limits{Rounds: 16})
			if err != nil {
				t.Fatal(err)
			}
			if hir.Dump(p) != hir.Dump(z) || !reflect.DeepEqual(stats, generic) {
				t.Fatal("filtered actions differ from generic adapter")
			}
			before := hir.Dump(p)
			again, err := CopyProp(p)
			if err != nil {
				t.Fatal(err)
			}
			if again.CopySites+again.DeadStores != 0 || hir.Dump(p) != before {
				t.Fatal("not idempotent")
			}
		})
	}
}
func cloneStoreStmt(s *hir.Stmt) *hir.Stmt {
	if s == nil {
		return nil
	}
	out := *s
	out.X = cloneStoreExpr(s.X)
	out.Y = cloneStoreExpr(s.Y)
	out.Body = cloneStoreStmt(s.Body)
	out.Else = cloneStoreStmt(s.Else)
	out.List = make([]*hir.Stmt, len(s.List))
	for i, v := range s.List {
		out.List[i] = cloneStoreStmt(v)
	}
	return &out
}
func cloneStoreExpr(e *hir.Expr) *hir.Expr {
	if e == nil {
		return nil
	}
	out := *e
	out.X = cloneStoreExpr(e.X)
	out.Y = cloneStoreExpr(e.Y)
	out.Z = cloneStoreExpr(e.Z)
	out.Stmt = cloneStoreStmt(e.Stmt)
	out.Args = make([]*hir.Expr, len(e.Args))
	for i, v := range e.Args {
		out.Args[i] = cloneStoreExpr(v)
	}
	return &out
}

func TestStoreTypeAndAliasGuards(t *testing.T) {
	i, n := hir.T(hir.I32), hir.T(hir.Optional, hir.T(hir.I32))
	a := hir.T(hir.Array, i)
	cases := []struct {
		name      string
		body      *hir.Stmt
		params    []hir.Param
		result    hir.Type
		copy, dse int
	}{
		{"conversion", hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "v", Type: n, X: hir.L(i, 1)}, &hir.Stmt{Kind: hir.Return, X: hir.V("v", n)}), nil, n, 0, 0},
		{"runtime-read", hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "v", Type: i, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: i, X: hir.V("a", a)}}, &hir.Stmt{Kind: hir.Return, X: hir.V("v", i)}), []hir.Param{{Name: "a", Type: a}}, i, 1, 0},
		{"alias-write", hir.B(
			&hir.Stmt{Kind: hir.VarDecl, Name: "alias", Type: a, X: hir.V("a", a)},
			&hir.Stmt{Kind: hir.VarDecl, Name: "v", Type: i, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: i, X: hir.V("a", a)}},
			&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: i, X: hir.V("alias", a), Args: []*hir.Expr{hir.L(i, 3)}}},
			&hir.Stmt{Kind: hir.Return, X: hir.V("v", i)}), []hir.Param{{Name: "a", Type: a}}, i, 0, 0},
		{"unused-allocation", hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "v", Type: a, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.slice0", Type: a, X: hir.V("a", a)}}, &hir.Stmt{Kind: hir.Return, X: hir.L(i, 0)}), []hir.Param{{Name: "a", Type: a}}, i, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Params: tc.params, Result: tc.result, Body: tc.body}}}}}
			st, err := CopyProp(p)
			if err != nil {
				t.Fatal(err)
			}
			if st.CopySites != tc.copy || st.DeadStores != tc.dse {
				t.Fatalf("%s", StoreReport(st))
			}
		})
	}
}

func TestStoreClosureCapture(t *testing.T) {
	i := hir.T(hir.I32)
	p := &hir.Program{Classes: []*hir.Class{
		{Name: "closure.1", Fields: []hir.Field{{Name: "v", Type: i}}, Ctor: &hir.Method{Name: "constructor", Params: []hir.Param{{Name: "v", Type: i}}, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Name: "v", Type: i, X: &hir.Expr{Kind: hir.This, Type: hir.Ref("closure.1")}}, Y: hir.V("v", i)})}},
		{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Result: hir.Ref("closure.1"), Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "v", Type: i, X: hir.L(i, 1)}, &hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Owner: "closure.1", Type: hir.Ref("closure.1"), Args: []*hir.Expr{hir.V("v", i)}}})}}},
	}}
	st, err := CopyProp(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.CopySites+st.DeadStores != 0 {
		t.Fatal(StoreReport(st))
	}
}

func TestStoreDependentPlans(t *testing.T) {
	i := hir.T(hir.I32)
	makeP := func(effectful bool) *hir.Program {
		rhs := hir.L(i, 3)
		if effectful {
			rhs = &hir.Expr{Kind: hir.DirectCall, Owner: "C", Name: "effect", Type: i}
		}
		return &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{
			{Name: "run", Static: true, Result: i, Body: hir.B(
				&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: i, X: rhs},
				&hir.Stmt{Kind: hir.VarDecl, Name: "b", Type: i, X: hir.V("a", i)},
				&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.DirectCall, Owner: "C", Name: "effect", Type: i}},
				&hir.Stmt{Kind: hir.Return, X: hir.V("b", i)})},
			{Name: "effect", Static: true, Result: i, Body: hir.B(&hir.Stmt{Kind: hir.Throw, X: hir.L(i, 1)})},
		}}}}
	}
	for _, effectful := range []bool{false, true} {
		p := makeP(effectful)
		st, err := CopyProp(p)
		if err != nil {
			t.Fatal(err)
		}
		body := p.Classes[0].Methods[0].Body
		if effectful {
			if st.CopySites != 1 || body.List[1].X.Kind != hir.DirectCall || body.List[3].X.Kind != hir.Local {
				t.Fatal("effectful dependent copy crossed a call")
			}
		} else {
			if st.CopySites != 1 || body.List[1].X.Kind != hir.Lit {
				t.Fatal("dependent copy lost its initializer")
			}
		}
	}
}

func TestStoreOptIn(t *testing.T) {
	i := hir.T(hir.I32)
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Result: i, Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "v", Type: i, X: hir.L(i, 1)}, &hir.Stmt{Kind: hir.Return, X: hir.V("v", i)})}}}}}
	t.Setenv("ABAPITI_COPYPROP", "")
	before := hir.Dump(p)
	st, err := BeforeEmission(p)
	if err != nil || st.CopySites != 0 || hir.Dump(p) != before {
		t.Fatal("default pass mutated HIR")
	}
	t.Setenv("ABAPITI_COPYPROP", "1")
	st, err = BeforeEmission(p)
	if err != nil || st.CopySites != 1 {
		t.Fatalf("opt-in: %v %s", err, StoreReport(st))
	}
}

func TestStoreCompactCFGParity(t *testing.T) {
	i := hir.T(hir.I32)
	body := hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "v", Type: i, X: hir.L(i, 1)},
		&hir.Stmt{Kind: hir.Finally, Body: hir.B(&hir.Stmt{Kind: hir.Try, Name: "err", Type: i, Body: hir.B(&hir.Stmt{Kind: hir.Throw, X: hir.V("v", i)}), Else: hir.B(&hir.Stmt{Kind: hir.Assign, X: hir.V("v", i), Y: hir.V("err", i)})}), Else: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: hir.V("v", i)})},
		&hir.Stmt{Kind: hir.Return, X: hir.V("v", i)})
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Result: i, Body: body}}}}}
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs)
	}
	full, compact := Extract(p), extractStoreFacts(p, nil, nil)
	for _, pred := range []string{"def", "use", "next", "local_ref", "observed"} {
		if !reflect.DeepEqual(full.Facts(pred), compact.Facts(pred)) {
			t.Fatalf("compact differs: %s", pred)
		}
	}
}

func TestStoreActionsRecheckSafety(t *testing.T) {
	i := hir.T(hir.I32)
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Result: i, Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "v", Type: i, X: hir.L(i, 1)}, &hir.Stmt{Kind: hir.Return, X: hir.V("v", i)})}}}}}
	_, rules, err := Parse(`(grace unsafe-removal 1 (match (node ?site var)) (where) (action (remove-statement ?site)))`)
	if err != nil {
		t.Fatal(err)
	}
	before := hir.Dump(p)
	stats, err := Rewrite(p, rules, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.DeadStores != 0 || hir.Dump(p) != before {
		t.Fatal("adapter removed a read store")
	}
}

func TestStoreNativeArithmeticRaises(t *testing.T) {
	for _, kind := range []hir.Kind{hir.I32, hir.I64, hir.Number} {
		typ := hir.T(kind)
		for _, op := range []string{"+", "-", "*", "negate"} {
			t.Run(string(kind)+op, func(t *testing.T) {
				rhs := &hir.Expr{Kind: hir.Binary, Op: op, Type: typ, X: hir.V("x", typ), Y: hir.V("y", typ)}
				if op == "negate" {
					rhs.Kind = hir.Unary
					rhs.Op = "-"
					rhs.Y = nil
				}
				p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Params: []hir.Param{{Name: "x", Type: typ}, {Name: "y", Type: typ}}, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "unused", Type: typ, X: rhs})}}}}}
				before := hir.Dump(p)
				st, err := CopyProp(p)
				if err != nil {
					t.Fatal(err)
				}
				if st.CopySites+st.DeadStores != 0 || hir.Dump(p) != before {
					t.Fatal("unused native overflow/finite check was removed")
				}
			})
		}
	}
}

func TestStoreUninitializedRuntimeReceiver(t *testing.T) {
	i := hir.T(hir.I32)
	arr := hir.T(hir.Array, i)
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{{Name: "run", Static: true, Result: hir.T(hir.Void), Body: hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Name: "a", Type: arr},
		&hir.Stmt{Kind: hir.VarDecl, Name: "unused", Type: i, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: i, X: hir.V("a", arr)}},
	)}}}}}
	before := hir.Dump(p)
	stats, err := CopyProp(p)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CopySites+stats.DeadStores != 0 || hir.Dump(p) != before {
		t.Fatal("removed a potentially nil receiver dereference")
	}
}

func TestStoreSharedSyntax(t *testing.T) {
	i := hir.T(hir.I32)
	local := hir.V("v", i)
	makeMethod := func(name string, n int) *hir.Method {
		return &hir.Method{Name: name, Static: true, Result: i, Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "v", Type: i, X: hir.L(i, n)}, &hir.Stmt{Kind: hir.Return, X: local})}
	}
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{makeMethod("a", 1), makeMethod("b", 2)}}}}
	stats, err := CopyProp(p)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CopySites != 2 {
		t.Fatal(StoreReport(stats))
	}
	for j, m := range p.Classes[0].Methods {
		if m.Body.List[1].X.Kind != hir.Lit || m.Body.List[1].X.Value != j+1 {
			t.Fatal("shared local crossed method bindings")
		}
	}
	shared := &hir.Stmt{Kind: hir.VarDecl, Name: "v", Type: i, X: hir.L(i, 7)}
	p = &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{
		{Name: "a", Static: true, Result: i, Body: hir.B(shared, &hir.Stmt{Kind: hir.Return, X: hir.V("v", i)})},
		{Name: "b", Static: true, Result: i, Body: hir.B(shared, &hir.Stmt{Kind: hir.Return, X: hir.L(i, 0)})},
	}}}}
	stats, err = CopyProp(p)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CopySites != 1 || stats.DeadStores != 1 || p.Classes[0].Methods[0].Body.List[1].X.Value != 7 {
		t.Fatal("shared statement lost a live store")
	}
	before := hir.Dump(p)
	stats, err = CopyProp(p)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CopySites+stats.DeadStores != 0 || hir.Dump(p) != before {
		t.Fatal("shared syntax not idempotent")
	}
}

func TestStoreSharedMethod(t *testing.T) {
	i := hir.T(hir.I32)
	method := &hir.Method{Name: "run", Static: true, Params: []hir.Param{{Name: "x", Type: i}}, Result: i, Body: hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Name: "v", Type: i, X: hir.V("x", i)}, &hir.Stmt{Kind: hir.Return, X: hir.V("v", i)})}
	p := &hir.Program{Classes: []*hir.Class{{Name: "A", Methods: []*hir.Method{method}}, {Name: "B", Methods: []*hir.Method{method}}}}
	stats, err := CopyProp(p)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CopySites != 2 || len(stats.Methods) != 2 {
		t.Fatal(StoreReport(stats))
	}
	for _, c := range p.Classes {
		ret := c.Methods[0].Body.List[1].X
		if ret.Kind != hir.Local || ret.Name != "x" {
			t.Fatal("lost the frame parameter")
		}
	}
}

func TestStoreParameterScope(t *testing.T) {
	i := hir.T(hir.I32)
	p := &hir.Program{Classes: []*hir.Class{{Name: "src/local/demo.C", Methods: []*hir.Method{{Name: "run", Static: true, Params: []hir.Param{{Name: "p", Type: i}}, Result: i, Body: hir.B(
		&hir.Stmt{Kind: hir.Assign, X: hir.V("p", i), Y: hir.L(i, 2)},
		&hir.Stmt{Kind: hir.Return, X: hir.L(i, 0)},
	)}}}}}
	before := hir.Dump(p)
	stats, err := CopyProp(p)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CopySites+stats.DeadStores != 0 || hir.Dump(p) != before {
		t.Fatal("source path treated a parameter as a local declaration")
	}
}
