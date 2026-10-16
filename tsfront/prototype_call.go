package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// prototypeCall lowers `C.prototype.m.call(recv, ...args)` (frontend-
// provenance.mjs: `core.Registry.prototype.getObjects.call(reg)`): C's own
// m runs with this = recv. That is the virtual call recv.m(...args) exactly
// when no lowered subclass of C overrides m; the receiver is checked to be
// a C (a structural non-C receiver, which JS would accept, raises instead).
func (l *lowerer) prototypeCall(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures || n.Kind != ast.KindCallExpression {
		return nil, false
	}
	callee := n.Expression()
	if callee.Kind != ast.KindPropertyAccessExpression || callee.Name().Text() != "call" || n.AsCallExpression().QuestionDotToken != nil {
		return nil, false
	}
	member := callee.Expression()
	if member.Kind != ast.KindPropertyAccessExpression || member.AsPropertyAccessExpression().QuestionDotToken != nil {
		return nil, false
	}
	proto := member.Expression()
	if proto.Kind != ast.KindPropertyAccessExpression || proto.Name().Text() != "prototype" {
		return nil, false
	}
	c := l.classOf(l.resolve(proto.Expression()))
	m := l.methodOf(l.resolve(member))
	if c == nil || m == nil || m.Static {
		l.diagf(n, "unsupported-call", "prototype method call needs a lowered class and instance method")
		return nil, true
	}
	name := member.Name().Text()
	for _, sub := range l.out.Classes {
		if sub == c || !l.classExtends(sub, c.Name) {
			continue
		}
		for _, own := range sub.Methods {
			if own.Name == m.Name && !own.Static {
				l.diagf(n, "unsupported-call", "%s.prototype.%s.call: subclass %s overrides it, so a virtual call is not C's own method", c.Name, name, sub.Name)
				return nil, true
			}
		}
	}
	args := n.Arguments()
	if len(args) == 0 || args[0].Kind == ast.KindSpreadElement {
		l.diagf(n, "unsupported-call", "%s.prototype.%s.call needs an explicit receiver", c.Name, name)
		return nil, true
	}
	recv := l.expr(args[0])
	if recv == nil {
		return nil, true
	}
	recv = l.coerce(recv, hir.Ref(c.Name))
	if !l.acceptsType(hir.Ref(c.Name), recv.Type) {
		l.diagf(n, "unsupported-call", "%s.prototype.%s.call: receiver %s is not a %s", c.Name, name, recv.Type, c.Name)
		return nil, true
	}
	recv = l.tempInit(n, recv.Type, recv)
	rest, ok := l.callArgsMethod(n, args[1:], m, 0)
	if !ok {
		return nil, true
	}
	return &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), X: recv, Name: m.Name, Type: m.Result, Args: rest}, true
}

// classExtends reports whether sub has ancestor name.
func (l *lowerer) classExtends(sub *hir.Class, name string) bool {
	for seen, cur := 0, sub.Super; cur != "" && seen < 64; seen++ {
		if cur == name {
			return true
		}
		next := ""
		for _, c := range l.out.Classes {
			if c.Name == cur {
				next = c.Super
				break
			}
		}
		cur = next
	}
	return false
}
