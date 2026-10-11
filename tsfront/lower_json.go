package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// JSON intrinsics are recognized by their default-library binding, never by
// spelling alone. Revivers, replacers and indentation remain explicit refusals.
func (l *lowerer) jsonCall(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	c := n.Expression()
	if c == nil || c.Kind != ast.KindPropertyAccessExpression {
		return nil, false
	}
	receiver := c.Expression()
	if receiver.Kind != ast.KindIdentifier || receiver.Text() != "JSON" {
		return nil, false
	}
	file := l.fileOfSymbol(l.resolve(receiver))
	if file == nil || !l.prog.prog.IsSourceFileDefaultLibrary(file.Path()) {
		return nil, false
	}
	name := c.Name().Text()
	if name != "parse" && name != "stringify" {
		return nil, false
	}
	args := n.Arguments()
	if len(args) != 1 || args[0].Kind == ast.KindSpreadElement {
		l.diagf(n, "unsupported-json", "JSON.%s requires exactly one argument; revivers/replacers/space are unsupported", name)
		return nil, true
	}
	value := l.expr(args[0])
	if value == nil {
		return nil, true
	}
	if name == "parse" {
		if value.Type.Kind != hir.String {
			l.diagf(n, "unsupported-json", "JSON.parse input coercion is unsupported")
			return nil, true
		}
		return l.rtOp("json.parseSubset", value, hir.T(hir.Dynamic)), true
	}
	// The writer preserves tagged-record insertion order, array positions and
	// primitive encoding. Native instances without an own-key-order proof trap.
	return l.rtOp("json.stringifySubset", value, hir.T(hir.String)), true
}
