package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// matchAllIterator owns a clone of the RegExp, including its initial cursor.
// It shares the generator consumer ABI, but IteratorClose is a no-op: native
// RegExp string iterators have no return method and remain resumable on break.
func (l *lowerer) matchAllIterator(n *ast.Node, recv *hir.Expr) *hir.Expr {
	if recv.Type.Kind != hir.String {
		l.diagf(n, "unsupported-call", "matchAll receiver is not a string")
		return nil
	}
	if len(n.Arguments()) != 1 {
		l.diagf(n, "unsupported-regex", "matchAll requires one RegExp argument")
		return nil
	}
	recv = l.tempInit(n, recv.Type, recv)
	re := l.expr(n.Arguments()[0])
	if re == nil {
		return nil
	}
	if re.Type.Kind != hir.RegExp {
		l.diagf(n, "unsupported-regex", "matchAll requires a RegExp argument")
		return nil
	}
	re = l.tempInit(n, re.Type, re)
	str, b := hir.T(hir.String), hir.T(hir.Bool)
	flags := l.rtOp("regexp.flags", re, str)
	flags.Node = l.node(n)
	missing := &hir.Expr{Kind: hir.Binary, Op: "<", Type: b, X: l.rtOp("string.indexOf", flags, hir.T(hir.I32), hir.L(str, "g")), Y: hir.L(hir.T(hir.I32), 0)}
	errorType := hir.Ref(l.builtinTypeError())
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: missing, Body: hir.B(&hir.Stmt{Kind: hir.Throw, Node: l.node(n), X: &hir.Expr{Kind: hir.New, Node: l.node(n), Type: errorType, Args: []*hir.Expr{l.coerce(hir.L(str, "String.prototype.matchAll called with a non-global RegExp argument"), hir.T(hir.Optional, str))}}})})
	a := l.ensureGeneratorABI(hir.T(hir.Array, str))
	typ := hir.Ref("regexp.stringIterator")
	if l.classByName(typ.Name) == nil {
		node := l.node(n)
		self := &hir.Expr{Kind: hir.This, Type: typ}
		field := func(name string, t hir.Type) *hir.Expr {
			return &hir.Expr{Kind: hir.FieldGet, X: self, Name: name, Type: t}
		}
		regex, text, done := field("regex", hir.T(hir.RegExp)), field("text", str), field("done", b)
		c := &hir.Class{Node: node, Name: typ.Name, Implements: []string{a.iterator.Name}, Fields: []hir.Field{{Name: "regex", Type: regex.Type}, {Name: "text", Type: str}, {Name: "done", Type: b}}}
		c.Ctor = &hir.Method{Node: node, Name: "constructor", Result: hir.T(hir.Void), Params: []hir.Param{{Name: "regex", Type: regex.Type}, {Name: "text", Type: str}}, Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: regex, Y: hir.V("regex", regex.Type)}, &hir.Stmt{Kind: hir.Assign, X: text, Y: hir.V("text", str)})}
		value := hir.V("match", hir.T(hir.Optional, a.element))
		end := &hir.Expr{Kind: hir.New, Type: a.result, Args: []*hir.Expr{hir.L(b, true), hir.L(value.Type, nil)}}
		advance := l.rtOp("regexp.matchAllNext", regex, value.Type, text)
		advance.Node = node
		next := &hir.Method{Node: node, Name: "next", Virtual: true, Result: a.result, Body: hir.B(
			&hir.Stmt{Kind: hir.If, X: done, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: end})},
			&hir.Stmt{Kind: hir.VarDecl, Name: value.Name, Type: value.Type, X: advance},
			&hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.IsUndefined, Type: b, X: value}, Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: done, Y: hir.L(b, true)}, &hir.Stmt{Kind: hir.Return, X: end})},
			&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Type: a.result, Args: []*hir.Expr{hir.L(b, false), value}}})}
		c.Methods = []*hir.Method{next, {Node: node, Name: "close", Virtual: true, Result: hir.T(hir.Void), Body: hir.B()}, {Node: node, Name: "iterate", Virtual: true, Result: a.iterator, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.Cast, Type: a.iterator, X: self}})}}
		l.out.Classes = append(l.out.Classes, c)
	}
	clone := l.rtOp("regexp.clone", re, re.Type)
	clone.Node = l.node(n)
	return &hir.Expr{Kind: hir.Cast, Node: l.node(n), Type: a.iterator, X: &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ, Args: []*hir.Expr{clone, recv}}}
}

// Named groups retain an object of possibly undefined values. The lib.d.ts
// string index signature does not prove participation of any capture.
func (l *lowerer) matchGroupValue(n *ast.Node, x *hir.Expr) bool {
	for x != nil && x.Kind == hir.Seq {
		x = x.Y
	}
	return x != nil && x.Kind == hir.RuntimeOp && x.Op == "array.matchGroups" || n != nil && n.Kind == ast.KindIdentifier && l.matchGroupBindings[l.resolve(n)]
}
