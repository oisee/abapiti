package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
)

// A source-pinned adapter for unused match arrays: only null/truth checks.
// This deliberately does not expose a general String.match/capture-array ABI.
func (l *lowerer) pinnedMatchTest(n *ast.Node) (*hir.Expr, bool) {
	call, negate := n, false
	if n.Kind == ast.KindBinaryExpression {
		binary := n.AsBinaryExpression()
		if binary.Right.Kind != ast.KindNullKeyword {
			return nil, false
		}
		switch binary.OperatorToken.Kind {
		case ast.KindEqualsEqualsEqualsToken, ast.KindEqualsEqualsToken:
			negate = true
		case ast.KindExclamationEqualsEqualsToken, ast.KindExclamationEqualsToken:
		default:
			return nil, false
		}
		call = binary.Left
	}
	if call.Kind != ast.KindCallExpression || call.Expression().Kind != ast.KindPropertyAccessExpression || call.Expression().Name().Text() != "match" || len(call.Arguments()) != 1 {
		return nil, false
	}
	text := l.file.Text()[scanner.GetTokenPosOfNode(call, l.file, false):call.End()]
	permitted := false
	for parent := call; parent != nil; parent = parent.Parent {
		if entry, ok := l.overrides[parent]; ok && entry.Patterns != nil && entry.Patterns.MatchTests[text] {
			l.diagf(n, "note-override", "%s: %s", entry.ID, entry.Rationale)
			permitted = true
			break
		}
	}
	if !permitted {
		return nil, false
	}
	receiver := l.expr(call.Expression().Expression())
	if receiver == nil {
		return nil, true
	}
	if receiver.Type.Kind != hir.String {
		l.diagf(n, "unsupported-match-test", "pinned match receiver is not a string")
		return nil, true
	}
	// JS evaluates the string receiver before evaluating the regexp argument.
	receiver = l.tempInit(n, receiver.Type, receiver)
	pattern := l.expr(call.Arguments()[0])
	if pattern == nil {
		return nil, true
	}
	if pattern.Type.Kind != hir.RegExp {
		l.diagf(n, "unsupported-match-test", "pinned match needs a lowered RegExp")
		return nil, true
	}
	result := l.rtOp("regexp.match_test", pattern, hir.T(hir.Bool), receiver)
	if negate {
		result = &hir.Expr{Kind: hir.Unary, Type: hir.T(hir.Bool), Op: "!", X: result}
	}
	return result, true
}
