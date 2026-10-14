package tsfront

import (
	"github.com/oisee/abapiti/internal/tsgo/ast"

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
