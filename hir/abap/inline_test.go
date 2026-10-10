package abap

import (
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/hir/rewrite"
)

func TestInlineGrace(t *testing.T) {
	want := inlineFixture().p
	n, callees := hir.InlineStats(want)
	p := inlineFixture().p
	stats, err := rewrite.Inline(p)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CallSites != n || !reflect.DeepEqual(stats.Callees, callees) {
		t.Fatalf("Grace stats %+v, want %d sites, %v", stats, n, callees)
	}
	t.Setenv("ABAPITI_INLINE_STATS", "1")
	var outputs []string
	for _, mode := range []string{"", "grace"} {
		t.Setenv("ABAPITI_INLINE", mode)
		p := inlineFixture().p
		func() {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			stderr := os.Stderr
			os.Stderr = w
			defer func() { os.Stderr = stderr }()
			err = inline(p)
			_ = w.Close()
			if err != nil {
				t.Fatal(err)
			}
			out, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			outputs = append(outputs, string(out))
		}()
		if hir.Dump(p) != hir.Dump(want) {
			t.Fatalf("mode %q changed inline output", mode)
		}
	}
	if outputs[0] != outputs[1] || !strings.HasPrefix(outputs[1], "inline: 13 call sites, 5 callees\n") {
		t.Fatalf("inline stats differ: %q / %q", outputs[0], outputs[1])
	}
	// Grace checks input before rewriting, even when there are no call sites.
	// This distinguishes flag selection from accidentally using hir.Inline.
	t.Setenv("ABAPITI_INLINE", "grace")
	bad := &hir.Program{Classes: []*hir.Class{{Name: "Duplicate"}, {Name: "Duplicate"}}}
	if err := inline(bad); err == nil || !strings.Contains(err.Error(), "Grace HIR inlining") {
		t.Fatalf("flag did not select Grace: %v", err)
	}
}

// inlineFixture exercises the inlining pass at run time (TestFixtures runs it
// on both OSG runtimes through hir-unit.sh): a guarded-return chain, local
// declarations in a nested block, argument evaluation order, a side-effecting
// void method with an early return used as a statement, a receiver evaluated
// once, a recursive method, a shadowing block, a declaration without
// initializer called twice and an overridden method (the last four stay calls). run returns 42, or the number of the failed check.
func inlineFixture() fixture {
	ctr := hir.Ref("InlCounter")
	this := &hir.Expr{Kind: hir.This, Type: ctr}
	field := func(x *hir.Expr, n string) *hir.Expr {
		return &hir.Expr{Kind: hir.FieldGet, X: x, Name: n, Type: i32}
	}
	set := func(x *hir.Expr, n string, v *hir.Expr) *hir.Stmt {
		return &hir.Stmt{Kind: hir.Assign, X: field(x, n), Y: v}
	}
	add := func(a, b *hir.Expr) *hir.Expr { return binary("+", a, b, i32) }
	mul := func(a, b *hir.Expr) *hir.Expr { return binary("*", a, b, i32) }
	virtual := func(n string, result hir.Type, params []hir.Param, body ...*hir.Stmt) *hir.Method {
		m := method(n, result, hir.B(body...))
		m.Virtual, m.Params = true, params
		return m
	}
	ifs := func(c *hir.Expr, then *hir.Stmt, els *hir.Stmt) *hir.Stmt {
		return &hir.Stmt{Kind: hir.If, X: c, Body: then, Else: els}
	}
	k := local("k", i32)
	a, b := local("a", i32), local("b", i32)
	counter := &hir.Class{Name: "InlCounter", Fields: []hir.Field{{Name: "n", Type: i32}, {Name: "log", Type: i32}}, Methods: []*hir.Method{
		virtual("cur", i32, nil,
			ifs(binary("<", field(this, "n"), lit(0), boolean), hir.B(ret(lit(100))),
				hir.B(ifs(binary(">=", field(this, "n"), lit(10), boolean), hir.B(ret(lit(200))), nil))),
			ret(mul(field(this, "n"), lit(2)))),
		virtual("next", i32, nil, set(this, "n", add(field(this, "n"), lit(1))), ret(field(this, "n"))),
		virtual("pair", i32, []hir.Param{{Name: "a", Type: i32}, {Name: "b", Type: i32}},
			decl("x", i32, mul(a, lit(10))),
			hir.B(decl("y", i32, add(local("x", i32), b)), assign("x", i32, local("y", i32))),
			ret(local("x", i32))),
		virtual("bump", hir.T(hir.Void), []hir.Param{{Name: "k", Type: i32}},
			set(this, "log", add(mul(field(this, "log"), lit(10)), k)),
			ifs(binary("==", k, lit(0), boolean), hir.B(&hir.Stmt{Kind: hir.Return}), nil),
			set(this, "n", add(field(this, "n"), k))),
		virtual("me", ctr, nil, set(this, "log", add(mul(field(this, "log"), lit(10)), lit(9))), ret(this)),
		virtual("fact", i32, []hir.Param{{Name: "k", Type: i32}},
			ifs(binary("<=", k, lit(1), boolean), hir.B(ret(lit(1))), nil),
			ret(mul(k, call(hir.VirtualCall, this, "", "fact", i32, binary("-", k, lit(1), i32))))),
		virtual("shadow", i32, []hir.Param{{Name: "a", Type: i32}},
			decl("x", i32, a),
			hir.B(decl("x", i32, lit(5)), set(this, "log", local("x", i32))),
			ret(local("x", i32))),
		virtual("late", i32, []hir.Param{{Name: "k", Type: i32}},
			decl("r", i32, nil),
			ifs(binary(">", k, lit(0), boolean), hir.B(assign("r", i32, k)), nil),
			ret(local("r", i32))),
	}}
	base := &hir.Class{Name: "InlBase", Methods: []*hir.Method{virtual("val", i32, nil, ret(lit(1)))}}
	sub := &hir.Class{Name: "InlSub", Super: "InlBase", Methods: []*hir.Method{virtual("val", i32, nil, ret(lit(2)))}}
	c := local("c", ctr)
	vc := func(n string, t hir.Type, args ...*hir.Expr) *hir.Expr {
		return call(hir.VirtualCall, c, "", n, t, args...)
	}
	fail := func(cond *hir.Expr, code int) *hir.Stmt { return ifs(cond, ret(lit(code)), nil) }
	ne := func(x *hir.Expr, v int) *hir.Expr { return binary("!=", x, lit(v), boolean) }
	or := func(x, y *hir.Expr) *hir.Expr { return binary("||", x, y, boolean) }
	run := method("run", i32, hir.B(
		decl("c", ctr, newObj(ctr)),
		fail(ne(vc("cur", i32), 0), 1),
		set(c, "n", lit(-3)), fail(ne(vc("cur", i32), 100), 2),
		set(c, "n", lit(12)), fail(ne(vc("cur", i32), 200), 3),
		set(c, "n", lit(4)), fail(ne(vc("cur", i32), 8), 4),
		set(c, "n", lit(1)), fail(ne(vc("pair", i32, vc("next", i32), vc("next", i32)), 23), 5),
		run(vc("bump", hir.T(hir.Void), lit(0))), run(vc("bump", hir.T(hir.Void), lit(4))),
		fail(or(ne(field(c, "log"), 4), ne(field(c, "n"), 7)), 6),
		set(c, "log", lit(0)),
		fail(ne(call(hir.VirtualCall, vc("me", ctr), "", "cur", i32), 14), 7),
		fail(ne(field(c, "log"), 9), 8),
		fail(ne(vc("fact", i32, lit(5)), 120), 9),
		fail(ne(vc("shadow", i32, lit(6)), 6), 10),
		fail(ne(field(c, "log"), 5), 11),
		decl("b", hir.Ref("InlBase"), newObj(hir.Ref("InlSub"))),
		fail(ne(call(hir.VirtualCall, local("b", hir.Ref("InlBase")), "", "val", i32), 2), 12),
		run(vc("bump", hir.T(hir.Void), vc("next", i32))),
		fail(or(ne(field(c, "log"), 58), ne(field(c, "n"), 16)), 13),
		fail(ne(vc("late", i32, lit(5)), 5), 14),
		fail(ne(vc("late", i32, lit(-1)), 0), 15),
		ret(lit(42)),
	))
	run.Static = true
	return fixture{"inline_fixture", &hir.Program{Classes: []*hir.Class{counter, base, sub, {Name: "inline_fixture", Methods: []*hir.Method{run}}}}, 42}
}

func TestInlineCandidates(t *testing.T) {
	p := inlineFixture().p
	n, stats := hir.InlineStats(p)
	if errs := hir.Verify(p); len(errs) > 0 {
		t.Fatal(errs[0])
	}
	want := map[string]int{"InlCounter.cur": 5, "InlCounter.next": 3, "InlCounter.pair": 1, "InlCounter.bump": 3, "InlCounter.me": 1}
	total := 0
	for k, v := range want {
		total += v
		if stats[k] != v {
			t.Errorf("%s inlined at %d sites, want %d (all: %v)", k, stats[k], v, stats)
		}
	}
	if n != total || len(stats) != len(want) {
		t.Errorf("inlined %d sites %v, want %d %v", n, stats, total, want)
	}
	// Overridden (Base.val), recursive (fact), shadowing (shadow) and
	// uninitialized-declaration (late) callees stay calls.
	calls := map[string]int{}
	var walk func(x *hir.Expr)
	var walkStmt func(s *hir.Stmt)
	walk = func(x *hir.Expr) {
		if x == nil {
			return
		}
		if x.Kind == hir.VirtualCall {
			calls[x.Name]++
		}
		walk(x.X)
		walk(x.Y)
		walk(x.Z)
		for _, a := range x.Args {
			walk(a)
		}
		walkStmt(x.Stmt)
	}
	walkStmt = func(s *hir.Stmt) {
		if s == nil {
			return
		}
		walk(s.X)
		walk(s.Y)
		walkStmt(s.Body)
		walkStmt(s.Else)
		for _, x := range s.List {
			walkStmt(x)
		}
	}
	walkStmt(p.Classes[3].Methods[0].Body)
	if len(calls) != 4 || calls["val"] != 1 || calls["fact"] != 1 || calls["shadow"] != 1 || calls["late"] != 2 {
		t.Fatalf("remaining calls in run: %v", calls)
	}
}

func TestInlineArgumentOrder(t *testing.T) {
	p := inlineFixture().p
	hir.Inline(p)
	var pair *hir.Expr
	var find func(x *hir.Expr)
	find = func(x *hir.Expr) {
		if x == nil || pair != nil {
			return
		}
		if x.Kind == hir.Seq && len(x.Stmt.List) == 3 && strings.HasSuffix(x.Stmt.List[1].Name, "_a") {
			pair = x
			return
		}
		find(x.X)
		find(x.Y)
		find(x.Z)
		for _, a := range x.Args {
			find(a)
		}
	}
	for _, s := range p.Classes[3].Methods[0].Body.List {
		find(s.X)
	}
	if pair == nil {
		t.Fatal("no inlined pair call")
	}
	// Receiver, then a, then b: each bound once, before the body.
	got := []string{}
	for _, s := range pair.Stmt.List {
		got = append(got, s.Name[strings.LastIndex(s.Name, "_")+1:])
		if s.Kind != hir.VarDecl || s.X == nil {
			t.Fatalf("binding %s is not a declaration", s.Name)
		}
	}
	if strings.Join(got, ",") != "self,a,b" {
		t.Fatalf("bindings %v", got)
	}
	if pair.Stmt.List[1].X.Kind != hir.Seq || pair.Stmt.List[2].X.Kind != hir.Seq {
		t.Fatalf("next() arguments not inlined into their bindings")
	}
}

func TestInlineDisabled(t *testing.T) {
	names := hir.NewNames()
	cur := "->" + names.Get("member.cur") + "("
	t.Setenv("ABAPITI_INLINE", "0")
	p := inlineFixture().p
	before := hir.Dump(p)
	off, err := Emit(p)
	if err != nil {
		t.Fatal(err)
	}
	if hir.Dump(p) != before {
		t.Fatal("ABAPITI_INLINE=0 changed the program")
	}
	t.Setenv("ABAPITI_INLINE", "")
	on, err := Emit(inlineFixture().p)
	if err != nil {
		t.Fatal(err)
	}
	run := names.Get("inline_fixture") + ".clas.abap"
	if !strings.Contains(off[run], cur) || strings.Contains(on[run], cur) {
		t.Fatalf("cur calls: disabled %v, enabled %v", strings.Contains(off[run], cur), strings.Contains(on[run], cur))
	}
	// Without inlinable calls the pass changes nothing.
	for i, f := range fixtures() {
		if f.name == "inline_fixture" {
			continue
		}
		t.Setenv("ABAPITI_INLINE", "0")
		off, err := Emit(f.p)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("ABAPITI_INLINE", "")
		on, err := Emit(fixtures()[i].p)
		if err != nil {
			t.Fatal(err)
		}
		if len(on) != len(off) {
			t.Fatalf("%s: %d files inlined, %d disabled", f.name, len(on), len(off))
		}
		for k, v := range off {
			if on[k] != v {
				t.Fatalf("%s: %s differs with inlining", f.name, k)
			}
		}
	}
}
