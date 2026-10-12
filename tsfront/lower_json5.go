package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Recognize the checked package function (including renamed imports), not the
// spelling JSON5. Revivers and coercion remain explicit refusals.
func (l *lowerer) json5Call(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	callee := n.Expression()
	if callee == nil {
		return nil, false
	}
	sym := l.resolve(callee)
	if sym == nil || sym.Name != "parse" {
		return nil, false
	}
	file := l.fileOfSymbol(sym)
	if file == nil || !strings.Contains(strings.ReplaceAll(file.FileName(), "\\", "/"), "/node_modules/json5/") {
		return nil, false
	}
	args := n.Arguments()
	if len(args) != 1 || args[0].Kind == ast.KindSpreadElement {
		l.diagf(n, "unsupported-json5", "JSON5.parse requires one string argument; revivers are unsupported")
		return nil, true
	}
	saved := l.hint
	l.hint = hir.Type{}
	value := l.expr(args[0])
	l.hint = saved
	if value == nil {
		return nil, true
	}
	if value.Type.Kind != hir.String {
		l.diagf(n, "unsupported-json5", "JSON5.parse input coercion is unsupported")
		return nil, true
	}
	return l.rtOp("json5.parse", value, hir.T(hir.Dynamic)), true
}
