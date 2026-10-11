package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// inlineReturnBody gives a non-escaping callback its own return boundary.
// Guarding subsequent statements preserves early returns without returning
// from the containing method (or breaking the containing array loop).
// Returns inside loops/try/switch need a richer completion ABI and are refused.
func (l *lowerer) inlineReturnBody(fn *ast.Node) *hir.Expr {
	sigs := l.ck.GetSignaturesOfType(l.ck.GetTypeAtLocation(fn), checker.SignatureKindCall)
	if len(sigs) != 1 {
		l.diagf(fn, "unsupported-callback", "inline function needs one checked call signature")
		return nil
	}
	typ := l.mapCheckerType(fn, l.ck.GetReturnTypeOfSignature(sigs[0]))
	if typ.Kind == hir.Void {
		l.diagf(fn, "unsupported-callback", "inline return boundary needs a value result")
		return nil
	}
	var zero any
	switch typ.Kind {
	case hir.Number, hir.I32, hir.I64:
		zero = 0
	case hir.String:
		zero = ""
	case hir.Bool:
		zero = false
	}
	out := l.tempInit(fn, typ, hir.L(typ, zero))
	done := l.tempInit(fn, hir.T(hir.Bool), hir.L(hir.T(hir.Bool), false))
	active := func() *hir.Expr { return &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: done} }
	var lower func(*ast.Node) *hir.Stmt
	lower = func(n *ast.Node) *hir.Stmt {
		switch n.Kind {
		case ast.KindBlock:
			l.push()
			defer l.pop()
			var sequence func([]*ast.Node) *hir.Stmt
			sequence = func(stmts []*ast.Node) *hir.Stmt {
				if len(stmts) == 0 {
					return hir.B()
				}
				saved := l.pend
				l.pend = nil
				x := lower(stmts[0])
				pre := l.pend
				l.pend = saved
				if x != nil {
					if stmts[0].Kind == ast.KindVariableStatement && x.Kind == hir.Block {
						pre = append(pre, x.List...)
					} else {
						pre = append(pre, x)
					}
				}
				// Nest the remainder, so declarations remain visible without
				// running their initializers after an early return.
				if len(stmts) > 1 {
					pre = append(pre, &hir.Stmt{Kind: hir.If, X: active(), Body: sequence(stmts[1:])})
				}
				return hir.B(pre...)
			}
			return sequence(n.AsBlock().Statements.Nodes)
		case ast.KindReturnStatement:
			value := n.AsReturnStatement().Expression
			if value == nil {
				l.diagf(n, "unsupported-callback", "inline value callback cannot return without a value")
				return nil
			}
			keep := l.hint
			l.hint = typ
			x := l.expr(value)
			l.hint = keep
			if x == nil {
				return nil
			}
			return hir.B(&hir.Stmt{Kind: hir.Assign, X: out, Y: l.coerce(x, typ)}, &hir.Stmt{Kind: hir.Assign, X: done, Y: hir.L(hir.T(hir.Bool), true)})
		case ast.KindIfStatement:
			s := n.AsIfStatement()
			cond := l.expr(s.Expression)
			if cond == nil {
				return nil
			}
			branch := func(node *ast.Node) *hir.Stmt {
				saved := l.pend
				l.pend = nil
				body := lower(node)
				pre := l.pend
				l.pend = saved
				if body != nil {
					pre = append(pre, body)
				}
				return hir.B(pre...)
			}
			yes := branch(s.ThenStatement)
			var no *hir.Stmt
			if s.ElseStatement != nil {
				no = branch(s.ElseStatement)
			}
			return &hir.Stmt{Kind: hir.If, X: l.toBool(n, cond), Body: yes, Else: no}
		default:
			if l.containsReturn(n) {
				l.diagf(n, "unsupported-callback", "inline return inside %s needs a completion ABI (only blocks and if are supported)", n.Kind.String())
				return nil
			}
			return hir.B(l.stmts(n)...)
		}
	}
	body := lower(fn.Body())
	l.pendStmt(body)
	return out
}

// iifeCall evaluates a parameterless lexical arrow immediately. No closure
// escapes, and reads/writes retain the enclosing lexical bindings.
func (l *lowerer) iifeCall(n, callee *ast.Node) (*hir.Expr, bool) {
	for callee.Kind == ast.KindParenthesizedExpression {
		callee = callee.Expression()
	}
	if callee.Kind != ast.KindArrowFunction {
		return nil, false
	}
	if callee.ModifierFlags()&ast.ModifierFlagsAsync != 0 {
		l.diagf(n, "unsupported-async", "immediate async arrow needs a Promise/suspension ABI")
		return nil, true
	}
	if len(callee.Parameters()) != 0 || len(n.Arguments()) != 0 {
		l.diagf(n, "unsupported-call", "immediate arrow invocation currently requires zero parameters and arguments")
		return nil, true
	}
	l.push()
	defer l.pop()
	if callee.Body().Kind != ast.KindBlock {
		return l.expr(callee.Body()), true
	}
	return l.inlineReturnBody(callee), true
}
