package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// An erased initializer can be undefined despite a primitive annotation. A
// constant observed only by strict comparisons can retain that absence without
// widening any parameter, return, capture or mutable binding ABI.
func (l *lowerer) comparisonScalar(d *ast.Node, typ hir.Type) bool {
	if !l.unprunedFeatures || d.Type() == nil || d.Initializer() == nil || d.Parent == nil || d.Parent.Flags&ast.NodeFlagsConst == 0 || (typ.Kind != hir.String && typ.Kind != hir.Number && typ.Kind != hir.Bool) {
		return false
	}
	initial := l.ck.GetTypeAtLocation(d.Initializer())
	if initial == nil || initial.Flags()&checker.TypeFlagsAny == 0 {
		return false
	}
	function := d.Parent
	for function != nil && !ast.IsFunctionLike(function) {
		function = function.Parent
	}
	if function == nil || function.Body() == nil {
		return false
	}
	valid, used := true, false
	var walk func(*ast.Node)
	walk = func(n *ast.Node) {
		if n == nil || !valid {
			return
		}
		if n.Kind == ast.KindIdentifier && n != d.Name() {
			symbol := l.resolve(n)
			if n.Parent != nil && n.Parent.Kind == ast.KindShorthandPropertyAssignment {
				symbol = l.ck.GetShorthandAssignmentValueSymbol(n.Parent)
			}
			if symbol == d.Symbol() {
				used = true
				for parent := n.Parent; parent != nil && parent != function; parent = parent.Parent {
					if ast.IsFunctionLike(parent) {
						valid = false
						return
					}
				}
				parent := n.Parent
				for parent != nil && parent.Kind == ast.KindParenthesizedExpression {
					parent = parent.Parent
				}
				if parent == nil || parent.Kind != ast.KindBinaryExpression {
					valid = false
					return
				}
				op := parent.AsBinaryExpression().OperatorToken.Kind
				if op != ast.KindEqualsEqualsEqualsToken && op != ast.KindExclamationEqualsEqualsToken {
					valid = false
					return
				}
			}
		}
		n.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
	}
	walk(function.Body())
	return valid && used
}
