package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// prototypeCall lowers `C.prototype.m.call(recv, ...args)` (frontend-
// provenance.mjs: `core.Registry.prototype.getObjects.call(reg)`): C's own
// m runs with this = recv. That is the virtual call recv.m(...args) exactly
// when no lowered subclass of C overrides m; the receiver is checked to be
// a C (a structural non-C receiver, which JS would accept, raises instead;
// an interface-typed receiver is cast with that check).
func (l *lowerer) prototypeCall(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures || n.Kind != ast.KindCallExpression {
		return nil, false
	}
	callee := n.Expression()
	if callee.Kind == ast.KindIdentifier && l.prototypeAlias(l.resolve(callee)) != nil {
		l.diagf(n, "unsupported-call", "unbound prototype method %s is lowered only as %s.call(receiver, ...); JavaScript binds this to undefined otherwise", callee.Text(), callee.Text())
		return nil, true
	}
	if callee.Kind != ast.KindPropertyAccessExpression || callee.Name().Text() != "call" || n.AsCallExpression().QuestionDotToken != nil {
		return nil, false
	}
	member := callee.Expression()
	if member.Kind == ast.KindIdentifier {
		// A module const holding C.prototype.m is that method value.
		if init := l.prototypeAlias(l.resolve(member)); init != nil {
			member = init
		}
	}
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
	if recv.Type.Kind == hir.InterfaceRef {
		// A receiver typed by an interface the class implements: checked
		// downcast (castRef raises for another implementation).
		recv = &hir.Expr{Kind: hir.Cast, Node: l.node(args[0]), Type: hir.Ref(c.Name), X: recv}
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

// prototypeAlias returns the initializer C.prototype.m of a top-level const
// binding (frontend-inventory.mjs: const enumerate =
// core.Registry.prototype.getObjects) when C is a lowered class and m one of
// its instance methods. The binding holds the method captured at module
// initialization; it is lowered only as alias.call(receiver, ...) through
// prototypeCall and has no storage of its own.
func (l *lowerer) prototypeAlias(sym *ast.Symbol) *ast.Node {
	if !l.unprunedFeatures || sym == nil || sym.ValueDeclaration == nil {
		return nil
	}
	d := sym.ValueDeclaration
	if d.Kind != ast.KindVariableDeclaration || d.Name() == nil || d.Name().Kind != ast.KindIdentifier || d.Parent == nil || d.Parent.Kind != ast.KindVariableDeclarationList || d.Parent.Flags&ast.NodeFlagsConst == 0 {
		return nil
	}
	if stmt := d.Parent.Parent; stmt == nil || stmt.Kind != ast.KindVariableStatement || stmt.Parent == nil || stmt.Parent.Kind != ast.KindSourceFile {
		return nil
	}
	init := d.Initializer()
	for init != nil && init.Kind == ast.KindParenthesizedExpression {
		init = init.Expression()
	}
	if init == nil || init.Kind != ast.KindPropertyAccessExpression || init.AsPropertyAccessExpression().QuestionDotToken != nil {
		return nil
	}
	proto := init.Expression()
	if proto.Kind != ast.KindPropertyAccessExpression || proto.Name().Text() != "prototype" || proto.AsPropertyAccessExpression().QuestionDotToken != nil {
		return nil
	}
	if c := l.classOf(l.resolve(proto.Expression())); c == nil {
		return nil
	}
	m := l.resolve(init)
	if m == nil || m.ValueDeclaration == nil || m.ValueDeclaration.Kind != ast.KindMethodDeclaration || ast.IsStatic(m.ValueDeclaration) {
		return nil
	}
	return init
}
