// Package fixtures contains backend-neutral HIR parity fixtures.
package fixtures

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/oisee/abapiti/hir"
)

//go:embed testdata/xml-validation.json
var XMLOracle []byte

type XMLCase struct {
	Input  string          `json:"input"`
	Result json.RawMessage `json:"result"`
}

func XMLCases() []XMLCase {
	var cases []XMLCase
	if err := json.Unmarshal(XMLOracle, &cases); err != nil {
		panic(err)
	}
	return cases
}

var i32 = hir.T(hir.I32)
var str = hir.T(hir.String)
var boolean = hir.T(hir.Bool)
var dyn = hir.T(hir.Dynamic)

func rt(op string, x *hir.Expr, t hir.Type, args ...*hir.Expr) *hir.Expr {
	return &hir.Expr{Kind: hir.RuntimeOp, Op: op, X: x, Type: t, Args: args}
}
func ret(x *hir.Expr) *hir.Stmt             { return &hir.Stmt{Kind: hir.Return, X: x} }
func get(x *hir.Expr, key string) *hir.Expr { return rt("dynamic.get", x, dyn, hir.L(str, key)) }
func eq(x, y *hir.Expr) *hir.Expr {
	return &hir.Expr{Kind: hir.Binary, Op: "==", X: x, Y: y, Type: boolean}
}
func and(x, y *hir.Expr) *hir.Expr {
	return &hir.Expr{Kind: hir.Binary, Op: "&&", X: x, Y: y, Type: boolean}
}
func checkMethod(name string, init []*hir.Stmt, check *hir.Expr) *hir.Method {
	init = append(init, &hir.Stmt{Kind: hir.If, X: check, Body: hir.B(ret(hir.L(i32, 1)))}, ret(hir.L(i32, 0)))
	return &hir.Method{Name: name, Static: true, Result: i32, Body: hir.B(init...)}
}
func runner(c *hir.Class) {
	sum := hir.L(i32, 0)
	for _, m := range c.Methods {
		sum = &hir.Expr{Kind: hir.Binary, Op: "+", X: sum, Y: &hir.Expr{Kind: hir.DirectCall, Owner: c.Name, Name: m.Name, Type: i32}, Type: i32}
	}
	c.Methods = append(c.Methods, &hir.Method{Name: "run", Static: true, Result: i32, Body: hir.B(ret(sum))})
}

// XMLValidation compares every result field, including absent col, to the
// pinned Node validator oracle used by the TS rule equality test (444 inputs).
func XMLValidation() *hir.Program {
	c := &hir.Class{Name: "xml_validation_fixture"}
	for i, tc := range XMLCases() {
		init := []*hir.Stmt{{Kind: hir.VarDecl, Name: "result", Type: dyn, X: rt("xml.validate", hir.L(str, tc.Input), dyn)}}
		res := hir.V("result", dyn)
		var check *hir.Expr
		if string(tc.Result) == "true" {
			check = rt("dynamic.strictEquals", res, boolean, rt("dynamic.of", hir.L(boolean, true), dyn))
		} else {
			var want struct {
				Err struct {
					Code, Msg string
					Line      int
					Col       *int
				}
			}
			if err := json.Unmarshal(tc.Result, &want); err != nil {
				panic(err)
			}
			init = append(init, &hir.Stmt{Kind: hir.VarDecl, Name: "err", Type: dyn, X: get(res, "err")})
			err := hir.V("err", dyn)
			check = and(eq(rt("dynamic.asString", get(err, "code"), str), hir.L(str, want.Err.Code)), eq(rt("dynamic.asString", get(err, "msg"), str), hir.L(str, want.Err.Msg)))
			check = and(check, eq(rt("dynamic.asNumber", get(err, "line"), hir.T(hir.Number)), hir.L(hir.T(hir.Number), float64(want.Err.Line))))
			if want.Err.Col == nil {
				check = and(check, rt("dynamic.isNullish", get(err, "col"), boolean))
			} else {
				check = and(check, eq(rt("dynamic.asNumber", get(err, "col"), hir.T(hir.Number)), hir.L(hir.T(hir.Number), float64(*want.Err.Col))))
			}
		}
		c.Methods = append(c.Methods, checkMethod(fmt.Sprintf("case_%03d", i), init, check))
	}
	runner(c)
	return &hir.Program{Classes: []*hir.Class{c}}
}

// Properties covers own/inherited fields, prototypes, unknown keys, optional
// absence, falsy values, mutation, subclass dispatch and stable token identity.
func Properties() (*hir.Program, int) {
	opt := hir.T(hir.Optional, str)
	base := &hir.Class{Name: "property_base", Fields: []hir.Field{{Name: "severity", Type: str}, {Name: "absent", Type: opt}}, Methods: []*hir.Method{{Name: "method", Result: i32, Body: hir.B(ret(hir.L(i32, 7)))}}}
	config := &hir.Class{Name: "property_config", Super: base.Name, Fields: []hir.Field{{Name: "enabled", Type: boolean}, {Name: "count", Type: hir.T(hir.Number)}, {Name: "text", Type: str}, {Name: "count64", Type: hir.T(hir.I64)}, {Name: "optional64", Type: hir.T(hir.Optional, hir.T(hir.I64))}}}
	derived := &hir.Class{Name: "property_derived", Super: config.Name, Fields: []hir.Field{{Name: "extra", Type: str}}}
	probe := &hir.Class{Name: "property_fixture"}
	ref := hir.Ref(config.Name)
	recv := hir.V("config", ref)
	init := func() []*hir.Stmt {
		return []*hir.Stmt{{Kind: hir.VarDecl, Name: "config", Type: ref, X: &hir.Expr{Kind: hir.New, Type: hir.Ref(derived.Name)}}}
	}
	add := func(check *hir.Expr) {
		probe.Methods = append(probe.Methods, checkMethod(fmt.Sprintf("case_%03d", len(probe.Methods)), init(), check))
	}
	for _, tc := range []struct {
		key      string
		own, has bool
	}{{"severity", true, true}, {"absent", true, true}, {"enabled", true, true}, {"count", true, true}, {"text", true, true}, {"extra", true, true}, {"method", false, true}, {"constructor", false, true}, {"__proto__", false, true}, {"toString", false, true}, {"unknown", false, false}} {
		add(eq(rt("object.hasOwn", recv, boolean, hir.L(str, tc.key)), hir.L(boolean, tc.own)))
		add(eq(rt("object.has", recv, boolean, hir.L(str, tc.key)), hir.L(boolean, tc.has)))
	}
	read := func(key string) *hir.Expr { return rt("object.get", recv, dyn, hir.L(str, key)) }
	add(rt("dynamic.isNullish", read("absent"), boolean))
	add(rt("dynamic.isNullish", read("unknown"), boolean))
	add(eq(rt("dynamic.asBoolean", read("enabled"), boolean), hir.L(boolean, false)))
	add(eq(rt("dynamic.asNumber", read("count"), hir.T(hir.Number)), hir.L(hir.T(hir.Number), 0.0)))
	add(eq(rt("dynamic.asString", read("extra"), str), hir.L(str, "")))
	add(and(and(rt("dynamic.isFunction", read("method"), boolean), rt("dynamic.isFunction", read("constructor"), boolean)), &hir.Expr{Kind: hir.InstanceOf, X: recv, Owner: config.Name, Type: boolean}))
	add(and(rt("dynamic.strictEquals", read("toString"), boolean, read("toString")), rt("dynamic.strictEquals", read("toString"), boolean, rt("object.get", &hir.Expr{Kind: hir.New, Type: ref}, dyn, hir.L(str, "toString")))))
	assign := init()
	assign = append(assign, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, X: recv, Owner: config.Name, Name: "text", Type: str}, Y: hir.L(str, "changed")})
	for _, field := range []struct {
		name  string
		typ   hir.Type
		value *hir.Expr
	}{
		{"severity", str, hir.L(str, "Error")},
		{"absent", opt, hir.L(str, "present")},
		{"count64", hir.T(hir.I64), hir.L(hir.T(hir.I64), int64(9007199254740991))},
		{"optional64", hir.T(hir.Optional, hir.T(hir.I64)), hir.L(hir.T(hir.I64), int64(7))},
	} {
		assign = append(assign, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, X: recv, Owner: config.Name, Name: field.name, Type: field.typ}, Y: field.value})
	}
	mutated := eq(rt("dynamic.asString", read("text"), str), hir.L(str, "changed"))
	mutated = and(mutated, eq(rt("dynamic.asString", read("severity"), str), hir.L(str, "Error")))
	mutated = and(mutated, eq(rt("dynamic.asString", read("absent"), str), hir.L(str, "present")))
	mutated = and(mutated, eq(rt("dynamic.asNumber", read("count64"), hir.T(hir.Number)), hir.L(hir.T(hir.Number), float64(9007199254740991))))
	mutated = and(mutated, eq(rt("dynamic.asNumber", read("optional64"), hir.T(hir.Number)), hir.L(hir.T(hir.Number), 7.0)))
	probe.Methods = append(probe.Methods, checkMethod("mutation", assign, mutated))

	count := len(probe.Methods)
	runner(probe)
	return &hir.Program{Classes: []*hir.Class{base, config, derived, probe}}, count
}

// PropertiesInSeq isolates operations hidden in a Seq statement body. No
// ordinary expression in this program enables descriptor emission for it.
func PropertiesInSeq() *hir.Program {
	base := &hir.Class{Name: "seq_property_base"}
	child := &hir.Class{Name: "seq_property_child", Super: base.Name, Fields: []hir.Field{{Name: "extra", Type: str}}}
	ref := hir.Ref(base.Name)
	seq := &hir.Expr{Kind: hir.Seq, Type: boolean, Stmt: hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Name: "config", Type: ref, X: &hir.Expr{Kind: hir.New, Type: hir.Ref(child.Name)}},
		&hir.Stmt{Kind: hir.VarDecl, Name: "has", Type: boolean, X: rt("object.has", hir.V("config", ref), boolean, hir.L(str, "extra"))},
	), Y: hir.V("has", boolean)}
	probe := &hir.Class{Name: "seq_property_fixture", Methods: []*hir.Method{checkMethod("case_000", nil, seq)}}
	runner(probe)
	return &hir.Program{Classes: []*hir.Class{base, child, probe}}
}
