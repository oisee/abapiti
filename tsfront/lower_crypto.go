package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Recognize an unescaped, fresh Node SHA-1/SHA-256 hash pipeline, not
// general Hash objects. createHash may be a namespace member or a named
// (possibly renamed) import of node:crypto. No intermediate state can be
// observed or reused in this form.
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
	if create == nil || create.Kind != ast.KindCallExpression || !l.nodeBuiltinCallee(create.Expression(), "crypto", "createHash") {
		return nil, false
	}
	literalArg := func(call *ast.Node, want string) bool {
		args := call.Arguments()
		return len(args) == 1 && args[0].Kind == ast.KindStringLiteral && args[0].Text() == want
	}
	op := ""
	switch {
	case literalArg(create, "sha1"):
		op = "string.sha1Hex"
	case literalArg(create, "sha256"):
		op = "string.sha256Hex"
	}
	if op == "" || !literalArg(n, "hex") || len(update.Arguments()) != 1 || update.Arguments()[0].Kind == ast.KindSpreadElement {
		l.diagf(n, "unsupported-crypto", "fresh hash pipeline requires sha1 or sha256, one UTF-8 string update and hex digest")
		return nil, true
	}
	old := l.hint
	l.hint = hir.Type{}
	value := l.expr(update.Arguments()[0])
	l.hint = old
	if value == nil {
		return nil, true
	}
	if value.Type.Kind == hir.Dynamic {
		// An untyped JavaScript input is hashed only when it holds a string;
		// any other payload (Buffer, number) raises at the checked unboxing.
		value = l.coerce(value, hir.T(hir.String))
	}
	if value.Type.Kind != hir.String {
		l.diagf(n, "unsupported-crypto", "hash input coercion/buffers are unsupported")
		return nil, true
	}
	return l.rtOp(op, value, hir.T(hir.String)), true
}
