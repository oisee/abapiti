package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Phase-2 expression lowering: element access, enums, class values,
// namespaces, dynamic (typeof) dispatch, optional chaining, regexes, the
// array higher-order calls inlined into loops (ADR-0006), spread arguments
// and the Object.* helpers.

// pendStmt appends a prelude statement.
func (l *lowerer) pendStmt(s *hir.Stmt) { l.pend = append(l.pend, s) }

// tempVar declares a fresh local of type t and returns it.
func (l *lowerer) tempVar(at *ast.Node, t hir.Type) *hir.Expr {
	l.serial++
	name := "t" + itoa(l.serial)
	l.declare(name, t)
	l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(at), Name: name, Type: t,
		X: &hir.Expr{Kind: hir.New, Node: l.node(at), Type: t}})
	return hir.V(name, t)
}

// elementAccess lowers m[k] and m["k"]: records and namespace maps are
// OrderedMaps (map.get), arrays index.
func (l *lowerer) elementAccess(n *ast.Node) *hir.Expr {
	e := n.AsElementAccessExpression()
	recv := l.expr(e.Expression)
	if recv == nil {
		return nil
	}
	arg := e.ArgumentExpression
	if arg == nil {
		l.diagf(n, "unsupported-expr", "element access without an argument")
		return nil
	}
	switch recv.Type.Kind {
	case hir.OrderedMap:
		k := l.expr(arg)
		if k == nil {
			return nil
		}
		return l.rtOp("map.get", recv, hir.T(hir.Optional, recv.Type.Args[1]), k)
	case hir.Array:
		k := l.expr(arg)
		if k == nil {
			return nil
		}
		return &hir.Expr{Kind: hir.IndexGet, Node: l.node(n), Type: recv.Type.Args[0], X: recv, Y: l.indexValue(k)}
	case hir.Dynamic:
		l.diagf(n, "unsupported-expr", "element access on a dynamic value is not lowered")
		return nil
	}
	l.diagf(n, "unsupported-expr", "element access on %s is not lowered", recv.Type.Kind)
	return nil
}

// enumMember lowers E.m to the member's string value.
func (l *lowerer) enumMember(n *ast.Node, p *ast.PropertyAccessExpression) (*hir.Expr, bool) {
	sym := l.resolve(n.Expression())
	if sym == nil {
		return nil, false
	}
	members := l.enumOf(sym)
	if members == nil {
		return nil, false
	}
	if p.Name() == nil {
		return nil, false
	}
	v, ok := members[p.Name().Text()]
	if !ok {
		return nil, false
	}
	l.diagf(n, "note-enum-string", "enum member %s.%s lowered as its string value %q", sym.Name, p.Name().Text(), v)
	return &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.String), Value: v}, true
}

// classValueRef lowers a reference to a class used as a value (identifier or
// Ns.member) to the class descriptor.
func (l *lowerer) classValueRef(n *ast.Node) (*hir.Expr, bool) {
	sym := l.resolve(n)
	if sym == nil {
		return nil, false
	}
	c := l.classOf(sym)
	if c == nil {
		return nil, false
	}
	l.diagf(n, "note-class-value", "class %s used as a value", c.Name)
	return &hir.Expr{Kind: hir.ClassOf, Node: l.node(n), Owner: c.Name, Type: hir.T(hir.ClassValue)}, true
}

// namespaceRef lowers a namespace alias used as a value to the module's
// export map.
func (l *lowerer) namespaceRef(n *ast.Node) (*hir.Expr, bool) {
	sym := l.resolve(n)
	if sym == nil || sym.Flags&ast.SymbolFlagsValueModule == 0 {
		return nil, false
	}
	mod := l.fileOfSymbol(sym)
	if mod == nil {
		return nil, false
	}
	var m *hir.Class
	for f, candidate := range l.modules {
		if f.FileName() == mod.FileName() {
			m = candidate
			break
		}
	}
	if m == nil {
		return nil, false
	}
	for _, f := range m.Fields {
		if f.Name == "ns" && f.Static {
			return &hir.Expr{Kind: hir.StaticGet, Node: l.node(n), Owner: m.Name, Name: "ns", Type: f.Type}, true
		}
	}
	return nil, false
}

// typeofExpr lowers `typeof x` compared with a string, the only use in the
// closure: dispatch on the dynamic box, or a statically true test on class
// values.
func (l *lowerer) typeofExpr(n *ast.Node) (*hir.Expr, bool) {
	u := n.AsTypeOfExpression()
	x := l.expr(u.Expression)
	if x == nil {
		return nil, false
	}
	// The comparison is built by the caller through binary(); here we only
	// answer the three-way test as a small tagged value.
	switch x.Type.Kind {
	case hir.String:
		return &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.String), Value: "string"}, true
	case hir.ClassValue:
		return &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.String), Value: "function"}, true
	case hir.Dynamic:
		// typeof produces a string even when stored before comparison.
		return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: hir.T(hir.String), X: l.rtOp("dynamic.isString", x, hir.T(hir.Bool)), Y: hir.L(hir.T(hir.String), "string"), Z: &hir.Expr{Kind: hir.Conditional, Type: hir.T(hir.String), X: l.rtOp("dynamic.isFunction", x, hir.T(hir.Bool)), Y: hir.L(hir.T(hir.String), "function"), Z: hir.L(hir.T(hir.String), "object")}}, true
	}
	l.diagf(n, "unsupported-expr", "typeof on %s is not lowered", x.Type.Kind)
	return nil, false
}

// typeofCompare lowers `typeof x === "string"|"function"` (or !==).
func (l *lowerer) typeofCompare(n *ast.Node, x *ast.Node, want string, negated bool) *hir.Expr {
	e := l.expr(x)
	if e == nil {
		return nil
	}
	var test *hir.Expr
	switch e.Type.Kind {
	case hir.Dynamic:
		op := "dynamic.isString"
		if want == "function" {
			op = "dynamic.isFunction"
		}
		test = l.rtOp(op, e, hir.T(hir.Bool))
	case hir.Optional:
		if e.Type.Args[0].Kind != hir.ClassValue || want != "function" {
			l.diagf(n, "unsupported-expr", "typeof comparison on %s is not lowered", e.Type)
			return nil
		}
		test = &hir.Expr{Kind: hir.Unary, Node: l.node(n), Type: hir.T(hir.Bool), Op: "!", X: &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: e}}
	case hir.String:
		test = hir.L(hir.T(hir.Bool), want == "string")
	case hir.ClassValue:
		l.diagf(n, "note-typeof-classvalue", "typeof on a class value is always \"function\"")
		test = hir.L(hir.T(hir.Bool), want == "function")
	default:
		l.diagf(n, "unsupported-expr", "typeof comparison on %s is not lowered", e.Type.Kind)
		return nil
	}
	if negated {
		return &hir.Expr{Kind: hir.Unary, Node: l.node(n), Op: "!", Type: hir.T(hir.Bool), X: test}
	}
	return test
}

// regexLiteral lowers /pattern/flags to a runtime RegExp construction.
func (l *lowerer) regexLiteral(n *ast.Node) *hir.Expr {
	text := n.Text() // /pattern/flags
	slash := strings.LastIndex(text[1:], "/")
	if slash < 0 {
		l.diagf(n, "unsupported-regex", "malformed regular expression %s", text)
		return nil
	}
	pattern := text[1 : 1+slash]
	flags := text[2+slash:]
	l.diagf(n, "note-regex", "regex /%s/%s lowered to the RegExp runtime (FIND REGEX, POSIX classes)", pattern, flags)
	return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.T(hir.RegExp),
		Args: []*hir.Expr{
			{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.String), Value: pattern},
			{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.String), Value: flags},
		}}
}

// newRegExp lowers new RegExp(pattern[, flags]).
func (l *lowerer) newRegExp(n *ast.Node) (*hir.Expr, bool) {
	sym := l.resolve(n.Expression())
	if sym == nil || sym.Name != "RegExp" || l.classOf(sym) != nil {
		return nil, false
	}
	args := n.Arguments()
	if len(args) == 0 || len(args) > 2 {
		return nil, false
	}
	p := l.expr(args[0])
	if p == nil {
		return nil, false
	}
	argv := []*hir.Expr{p}
	if len(args) == 2 {
		f := l.expr(args[1])
		if f == nil {
			return nil, false
		}
		argv = append(argv, f)
	} else {
		argv = append(argv, &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.String), Value: ""})
	}
	l.diagf(n, "note-regex", "new RegExp(...) lowered to the RegExp runtime (dynamic pattern)")
	return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.T(hir.RegExp), Args: argv}, true
}

// objectStatic lowers the Object.* helpers the closure uses.
func (l *lowerer) objectStatic(n *ast.Node, name string) (*hir.Expr, bool) {
	switch name {
	case "freeze":
		// Object.freeze(x) is the identity here: nothing mutates through the
		// frozen view in the closure.
		l.diagf(n, "note-object-freeze", "Object.freeze lowered as the identity")
		args := n.Arguments()
		if len(args) != 1 {
			return nil, false
		}
		// Keep an outer contextual hint (the variable's annotation); only
		// derive one from the argument when the context has none.
		hint := l.hint
		if hint.Kind == hir.Void {
			if t := l.ck.GetTypeAtLocation(args[0]); t != nil {
				before := len(l.diags)
				mapped := l.mapCheckerType(args[0], t)
				if len(l.diags) == before && mapped.Kind != hir.Void {
					hint = mapped
				} else {
					l.diags = l.diags[:before]
				}
			}
		}
		l.hint = hint
		x := l.expr(args[0])
		l.hint = hir.Type{}
		if x == nil {
			return nil, false
		}
		return x, true
	case "keys":
		args := n.Arguments()
		if len(args) != 1 {
			return nil, false
		}
		x := l.expr(args[0])
		if x == nil {
			return nil, false
		}
		switch x.Type.Kind {
		case hir.OrderedMap:
			return l.rtOp("map.keys", x, hir.T(hir.Array, x.Type.Args[0])), true
		}
	case "values":
		args := n.Arguments()
		if len(args) != 1 {
			return nil, false
		}
		// Object.values(enum): the enum's value array.
		if args[0].Kind == ast.KindIdentifier {
			if sym := l.resolve(args[0]); sym != nil {
				if owner, field, ok := l.enumValuesField(sym); ok {
					return &hir.Expr{Kind: hir.StaticGet, Node: l.node(n), Owner: owner, Name: field,
						Type: hir.T(hir.Array, hir.T(hir.String))}, true
				}
			}
		}
		x := l.expr(args[0])
		if x != nil && x.Type.Kind == hir.OrderedMap {
			typ := hir.T(hir.Array, x.Type.Args[1])
			// Snapshot keys once and read each value in insertion order.
			recv := l.tempInit(n, x.Type, x)
			result := l.tempVar(n, typ)
			l.serial++
			key := hir.V("k"+itoa(l.serial), x.Type.Args[0])
			value := l.rtOp("map.get", recv, hir.T(hir.Optional, x.Type.Args[1]), key)
			value = &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: x.Type.Args[1], X: value}
			loop := &hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: key.Name, Type: key.Type, X: l.rtOp("map.keys", recv, hir.T(hir.Array, key.Type)), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", result, hir.T(hir.I32), value)})}
			l.pendStmt(loop)
			return result, true
		}
	case "fromEntries":
		args := n.Arguments()
		if len(args) != 1 {
			return nil, false
		}
		hint := l.hint
		if hint.Kind == hir.OrderedMap {
			l.hint = hir.T(hir.Array, l.tupleShape(n, []hir.Type{hint.Args[0], hint.Args[1]}))
		}
		x := l.expr(args[0])
		l.hint = hint
		if x == nil {
			return nil, false
		}
		if x.Type.Kind != hir.Array {
			return nil, false
		}
		// Pairs of the shape [key, value]: read f0/f1 per row.
		row := x.Type.Args[0]
		if row.Kind != hir.ClassRef {
			return nil, false
		}
		keyT := hir.T(hir.String)
		valT := hir.T(hir.Dynamic)
		for _, c := range l.out.Classes {
			if c.Name != row.Name {
				continue
			}
			for _, f := range c.Fields {
				if f.Name == "f1" {
					valT = f.Type
				}
			}
		}
		typ := hir.T(hir.OrderedMap, keyT, valT)
		m := l.tempVar(n, typ)
		v := hir.V("v", row)
		l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: "v", Type: row, X: x,
			Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n),
				X: l.rtOp("map.set", m, typ,
					&hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: "f0", Type: keyT, X: v},
					&hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: "f1", Type: valT, X: v})})})
		return m, true
	}
	return nil, false
}

// viewRef returns (creating or growing) the cast-view interface for a
// vendored class that is not lowered, adding the member currently used. The
// cast itself is an unchecked Cast (TypeScript `as` semantics).
func (l *lowerer) viewRef(n *ast.Node, classSym *ast.Symbol, member string) (hir.Type, bool) {
	key := "view." + classSym.Name
	i := l.views[key]
	if i == nil {
		i = &hir.Interface{Node: l.node(n), Name: key}
		l.views[key] = i
		l.out.Interfaces = append(l.out.Interfaces, i)
		l.diagf(n, "note-cast-view", "cast view interface %s for the non-lowered class %s", key, classSym.Name)
	}
	for _, m := range i.Methods {
		if m.Name == member {
			return hir.Type{Kind: hir.InterfaceRef, Name: key}, true
		}
	}
	// Read the member's signature from the class declaration (the instance
	// type carries the members; the symbol type is the constructor side).
	classType := l.ck.GetDeclaredTypeOfSymbol(classSym)
	if classType == nil {
		classType = l.ck.GetTypeOfSymbol(classSym)
	}
	if classType == nil {
		return hir.Type{}, false
	}
	prop := l.ck.GetPropertyOfType(classType, member)
	if prop == nil {
		l.diagf(n, "unsupported-cast-view", "class %s has no member %s", classSym.Name, member)
		return hir.Type{}, false
	}
	propType := l.ck.GetTypeOfSymbol(prop)
	if propType == nil || len(l.ck.GetSignaturesOfType(propType, checker.SignatureKindCall)) == 0 {
		l.diagf(n, "skipped-cast-view", "view member %s.%s is not a method and is skipped", classSym.Name, member)
		return hir.Type{Kind: hir.InterfaceRef, Name: key}, true
	}
	sig := l.ck.GetSignaturesOfType(propType, checker.SignatureKindCall)[0]
	hm := &hir.Method{Node: l.node(n), Name: member, Virtual: true, Result: hir.T(hir.Void)}
	for _, group := range l.ck.GetExpandedParameters(sig, false) {
		if len(group) != 1 {
			l.diagf(n, "skipped-cast-view", "view member %s.%s: union parameter is skipped", classSym.Name, member)
			return hir.Type{}, false
		}
		ps := group[0]
		before := len(l.diags)
		pt := l.mapCheckerType(n, l.ck.GetTypeOfSymbol(ps))
		if hasBlocking(l.diags[before:]) || pt.Kind == hir.Void {
			l.diags = l.diags[:before]
			l.diagf(n, "skipped-cast-view", "view member %s.%s: parameter %s is skipped", classSym.Name, member, ps.Name)
			return hir.Type{}, false
		}
		hm.Params = append(hm.Params, hir.Param{Name: ps.Name, Type: pt})
	}
	before := len(l.diags)
	if rt := l.ck.GetReturnTypeOfSignature(sig); rt != nil {
		hm.Result = l.mapCheckerType(n, rt)
	}
	if hasBlocking(l.diags[before:]) || hm.Result.Kind == hir.Void && l.ck.GetReturnTypeOfSignature(sig) != nil && !isIntrinsicVoid(l.ck.GetReturnTypeOfSignature(sig)) {
		l.diags = l.diags[:before]
		l.diagf(n, "skipped-cast-view", "view member %s.%s: result type is skipped", classSym.Name, member)
		return hir.Type{Kind: hir.InterfaceRef, Name: key}, true
	}
	i.Methods = append(i.Methods, hm)
	l.methodsBy[key+"."+member] = hm
	return hir.Type{Kind: hir.InterfaceRef, Name: key}, true
}

func isIntrinsicVoid(t *checker.Type) bool { return t != nil && t.Flags()&checker.TypeFlagsVoid != 0 }

// viewCast wraps an expression whose checker type is a non-lowered class in
// a cast to the (lazily grown) view interface, adding member.
func (l *lowerer) viewCast(n *ast.Node, member string) (*hir.Expr, bool) {
	if n == nil || n.Kind != ast.KindIdentifier && n.Kind != ast.KindPropertyAccessExpression && n.Kind != ast.KindCallExpression {
		return nil, false
	}
	t := l.ck.GetTypeAtLocation(n)
	if t == nil || t.Symbol() == nil || !t.IsClass() {
		return nil, false
	}
	if l.classOf(t.Symbol()) != nil {
		return nil, false // lowered classes do not need views
	}
	// Library globals (Object and friends) are not vendored classes.
	if f := l.fileOfSymbol(t.Symbol()); f == nil || strings.Contains(f.FileName(), "/libs/") || strings.Contains(f.FileName(), "lib.") {
		return nil, false
	}
	vt, ok := l.viewRef(n, t.Symbol(), member)
	if !ok {
		return nil, false
	}
	x := l.expr(n)
	if x == nil {
		return nil, false
	}
	return &hir.Expr{Kind: hir.Cast, Node: l.node(n), Type: vt, X: x}, true
}

// recordLiteral lowers an object literal whose contextual type is an
// index-signature record: a fresh map, spread operands copy their entries,
// literal and computed (constant) keys are set.
func (l *lowerer) recordLiteral(n *ast.Node, typ hir.Type) *hir.Expr {
	m := l.tempVar(n, typ)
	keyT, valT := typ.Args[0], typ.Args[1]
	for _, p := range n.AsObjectLiteralExpression().Properties.Nodes {
		switch p.Kind {
		case ast.KindSpreadAssignment:
			sv := l.expr(p.Expression())
			if sv == nil || sv.Type.Kind != hir.OrderedMap {
				l.diagf(p, "unsupported-type", "record spread needs a map")
				return nil
			}
			sv = l.tempInit(p, sv.Type, sv)
			l.diagf(p, "note-object-spread", "record spread copies the entries")
			l.serial++
			k := "k" + itoa(l.serial)
			l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(p), Name: k, Type: keyT,
				X: l.rtOp("map.keys", sv, hir.T(hir.Array, keyT)),
				Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(p),
					X: l.rtOp("map.set", m, typ, hir.V(k, keyT),
						l.coerce(&hir.Expr{Kind: hir.Narrow, Type: sv.Type.Args[1], X: l.rtOp("map.get", sv, hir.T(hir.Optional, sv.Type.Args[1]), hir.V(k, keyT))}, valT))})})
		case ast.KindPropertyAssignment, ast.KindShorthandPropertyAssignment:
			var keyExpr *hir.Expr
			var value *ast.Node
			if p.Kind == ast.KindPropertyAssignment {
				value = p.Initializer()
			} else {
				value = p.Name()
			}
			name := p.Name()
			if name == nil {
				l.diagf(p, "unsupported-type", "record property without a name")
				return nil
			}
			switch name.Kind {
			case ast.KindIdentifier, ast.KindStringLiteral:
				keyExpr = hir.L(keyT, name.Text())
			case ast.KindComputedPropertyName:
				keyExpr = l.expr(name.Expression())
			default:
				l.diagf(p, "unsupported-type", "record property name %s is not lowered", name.Kind.String())
				return nil
			}
			if keyExpr == nil {
				return nil
			}
			l.hint = hir.T(hir.Optional, valT)
			v := l.expr(value)
			l.hint = hir.Type{}
			if v == nil {
				return nil
			}
			l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(p),
				X: l.rtOp("map.set", m, typ, keyExpr, l.coerce(v, valT))})
		default:
			l.diagf(p, "unsupported-type", "record property %s is not lowered", p.Kind.String())
			return nil
		}
	}
	return m
}

// constantKeyName resolves a computed property name node to its constant
// string (enum member or string literal).
func (l *lowerer) constantKeyName(n *ast.Node) (string, bool) {
	if n == nil || n.Kind != ast.KindComputedPropertyName {
		return "", false
	}
	e := n.AsComputedPropertyName().Expression
	if e == nil {
		return "", false
	}
	if e.Kind == ast.KindStringLiteral {
		return e.Text(), true
	}
	if e.Kind == ast.KindPropertyAccessExpression {
		if x, ok := l.enumMember(e, e.AsPropertyAccessExpression()); ok {
			if v, ok := x.Value.(string); ok {
				return v, true
			}
		}
	}
	return "", false
}

// propertyNameNode returns the raw name node of a property assignment
// (a DeclarationName IS a Node).
func propertyNameNode(p *ast.Node) *ast.Node {
	if nm := p.Name(); nm != nil {
		return nm
	}
	return nil
}

// optionalChain lowers `a?.b`, `a?.[i]` and `a?.m(...)`: absent a yields
// undefined, otherwise the access proceeds on the unwrapped receiver.
func (l *lowerer) optionalChain(n *ast.Node) *hir.Expr {
	base := n.Expression()
	if n.Kind == ast.KindCallExpression {
		base = base.Expression()
	}
	recv := l.expr(base)
	if base.Kind == ast.KindElementAccessExpression {
		arr := l.expr(base.Expression())
		if arr != nil && arr.Type.Kind == hir.Array {
			recv = l.rtOp("array.get", arr, hir.T(hir.Optional, arr.Type.Args[0]), l.expr(base.AsElementAccessExpression().ArgumentExpression))
		}
	}
	if recv == nil {
		return nil
	}
	if recv.Type.Kind != hir.Optional && !recv.Type.IsRef() {
		l.diagf(n, "unsupported-expr", "optional chain needs a reference or optional receiver")
		return nil
	}
	rv := l.tempInit(n, recv.Type, recv)
	elem := recv.Type
	if elem.Kind == hir.Optional {
		elem = elem.Args[0]
	}
	unwrapped := &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: elem, X: rv}
	if recv.Type.Kind != hir.Optional {
		unwrapped = rv
	}
	saved := l.pend
	l.pend = nil
	value := l.withLocal(base, unwrapped, func() *hir.Expr {
		switch n.Kind {
		case ast.KindCallExpression:
			return l.call(n)
		case ast.KindPropertyAccessExpression:
			return l.propertyAccess(n)
		default:
			return l.elementAccess(n)
		}
	})
	pre := l.pend
	l.pend = saved
	if value == nil {
		return nil
	}
	test := &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: rv}
	if value.Type.Kind == hir.Void {
		body := append(pre, &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: value})
		return &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: hir.T(hir.Bool), Stmt: hir.B(&hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: test}, Body: hir.B(body...)}), Y: hir.L(hir.T(hir.Bool), false)}
	}
	if len(pre) > 0 {
		value = &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: value.Type, Stmt: hir.B(pre...), Y: value}
	}
	typ := value.Type
	if typ.Kind != hir.Optional {
		typ = hir.T(hir.Optional, typ)
	}
	return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: typ, X: test, Y: &hir.Expr{Kind: hir.Lit, Type: typ}, Z: value}
}

// withLocal lowers f with the identifier node `name` bound to a replacement
// expression (used to rewrite optional-chain receivers).
func (l *lowerer) withLocal(name *ast.Node, repl *hir.Expr, f func() *hir.Expr) *hir.Expr {
	scope := map[string]hir.Type{}
	if name.Kind == ast.KindIdentifier {
		scope[name.Text()] = repl.Type
	}
	l.scope = append(l.scope, scope)
	l.replacements = append(l.replacements, [2]any{name, repl})
	x := f()
	l.replacements = l.replacements[:len(l.replacements)-1]
	l.scope = l.scope[:len(l.scope)-1]
	return x
}
