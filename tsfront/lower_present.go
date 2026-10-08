package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// checkerMayBeUndefined reports whether the checker admits undefined at n.
func (l *lowerer) checkerMayBeUndefined(n *ast.Node) bool {
	t := l.ck.GetTypeAtLocation(n)
	if t == nil {
		return true
	}
	if t.Flags()&(checker.TypeFlagsUndefined|checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
		return true
	}
	if t.IsUnion() {
		for _, c := range t.AsUnionOrIntersectionType().Types() {
			if c.Flags()&(checker.TypeFlagsUndefined|checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
				return true
			}
		}
	}
	return false
}

// presentValue narrows an optional reference the checker typed as present
// (an index-signature read, for example) to the declared value type. The
// checked narrowing raises when the value is absent at run time instead of
// storing an absent reference where TypeScript promised a value.
func (l *lowerer) presentValue(n *ast.Node, x *hir.Expr, dst hir.Type) *hir.Expr {
	if x == nil || x.Type.Kind != hir.Optional || dst.Kind == hir.Optional || !dst.IsRef() || !x.Type.Args[0].Equal(dst) {
		return x
	}
	if l.checkerMayBeUndefined(n) {
		return x
	}
	return &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: dst, X: x}
}

// ensureNamespaceField declares the export map of a namespace module that is
// lowered outside the selected file list (a dependency imported as a value).
func (l *lowerer) ensureNamespaceField(mod *hir.Class, typ hir.Type) {
	for _, f := range mod.Fields {
		if f.Name == "ns" && f.Static {
			return
		}
	}
	mod.Fields = append(mod.Fields, hir.Field{Name: "ns", Static: true, Type: typ})
}

// namespaceMemberDeclarations resolves `Ns.member` (a property access whose
// receiver is a module namespace) to the member's declarations. The member
// identifier is the property name, which the identifier walk skips, yet the
// reference is a value edge: `node.findDirectExpression(Expressions.For)`
// instantiates the class For.
func (l *lowerer) namespaceMemberDeclarations(c *ast.Node) []*ast.Node {
	p := c.AsPropertyAccessExpression()
	if p.Expression == nil || p.Expression.Kind != ast.KindIdentifier {
		return nil
	}
	ns := l.resolve(p.Expression)
	if ns == nil || ns.Flags&ast.SymbolFlagsModule == 0 {
		return nil
	}
	sym := l.resolve(c)
	if sym == nil {
		return nil
	}
	if sym.Flags&ast.SymbolFlagsAlias != 0 {
		if target, ok := l.ck.ResolveAlias(sym); ok {
			sym = target
		}
	}
	return sym.Declarations
}
