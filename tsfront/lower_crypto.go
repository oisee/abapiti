package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Recognize an unescaped, fresh Node SHA-1 hash pipeline, not general Hash
// objects. No intermediate state can be observed or reused in this form.
func (l *lowerer) cryptoHashCall(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	methodCall := func(call *ast.Node, name string) *ast.Node {
		if call == nil || call.Kind != ast.KindCallExpression {
			return nil
		}
		prop := call.Expression()
		if prop == nil || prop.Kind != ast.KindPropertyAccessExpression || prop.Name().Text() != name || prop.AsPropertyAccessExpression().QuestionDotToken != nil {
			return nil
		}
		return prop.Expression()
	}
	update := methodCall(n, "digest")
	create := methodCall(update, "update")
	receiver := methodCall(create, "createHash")
	if receiver == nil || receiver.Kind != ast.KindIdentifier {
		return nil, false
	}
	// Check the actual namespace import binding, including aliased names;
	// never recognize a local object merely named crypto.
	sym := l.ck.GetSymbolAtLocation(receiver)
	if sym == nil {
		return nil, false
	}
	imported := false
	for _, decl := range sym.Declarations {
		if decl.Kind != ast.KindNamespaceImport {
			continue
		}
		parent := decl.Parent
		if parent == nil || parent.Kind != ast.KindImportClause {
			continue
		}
		parent = parent.Parent
		if parent != nil && parent.Kind == ast.KindImportDeclaration {
			spec := parent.AsImportDeclaration().ModuleSpecifier
			imported = spec != nil && (spec.Text() == "crypto" || spec.Text() == "node:crypto")
		}
	}
	if !imported {
		return nil, false
	}
	literalArg := func(call *ast.Node, want string) bool {
		args := call.Arguments()
		return len(args) == 1 && args[0].Kind == ast.KindStringLiteral && args[0].Text() == want
	}
	if !literalArg(create, "sha1") || !literalArg(n, "hex") || len(update.Arguments()) != 1 || update.Arguments()[0].Kind == ast.KindSpreadElement {
		l.diagf(n, "unsupported-crypto", "fresh hash pipeline requires sha1, one UTF-8 string update and hex digest")
		return nil, true
	}
	old := l.hint
	l.hint = hir.Type{}
	value := l.expr(update.Arguments()[0])
	l.hint = old
	if value == nil {
		return nil, true
	}
	if value.Type.Kind != hir.String {
		l.diagf(n, "unsupported-crypto", "hash input coercion/buffers are unsupported")
		return nil, true
	}
	return l.rtOp("string.sha1Hex", value, hir.T(hir.String)), true
}
