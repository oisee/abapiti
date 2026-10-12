package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// The existing reviewed XML runtime implements these exact fast-xml-parser
// options. Recognize the checked external binding, not a class named XMLParser.
// No parser instance, mutable options or arbitrary option defaults are modeled.
func (l *lowerer) xmlParserCall(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	callee := n.Expression()
	if callee == nil || callee.Kind != ast.KindPropertyAccessExpression || callee.Name().Text() != "parse" {
		return nil, false
	}
	factory := callee.Expression()
	if factory.Kind != ast.KindNewExpression || factory.Expression() == nil {
		return nil, false
	}
	sym := l.resolve(factory.Expression())
	if sym == nil || sym.Name != "XMLParser" {
		return nil, false
	}
	file := l.fileOfSymbol(sym)
	if file == nil || !strings.Contains(strings.ReplaceAll(file.FileName(), "\\", "/"), "/node_modules/fast-xml-parser/") {
		return nil, false
	}
	args := factory.Arguments()
	valid := len(args) == 1 && args[0].Kind == ast.KindObjectLiteralExpression
	seen := map[string]bool{}
	if valid {
		for _, p := range args[0].AsObjectLiteralExpression().Properties.Nodes {
			if p.Kind != ast.KindPropertyAssignment || p.Name() == nil {
				valid = false
				break
			}
			name := p.Name().Text()
			wanted := ast.KindFalseKeyword
			switch name {
			case "parseTagValue", "trimValues":
			case "ignoreAttributes":
				wanted = ast.KindTrueKeyword
			default:
				valid = false
			}
			if seen[name] || p.Initializer() == nil || p.Initializer().Kind != wanted {
				valid = false
			}
			seen[name] = true
		}
	}
	if !valid || len(seen) != 3 || len(n.Arguments()) != 1 || n.Arguments()[0].Kind == ast.KindSpreadElement {
		l.diagf(n, "unsupported-xml", "XMLParser.parse requires literal parseTagValue:false, ignoreAttributes:true, trimValues:false and one string argument")
		return nil, true
	}
	value := l.expr(n.Arguments()[0])
	if value == nil {
		return nil, true
	}
	if value.Type.Kind != hir.String {
		l.diagf(n, "unsupported-xml", "XMLParser.parse input coercion is not lowered")
		return nil, true
	}
	l.builtinError()
	return l.rtOp("xml.parseWithErrors", value, hir.T(hir.Dynamic)), true
}
