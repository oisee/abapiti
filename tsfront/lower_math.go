package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Numeric-only Math.ceil keeps IEEE-754 results, including signed zero, NaN
// and infinities. ToNumber objects/strings and spread arguments refuse.
func (l *lowerer) mathCeilCall(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	callee := n.Expression()
	if callee == nil || callee.Kind != ast.KindPropertyAccessExpression || callee.Name().Text() != "ceil" {
		return nil, false
	}
	receiver := callee.Expression()
	if receiver.Kind != ast.KindIdentifier || receiver.Text() != "Math" {
		return nil, false
	}
	// A global can merge library and user declarations. Dispatch only when
	// the complete binding has library provenance; one declaration is insufficient.
	if !l.librarySymbol(l.resolve(receiver)) {
		return nil, false
	}
	args := n.Arguments()
	if len(args) != 1 || args[0].Kind == ast.KindSpreadElement {
		l.diagf(n, "unsupported-call", "Math.ceil requires exactly one numeric argument")
		return nil, true
	}
	x := l.expr(args[0])
	if x == nil {
		return nil, true
	}
	if x.Type.Kind != hir.Number && x.Type.Kind != hir.I32 && x.Type.Kind != hir.I64 {
		l.diagf(n, "unsupported-call", "Math.ceil input coercion is unsupported")
		return nil, true
	}
	return l.rtOp("number.ceil", l.coerce(x, hir.T(hir.Number)), hir.T(hir.Number)), true
}
