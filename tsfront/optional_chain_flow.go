package tsfront

import "github.com/oisee/abapiti/internal/tsgo/ast"

// Distinguish actual flow elimination of undefined from a nonoptional static
// annotation: optional access on an erased undefined still returns undefined.
func (l *lowerer) optionalChainFlowPresent(n *ast.Node) bool {
	if n.Kind != ast.KindPropertyAccessExpression || l.checkerMayBeUndefined(n) {
		return false
	}
	if sym := l.resolve(n); sym != nil && typeMayBeUndefined(l.ck.GetTypeOfSymbol(sym)) {
		return true
	}
	base := n.Expression()
	if base != nil && !l.checkerMayBeUndefined(base) {
		if sym := l.resolve(base); sym != nil && typeMayBeUndefined(l.ck.GetTypeOfSymbol(sym)) {
			return true
		}
	}
	return false
}
