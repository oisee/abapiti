package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Array assignment consumes the original array in order. In particular, an
// earlier write may change the next element when source and target alias.
func (l *lowerer) arrayAssignment(at, pattern, rhs *ast.Node) *hir.Expr {
	keep := l.hint
	l.hint = hir.Type{}
	literal := rhs
	for literal.Kind == ast.KindParenthesizedExpression {
		literal = literal.Expression()
	}
	if literal.Kind == ast.KindArrayLiteralExpression && l.unprunedFeatures {
		typ := l.mapCheckerType(rhs, l.ck.GetTypeAtLocation(rhs))
		if typ.Kind == hir.Array && (typ.Args[0].Kind == hir.Number || typ.Args[0].Kind == hir.String || typ.Args[0].Kind == hir.Bool) {
			// TypeScript's unchecked index types can hide an absent local.
			// Snapshot literal operands without unwrapping those values.
			l.hint = hir.T(hir.Array, hir.T(hir.Optional, typ.Args[0]))
		}
	}
	value := l.expr(rhs)
	l.hint = keep
	if value == nil {
		return nil
	}
	if value.Type.Kind != hir.Array {
		l.diagf(rhs, "unsupported-assignment", "array assignment requires lowered array storage")
		return nil
	}
	value = l.tempInit(rhs, value.Type, value)
	done := l.tempInit(at, hir.T(hir.Bool), hir.L(hir.T(hir.Bool), false))
	step := func(el *ast.Node, i int) *hir.Expr {
		end := &hir.Expr{Kind: hir.Binary, Node: l.node(el), Type: hir.T(hir.Bool), Op: ">=", X: hir.L(hir.T(hir.I32), i), Y: &hir.Expr{Kind: hir.RuntimeOp, Node: l.node(el), Op: "array.length", Type: hir.T(hir.I32), X: value}}
		l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(el), X: end, Body: hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(el), X: done, Y: hir.L(hir.T(hir.Bool), true)})})
		item := l.rtOp("array.get", value, hir.Type{}, hir.L(hir.T(hir.I32), i))
		return &hir.Expr{Kind: hir.Conditional, Node: l.node(el), Type: item.Type, X: done, Y: &hir.Expr{Kind: hir.Lit, Node: l.node(el), Type: item.Type}, Z: item}
	}
	for i, el := range pattern.AsArrayLiteralExpression().Elements.Nodes {
		if el.Kind == ast.KindOmittedExpression {
			step(el, i)
			continue
		}
		if el.Kind != ast.KindIdentifier && el.Kind != ast.KindPropertyAccessExpression && el.Kind != ast.KindElementAccessExpression {
			l.diagf(el, "unsupported-assignment", "array assignment element %s is not lowered", el.Kind.String())
			return nil
		}
		target := l.assignTarget(el)
		if target == nil {
			return nil
		}
		// Evaluate the assignment reference before advancing the iterator.
		switch target.Kind {
		case hir.FieldGet:
			target.X = l.tempInit(el, target.X.Type, target.X)
		case hir.IndexGet:
			target.X = l.tempInit(el, target.X.Type, target.X)
			target.Y = l.tempInit(el, target.Y.Type, target.Y)
		case hir.Local, hir.StaticGet:
		default:
			l.diagf(el, "unsupported-assignment", "array assignment requires a stored target")
			return nil
		}
		item := step(el, i)
		l.pendStmt(&hir.Stmt{Kind: hir.Assign, Node: l.node(el), X: target, Y: l.coerce(item, target.Type)})
	}
	return value
}

// A mutable primitive binding receiving array elements must retain undefined
// even when TypeScript's unchecked array-index type says the element exists.
func (l *lowerer) arrayAssignmentScalar(d *ast.Node, typ hir.Type) bool {
	if !l.unprunedFeatures || (typ.Kind != hir.Number && typ.Kind != hir.String && typ.Kind != hir.Bool) {
		return false
	}
	fn := d.Parent
	for fn != nil && !ast.IsFunctionLike(fn) {
		fn = fn.Parent
	}
	if fn == nil || fn.Body() == nil {
		return false
	}
	found := false
	var walk func(*ast.Node)
	walk = func(n *ast.Node) {
		if n == nil || found {
			return
		}
		if n.Kind == ast.KindBinaryExpression {
			b := n.AsBinaryExpression()
			if b.OperatorToken.Kind == ast.KindEqualsToken && b.Left.Kind == ast.KindArrayLiteralExpression {
				for _, el := range b.Left.AsArrayLiteralExpression().Elements.Nodes {
					if el.Kind == ast.KindIdentifier && l.resolve(el) == d.Symbol() {
						found = true
						return
					}
				}
			}
		}
		n.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(fn.Body())
	return found
}

func isArrayAssignment(n *ast.Node) bool {
	for n != nil && n.Kind == ast.KindParenthesizedExpression {
		n = n.Expression()
	}
	return n != nil && n.Kind == ast.KindBinaryExpression && n.AsBinaryExpression().OperatorToken.Kind == ast.KindEqualsToken && n.AsBinaryExpression().Left.Kind == ast.KindArrayLiteralExpression
}
