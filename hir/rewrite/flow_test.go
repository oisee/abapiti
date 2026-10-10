package rewrite_test

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
	"github.com/oisee/abapiti/internal/gracecheck"
	"testing"
)

func TestFlowLiveness(t *testing.T) {
	num := hir.T(hir.I32)
	arr := hir.T(hir.Array, num)
	v := func() *hir.Expr { return hir.V("a", arr) }
	read := func() *hir.Stmt {
		return &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: num, X: v()}}
	}
	cases := []struct {
		name string
		body *hir.Stmt
		last bool
	}{
		{"straight", hir.B(read(), read()), false},
		{"last", hir.B(read()), true},
		{"if-then", hir.B(read(), &hir.Stmt{Kind: hir.If, X: hir.L(hir.T(hir.Bool), true), Body: hir.B(read()), Else: hir.B()}), false},
		{"if-else", hir.B(read(), &hir.Stmt{Kind: hir.If, X: hir.L(hir.T(hir.Bool), true), Body: hir.B(), Else: hir.B(read())}), false},
		{"early-return", hir.B(read(), &hir.Stmt{Kind: hir.Return}, read()), true},
		{"kill", hir.B(read(), &hir.Stmt{Kind: hir.Assign, X: v(), Y: &hir.Expr{Kind: hir.New, Type: arr}}, read()), true},
		{"try-finally", hir.B(read(), &hir.Stmt{Kind: hir.Finally, Body: hir.B(), Else: hir.B(read())}), false},
		{"try-catch", hir.B(read(), &hir.Stmt{Kind: hir.Try, Name: "err", Type: num, Body: hir.B(&hir.Stmt{Kind: hir.Throw, Type: num, X: hir.L(num, 1)}), Else: hir.B(read())}), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Params: []hir.Param{{Name: "a", Type: arr}}, Body: tc.body}
			p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{m}}}}
			db, err := rewrite.Analyze(p)
			if err != nil {
				t.Fatal(err)
			}
			if got := db.Has("not_read_after", "C::run/param/a", "C::run/body/s0/x/0"); got != tc.last {
				t.Fatalf("last=%t want %t", got, tc.last)
			}
			gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
		})
	}
}
func TestFlowNestedLoopsAndSeq(t *testing.T) {
	num := hir.T(hir.I32)
	arr := hir.T(hir.Array, num)
	read := func() *hir.Stmt {
		return &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: num, X: hir.V("a", arr)}}
	}
	inner := &hir.Stmt{Kind: hir.ForEach, Name: "j", Type: num, X: hir.V("a", arr), Body: hir.B(read(), &hir.Stmt{Kind: hir.Continue})}
	outer := &hir.Stmt{Kind: hir.ForEach, Name: "i", Type: num, X: hir.V("a", arr), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.Seq, Type: num, Stmt: hir.B(inner), Y: hir.L(num, 1)}}, &hir.Stmt{Kind: hir.Break})}
	m := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Params: []hir.Param{{Name: "a", Type: arr}}, Body: hir.B(outer)}
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{m}}}}
	db, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	path := "C::run/body/s0/body/s0/x/seq/s0"
	if !db.Has("in_body", path+"/body/s0", "C::run/body/s0") || !db.Has("in_body", path+"/body/s0", path) {
		t.Fatal("nested membership")
	}
	if db.Has("not_read_after", "C::run/param/a", path+"/body/s0/x/0") {
		t.Fatal("back-edge read lost")
	}
	gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
}
func TestWriteSummaries(t *testing.T) {
	num := hir.T(hir.I32)
	arr := hir.T(hir.Array, num)
	mut := &hir.Method{Name: "mut", Static: true, Result: hir.T(hir.Void), Params: []hir.Param{{Name: "a", Type: arr}}, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: num, X: hir.V("a", arr), Args: []*hir.Expr{hir.L(num, 1)}}})}
	own := &hir.Method{Name: "own", Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: num, Name: "n", X: &hir.Expr{Kind: hir.This, Type: hir.Ref("C")}}, Y: hir.L(num, 1)})}
	caller := &hir.Method{Name: "call", Static: true, Result: hir.T(hir.Void), Params: []hir.Param{{Name: "a", Type: arr}}, Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "b", Type: arr, X: hir.V("a", arr)}, &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.DirectCall, Owner: "C", Name: "mut", Type: hir.T(hir.Void), Args: []*hir.Expr{hir.V("b", arr)}}})}
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Fields: []hir.Field{{Name: "n", Type: num}}, Methods: []*hir.Method{mut, own, caller, {Name: "none", Static: true, Result: hir.T(hir.Void), Body: hir.B()}}}}}
	db, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range [][2]string{{"C::own", "own_fields"}, {"C::mut", "any"}, {"C::call", "any"}, {"C::none", "none"}} {
		if !db.Has("writes", r[0], r[1]) {
			t.Fatalf("missing writes %v", r)
		}
	}
	if !db.Has("site_writes", "C::call/body/s1/x", "C::call/param/a") {
		t.Fatal("alias write through call lost")
	}
	gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
}

func TestSeqLoopBreakKeepsExitReads(t *testing.T) {
	num := hir.T(hir.I32)
	arr := hir.T(hir.Array, num)
	length := func() *hir.Expr {
		return &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: num, X: hir.V("a", arr)}
	}
	seq := &hir.Expr{Kind: hir.Seq, Type: num, Stmt: hir.B(&hir.Stmt{Kind: hir.While, X: hir.L(hir.T(hir.Bool), true), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: length()}, &hir.Stmt{Kind: hir.Break})}, &hir.Stmt{Kind: hir.ExprStmt, X: length()}), Y: hir.L(num, 1)}
	m := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Params: []hir.Param{{Name: "a", Type: arr}}, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: seq})}
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{m}}}}
	db, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	if db.Has("not_read_after", "C::run/param/a", "C::run/body/s0/x/seq/s0/body/s0/x/0") {
		t.Fatal("Seq loop break lost following read")
	}
	gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
}

func TestConditionalWriteAliases(t *testing.T) {
	num := hir.T(hir.I32)
	arr := hir.T(hir.Array, num)
	recv := &hir.Expr{Kind: hir.Conditional, Type: arr, X: hir.L(hir.T(hir.Bool), true), Y: hir.V("a", arr), Z: hir.V("b", arr)}
	m := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Params: []hir.Param{{Name: "a", Type: arr}, {Name: "b", Type: arr}}, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: num, X: recv, Args: []*hir.Expr{hir.L(num, 1)}}})}
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{m}}}}
	db, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if !db.Has("site_writes", "C::run/body/s0/x", "C::run/param/"+name) {
			t.Fatalf("missing conditional write to %s", name)
		}
	}
	gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
}

func TestOwnSummaryDoesNotTreatMayAliasAsOwnership(t *testing.T) {
	num := hir.T(hir.I32)
	typ := hir.Ref("C")
	m := &hir.Method{Name: "run", Result: hir.T(hir.Void), Params: []hir.Param{{Name: "other", Type: typ}}, Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "r", Type: typ, X: &hir.Expr{Kind: hir.This, Type: typ}}, &hir.Stmt{Kind: hir.Assign, X: hir.V("r", typ), Y: hir.V("other", typ)}, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: num, Name: "n", X: hir.V("r", typ)}, Y: hir.L(num, 1)})}
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Fields: []hir.Field{{Name: "n", Type: num}}, Methods: []*hir.Method{m}}}}
	db, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	if !db.Has("writes", "C::run", "any") || db.Has("writes", "C::run", "own_fields") {
		t.Fatal("possible receiver alias became an ownership proof")
	}
	gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
}

func TestSummaryUsesEffectsRatherThanPurity(t *testing.T) {
	m := &hir.Method{Name: "run", Static: true, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "clock.telemetry", Type: hir.T(hir.Number), X: hir.L(hir.T(hir.I32), 0)}})}
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{m}}}}
	db, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	if db.Has("pure", "C::run") || !db.Has("writes", "C::run", "none") || db.Has("allocates", "C::run") || db.Has("may_diverge", "C::run") || db.Has("may_raise", "C::run") {
		t.Fatal("global read incorrectly classified as a write/allocation/raise/divergence")
	}
	gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
}

func TestVirtualWriteTargetsAndInitializerSummary(t *testing.T) {
	num := hir.T(hir.I32)
	arr := hir.T(hir.Array, num)
	void := hir.T(hir.Void)
	params := []hir.Param{{Name: "a", Type: arr}}
	base := &hir.Class{Name: "Base", Abstract: true, Methods: []*hir.Method{{Name: "mut", Virtual: true, Abstract: true, Result: void, Params: params}}}
	a := &hir.Class{Name: "A", Super: "Base", Methods: []*hir.Method{{Name: "mut", Virtual: true, Result: void, Params: params, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: num, X: hir.V("a", arr), Args: []*hir.Expr{hir.L(num, 1)}}})}}}
	b := &hir.Class{Name: "B", Super: "Base", Methods: []*hir.Method{{Name: "mut", Virtual: true, Result: void, Params: params, Body: hir.B()}}}
	run := &hir.Method{Name: "run", Static: true, Result: void, Params: []hir.Param{{Name: "r", Type: hir.Ref("Base")}, {Name: "a", Type: arr}}, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.VirtualCall, Name: "mut", Type: void, X: hir.V("r", hir.Ref("Base")), Args: []*hir.Expr{hir.V("a", arr)}}})}
	init := &hir.Method{Name: "class_constructor", Static: true, Result: void, Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.StaticGet, Owner: "Caller", Name: "items", Type: arr}, Y: &hir.Expr{Kind: hir.New, Type: arr}})}
	p := &hir.Program{Classes: []*hir.Class{base, a, b, {Name: "Caller", Fields: []hir.Field{{Name: "items", Static: true, Type: arr}}, Methods: []*hir.Method{run, init}}}}
	db, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	if !db.Has("site_writes", "Caller::run/body/s0/x", "Caller::run/param/a") {
		t.Fatal("virtual receiver mutation lost")
	}
	if !db.Has("writes", "Caller::run", "any") || !db.Has("allocates", "Caller::run") || !db.Has("calls", "Caller::run", "Caller::class_constructor", "Caller::run/entry/init/Caller") {
		t.Fatal("initializer effects missing")
	}
	gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
}

func TestOrderedReadsAcrossEmptyTransfers(t *testing.T) {
	num := hir.T(hir.I32)
	arr := hir.T(hir.Array, num)
	length := func() *hir.Expr {
		return &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: num, X: hir.V("a", arr)}
	}
	body := hir.B()
	for i := 0; i < 200; i++ {
		body.List = append(body.List, &hir.Stmt{Kind: hir.ExprStmt, X: hir.L(num, i)})
	}
	body.List = append(body.List, &hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.Binary, Op: "+", Type: num, X: length(), Y: length()}})
	m := &hir.Method{Name: "run", Static: true, Result: num, Params: []hir.Param{{Name: "a", Type: arr}, {Name: "unused", Type: num}}, Body: body}
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{m}}}}
	db, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	if db.Has("not_read_after", "C::run/param/a", "C::run/body/s200/x/0/0") || !db.Has("not_read_after", "C::run/param/a", "C::run/body/s200/x/1/0") {
		t.Fatal("operand ordering lost")
	}
	if !db.Has("use_count", "C::run/param/a", "2") || !db.Has("use_count", "C::run/param/unused", "0") || !db.Has("def", "C::run/param/a", "C::run/entry") {
		t.Fatal("def/use counts missing")
	}
	if db.Count("next") > 4 {
		t.Fatalf("empty transfers not contracted: %d edges", db.Count("next"))
	}
	gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
}

func TestRecursiveCallSeeds(t *testing.T) {
	void := hir.T(hir.Void)
	call := func(owner, name string) *hir.Stmt {
		return &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.DirectCall, Owner: owner, Name: name, Type: void}}
	}
	methods := []*hir.Method{{Name: "a", Static: true, Result: void, Body: hir.B(call("C", "b"))}, {Name: "b", Static: true, Result: void, Body: hir.B(call("C", "a"))}, {Name: "entry", Static: true, Result: void, Body: hir.B(call("C", "a"))}, {Name: "leaf", Static: true, Result: void, Body: hir.B()}}
	p := &hir.Program{Classes: []*hir.Class{{Name: "C", Methods: methods}}}
	db, err := rewrite.Analyze(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if !db.Has("recursive_call", "C::"+name) || !db.Has("may_diverge", "C::"+name) {
			t.Fatal("cycle missing")
		}
	}
	if db.Has("recursive_call", "C::entry") || !db.Has("may_diverge", "C::entry") || db.Has("may_diverge", "C::leaf") {
		t.Fatal("cycle predecessor/leaf classification")
	}
	gracecheck.CheckRecursiveSeeds(t, db)
	gracecheck.Equal(t, db, gracecheck.Evaluate(t, rewrite.Extract(p), gracecheck.Source(t, "analysis")))
}
