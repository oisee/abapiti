package rewrite

import (
	"github.com/oisee/abapiti/hir"
	"testing"
)

func TestRuntimeEffectsCoverage(t *testing.T) {
	check := func(op string) {
		e, ok := RuntimeEffects[op]
		if !ok {
			t.Errorf("missing effects for %s", op)
			return
		}
		switch e.Writes {
		case "None", "Receiver", "Arg(0)", "Arg(1)", "Arg(2)":
		default:
			t.Errorf("%s writes %q", op, e.Writes)
		}
		switch e.MayRaise {
		case "None", "Trap", "Catchable":
		default:
			t.Errorf("%s MayRaise %q", op, e.MayRaise)
		}
		switch e.Aliases {
		case "None", "Receiver":
		default:
			t.Errorf("%s aliases %q", op, e.Aliases)
		}
		if e.Reads.Global != (op == "clock.telemetry") {
			t.Errorf("%s global read", op)
		}
	}
	for op, spec := range hir.RuntimeSpecs {
		check(op)
		if RuntimeEffects[op].Reads.Args && spec.Arity == 0 {
			t.Errorf("%s cannot read nonexistent arguments", op)
		}
	}
	for op := range hir.SpecialOps {
		check(op)
	}
	for op := range RuntimeEffects {
		if _, ok := hir.RuntimeSpecs[op]; !ok && !hir.SpecialOps[op] {
			t.Errorf("unknown op %s", op)
		}
	}
	db := Extract(&hir.Program{})
	for op := range RuntimeEffects {
		for _, p := range []string{"op_reads", "op_writes", "op_allocates", "op_may_raise", "op_aliases"} {
			found := false
			for _, r := range db.Facts(p) {
				if r[0] == op {
					found = true
				}
			}
			if !found {
				t.Errorf("%s missing base fact for %s", p, op)
			}
		}
	}
	t.Logf("effects coverage: %d catalogue + %d SpecialOps = %d records", len(hir.RuntimeSpecs), len(hir.SpecialOps), len(RuntimeEffects))
}

func TestRuntimeEffectSemantics(t *testing.T) {
	for _, op := range []string{"array.get", "array.pop", "map.get", "string.at"} {
		if RuntimeEffects[op].MayRaise != "None" {
			t.Errorf("%s Optional access raises", op)
		}
	}
	for _, op := range []string{"string.charCodeAt", "string.localeCompareNames"} {
		if RuntimeEffects[op].MayRaise != "Trap" {
			t.Errorf("%s must trap", op)
		}
	}
	for _, op := range []string{"dynamic.asNumber", "dynamic.asBoolean", "dynamic.asString", "dynamic.asClassValue", "dynamic.asRef", "dynamic.materialize", "classvalue.new"} {
		if RuntimeEffects[op].MayRaise != "Catchable" {
			t.Errorf("%s type check must raise", op)
		}
	}
	if e := RuntimeEffects["set.fromArray"]; !e.Allocates || e.Writes != "None" {
		t.Errorf("set.fromArray %+v", e)
	}
	if e := RuntimeEffects["array.splice1_view"]; e.Aliases != "Receiver" || e.Writes != "Receiver" {
		t.Errorf("splice1_view %+v", e)
	}
	// A wrong dynamic tag must affect may_throw through op_may_raise, while an
	// ordinary Optional lookup remains non-throwing. The extractor emits no
	// approximate raises fact for reviewed RuntimeOps.
	d := Extract(&hir.Program{Classes: []*hir.Class{{Name: "C", Methods: []*hir.Method{
		{Name: "tag", Static: true, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.asNumber", Type: hir.T(hir.Number), X: &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.null", Type: hir.T(hir.Dynamic), X: hir.L(hir.T(hir.I32), 0)}}})},
	}}}})
	if d.Count("raises") != 0 {
		t.Fatal("runtime raises must come from the effect relation")
	}
	src, err := ruleFiles.ReadFile("rules/analysis.grace")
	if err != nil {
		t.Fatal(err)
	}
	_, rules, err := Parse(string(src))
	if err != nil {
		t.Fatal(err)
	}
	if err := Evaluate(d, rules); err != nil {
		t.Fatal(err)
	}
	if !d.Has("may_throw", "C::tag") {
		t.Fatal("dynamic tag failure lost")
	}
}

func TestNativeTypeChecksAndIntegerOverflowMayThrow(t *testing.T) {
	i := hir.T(hir.I64)
	root, a := hir.Ref(hir.RootObject), hir.Ref("A")
	cast := &hir.Expr{Kind: hir.Cast, Type: a, X: hir.V("p", root)}
	narrow := &hir.Expr{Kind: hir.Narrow, Type: a, X: hir.V("p", root)}
	unary := &hir.Expr{Kind: hir.Unary, Op: "-", Type: i, X: hir.L(i, int64(1)), CheckIntegerOverflow: true}
	binary := &hir.Expr{Kind: hir.Binary, Op: "+", Type: i, X: hir.L(i, int64(1)), Y: hir.L(i, int64(2)), CheckIntegerOverflow: true}
	c := &hir.Class{Name: "C"}
	for name, e := range map[string]*hir.Expr{"cast": cast, "narrow": narrow, "unary": unary, "binary": binary} {
		m := method(name, exp(e))
		if name == "cast" || name == "narrow" {
			m.Params = []hir.Param{{Name: "p", Type: root}}
		}
		c.Methods = append(c.Methods, m)
	}
	c.Methods = append(c.Methods, method("unchecked", exp(&hir.Expr{Kind: hir.Unary, Op: "-", Type: i, X: hir.L(i, int64(1))})))
	d := analyze(t, c, &hir.Class{Name: "A"})
	for _, name := range []string{"cast", "narrow", "unary", "binary"} {
		assertFact(t, d, true, "may_throw", "C::"+name)
	}
	assertFact(t, d, false, "may_throw", "C::unchecked")
}

func TestOptionalAllocationDoesNotProveFreshReference(t *testing.T) {
	i := hir.T(hir.I32)
	arr := hir.T(hir.Array, i)
	nested := hir.T(hir.Array, arr)
	opt := hir.T(hir.Optional, arr)
	get := &hir.Expr{Kind: hir.RuntimeOp, Op: "array.get", Type: opt, X: hir.V("input", nested), Args: []*hir.Expr{hir.L(i, 0)}}
	view := &hir.Expr{Kind: hir.Narrow, Type: arr, X: get}
	push := &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: i, X: view, Args: []*hir.Expr{hir.L(i, 1)}}
	m := method("mutate", exp(push))
	m.Params = []hir.Param{{Name: "input", Type: nested}}
	d := analyze(t, &hir.Class{Name: "C", Methods: []*hir.Method{m}})
	assertFact(t, d, false, "pure", "C::mutate")
	assertFact(t, d, false, "fresh", "C::mutate", "C::mutate/body/s0/x/0/0")
}

func TestRuntimeConstructionKeepsUnknownConstructorEffects(t *testing.T) {
	ref := hir.Ref("C")
	ctor := &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Body: hir.B()}
	factory := method("factory", exp(&hir.Expr{Kind: hir.RuntimeOp, Op: "classvalue.new", Type: ref, X: &hir.Expr{Kind: hir.ClassOf, Type: hir.T(hir.ClassValue), Owner: "C"}}))
	project := method("project", exp(&hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.materialize", Type: ref, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.null", Type: hir.T(hir.Dynamic), X: hir.L(hir.T(hir.I32), 0)}}))
	d := analyze(t, &hir.Class{Name: "C", Ctor: ctor, Methods: []*hir.Method{factory, project}})
	// The table describes direct operation effects. Invoked constructors may
	// write statics or fields; allocation does not discharge those calls.
	for _, name := range []string{"factory", "project"} {
		assertFact(t, d, true, "unknown_effect", "C::"+name)
		assertFact(t, d, false, "pure", "C::"+name)
	}
}
