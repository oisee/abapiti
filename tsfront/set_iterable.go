package tsfront

import "github.com/oisee/abapiti/hir"

// A Set iterable creates a fresh cursor on each iteration. Its private sparse
// entry log stays live across deletes, reinsertions, clear and append. Native
// Set iterators have no return method: consumer break does not exhaust them.
func (l *lowerer) generatorSetAdapter(x *hir.Expr, dst hir.Type) *hir.Expr {
	node := x.Node
	if node.Source == "" && l.method != nil {
		node = l.method.Node
	}
	a := l.generatorABIFor(dst)
	typ := hir.Ref("generator.set." + typeSig(a.element)[:16])
	if l.classByName(typ.Name) == nil {
		setType := hir.T(hir.OrderedSet, a.element)
		arrayType := hir.T(hir.Array, a.element)
		num, b := hir.T(hir.Number), hir.T(hir.Bool)
		self := &hir.Expr{Kind: hir.This, Type: typ}
		field := func(name string, t hir.Type) *hir.Expr {
			return &hir.Expr{Kind: hir.FieldGet, Node: node, Name: name, Type: t, X: self}
		}
		set, index, done := field("set", setType), field("index", num), field("done", b)
		c := &hir.Class{Name: typ.Name, Implements: []string{a.iterator.Name}, Fields: []hir.Field{{Name: "set", Type: setType}, {Name: "index", Type: num}, {Name: "done", Type: b}}}
		c.Ctor = &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Params: []hir.Param{{Name: "set", Type: setType}}, Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: set, Y: hir.V("set", setType)})}
		end := &hir.Expr{Kind: hir.New, Type: a.result, Args: []*hir.Expr{hir.L(b, true), hir.L(hir.T(hir.Optional, a.element), nil)}}
		entries := hir.V("entries", arrayType)
		old := hir.V("entryIndex", num)
		more := &hir.Expr{Kind: hir.Binary, Type: b, Op: "<", X: index, Y: l.rtOp("array.length", entries, num)}
		next := &hir.Method{Name: "next", Virtual: true, Result: a.result, Body: hir.B(
			&hir.Stmt{Kind: hir.If, X: done, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: end})},
			&hir.Stmt{Kind: hir.VarDecl, Name: entries.Name, Type: entries.Type, X: l.rtOp("set.iterationValues", set, arrayType)},
			&hir.Stmt{Kind: hir.While, X: more, Body: hir.B(
				&hir.Stmt{Kind: hir.VarDecl, Name: old.Name, Type: num, X: index},
				&hir.Stmt{Kind: hir.Assign, X: index, Y: &hir.Expr{Kind: hir.Binary, Type: num, Op: "+", X: index, Y: hir.L(num, 1)}},
				&hir.Stmt{Kind: hir.If, X: l.rtOp("array.has", entries, b, old), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Type: a.result, Args: []*hir.Expr{hir.L(b, false), l.rtOp("array.get", entries, hir.T(hir.Optional, a.element), old)}}})},
			)},
			&hir.Stmt{Kind: hir.Assign, X: done, Y: hir.L(b, true)}, &hir.Stmt{Kind: hir.Return, X: end})}
		iterate := &hir.Method{Name: "iterate", Virtual: true, Result: a.iterator, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.Cast, Type: a.iterator, X: &hir.Expr{Kind: hir.New, Type: typ, Args: []*hir.Expr{set}}}})}
		c.Methods = []*hir.Method{next, iterate, {Name: "close", Virtual: true, Result: hir.T(hir.Void), Body: hir.B()}}
		l.out.Classes = append(l.out.Classes, c)
	}
	return &hir.Expr{Kind: hir.Cast, Node: x.Node, Type: dst, X: &hir.Expr{Kind: hir.New, Node: x.Node, Type: typ, Args: []*hir.Expr{x}}}
}
