package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Trap mode can retain a synchronous arm beside an unsupported Promise call.
// The refused call still traps when selected; it never fabricates settlement
// or schedules a callback. Other lowering failures keep the whole-body policy.
func (l *lowerer) asyncRefusalExpression(n *ast.Node, start int) *hir.Expr {
	if !l.integerOptions.TrapOnFailure || !l.settledEnabled || n == nil || n.Kind != ast.KindCallExpression {
		return nil
	}
	then := false
	callee := n.Expression()
	if callee != nil && callee.Kind == ast.KindPropertyAccessExpression && callee.Name().Text() == "then" {
		t := l.ck.GetTypeAtLocation(callee.Expression())
		if t != nil {
			t = l.ck.GetNonNullableType(t)
			then = t.Symbol() != nil && t.Symbol().Name == "Promise" && l.librarySymbol(t.Symbol())
		}
	}
	var messages []string
	for _, d := range l.diags[start:] {
		if strings.HasPrefix(d.Category, "note-") {
			continue
		}
		if d.Category != "unsupported-async" && !(then && d.Category == "unsupported-call" && strings.Contains(d.Message, "method then")) {
			return nil
		}
		messages = append(messages, d.String())
	}
	if len(messages) == 0 {
		return nil
	}
	before := len(l.diags)
	typ := l.mapCheckerType(n, l.ck.GetTypeAtLocation(n))
	if hasBlocking(l.diags[before:]) || optionalBase(typ).Kind != hir.Promise {
		return nil
	}
	for i := start; i < before; i++ {
		if !strings.HasPrefix(l.diags[i].Category, "note-") {
			l.diags[i].Category = "note-trap-failure"
		}
	}
	// HIR checks unreachable operands too. Express the dummy result as a
	// checked unwrap of absence; execution always stops at the preceding trap.
	value := &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: typ}
	if typ.Kind != hir.Optional {
		value = &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: typ, X: &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.Optional, typ)}}
	}
	return &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: typ, Stmt: failureBody(l.node(n), strings.Join(messages, "; ")), Y: value}
}
