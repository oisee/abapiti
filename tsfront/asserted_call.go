package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// A direct module call still observes an explicit assertion on its function's
// result. The implementation's declared ABI supplies the call; the asserted
// data result is a checked, live view, just as an assertion on the value is.
func (l *lowerer) assertedDirectCallResult(n *ast.Node, value *hir.Expr) *hir.Expr {
	if !l.unprunedFeatures {
		return value
	}
	asserted := false
	callee := n.Expression()
	for callee != nil {
		if callee.Kind == ast.KindAsExpression || callee.Kind == ast.KindTypeAssertionExpression {
			asserted = true
		} else if callee.Kind != ast.KindParenthesizedExpression {
			break
		}
		callee = callee.Expression()
	}
	if !asserted {
		return value
	}
	before := len(l.diags)
	target := l.mapCheckerType(n, l.ck.GetTypeAtLocation(n))
	if hasBlocking(l.diags[before:]) {
		l.diags = l.diags[:before]
		return value
	}
	if target.Equal(value.Type) {
		return value
	}
	if target.Kind == hir.ClassRef && l.shapeLike[target.Name] {
		if value.Type.Kind == hir.UnionRef {
			if projected := l.narrowed(n, value, target); projected != value {
				return projected
			}
		}
		return l.dynamicReference(l.coerce(value, hir.T(hir.Dynamic)), target)
	}
	if target.Kind == hir.UnionRef {
		if family := l.out.Union(target.Name); family != nil && family.Complete {
			return l.rtOp("union.project", l.coerce(value, hir.T(hir.Dynamic)), target)
		}
	}
	return value
}
