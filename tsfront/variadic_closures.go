package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// fixedRestTuple returns the element types of a rest parameter whose type
// is a tuple of required elements only (...args: [string, number]), or nil.
// Such a signature takes exactly that fixed parameter list.
func (l *lowerer) fixedRestTuple(p *ast.Symbol) []*checker.Type {
	d := p.ValueDeclaration
	if d == nil || d.Kind != ast.KindParameter || d.AsParameterDeclaration().DotDotDotToken == nil {
		return nil
	}
	t := l.ck.GetTypeOfSymbol(p)
	if t == nil || !t.IsTupleType() {
		return nil
	}
	flags := t.Target().AsTupleType().ElementFlags()
	if len(flags) == 0 {
		return nil
	}
	out := []*checker.Type{}
	for i, f := range flags {
		if f != checker.ElementFlagsRequired {
			return nil
		}
		sym := l.ck.GetPropertyOfType(t, itoa(i))
		if sym == nil {
			return nil
		}
		out = append(out, l.ck.GetTypeOfSymbol(sym))
	}
	return out
}

// variadicClosureAdapter converts a function value between a signature
// ending in a rest array (an erased variadic tuple type parameter:
// (ctx, obj, ...args: unknown[]) => T) and a fixed one with the same
// leading parameters ((ctx, obj, lower, session) => T), as JavaScript calls
// it: fixed arguments travel as the rest array's elements in order, a rest
// element beyond the array's length is undefined. handled reports that the
// pair has this shape; a nil result then refuses the conversion.
func (l *lowerer) variadicClosureAdapter(n *ast.Node, x *hir.Expr, dst hir.Type, inner, outer *hir.Method) (*hir.Expr, bool) {
	variadic := func(m *hir.Method) bool {
		k := len(m.Params)
		return k > 0 && m.Params[k-1].Variadic && m.Params[k-1].Type.Kind == hir.Array && len(m.Params[k-1].Type.Args) == 1
	}
	fixed := func(m *hir.Method) bool {
		for _, p := range m.Params {
			if p.Variadic {
				return false
			}
		}
		return true
	}
	outerRest, innerRest := variadic(outer) && fixed(inner), variadic(inner) && fixed(outer)
	if !outerRest && !innerRest {
		return nil, false
	}
	prefix := len(outer.Params) - 1
	if innerRest {
		prefix = len(inner.Params) - 1
	}
	if len(inner.Params) < prefix || len(outer.Params) < prefix {
		return nil, true
	}
	saved := l.pend
	l.pend = nil
	fail := func() (*hir.Expr, bool) {
		l.pend = saved
		return nil, true
	}
	l.serial++
	name := "closure.adapt." + itoa(l.serial)
	self := &hir.Expr{Kind: hir.This, Type: hir.Ref(name)}
	field := &hir.Expr{Kind: hir.FieldGet, Name: "inner", Type: x.Type, X: self}
	i32 := hir.T(hir.I32)
	var params []hir.Param
	var args []*hir.Expr
	var stmts []*hir.Stmt
	for i, p := range outer.Params {
		params = append(params, hir.Param{Name: "arg" + itoa(i), Type: p.Type})
	}
	// Conversion preludes run in the adapter body, before the call.
	take := func() []*hir.Stmt {
		p := l.pend
		l.pend = nil
		return p
	}
	// The instantiation proves each argument has the inner parameter's type
	// (ctx: C erased to its constraint, passed on as the narrower C): a
	// checked downcast, raising on mismatch, never retagging.
	convert := func(v *hir.Expr, want hir.Type) *hir.Expr {
		if c := l.coerce(v, want); c != nil && l.acceptsType(want, c.Type) {
			return c
		}
		l.pend = nil
		if !want.IsRef() || !v.Type.IsRef() && v.Type.Kind != hir.Dynamic {
			return nil
		}
		// The erased storage boundary: a live checked view of the same
		// object (identity, extra keys and mutations kept).
		return l.dynamicReference(l.coerce(v, hir.T(hir.Dynamic)), want)
	}
	for i := 0; i < prefix; i++ {
		a := convert(hir.V("arg"+itoa(i), outer.Params[i].Type), inner.Params[i].Type)
		if a == nil || !l.acceptsType(inner.Params[i].Type, a.Type) {
			return fail()
		}
		stmts = append(stmts, take()...)
		args = append(args, a)
	}
	if outerRest {
		rest := hir.V("arg"+itoa(prefix), outer.Params[prefix].Type)
		elem := rest.Type.Args[0]
		for i := prefix; i < len(inner.Params); i++ {
			want := inner.Params[i].Type
			k := hir.L(i32, int64(i-prefix))
			value := convert(&hir.Expr{Kind: hir.Narrow, Type: elem, X: l.rtOp("array.get", rest, hir.T(hir.Optional, elem), k)}, want)
			if value == nil || !l.acceptsType(want, value.Type) {
				return fail()
			}
			prelude := take()
			if want.Kind != hir.Optional {
				// A missing element would bind undefined to a parameter
				// that cannot hold it: the checked read raises.
				stmts = append(stmts, prelude...)
				args = append(args, value)
				continue
			}
			// A missing rest element is undefined.
			slot := hir.V("elem"+itoa(i), want)
			present := &hir.Expr{Kind: hir.Binary, Op: "<", Type: hir.T(hir.Bool), X: hir.L(i32, int64(i-prefix)), Y: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.length", Type: i32, X: rest}}
			stmts = append(stmts, &hir.Stmt{Kind: hir.VarDecl, Name: slot.Name, Type: want, X: &hir.Expr{Kind: hir.Lit, Type: want}},
				&hir.Stmt{Kind: hir.If, X: present, Body: hir.B(append(prelude, &hir.Stmt{Kind: hir.Assign, X: slot, Y: value})...)})
			args = append(args, slot)
		}
	} else {
		restType := inner.Params[prefix].Type
		rest := hir.V("rest", restType)
		stmts = append(stmts, &hir.Stmt{Kind: hir.VarDecl, Name: rest.Name, Type: restType, X: &hir.Expr{Kind: hir.New, Type: restType}})
		for i := prefix; i < len(outer.Params); i++ {
			v := l.coerce(hir.V("arg"+itoa(i), outer.Params[i].Type), restType.Args[0])
			if v == nil || !l.acceptsType(restType.Args[0], v.Type) {
				return fail()
			}
			stmts = append(stmts, take()...)
			stmts = append(stmts, &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: i32, X: rest, Args: []*hir.Expr{v}}})
		}
		args = append(args, rest)
	}
	call := &hir.Expr{Kind: hir.VirtualCall, Name: inner.Name, Type: inner.Result, X: field, Args: args}
	switch {
	case outer.Result.Kind == hir.Void:
		stmts = append(stmts, &hir.Stmt{Kind: hir.ExprStmt, X: call})
	case inner.Result.Kind == hir.Void && outer.Result.Kind == hir.Dynamic:
		stmts = append(stmts, &hir.Stmt{Kind: hir.ExprStmt, X: call}, &hir.Stmt{Kind: hir.Return, X: l.rtOp("dynamic.of", &hir.Expr{Kind: hir.Lit, Type: hir.T(hir.Optional, hir.T(hir.Dynamic))}, hir.T(hir.Dynamic))})
	default:
		result := l.coerce(call, outer.Result)
		if result == nil || !l.acceptsType(outer.Result, result.Type) {
			return fail()
		}
		stmts = append(stmts, take()...)
		stmts = append(stmts, &hir.Stmt{Kind: hir.Return, X: result})
	}
	l.pend = saved
	l.diagf(n, "note-closure-adapter", "function value converted between rest-array and fixed signatures")
	return l.adapterClass(n, name, x, dst, &hir.Method{Name: outer.Name, Params: params, Result: outer.Result, Body: hir.B(stmts...)}), true
}

// adapterClass registers closure adapter class name wrapping x as dst with
// method m, and returns the wrapped value.
func (l *lowerer) adapterClass(n *ast.Node, name string, x *hir.Expr, dst hir.Type, m *hir.Method) *hir.Expr {
	field := &hir.Expr{Kind: hir.FieldGet, Name: "inner", Type: x.Type, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(name)}}
	c := &hir.Class{Node: l.node(n), Name: name, Implements: []string{dst.Name},
		Fields: []hir.Field{{Name: "inner", Type: x.Type}}}
	c.Ctor = &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Params: []hir.Param{{Name: "inner", Type: x.Type}},
		Body: hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: field, Y: hir.V("inner", x.Type)})}
	m.Body.Node = l.node(n)
	for _, st := range m.Body.List {
		st.Node = l.node(n)
	}
	c.Methods = []*hir.Method{m}
	l.out.Classes = append(l.out.Classes, c)
	return &hir.Expr{Kind: hir.Cast, Node: l.node(n), Type: dst, X: &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(name), Args: []*hir.Expr{x}}}
}
