package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Bounded callable adapters: abaplint uses Class.staticMethod as an array
// callback. Also support obj.method.bind(obj), with one simple receiver and
// no partial arguments. An unbound instance method is never rebound to obj:
// it has undefined this in JS and remains a typed refusal in our receiver ABI.
func (l *lowerer) methodCallbackBody(at, cb *ast.Node, elem hir.Type) (*hir.Expr, []string, bool) {
	member := cb
	var bound *ast.Node
	if cb.Kind == ast.KindCallExpression {
		callee := cb.Expression()
		if callee == nil || callee.Kind != ast.KindPropertyAccessExpression || callee.Name().Text() != "bind" || len(cb.Arguments()) != 1 {
			l.diagf(cb, "unsupported-callback", "method callback supports only bind(receiver) without partial arguments")
			return nil, nil, false
		}
		member, bound = callee.Expression(), cb.Arguments()[0]
	}
	if member == nil || member.Kind != ast.KindPropertyAccessExpression {
		l.diagf(cb, "unsupported-callback", "callback is not a declared method reference")
		return nil, nil, false
	}
	sym := l.resolve(member)
	method := l.methodOf(sym)
	if sym == nil || method == nil || method.Name == "class_constructor" {
		l.diagf(cb, "unsupported-callback", "callback property is not a lowered declared method")
		return nil, nil, false
	}
	owner := l.classOf(sym.Parent)
	if owner == nil {
		l.diagf(cb, "unsupported-callback", "callback method owner is not lowered")
		return nil, nil, false
	}
	var receiver *hir.Expr
	if method.Static {
		if bound != nil || member.Expression().Kind != ast.KindIdentifier || l.classOf(l.resolve(member.Expression())) == nil || sym.ValueDeclaration == nil || callbackUsesThis(sym.ValueDeclaration.Body()) {
			l.diagf(cb, "unsupported-callback", "unbound static callback requires a class name and a receiver-free implementation")
			return nil, nil, false
		}
	} else {
		if bound == nil {
			l.diagf(cb, "unsupported-callback", "unbound instance method callback has undefined this; receiver ABI is not lowered")
			return nil, nil, false
		}
		base := member.Expression()
		same := base.Kind == ast.KindThisKeyword && bound.Kind == ast.KindThisKeyword
		same = same || (base.Kind == ast.KindIdentifier && bound.Kind == ast.KindIdentifier && l.resolve(base) != nil && l.resolve(base) == l.resolve(bound))
		if !same {
			l.diagf(cb, "unsupported-callback", "bound method callback requires the same simple receiver")
			return nil, nil, false
		}
		receiver = l.expr(base)
		if receiver == nil {
			return nil, nil, false
		}
		if receiver.Type.Kind != hir.ClassRef {
			l.diagf(cb, "unsupported-callback", "bound callback receiver must be a present class reference")
			return nil, nil, false
		}
		receiver = l.tempInit(cb, receiver.Type, receiver)
		// Method lookup happens while constructing the callback, even when
		// the array is empty. Refuse absence here, not only on invocation.
		l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(cb), X: &hir.Expr{Kind: hir.IsUndefined, Node: l.node(cb), Type: hir.T(hir.Bool), X: receiver}, Body: failureBody(l.node(cb), "method reference receiver is undefined")})
	}
	saved := l.pend
	l.pend = nil
	l.push()
	call, names, ok := l.callbackInvocation(at, cb, elem, funcRef{owner: owner.Name, method: method.Name}, nil, receiver)
	l.pop()
	l.pend = saved
	return call, names, ok
}
