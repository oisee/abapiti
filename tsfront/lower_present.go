package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"github.com/oisee/abapiti/internal/tsgo/jsnum"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
)

// checkerMayBeUndefined reports whether the checker admits undefined at n.
func (l *lowerer) checkerMayBeUndefined(n *ast.Node) bool {
	return typeMayBeUndefined(l.ck.GetTypeAtLocation(n))
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
	owner := l.classByName(recv.Name)
	if recv.Kind != hir.ClassRef || owner == nil {
		return false
	}
	own := false
	for _, m := range owner.Methods {
		own = own || m == hm
	}
	if !own {
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
		sym := l.resolve(c)
		if c.Kind == ast.KindIdentifier && c.Parent != nil && c.Parent.Kind == ast.KindShorthandPropertyAssignment {
			sym = l.ck.GetShorthandAssignmentValueSymbol(c.Parent)
		}
		if c.Kind == ast.KindIdentifier && c != d.Name() && sym == d.Symbol() {
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

// typeMayBeUndefined reports whether the checker type admits undefined.
func typeMayBeUndefined(t *checker.Type) bool {
	if t == nil || t.Flags()&(checker.TypeFlagsUndefined|checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
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

// elementDeclaresAbsence reports whether an index read's element type
// itself admits undefined (`(T | undefined)[]`): a present location type is
// then a guard the checker proved, not an unchecked index assumption.
func (l *lowerer) elementDeclaresAbsence(n *ast.Node) bool {
	e := n.AsElementAccessExpression()
	t := l.ck.GetTypeAtLocation(e.Expression)
	if t == nil || !l.ck.IsArrayType(t) {
		return false
	}
	return typeMayBeUndefined(l.ck.GetElementTypeOfArrayType(t))
}

// paramTypeOverride applies a class override's Types["method.param"] entry
// to a parameter (an `any[]` parameter whose only callers pass node arrays).
func (l *lowerer) paramTypeOverride(node *ast.Node, method, param string) (hir.Type, bool) {
	if node == nil || node.Parent == nil {
		return hir.Type{}, false
	}
	e, ok := l.overrides[node.Parent]
	if !ok || e.Types == nil {
		return hir.Type{}, false
	}
	t, ok := e.Types[method+"."+param]
	if !ok || t.Kind == "" {
		return hir.Type{}, false
	}
	l.diagf(node, "note-override", "%s: %s", e.ID, e.Rationale)
	return t, true
}

// unionShapeFor selects, for an object literal typed by a union view, the
// constituent data class whose fields the literal's properties satisfy.
func (l *lowerer) unionShapeFor(n *ast.Node, hint hir.Type) (hir.Type, bool) {
	var view *unionView
	for _, u := range l.unions {
		if u.iface.Name == hint.Name {
			view = u
			break
		}
	}
	if view == nil {
		return hir.Type{}, false
	}
	names := map[string]bool{}
	for _, p := range n.AsObjectLiteralExpression().Properties.Nodes {
		if p.Kind == ast.KindSpreadAssignment || p.Name() == nil {
			return hir.Type{}, false
		}
		names[p.Name().Text()] = true
	}
	for _, part := range view.parts {
		if part.Kind != hir.ClassRef {
			continue
		}
		c := l.classByName(part.Name)
		if c == nil || c.Ctor == nil {
			continue
		}
		fields := map[string]hir.Type{}
		for _, f := range c.Fields {
			fields[f.Name] = f.Type
		}
		ok := true
		for name := range names {
			if _, has := fields[name]; !has {
				ok = false
			}
		}
		for name, t := range fields {
			if !names[name] && t.Kind != hir.Optional {
				ok = false
			}
		}
		if ok {
			return part, true
		}
	}
	return hir.Type{}, false
}

// shapeAliasOf reports whether declared is a synthesized shape whose field
// names are exactly those of the data class actual (both possibly optional).
func (l *lowerer) shapeAliasOf(declared, actual hir.Type) bool {
	if declared.Kind == hir.Optional {
		declared = declared.Args[0]
	}
	if actual.Kind == hir.Optional {
		actual = actual.Args[0]
	}
	if declared.Kind != hir.ClassRef || actual.Kind != hir.ClassRef || !strings.HasPrefix(declared.Name, "shape.") || strings.HasPrefix(actual.Name, "shape.") {
		return false
	}
	a, b := l.classByName(declared.Name), l.classByName(actual.Name)
	if a == nil || b == nil || b.Ctor == nil || len(a.Fields) != len(b.Fields) {
		return false
	}
	for i := range a.Fields {
		if a.Fields[i].Name != b.Fields[i].Name {
			return false
		}
	}
	return true
}

// propertyAnnotationOverride applies a Patterns.Annotations entry of an
// enclosing override to the declared type node of a property signature (the
// members of a method's anonymous result type).
func (l *lowerer) propertyAnnotationOverride(p *ast.Symbol) (hir.Type, bool) {
	if p == nil || p.ValueDeclaration == nil || p.ValueDeclaration.Type() == nil {
		return hir.Type{}, false
	}
	typeNode := p.ValueDeclaration.Type()
	f := ast.GetSourceFileOfNode(typeNode)
	if f == nil {
		return hir.Type{}, false
	}
	for parent := typeNode; parent != nil; parent = parent.Parent {
		entry, ok := l.overrides[parent]
		if !ok || entry.Patterns == nil {
			continue
		}
		span := f.Text()[scanner.GetTokenPosOfNode(typeNode, f, false):typeNode.End()]
		if typ, ok := entry.Patterns.Annotations[span]; ok {
			l.diagf(typeNode, "note-override", "%s: %s", entry.ID, entry.Rationale)
			return typ, true
		}
	}
	return hir.Type{}, false
}

// narrowedOperand applies the checker's flow type to an operand the general
// narrowing skips (the left side of `??` after a type predicate).
func (l *lowerer) narrowedOperand(n *ast.Node, x *hir.Expr) *hir.Expr {
	if x.Type.Kind != hir.Optional || !x.Type.Args[0].IsRef() {
		return x
	}
	t := l.ck.GetTypeAtLocation(n)
	if t == nil || t.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
		return x
	}
	before := len(l.diags)
	typ := l.mapCheckerType(n, t)
	blocked := hasBlocking(l.diags[before:])
	l.diags = l.diags[:before]
	if blocked || typ.Kind != hir.Optional || !typ.Args[0].IsRef() || typ.Equal(x.Type) || !l.acceptsType(x.Type.Args[0], typ.Args[0]) {
		return x
	}
	return &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: typ, X: x}
}

// primitiveHint reports a string/number/bool hint, optional or not.
func (l *lowerer) primitiveHint(hint hir.Type) bool {
	if hint.Kind == hir.Optional {
		hint = hint.Args[0]
	}
	return hint.Kind == hir.String || hint.Kind == hir.Number || hint.Kind == hir.Bool
}

// taggedOperand coerces a short-circuit operand to the result type; an
// optional tagged operand stays absent when absent and is unboxed otherwise.
func (l *lowerer) taggedOperand(x *hir.Expr, typ hir.Type) *hir.Expr {
	if x.Type.Kind == hir.Optional && x.Type.Args[0].Kind == hir.Dynamic && typ.Kind != hir.Dynamic {
		unboxed := l.coerce(&hir.Expr{Kind: hir.Narrow, Type: hir.T(hir.Dynamic), X: x}, typ)
		if typ.Kind != hir.Optional {
			return unboxed
		}
		return &hir.Expr{Kind: hir.Conditional, Type: typ, X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: x}, Y: &hir.Expr{Kind: hir.Lit, Type: typ}, Z: unboxed}
	}
	return l.coerce(x, typ)
}

// completeInterfaceValueSlots adds `m_value` to an interface whose void slot
// m is implemented with a value by every implementer (TypeScript's `void`
// return admits any value; the ABI keeps the object). Interface calls in a
// value context dispatch through it.
func (l *lowerer) completeInterfaceValueSlots() {
	hasMethod := func(c *hir.Class, name string) bool {
		for ; c != nil; c = l.classByName(c.Super) {
			for _, m := range c.Methods {
				if m.Name == name {
					return true
				}
			}
		}
		return false
	}
	for _, iface := range l.out.Interfaces {
		var added []*hir.Method
		for _, m := range iface.Methods {
			if m.Result.Kind != hir.Void || strings.HasSuffix(m.Name, "_value") || l.interfaceMethod(iface.Name, m.Name+"_value") != nil {
				continue
			}
			implementers := 0
			all := true
			var abstracts []*hir.Class
			for _, c := range l.out.Classes {
				implements := false
				for base := c; base != nil; base = l.classByName(base.Super) {
					implements = implements || containsString(base.Implements, iface.Name)
				}
				if !implements {
					continue
				}
				implementers++
				if hasMethod(c, m.Name+"_value") {
					continue
				}
				if c.Abstract {
					// The abstract base declares the void slot; its concrete
					// subclasses carry the value slots.
					abstracts = append(abstracts, c)
					continue
				}
				all = false
			}
			if implementers == 0 || !all {
				continue
			}
			value := func() *hir.Method {
				return &hir.Method{Node: m.Node, Name: m.Name + "_value", Virtual: true, Result: hir.Ref(hir.RootObject), Params: append([]hir.Param(nil), m.Params...)}
			}
			for _, c := range abstracts {
				abstract := value()
				abstract.Abstract = true
				c.Methods = append(c.Methods, abstract)
			}
			added = append(added, value())
		}
		iface.Methods = append(iface.Methods, added...)
	}
}
