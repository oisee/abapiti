package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Stateless local classes need fresh constructor identity, but no captured
// instance/factory storage. Refuse richer classes until that ABI is available.
func (l *lowerer) localClassDeclaration(n *ast.Node) *hir.Stmt {
	refuse := func(message string) *hir.Stmt { l.diagf(n, "unsupported-local-class", "%s", message); return nil }
	if n.Name() == nil || n.Symbol() == nil || n.ModifierFlags()&ast.ModifierFlagsAbstract != 0 {
		return refuse("local class requires a concrete named declaration")
	}
	if n.ClassLikeData().HeritageClauses != nil {
		for _, clause := range n.ClassLikeData().HeritageClauses.Nodes {
			if clause.AsHeritageClause().Token == ast.KindExtendsKeyword {
				return refuse("local class inheritance requires a captured constructor frame")
			}
		}
	}
	for _, m := range n.Members() {
		if m.Kind != ast.KindMethodDeclaration || m.ModifierFlags()&ast.ModifierFlagsStatic != 0 || m.Name() == nil || m.Name().Kind != ast.KindIdentifier {
			return refuse("local class fields, constructors, static and computed members require a captured factory frame")
		}
		if len(l.freeLocals(m)) != 0 {
			return refuse("local class methods capture enclosing bindings")
		}
		self := false
		var walk func(*ast.Node)
		walk = func(x *ast.Node) {
			if x == nil {
				return
			}
			if x.Kind == ast.KindIdentifier && l.ck.GetSymbolAtLocation(x) == n.Symbol() {
				self = true
			}
			x.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
		}
		walk(m.Body())
		if self {
			return refuse("local class self-reference requires a captured constructor frame")
		}
	}
	savedClass, savedMethod, savedScope, savedHint := l.class, l.method, l.scope, l.hint
	c := l.classes[n.Symbol()]
	if c == nil {
		c = &hir.Class{Node: l.node(n), Name: l.qualifiedName(l.file, "scope"+itoa(n.Pos())+"."+n.Name().Text()), Scoped: true, ViewUnsafe: true}
		l.classes[n.Symbol()] = c
		l.out.Classes = append(l.out.Classes, c)
		l.classSignatures(n, c)
		l.lowerImplicitInitializers(n, c)
		l.class = c
		l.scope = nil
		for _, m := range n.Members() {
			l.lowerMethodBody(m)
		}
	}
	l.class, l.method, l.scope, l.hint = savedClass, savedMethod, savedScope, savedHint
	typ := hir.T(hir.ClassValue)
	name := n.Name().Text()
	l.declare(name, typ)
	declaration := &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: name, Type: typ, X: &hir.Expr{Kind: hir.FreshClassOf, Node: l.node(n), Owner: c.Name, Type: typ}}
	if l.declSymbols == nil {
		l.declSymbols = map[*hir.Stmt]*ast.Symbol{}
	}
	l.declSymbols[declaration] = n.Symbol()
	return declaration
}
