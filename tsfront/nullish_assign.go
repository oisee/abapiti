package tsfront

import (
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"

	"github.com/oisee/abapiti/hir"
)

// nullishAssign lowers a ??= b to "if a is undefined: a = b". The receiver of
// a field target is evaluated once, before the test; statements that b's
// lowering leaves pending run inside the branch, so b is evaluated only while
// a is absent, as in JavaScript.
func (l *lowerer) nullishAssign(n *ast.Node) (*hir.Stmt, *hir.Expr) {
	b := n.AsBinaryExpression()
	target := l.assignTarget(b.Left)
	if target == nil {
		return nil, nil
	}
	if target.Kind == hir.RuntimeOp && (target.Op == "dynamic.put" || target.Op == "map.set" && target.X.Type.Args[1].Kind == hir.Dynamic) {
		return l.nullishAssignDynamic(n, target)
	}
	if target.Type.Kind != hir.Optional && !target.Type.IsRef() {
		l.diagf(n, "unsupported-expr", "??= needs an optional local or field")
		return nil, nil
	}
	switch {
	case target.Kind == hir.Local:
	case target.Kind == hir.FieldGet && target.X != nil:
		if target.X.Kind != hir.This {
			target.X = l.tempInit(n, target.X.Type, target.X)
		}
	default:
		l.diagf(n, "unsupported-expr", "??= needs an optional local or field")
		return nil, nil
	}
	base := target.Type
	if base.Kind == hir.Optional {
		base = base.Args[0]
	}
	saved := l.pend
	l.pend = nil
	hint := l.hint
	l.hint = base
	value := l.expr(b.Right)
	l.hint = hint
	inner := l.pend
	l.pend = saved
	if value == nil {
		return nil, nil
	}
	body := append(inner, &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: target, Y: l.coerce(value, target.Type)})
	test := &hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: target}, Body: hir.B(body...)}
	read := target
	if target.Type.Kind == hir.Optional {
		read = &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: base, X: target}
	}
	return test, read
}

// nullishAssignExpr is a ??= b used as a value: the assignment runs as a
// prelude and the expression reads the (now present) target.
func (l *lowerer) nullishAssignExpr(n *ast.Node) *hir.Expr {
	test, read := l.nullishAssign(n)
	if test == nil {
		return nil
	}
	l.pendStmt(test)
	return read
}

// nullishAssignDynamic lowers r[k] ??= b on a tagged record key or a
// string-keyed record of untyped values (a key its index-signature type does
// not declare): read the key once (absent reads undefined); when it is
// undefined or null, evaluate b, store it and yield it; otherwise yield the
// value read.
func (l *lowerer) nullishAssignDynamic(n *ast.Node, target *hir.Expr) (*hir.Stmt, *hir.Expr) {
	dyn := hir.T(hir.Dynamic)
	recv := l.tempInit(n, target.X.Type, target.X)
	key := target.Args[0]
	if key.Kind != hir.Lit {
		key = l.tempInit(n, key.Type, key)
	}
	read := l.rtOp("dynamic.get", recv, dyn, key)
	if target.Op == "map.set" {
		read = l.coerce(l.rtOp("map.get", recv, hir.T(hir.Optional, dyn), key), dyn)
	}
	current := l.tempInit(n, dyn, read)
	saved := l.pend
	l.pend = nil
	hint := l.hint
	l.hint = dyn
	value := l.expr(n.AsBinaryExpression().Right)
	l.hint = hint
	inner := l.pend
	l.pend = saved
	if value == nil {
		return nil, nil
	}
	body := append(inner, &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: current, Y: l.coerce(value, dyn)},
		&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp(target.Op, recv, target.Type, key, current)})
	test := &hir.Stmt{Kind: hir.If, Node: l.node(n), X: l.rtOp("dynamic.isNullish", current, hir.T(hir.Bool)), Body: hir.B(body...)}
	// A declared key's static type views the stored value (checked).
	if left := l.ck.GetTypeAtLocation(n.AsBinaryExpression().Left); left != nil && left.Flags()&(checker.TypeFlagsUnknown|checker.TypeFlagsAny) == 0 {
		if want := l.mapCheckerType(n, l.ck.GetTypeAtLocation(n)); want.Kind != "" && want.Kind != hir.Dynamic {
			return test, l.coerce(current, want)
		}
	}
	return test, current
}
