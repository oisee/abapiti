package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"github.com/oisee/abapiti/internal/tsgo/jsnum"
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
	if x == nil || x.Type.Kind != hir.Optional || dst.Kind == hir.Optional || dst.Kind == hir.Dynamic {
		return x
	}
	base := x.Type.Args[0]
	if base.Kind == hir.Dynamic || (dst.Kind != "" && !base.Equal(dst) && !l.acceptsType(dst, base)) {
		return x
	}
	if l.checkerMayBeUndefined(n) {
		return x
	}
	return &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: base, X: x}
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

// ownSpecialized reports whether a call on a receiver of exactly the class
// that declares the specialized implementation may call it directly: the
// slot's erased parameters would reject arguments the implementation accepts
// (`iface.setDefinition(interfaceDef)` through a union slot typed by the class
// definition). No lowered subclass redefines the member, so the direct
// virtual call dispatches like the erased slot.
func (l *lowerer) ownSpecialized(recv hir.Type, hm *hir.Method, original string) bool {
	if recv.Kind != hir.ClassRef || l.methodsBy[recv.Name+"."+original] != hm {
		return false
	}
	for _, c := range l.out.Classes {
		if c.Name == recv.Name {
			continue
		}
		derived := false
		for base := l.classByName(c.Super); base != nil; base = l.classByName(base.Super) {
			if base.Name == recv.Name {
				derived = true
				break
			}
		}
		if !derived {
			continue
		}
		for _, m := range c.Methods {
			if m.Name == original || m.Name == original+"_instantiated_"+c.Name {
				return false
			}
		}
	}
	return true
}

// ambientLiteral lowers a read of an ambient namespace constant with a
// literal type (`DiagnosticSeverity.Error`, declared `const Error: 1` in a
// .d.ts) to that literal.
func (l *lowerer) ambientLiteral(n *ast.Node) (*hir.Expr, bool) {
	sym := l.resolve(n)
	if sym == nil || sym.ValueDeclaration == nil || sym.ValueDeclaration.Flags&ast.NodeFlagsAmbient == 0 || sym.ValueDeclaration.Kind != ast.KindVariableDeclaration {
		return nil, false
	}
	t := l.ck.GetTypeAtLocation(n)
	if t == nil {
		return nil, false
	}
	switch {
	case t.Flags()&checker.TypeFlagsNumberLiteral != 0:
		var f float64
		switch v := t.AsLiteralType().Value().(type) {
		case float64:
			f = v
		case jsnum.Number:
			f = float64(v)
		default:
			return nil, false
		}
		return hir.L(hir.T(hir.Number), f), true
	case t.Flags()&checker.TypeFlagsStringLiteral != 0:
		s, ok := t.AsLiteralType().Value().(string)
		if !ok {
			return nil, false
		}
		return hir.L(hir.T(hir.String), s), true
	}
	return nil, false
}

// definedGuard recognizes a test proving a local present: `x !== undefined`,
// `x != undefined`, `x !== null`, `x != null` or the truthiness of a local
// whose HIR type is optional.
func (l *lowerer) definedGuard(n *ast.Node) (*ast.Symbol, hir.Type, bool) {
	for n != nil && n.Kind == ast.KindParenthesizedExpression {
		n = n.Expression()
	}
	if n == nil {
		return nil, hir.Type{}, false
	}
	ident := n
	if n.Kind == ast.KindBinaryExpression {
		b := n.AsBinaryExpression()
		if b.OperatorToken.Kind != ast.KindExclamationEqualsEqualsToken && b.OperatorToken.Kind != ast.KindExclamationEqualsToken {
			return nil, hir.Type{}, false
		}
		absent := func(x *ast.Node) bool { return isUndefinedKeyword(x) || x.Kind == ast.KindNullKeyword }
		switch {
		case absent(b.Right):
			ident = b.Left
		case absent(b.Left):
			ident = b.Right
		default:
			return nil, hir.Type{}, false
		}
	}
	for ident != nil && ident.Kind == ast.KindParenthesizedExpression {
		ident = ident.Expression()
	}
	if ident == nil || ident.Kind != ast.KindIdentifier {
		return nil, hir.Type{}, false
	}
	t, ok := l.lookup(ident.Text())
	if !ok || t.Kind != hir.Optional || t.Args[0].Kind == hir.Dynamic {
		return nil, hir.Type{}, false
	}
	sym := l.resolve(ident)
	if sym == nil {
		return nil, hir.Type{}, false
	}
	return sym, t.Args[0], true
}

func (l *lowerer) withGuard(sym *ast.Symbol, typ hir.Type, f func() *hir.Expr) *hir.Expr {
	var x *hir.Expr
	l.withGuardStmt(sym, typ, func() *hir.Stmt { x = f(); return nil })
	return x
}

func (l *lowerer) withGuardStmt(sym *ast.Symbol, typ hir.Type, f func() *hir.Stmt) *hir.Stmt {
	if l.guards == nil {
		l.guards = map[*ast.Symbol]hir.Type{}
	}
	previous, had := l.guards[sym]
	l.guards[sym] = typ
	s := f()
	if had {
		l.guards[sym] = previous
	} else {
		delete(l.guards, sym)
	}
	return s
}

// evolvedArrayType types `const xs = []` by the checker's evolved element
// type at the last reference in the enclosing body instead of the declared
// `any[]`.
func (l *lowerer) evolvedArrayType(d *ast.Node, declared hir.Type) (hir.Type, bool) {
	if declared.Kind != hir.Array || declared.Args[0].Kind != hir.Dynamic || d.Symbol() == nil {
		return hir.Type{}, false
	}
	body := d.Parent
	for body != nil && body.Body() == nil && body.Kind != ast.KindSourceFile {
		body = body.Parent
	}
	if body == nil {
		return hir.Type{}, false
	}
	var found hir.Type
	ok := false
	var walk func(*ast.Node)
	walk = func(c *ast.Node) {
		if c == nil {
			return
		}
		if c.Kind == ast.KindIdentifier && c != d.Name() && l.resolve(c) == d.Symbol() {
			if t := l.ck.GetTypeAtLocation(c); t != nil && t.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) == 0 {
				before := len(l.diags)
				typ := l.mapCheckerType(c, t)
				blocked := hasBlocking(l.diags[before:])
				l.diags = l.diags[:before]
				if !blocked && typ.Kind == hir.Array && typ.Args[0].Kind != hir.Dynamic {
					found, ok = typ, true
				}
			}
		}
		c.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
	}
	walk(body)
	return found, ok
}
