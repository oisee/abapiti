package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Ordinary functions have a dynamic this binding. The lifted receiver ABI
// cannot supply strict-mode undefined; arrows retain their lexical binding.
func (l *lowerer) refuseUndefinedThis(fn *ast.Node) bool {
	if fn != nil && fn.Kind == ast.KindVariableDeclaration {
		fn = fn.Initializer()
	}
	if fn != nil && (fn.Kind == ast.KindFunctionExpression || fn.Kind == ast.KindFunctionDeclaration) && callbackUsesThis(fn.Body()) {
		l.diagf(fn, "unsupported-callback", "ordinary function has undefined this when called unbound; receiver ABI is not lowered")
		return true
	}
	return false
}

// Function-valued locals are captured once before iteration. The callback ABI
// supplies the operation's arguments (two elements for sort, otherwise value,
// index and array); excess required declarations fail explicitly.
func (l *lowerer) localValueCallback(n, cb *ast.Node, elem hir.Type) (*hir.Expr, []string, bool) {
	if callee := n.Expression(); callee != nil && callee.Kind == ast.KindPropertyAccessExpression && callee.Name().Text() == "reduce" {
		return nil, nil, false
	}
	typ, ok := l.lookup(cb.Text())
	// An optional function value (a defaulted or optional parameter) is
	// called after a narrowing read: an absent callback fails, as in JS.
	optional := ok && typ.Kind == hir.Optional && l.callableMethod(typ.Args[0]) != nil
	if optional {
		typ = typ.Args[0]
	}
	if !ok || l.callableMethod(typ) == nil {
		lf := l.localFns[cb.Text()]
		if pending := l.pendingFns[cb.Text()]; lf == nil && pending != nil {
			delete(l.pendingFns, cb.Text())
			lf = l.liftLocalFn(cb.Text(), pending)
		}
		if lf == nil {
			return nil, nil, false
		}
		if !l.namedCallbackArity(cb, lf.method.Params[len(lf.captures):]) {
			return nil, nil, true
		}
		saved := l.pend
		l.pend = nil
		x, names, valid := l.callbackInvocation(n, cb, elem, funcRef{owner: lf.owner.Name, method: lf.method.Name}, lf, nil)
		l.pend = saved
		if !valid {
			return nil, nil, true
		}
		return x, names, true
	}
	m := l.callableMethod(typ)
	if !l.namedCallbackArity(cb, m.Params) {
		return nil, nil, true
	}
	recv := l.expr(cb)
	if recv == nil {
		return nil, nil, true
	}
	if optional {
		recv = &hir.Expr{Kind: hir.Narrow, Node: l.node(cb), Type: typ, X: recv}
	}
	recv = l.tempInit(cb, typ, recv)
	types := []hir.Type{elem, hir.T(hir.Number), hir.T(hir.Array, elem)}
	minimum := 1
	if callee := n.Expression(); callee != nil && callee.Kind == ast.KindPropertyAccessExpression && callee.Name().Text() == "sort" {
		types, minimum = []hir.Type{elem, elem}, 2
		for i, p := range m.Params {
			if i >= 2 && p.Type.Kind != hir.Optional {
				l.diagf(cb, "unsupported-callback", "sort comparator requires more than two arguments")
				return nil, nil, true
			}
		}
	}
	count := len(m.Params)
	if count > len(types) {
		count = len(types)
	}
	if count < minimum {
		count = minimum
	}
	names := make([]string, count)
	var args []*hir.Expr
	for i := range names {
		l.serial++
		names[i] = "callbackArg" + itoa(l.serial)
		if i < len(m.Params) {
			args = append(args, l.coerce(hir.V(names[i], types[i]), m.Params[i].Type))
		}
	}
	for i := count; i < len(m.Params); i++ {
		args = append(args, &hir.Expr{Kind: hir.Lit, Type: m.Params[i].Type})
	}
	return &hir.Expr{Kind: hir.VirtualCall, Node: l.node(cb), X: recv, Name: "call", Type: m.Result, Args: args}, names, true
}

func (l *lowerer) namedCallbackArity(cb *ast.Node, params []hir.Param) bool {
	for i, p := range params {
		if i >= 3 && p.Type.Kind != hir.Optional {
			l.diagf(cb, "unsupported-callback", "named callback requires more than three arguments")
			return false
		}
	}
	for _, p := range params {
		if p.Variadic {
			l.diagf(cb, "unsupported-callback", "named callback rest parameters are not lowered")
			return false
		}
	}
	return true
}
