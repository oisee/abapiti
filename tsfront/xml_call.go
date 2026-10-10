package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"strings"
)

// xmlSubsetCall recognizes the immediate external parser boundary. It does
// not create an XMLParser object or promise arbitrary fast-xml-parser support.
func (l *lowerer) xmlSubsetCall(n, callee *ast.Node) (*hir.Expr, bool) {
	if callee.Kind != ast.KindPropertyAccessExpression || callee.Name().Text() != "parse" {
		return nil, false
	}
	ctor := callee.Expression()
	if ctor.Kind != ast.KindNewExpression || ctor.Expression() == nil {
		return nil, false
	}
	symbol := l.resolve(ctor.Expression())
	if symbol == nil || symbol.Name != "XMLParser" {
		return nil, false
	}
	f := l.fileOfSymbol(symbol)
	if f == nil || !strings.Contains(strings.ReplaceAll(f.FileName(), "\\", "/"), "/node_modules/fast-xml-parser/") {
		return nil, false
	}
	refuse := func() (*hir.Expr, bool) {
		l.diagf(n, "unsupported-xml-parser", "fast-xml-parser supports only immediate parse(string) with parseTagValue:false, ignoreAttributes:true, trimValues:false; input outside reviewed XML subset traps")
		return nil, true
	}
	args := ctor.Arguments()
	if len(args) != 1 || args[0].Kind != ast.KindObjectLiteralExpression || len(n.Arguments()) != 1 {
		return refuse()
	}
	expected := map[string]ast.Kind{"parseTagValue": ast.KindFalseKeyword, "ignoreAttributes": ast.KindTrueKeyword, "trimValues": ast.KindFalseKeyword}
	for _, prop := range args[0].AsObjectLiteralExpression().Properties.Nodes {
		if prop.Kind != ast.KindPropertyAssignment || prop.Name() == nil || prop.Name().Kind != ast.KindIdentifier {
			return refuse()
		}
		key := prop.Name().Text()
		kind, ok := expected[key]
		if !ok || prop.Initializer().Kind != kind {
			return refuse()
		}
		delete(expected, key)
	}
	if len(expected) != 0 {
		return refuse()
	}
	input := l.expr(n.Arguments()[0])
	if input == nil {
		return nil, true
	}
	if input.Type.Kind != hir.String {
		return refuse()
	}
	l.diagf(n, "note-xml-subset", "immediate fast-xml-parser parse uses reviewed XML subset; outside domain traps")
	return l.rtOp("xml.parseSubset", input, hir.T(hir.Dynamic)), true
}
