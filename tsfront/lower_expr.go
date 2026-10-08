package tsfront

import (
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
	"github.com/oisee/abapiti/internal/tsgo/stringutil"
)

// Expression lowering. Every expression is lowered to its natural HIR node
// first (locals and fields carry their declared type); the checker's type at
// the node is then compared with it, and flow narrowing (`if (x) ...`,
// `x === undefined` branches, dominating instanceof) becomes a hir.Narrow.

func (l *lowerer) rtOp(op string, x *hir.Expr, t hir.Type, args ...*hir.Expr) *hir.Expr {
	ps, result, ok := hir.RuntimeSignature(op, x.Type)
	if ok {
		for i, a := range args {
			if a != nil && i < len(ps) && ps[i].Kind == hir.I32 && a.Type.Kind == hir.Number {
				args[i] = &hir.Expr{Kind: hir.RuntimeOp, Node: a.Node, Op: "number.index", Type: hir.T(hir.I32), X: a}
			}
		}
		t = result
	}
	e := &hir.Expr{Kind: hir.RuntimeOp, Node: x.Node, Op: op, Type: t, X: x, Args: args}
	if t.Kind == hir.I32 {
		return &hir.Expr{Kind: hir.RuntimeOp, Node: x.Node, Op: "number.fromI32", Type: hir.T(hir.Number), X: e}
	}
	return e
}

// condition lowers an expression used where a boolean is required, applying
// JavaScript truthiness where the checker type is not boolean.
func (l *lowerer) condition(n *ast.Node) *hir.Expr {
	if n.Kind == ast.KindParenthesizedExpression {
		return l.condition(n.Expression())
	}
	if n.Kind == ast.KindBinaryExpression {
		b := n.AsBinaryExpression()
		if b.OperatorToken.Kind == ast.KindAmpersandAmpersandToken || b.OperatorToken.Kind == ast.KindBarBarToken {
			op := "&&"
			if b.OperatorToken.Kind == ast.KindBarBarToken {
				op = "||"
			}
			return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Type: hir.T(hir.Bool), Op: op, X: l.condition(b.Left), Y: l.condition(b.Right)}
		}
	}
	x := l.expr(n)
	if x == nil {
		return nil
	}
	if x.Type.Kind == hir.Bool {
		return x
	}
	return &hir.Expr{Kind: hir.ToBoolean, Node: x.Node, Type: hir.T(hir.Bool), X: x}
}

func (l *lowerer) expr(n *ast.Node) *hir.Expr {
	for i := len(l.replacements) - 1; i >= 0; i-- {
		if l.replacements[i][0] == n {
			x, ok := l.replacements[i][1].(*hir.Expr)
			if !ok {
				panic("tsfront: replacement is not an expression")
			}
			return x
		}
	}
	saved := l.pend
	l.pend = nil
	var x *hir.Expr
	if e, ok := l.overrides[n]; ok && e.Expression != nil {
		x = e.Expression()
		x.Node = l.node(n)
	} else {
		if n != nil {
			for parent := n.Parent; parent != nil; parent = parent.Parent {
				if e, ok := l.overrides[parent]; ok && e.Expressions != nil {
					span := l.file.Text()[scanner.GetTokenPosOfNode(n, l.file, false):n.End()]
					if build := e.Expressions[span]; build != nil {
						x = build()
						x.Node = l.node(n)
						break
					}
				}
			}
		}
		if x == nil {
			x = l.naturalExpr(n)
		}
	}
	if x != nil {
		x = l.narrowedAt(n, x)
	}
	pre := l.pend
	l.pend = saved
	if x == nil {
		return nil
	}
	if len(pre) > 0 {
		return &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: x.Type, Y: x, Stmt: hir.B(pre...)}
	}
	return x
}

// narrowedAt wraps x when the checker's type at the node is a narrower
// reference type than x's natural type. Mapping failures are quiet here: the
// natural type (from the declaration registry) stands.
func (l *lowerer) narrowedAt(n *ast.Node, x *hir.Expr) *hir.Expr {
	if n.Parent != nil && n.Parent.Kind == ast.KindBinaryExpression {
		b := n.Parent.AsBinaryExpression()
		if l.isUndefinedType(b.Left) || l.isUndefinedType(b.Right) || (b.Left == n && (b.OperatorToken.Kind == ast.KindBarBarToken || b.OperatorToken.Kind == ast.KindAmpersandAmpersandToken)) {
			return x
		}
	}
	// Contextual construction already selected the result type. Fresh object
	// literal types are structural views, not derived runtime classes.
	if n.Flags&ast.NodeFlagsOptionalChain != 0 || n.Kind == ast.KindObjectLiteralExpression || n.Kind == ast.KindArrayLiteralExpression ||
		(n.Kind == ast.KindCallExpression && n.Expression().Kind == ast.KindPropertyAccessExpression && n.Expression().AsPropertyAccessExpression().QuestionDotToken != nil) ||
		(n.Kind == ast.KindPropertyAccessExpression && n.AsPropertyAccessExpression().QuestionDotToken != nil) {
		return x
	}
	// An index signature does not prove that a particular key exists. Keep
	// lookup absence even when noUncheckedIndexedAccess is disabled upstream.
	if x.Type.Kind == hir.Optional && n.Kind == ast.KindElementAccessExpression {
		return x
	}
	if x.Type.Kind == hir.Optional && n.Kind == ast.KindIdentifier && !x.Type.Args[0].IsRef() {
		if sym := l.resolve(n); sym != nil {
			declared := l.ck.GetTypeOfSymbol(sym)
			if declared != nil && !declared.IsUnion() && sym.ValueDeclaration != nil && sym.ValueDeclaration.Kind == ast.KindVariableDeclaration {
				return x
			}
		}
	}
	t := l.ck.GetTypeAtLocation(n)
	if t == nil || t.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
		return x
	}
	before := len(l.diags)
	typ := l.mapCheckerType(n, t)
	if hasBlocking(l.diags[before:]) {
		l.diags = l.diags[:before] // quiet: narrowing is optional
		return x
	}
	l.diags = l.diags[:before]
	if x.Type.Kind == hir.ClassRef && typ.Kind == hir.ClassRef && strings.HasPrefix(typ.Name, "shape.") {
		var a, b *hir.Class
		for _, c := range l.out.Classes {
			if c.Name == x.Type.Name {
				a = c
			}
			if c.Name == typ.Name {
				b = c
			}
		}
		if a != nil && b != nil && len(a.Fields) == len(b.Fields) {
			same := true
			for i, f := range a.Fields {
				same = same && f.Name == b.Fields[i].Name && f.Type.Equal(b.Fields[i].Type)
			}
			if same {
				return x
			}
		}
	}
	return l.narrowed(n, x, typ)
}

func (l *lowerer) narrowed(n *ast.Node, x *hir.Expr, typ hir.Type) *hir.Expr {
	if typ.Kind == hir.Void || x.Type.Equal(typ) || ((x.Type.Kind == hir.Array || x.Type.Kind == hir.OrderedMap || x.Type.Kind == hir.OrderedSet) && typ.Kind != x.Type.Kind && typ.Kind != hir.Optional) {
		return x
	}
	if l.acceptsType(typ, x.Type) {
		return x
	}
	if x.Type.Kind == hir.Optional && len(x.Type.Args) == 1 && x.Type.Args[0].Kind == hir.Dynamic && typ.Kind != hir.Optional {
		x = &hir.Expr{Kind: hir.Narrow, Type: hir.T(hir.Dynamic), X: x}
	}
	if x.Type.Kind == hir.Dynamic && typ.Kind == hir.Bool {
		return l.rtOp("dynamic.asBoolean", x, typ)
	}
	if x.Type.Kind == hir.Dynamic && typ.Kind == hir.Number {
		return l.rtOp("dynamic.asNumber", x, typ)
	}
	if x.Type.Kind == hir.Dynamic && typ.Kind == hir.String {
		return l.rtOp("dynamic.asString", x, typ)
	}
	if x.Type.Kind == hir.Dynamic && typ.Kind == hir.ClassValue {
		return l.rtOp("dynamic.asClassValue", x, typ)
	}
	if x.Type.Kind == hir.Dynamic && (typ.Kind == hir.ClassRef || typ.Kind == hir.InterfaceRef) {
		return l.rtOp("dynamic.asRef", x, typ)
	}
	if (typ.Kind == hir.Optional && x.Type.Kind == hir.Optional && typ.Args[0].IsRef() && l.acceptsType(x.Type.Args[0], typ.Args[0])) || (typ.Kind == hir.Array && x.Type.Kind == hir.Array && l.acceptsType(x.Type, typ)) || typ.Kind == hir.ClassRef || typ.Kind == hir.InterfaceRef || (x.Type.Kind == hir.Optional && len(x.Type.Args) == 1 && x.Type.Args[0].Equal(typ)) {
		return &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: typ, X: x}
	}
	return x
}

func (l *lowerer) naturalExpr(n *ast.Node) *hir.Expr {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case ast.KindStringLiteral, ast.KindNoSubstitutionTemplateLiteral:
		// Preflight already diagnosed lone units. Never pass them to HIR,
		// where ordinary Go rune conversion would irreversibly replace them.
		value := stringutil.CombineSurrogatePairs(n.Text())
		if hasLoneSurrogate(value) {
			return nil
		}
		return &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.String), Value: value}
	case ast.KindTemplateExpression:
		return l.templateExpr(n)
	case ast.KindTemplateHead, ast.KindTemplateMiddle, ast.KindTemplateTail:
		value := stringutil.CombineSurrogatePairs(n.Text())
		if hasLoneSurrogate(value) {
			return nil
		}
		return &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.String), Value: value}
	case ast.KindNumericLiteral:
		return l.numericLiteral(n, hir.T(hir.Number), 1)
	case ast.KindTrueKeyword:
		return hir.L(hir.T(hir.Bool), true)
	case ast.KindNullKeyword:
		return l.rtOp("dynamic.null", hir.L(hir.T(hir.Number), 0), hir.T(hir.Dynamic))
	case ast.KindFalseKeyword:
		return hir.L(hir.T(hir.Bool), false)
	case ast.KindTypeOfExpression:
		x, _ := l.typeofExpr(n)
		return x
	case ast.KindIdentifier:
		return l.identifier(n)
	case ast.KindDeleteExpression:
		operand := n.AsDeleteExpression().Expression
		var receiver, key *hir.Expr
		var receiverNode *ast.Node
		switch operand.Kind {
		case ast.KindElementAccessExpression:
			p := operand.AsElementAccessExpression()
			receiverNode = p.Expression
			receiver = l.expr(p.Expression)
			key = l.expr(p.ArgumentExpression)
		case ast.KindPropertyAccessExpression:
			p := operand.AsPropertyAccessExpression()
			receiverNode = p.Expression
			receiver = l.expr(p.Expression)
			key = hir.L(hir.T(hir.String), p.Name().Text())
		default:
			l.diagf(n, "unsupported-expr", "delete needs a record property")
			return nil
		}
		if receiver == nil || key == nil {
			return nil
		}
		// Maps share the OrderedMap representation with records, but delete on a
		// Map only removes an own property, never an entry.
		if sym := l.ck.GetTypeAtLocation(receiverNode).Symbol(); sym != nil && (sym.Name == "Map" || sym.Name == "ReadonlyMap") {
			l.diagf(n, "unsupported-expr", "delete on a Map does not remove its entries")
			return nil
		}
		if receiver.Type.Kind != hir.OrderedMap || !receiver.Type.Args[0].Equal(key.Type) {
			l.diagf(n, "unsupported-expr", "delete supports ordinary lowered records only")
			return nil
		}
		return l.rtOp("record.delete", receiver, hir.T(hir.Bool), key)
	case ast.KindThisKeyword:
		return l.this(l.class)
	case ast.KindParenthesizedExpression:
		return l.expr(n.Expression())
	case ast.KindPropertyAccessExpression:
		if n.Flags&ast.NodeFlagsOptionalChain != 0 {
			return l.optionalChain(n)
		}
		return l.propertyAccess(n)
	case ast.KindCallExpression:
		if callee := n.Expression(); callee != nil && callee.Kind == ast.KindPropertyAccessExpression && n.Flags&ast.NodeFlagsOptionalChain != 0 {
			return l.optionalChain(n)
		}
		return l.call(n)
	case ast.KindNewExpression:
		return l.newExpression(n)
	case ast.KindBinaryExpression:
		return l.binary(n)
	case ast.KindPrefixUnaryExpression:
		return l.prefixUnary(n)
	case ast.KindPostfixUnaryExpression:
		u := n.AsPostfixUnaryExpression()
		switch u.Operator {
		case ast.KindPlusPlusToken, ast.KindMinusMinusToken:
			op := "+"
			if u.Operator == ast.KindMinusMinusToken {
				op = "-"
			}
			x := l.expr(u.Operand)
			if x == nil || x.Kind != hir.Local {
				l.diagf(n, "unsupported-expr", "postfix %s needs a local operand", op)
				return nil
			}
			one := hir.L(x.Type, 1)
			return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: op, Type: x.Type, X: x, Y: one}
		}
		l.diagf(n, "unsupported-expr", "postfix %s is not lowered", u.Operator.String())
		return nil
	case ast.KindConditionalExpression:
		c := n.AsConditionalExpression()
		x := l.condition(c.Condition)
		hint := l.hint
		if hint.Kind == hir.Void {
			l.hint = l.mapCheckerType(n, l.ck.GetTypeAtLocation(n))
		}
		branchHint := l.hint
		y := l.expr(c.WhenTrue)
		l.hint = branchHint
		z := l.expr(c.WhenFalse)
		l.hint = hint
		if x == nil || y == nil || z == nil {
			return nil
		}
		result := y.Type
		if !y.Type.Equal(z.Type) {
			result = hint
			if result.Kind == hir.Void {
				result = l.mapCheckerType(n, l.ck.GetTypeAtLocation(n))
			}
			if result.Kind == hir.Void {
				return nil
			}
		}
		return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: result, X: x, Y: l.coerce(y, result), Z: l.coerce(z, result)}
	case ast.KindAsExpression:
		return l.asExpression(n)
	case ast.KindSatisfiesExpression:
		old := l.hint
		l.hint = l.mapTypeNode(n.Type())
		x := l.expr(n.Expression())
		l.hint = old
		return x
	case ast.KindNonNullExpression:
		return l.expr(n.Expression())
	case ast.KindObjectLiteralExpression:
		return l.objectLiteral(n)
	case ast.KindArrayLiteralExpression:
		els := n.AsArrayLiteralExpression().Elements.Nodes
		if l.hint.Kind == hir.Optional && l.hint.Args[0].Kind == hir.Array {
			l.hint = l.hint.Args[0]
		}
		if len(els) == 0 {
			if l.hint.Kind != hir.Array && l.hint.Kind != hir.OrderedSet {
				if t := l.ck.GetContextualType(n, 0); t != nil {
					l.hint = l.mapCheckerType(n, t)
				}
			}
			if l.hint.Kind == hir.Optional && len(l.hint.Args) == 1 {
				l.hint = l.hint.Args[0]
			}
			if l.hint.Kind == hir.Array || l.hint.Kind == hir.OrderedSet {
				return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: l.hint}
			}
			l.diagf(n, "unsupported-array", "empty array literal outside a statement context")
			return nil
		}
		if l.hint.Kind == hir.ClassRef && strings.HasPrefix(l.hint.Name, "tuple.") {
			// Construct positional fields in their individual contexts.
			tuple := l.hint
			c := shapeClassOf(l.out.Classes, tuple)
			if c == nil || c.Ctor == nil || len(c.Ctor.Params) != len(els) {
				l.diagf(n, "unsupported-array", "tuple arity does not match its context")
				return nil
			}
			var elems []*hir.Expr
			for i, el := range els {
				l.hint = c.Ctor.Params[i].Type
				x := l.expr(el)
				if x == nil {
					l.hint = tuple
					return nil
				}
				elems = append(elems, l.coerce(x, c.Ctor.Params[i].Type))
			}
			l.hint = tuple
			return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: tuple, Args: elems}
		}
		// Lower as a fresh array plus pushes; spreads concat; the prelude
		// carries the loop.
		hint := l.hint
		elem := hir.Type{}
		if hint.Kind == hir.Array {
			elem = hint.Args[0]
		} else {
			mapped := l.mapCheckerType(n, l.ck.GetTypeAtLocation(n))
			if mapped.Kind == hir.Array {
				elem = mapped.Args[0]
			}
			for _, el := range els {
				if elem.Kind != hir.Void {
					break
				}
				if el.Kind == ast.KindSpreadElement {
					continue
				}
				elem = l.mapCheckerType(n, l.ck.GetTypeAtLocation(el))
				break
			}
		}
		if elem.Kind == hir.Void {
			mapped := l.mapCheckerType(n, l.ck.GetTypeAtLocation(n))
			if mapped.Kind != hir.Array {
				return nil
			}
			elem = mapped.Args[0]
		}
		typ := hir.T(hir.Array, elem)
		arr := l.tempVar(n, typ)
		for _, el := range els {
			if el.Kind == ast.KindSpreadElement {
				sv := l.expr(el.Expression())
				if sv == nil || sv.Type.Kind != hir.Array {
					l.diagf(el, "unsupported-array", "spread element needs an array")
					return nil
				}
				nxt := l.rtOp("array.concat", arr, typ, sv)
				l.serial++
				name := "a" + itoa(l.serial)
				l.declare(name, typ)
				l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(el), Name: name, Type: typ, X: nxt})
				arr = hir.V(name, typ)
				continue
			}
			l.hint = elem
			v := l.expr(el)
			l.hint = hint
			if v == nil {
				return nil
			}
			l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(el),
				X: l.rtOp("array.push", arr, hir.T(hir.I32), l.coerce(v, elem))})
		}
		return arr
	case ast.KindRegularExpressionLiteral:
		return l.regexLiteral(n)
	case ast.KindElementAccessExpression:
		if e := n.AsElementAccessExpression(); e.QuestionDotToken != nil {
			return l.optionalChain(n)
		}
		return l.elementAccess(n)
	}
	l.diagf(n, "unsupported-expr", "%s is not lowered", n.Kind.String())
	return nil
}

func (l *lowerer) numericLiteral(n *ast.Node, typ hir.Type, sign float64) *hir.Expr {
	text := strings.ReplaceAll(n.Text(), "_", "")
	v, err := strconv.ParseFloat(text, 64)
	if err != nil {
		if integer, intErr := strconv.ParseInt(text, 0, 64); intErr == nil {
			v, err = float64(integer), nil
		}
	}
	if err != nil {
		l.diagf(n, "unsupported-number", "numeric literal %s: %v", text, err)
		return nil
	}
	l.checkNumberLiteral(n, v*sign)
	return hir.L(typ, v*sign)
}

// identifier lowers a reference by resolving its symbol: locals and
// parameters first, then module values, then `undefined`.
func (l *lowerer) identifier(n *ast.Node) *hir.Expr {
	name := n.Text()
	for i := len(l.replacements) - 1; i >= 0; i-- {
		if node, ok := l.replacements[i][0].(*ast.Node); ok && node == n {
			x, ok := l.replacements[i][1].(*hir.Expr)
			if !ok {
				panic("tsfront: replacement is not an expression")
			}
			return x
		}
	}
	if t, ok := l.lookup(name); ok {
		x := hir.V(name, t)
		x.Node = l.node(n)
		return x
	}
	sym := l.resolve(n)
	if sym == nil {
		l.diagf(n, "unsupported-expr", "unresolved identifier %s", name)
		return nil
	}
	if sym.Name == "undefined" && sym.ValueDeclaration == nil {
		if l.hint.Kind == hir.Optional && len(l.hint.Args) == 1 {
			return &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.Optional, l.hint.Args[0])}
		}
		return &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.Optional, hir.T(hir.Dynamic))}
	}
	if owner, fieldName, ok := l.modvarOf(sym); ok {
		for _, mod := range l.out.Classes {
			if mod.Name != owner {
				continue
			}
			for _, f := range mod.Fields {
				if f.Name == fieldName {
					return &hir.Expr{Kind: hir.StaticGet, Node: l.node(n), Owner: mod.Name, Name: fieldName, Type: f.Type}
				}
			}
		}
	}
	if x, ok := l.enumNamespace(n); ok {
		return x
	}
	if x, ok := l.classValueRef(n); ok {
		return x
	}
	if x, ok := l.namespaceRef(n); ok {
		return x
	}
	l.diagf(n, "unsupported-expr", "identifier %s is not a lowered local or module value", name)
	return nil
}

// propertyAccess lowers member reads; call targets are handled in call.
func (l *lowerer) propertyAccess(n *ast.Node) *hir.Expr {
	p := n.AsPropertyAccessExpression()
	if x, ok := l.classValueRef(n); ok {
		return x
	}
	if x, ok := l.enumMember(n, p); ok {
		return x
	}
	sym := l.resolve(n)
	f, hasField := l.fieldOf(sym)
	if sym != nil && hasField {
		if f.Static {
			owner := l.classOf(sym.Parent)
			if p.Expression.Kind == ast.KindThisKeyword {
				owner = l.class
			}
			if owner != nil {
				return &hir.Expr{Kind: hir.StaticGet, Node: l.node(n), Owner: owner.Name, Name: f.Name, Type: f.Type}
			}
		}
		if p.Expression.Kind == ast.KindThisKeyword {
			return &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: f.Name, Type: f.Type, X: l.this(l.class)}
		}
		if p.Expression.Kind == ast.KindSuperKeyword {
			l.diagf(n, "unsupported-expr", "super field access is not lowered")
			return nil
		}
		oldHint := l.hint
		l.hint = hir.Type{}
		recv := l.expr(p.Expression)
		l.hint = oldHint
		if recv == nil {
			return nil
		}
		if recv.Type.Kind == hir.Optional {
			recv = &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: recv.Type.Args[0], X: recv}
		}
		return &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: f.Name, Type: f.Type, X: recv}
	}
	// A static field of a lowered class: ClassName.member.
	if p.Expression != nil && p.Expression.Kind == ast.KindIdentifier {
		if cls := l.resolve(p.Expression); cls != nil {
			if c, ok := l.classes[cls]; ok && c != l.class {
				for _, f := range c.Fields {
					if f.Static && f.Name == n.Name().Text() {
						return &hir.Expr{Kind: hir.StaticGet, Node: l.node(n), Owner: c.Name, Name: f.Name, Type: f.Type}
					}
				}
			}
		}
	}
	// Resolve fields through the receiver's HIR schema when transient checker
	// property symbols do not match the declaration registry.
	outputHint := l.hint
	l.hint = hir.Type{}
	recv := l.expr(p.Expression)
	l.hint = outputHint
	if recv == nil {
		return nil
	}
	base := recv.Type
	if base.Kind == hir.Optional {
		base = base.Args[0]
	}
	if base.Kind == hir.Dynamic {
		if recv.Type.Kind == hir.Optional {
			recv = &hir.Expr{Kind: hir.Narrow, Type: base, X: recv}
		}
		return l.rtOp("dynamic.get", recv, hir.T(hir.Dynamic), hir.L(hir.T(hir.String), n.Name().Text()))
	}
	if recv.Type.Kind == hir.Optional && (base.Kind == hir.Array || base.Kind == hir.OrderedMap || base.Kind == hir.OrderedSet || base.Kind == hir.String) {
		recv = &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: base, X: recv}
	}
	if base.Kind == hir.ClassRef {
		for c := l.classByName(base.Name); c != nil; c = l.classByName(c.Super) {
			for _, f := range c.Fields {
				if f.Name == n.Name().Text() && !f.Static {
					if recv.Type.Kind == hir.Optional {
						recv = &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: base, X: recv}
					}
					return &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: f.Name, Type: f.Type, X: recv}
				}
			}
		}
	}
	switch n.Name().Text() {
	case "length":
		switch recv.Type.Kind {
		case hir.String:
			return l.rtOp("string.length", recv, hir.T(hir.I32))
		case hir.Array:
			return l.rtOp("array.length", recv, hir.T(hir.I32))
		}
	case "size":
		switch recv.Type.Kind {
		case hir.OrderedSet:
			return l.rtOp("set.size", recv, hir.T(hir.I32))
		case hir.OrderedMap:
			return l.rtOp("map.size", recv, hir.T(hir.I32))
		}
	case "constructor":
		// x.constructor is the run-time class; compared with a class value or
		// asked for its name.
		if recv.Type.Kind == hir.ClassRef && recv.Type.Name == hir.RootObject {
			l.diagf(n, "unsupported-expr", "constructor of the object root is not lowered")
			return nil
		}
		if recv.Type.Kind == hir.ClassRef || recv.Type.Kind == hir.InterfaceRef {
			return l.rtOp("object.classOf", recv, hir.T(hir.ClassValue))
		}
	case "name":
		if recv.Type.Kind == hir.ClassValue {
			return l.rtOp("classvalue.name", recv, hir.T(hir.String))
		}
	case "source":
		if recv.Type.Kind == hir.RegExp {
			return l.rtOp("regexp.source", recv, hir.T(hir.String))
		}
	}
	// A namespace member that names an exported class is a class value.
	if p.Expression != nil && p.Expression.Kind == ast.KindIdentifier {
		if ns := l.resolve(p.Expression); ns != nil {
			if mod := l.fileOfSymbol(ns); mod != nil {
				if c := l.classesByName[mod.FileName()+" "+n.Name().Text()]; c != nil {
					if x, ok := l.classValueRef(n); ok {
						return x
					}
				}
			}
		}
	}
	// A record member with a literal name reads the map.
	if recv.Type.Kind == hir.OrderedMap && n.Name() != nil {
		key := n.Name().Text()
		opt := l.rtOp("map.get", recv, hir.T(hir.Optional, recv.Type.Args[1]), hir.L(hir.T(hir.String), key))
		return &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: recv.Type.Args[1], X: opt}
	}
	// Statics and class values through a namespace alias (Ns.member).
	if p.Expression != nil && p.Expression.Kind == ast.KindIdentifier {
		if ns := l.resolve(p.Expression); ns != nil {
			if x, ok := l.classValueRef(n); ok {
				return x
			}
			// A static member of a class in another module resolved through
			// the namespace: the existing static path below handles it when
			// the alias resolves to the class itself.
			_ = ns
		}
	}
	l.diagf(n, "unsupported-expr", "property %s is not lowered", n.Name().Text())
	return nil
}

// call lowers method calls: lowered class methods (virtual and super) and
// the library operations on strings, numbers and collections.
// isGlobalObjectExpr matches the `Object` global (its members are handled by
// objectStatic, never as a receiver).
func isGlobalObjectExpr(e *ast.Node) bool {
	return e != nil && e.Kind == ast.KindIdentifier && e.Text() == "Object"
}

func (l *lowerer) call(n *ast.Node) *hir.Expr {
	callee := n.Expression()
	if callee == nil {
		l.diagf(n, "unsupported-call", "call without a callee")
		return nil
	}
	if callee.Kind == ast.KindIdentifier {
		// Module-level functions and lifted local functions.
		name := callee.Text()
		if lf, ok := l.localFns[name]; ok {
			return l.localFnCall(n, lf, callee)
		}
		if node, ok := l.pendingFns[name]; ok && l.lookupPendingAllowed() {
			delete(l.pendingFns, name)
			if lf := l.liftLocalFn(name, node); lf != nil {
				return l.localFnCall(n, lf, callee)
			}
		}
		if _, isLocal := l.lookup(name); !isLocal {
			if sym := l.resolve(callee); sym != nil {
				if f := l.fileOfSymbol(sym); f != nil {
					if ref, ok := l.funcsBy[f.FileName()+" "+sym.Name]; ok {
						args, ok2 := l.callArgsStatic(n, n.Arguments(), ref)
						if !ok2 {
							return nil
						}
						return &hir.Expr{Kind: hir.DirectCall, Node: l.node(n), Owner: ref.owner, Name: ref.method,
							Type: l.callbackResultType(ref), Args: args}
					}
				}
			}
		}
	}
	if callee.Kind == ast.KindPropertyAccessExpression {
		p := callee.AsPropertyAccessExpression()
		name := callee.Name().Text()
		// Array higher-order calls inline into loops before anything else.
		if p.Expression != nil && p.QuestionDotToken == nil && (name == "map" || name == "filter" || name == "some" || name == "every" || name == "reduce" || name == "forEach") && !isGlobalObjectExpr(p.Expression) {
			outputHint := l.hint
			l.hint = hir.Type{}
			recvX := l.expr(p.Expression)
			l.hint = outputHint
			if recvX != nil && recvX.Type.Kind == hir.Array {
				if x, ok := l.hofCall(n, name, recvX, n.Arguments()); ok {
					return x
				}
			}

		}
		// Object.keys and friends (no lowered class is ever named Object).
		if isGlobalObjectExpr(p.Expression) {
			if x, ok := l.objectStatic(n, name); ok {
				return x
			}
		}
		if p.Expression.Kind == ast.KindSuperKeyword {
			hm := l.methodOf(l.resolve(callee))
			if hm == nil {
				l.diagf(n, "unsupported-call", "super call to %s is not a lowered method", name)
				return nil
			}
			args, ok := l.callArgs(n, n.Arguments(), hm.Params)
			if !ok {
				return nil
			}
			return &hir.Expr{Kind: hir.SuperCall, Node: l.node(n), Name: hm.Name, Type: hm.Result, Args: args}
		}
		if p.Expression != nil && p.Expression.Kind == ast.KindIdentifier {
			if cls := l.classOf(l.resolve(p.Expression)); cls != nil {
				for _, m := range cls.Methods {
					if m.Name != name {
						continue
					}
					if !m.Static || m.Name == "class_constructor" {
						l.diagf(n, "unsupported-call", "method %s of %s is not static", name, cls.Name)
						return nil
					}
					args, ok := l.callArgs(n, n.Arguments(), m.Params)
					if !ok {
						return nil
					}
					return &hir.Expr{Kind: hir.DirectCall, Node: l.node(n), Owner: cls.Name, Name: name, Type: m.Result, Args: args}
				}
			}
		}
		if hm := l.methodOf(l.resolve(callee)); hm != nil && hm.Name != "class_constructor" {
			if hm.Static {
				owner := l.class
				if p.Expression.Kind != ast.KindThisKeyword {
					owner = l.classOf(l.resolve(p.Expression))
				}
				if owner == nil {
					l.diagf(n, "unsupported-call", "unresolved static method owner")
					return nil
				}
				args, ok := l.callArgsMethod(n, n.Arguments(), hm, 0)
				if !ok {
					return nil
				}
				return &hir.Expr{Kind: hir.DirectCall, Node: l.node(n), Owner: owner.Name, Name: hm.Name, Type: hm.Result, Args: args}
			}
			recv := l.expr(p.Expression)
			if recv == nil {
				return nil
			}
			// Ordinary calls must use the erased virtual slot. The specialized
			// implementation name is reserved for its bridge and super calls.
			slot := hm
			if original, _, specialized := strings.Cut(hm.Name, "_instantiated_"); specialized {
				for c := l.classesByQualifiedName(recv.Type.Name); c != nil; c = l.classesByQualifiedName(c.Super) {
					if m := l.methodsBy[c.Name+"."+original]; m != nil && m.Name == original {
						slot = m
						break
					}
					for _, m := range c.Methods {
						if m.Name == original {
							slot = m
							break
						}
					}
					if slot != hm {
						break
					}
				}
				if slot == hm {
					l.diagf(n, "unsupported-generic-dispatch", "erased virtual slot %s is unavailable", original)
					return nil
				}
			}
			if slot.Result.Kind == hir.Void && hm.Result.IsRef() {
				for c := l.classesByQualifiedName(recv.Type.Name); c != nil; c = l.classesByQualifiedName(c.Super) {
					found := false
					for _, m := range c.Methods {
						if m.Name == slot.Name+"_value" {
							slot = m
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
			args, ok := l.callArgs(n, n.Arguments(), slot.Params)
			if !ok {
				return nil
			}
			call := &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), Name: slot.Name, Type: slot.Result, X: recv, Args: args}
			if !slot.Result.Equal(hm.Result) && hm.Result.Kind != hir.Void {
				return &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: hm.Result, X: call}
			}
			return call
		}
		// A receiver of a vendored but not-lowered class goes through a cast
		// view interface grown for exactly this member.
		if recvCast, ok := l.viewCast(p.Expression, name); ok {
			args, ok2 := l.callArgsMethod(n, n.Arguments(), l.methodsBy[recvCast.Type.Name+"."+name], 0)
			if !ok2 {
				return nil
			}
			return &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), Name: name,
				Type: l.methodsBy[recvCast.Type.Name+"."+name].Result, X: recvCast, Args: args}
		}
		// Library operation, dispatched on the lowered receiver type.
		recv := l.expr(p.Expression)
		if recv == nil {
			return nil
		}
		if recv.Type.Kind == hir.Optional {
			recv = &hir.Expr{Kind: hir.Narrow, Node: l.node(p.Expression), Type: recv.Type.Args[0], X: recv}
		}
		var method *hir.Method
		if recv.Type.Kind == hir.InterfaceRef {
			for _, iface := range l.out.Interfaces {
				if iface.Name == recv.Type.Name {
					for _, m := range iface.Methods {
						if m.Name == name {
							method = m
							break
						}
					}
				}
			}
		}
		if recv.Type.Kind == hir.ClassRef {
			for owner := recv.Type.Name; owner != ""; {
				var c *hir.Class
				for _, candidate := range l.out.Classes {
					if candidate.Name == owner {
						c = candidate
						break
					}
				}
				if c == nil {
					break
				}
				for _, m := range c.Methods {
					if m.Name == name {
						method = m
						break
					}
				}
				if method != nil {
					break
				}
				owner = c.Super
			}
		}
		if method != nil {
			args, ok := l.callArgsMethod(n, n.Arguments(), method, 0)
			if !ok {
				return nil
			}
			return &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), Type: method.Result, Name: name, X: recv, Args: args}
		}
		x, handled := l.libraryCall(n, name, recv)
		if x == nil && !handled {
			l.diagf(n, "unsupported-call", "method %s on %s is not lowered", name, recv.Type.Kind)
		}
		return x
	}
	l.diagf(n, "unsupported-call", "call to %s is not lowered", callee.Kind.String())
	return nil
}

func (l *lowerer) libraryCall(n *ast.Node, name string, recv *hir.Expr) (*hir.Expr, bool) {
	args := n.Arguments()
	one := func() *hir.Expr {
		if len(args) != 1 {
			return nil
		}
		return l.expr(args[0])
	}
	two := func() (*hir.Expr, *hir.Expr) {
		if len(args) != 2 {
			return nil, nil
		}
		return l.expr(args[0]), l.expr(args[1])
	}
	i32, str, boolT := hir.T(hir.I32), hir.T(hir.String), hir.T(hir.Bool)
	switch recv.Type.Kind {
	case hir.String:
		switch name {
		case "includes":
			if len(args) != 1 {
				l.diagf(n, "unsupported-call", "string.includes currently requires one search string")
				return nil, true
			}
			search := l.expr(args[0])
			if search == nil {
				return nil, true
			}
			if search.Type.Kind != hir.String {
				l.diagf(n, "unsupported-call", "string.includes requires a string search value")
				return nil, true
			}
			index := l.rtOp("string.indexOf", recv, i32, search)
			return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Type: boolT, Op: ">=", X: index, Y: hir.L(hir.T(hir.Number), float64(0))}, true
		case "charAt":
			return l.rtOp("string.charAt", recv, str, one()), true
		case "charCodeAt":
			return l.rtOp("string.charCodeAt", recv, i32, one()), true
		case "substring":
			if len(args) == 1 {
				recv = l.tempInit(n, recv.Type, recv)
				return l.rtOp("string.substring", recv, str, l.expr(args[0]), l.rtOp("string.length", recv, i32)), true
			}
			a, b := two()
			return l.rtOp("string.substring", recv, str, a, b), true
		case "lastIndexOf":
			if len(args) != 1 {
				return nil, false
			}
			return l.stringLastIndexOf(n, recv, args[0]), true
		case "substr":
			if len(args) == 1 {
				recv = l.tempInit(n, recv.Type, recv)
				return l.rtOp("string.substr", recv, str, l.expr(args[0]), l.rtOp("string.length", recv, i32)), true
			}
			a, b := two()
			return l.rtOp("string.substr", recv, str, a, b), true
		case "trim":
			return l.rtOp("string.trim", recv, str), true
		case "toUpperCase":
			return l.rtOp("string.toUpperCase", recv, str), true
		case "concat":
			return l.rtOp("string.concat", recv, str, one()), true
		case "replace", "replaceAll":
			return l.replaceCall(n, recv, args), true
		case "startsWith":
			return l.rtOp("string.startsWith", recv, boolT, one()), true
		case "endsWith":
			return l.rtOp("string.endsWith", recv, boolT, one()), true
		case "indexOf":
			return l.rtOp("string.indexOf", recv, i32, one()), true
		case "toLowerCase":
			return l.rtOp("string.toLowerCase", recv, str), true
		case "split":
			return l.rtOp("string.split", recv, hir.T(hir.Array, str), one()), true
		case "toString":
			return recv, true
		}
	case hir.Number:
		if name == "toString" {
			if len(args) != 0 {
				l.diagf(n, "unsupported-call", "number.toString arguments are not lowered")
				return nil, true
			}
			return l.rtOp("number.toString", recv, str), true
		}
	case hir.RegExp:
		switch name {
		case "test":
			return l.rtOp("regexp.test", recv, boolT, one()), true
		}
	case hir.Array:
		switch name {
		case "reverse":
			if len(args) != 0 {
				return nil, false
			}
			return l.rtOp("array.reverse", recv, recv.Type), true
		case "push":
			if len(args) == 1 && args[0].Kind == ast.KindSpreadElement {
				l.diagf(n, "note-spread-push", "push(...x) inlined into a loop")
				return l.spreadPush(n, recv, args[0]), true
			}
			if len(args) != 1 {
				return nil, false
			}
			old := l.hint
			l.hint = recv.Type.Args[0]
			value := l.expr(args[0])
			l.hint = old
			if value == nil {
				return nil, true
			}
			return l.rtOp("array.push", recv, i32, l.coerce(value, recv.Type.Args[0])), true
		case "length":
			return l.rtOp("array.length", recv, i32), true
		case "concat":
			return l.rtOp("array.concat", recv, recv.Type, one()), true
		case "slice":
			a := one()
			if a == nil {
				a, b := two()
				if a == nil {
					return l.rtOp("array.slice0", recv, recv.Type), true
				}
				return l.rtOp("array.slice2", recv, recv.Type, a, b), true
			}
			return l.rtOp("array.slice1", recv, recv.Type, a), true
		case "splice":
			if len(args) < 1 || len(args) > 3 {
				return nil, false
			}
			lowered := make([]*hir.Expr, 0, len(args))
			for _, arg := range args {
				x := l.expr(arg)
				if x == nil {
					return nil, true
				}
				lowered = append(lowered, x)
			}
			return l.rtOp("array.splice"+itoa(len(args)), recv, recv.Type, lowered...), true
		case "pop", "shift":
			return l.rtOp("array."+name, recv, hir.T(hir.Optional, recv.Type.Args[0])), true
		case "indexOf":
			return l.rtOp("array.indexOf", recv, i32, one()), true
		case "includes":
			return l.rtOp("array.includes", recv, boolT, one()), true
		case "join":
			sep := &hir.Expr{Kind: hir.Lit, Type: hir.T(hir.Optional, str)}
			if len(args) == 1 {
				sep = &hir.Expr{Kind: hir.Lit, Type: hir.T(hir.Optional, str), Value: nil}
				l.hint = hir.T(hir.Optional, str)
				x := l.expr(args[0])
				l.hint = hir.Type{}
				if x != nil {
					sep = x
				}
			}
			return l.rtOp("array.join", recv, str, sep), true
		}
	case hir.OrderedSet:
		switch name {
		case "has":
			return l.rtOp("set.has", recv, boolT, one()), true
		case "add":
			return l.rtOp("set.add", recv, recv.Type, one()), true
		case "size":
			return l.rtOp("set.size", recv, i32), true
		case "copy":
			return l.rtOp("set.copy", recv, recv.Type, one()), true
		}
	case hir.OrderedMap:
		switch name {
		case "get":
			return l.rtOp("map.get", recv, hir.T(hir.Optional, recv.Type.Args[1]), one()), true
		case "set":
			a, b := two()
			return l.rtOp("map.set", recv, recv.Type, a, b), true
		case "has":
			return l.rtOp("map.has", recv, boolT, one()), true
		case "size":
			return l.rtOp("map.size", recv, i32), true
		}
	}
	return nil, false
}

// replaceCall lowers s.replace(pattern, with): a RegExp (literal or
// constructed) maps to the regex-aware replace; a single-character literal
// pattern with the global flag keeps the phase-1 string.replaceAll mapping.
func (l *lowerer) replaceCall(n *ast.Node, recv *hir.Expr, args []*ast.Node) *hir.Expr {
	if len(args) != 2 {
		l.diagf(n, "unsupported-regex", "replace needs a pattern and a replacement")
		return nil
	}
	if (args[0].Kind == ast.KindRegularExpressionLiteral && args[1].Kind != ast.KindStringLiteral && args[1].Kind != ast.KindNoSubstitutionTemplateLiteral) || ((args[1].Kind == ast.KindStringLiteral || args[1].Kind == ast.KindNoSubstitutionTemplateLiteral) && strings.Contains(args[1].Text(), "$")) {
		l.diagf(args[1], "unsupported-regex", "replace requires a literal replacement without dollar substitutions")
		return nil
	}
	with := l.expr(args[1])
	if with == nil {
		return nil
	}
	if args[0].Kind == ast.KindRegularExpressionLiteral {
		if args[0].Text() == `/\r/g` && args[1].Kind == ast.KindStringLiteral && !strings.Contains(args[1].Text(), "$") {
			l.diagf(n, "note-regex-mapped", "literal global carriage-return replacement")
			return l.rtOp("string.replaceAll", recv, hir.T(hir.String), hir.L(hir.T(hir.String), "\r"), with)
		}
		if (args[0].Text() == `/^/g` || args[0].Text() == `/$/g`) || strings.Contains(args[0].Text(), `\0`) {
			l.diagf(n, "unsupported-regex", "anchor or octal replacement semantics are not lowered")
			return nil
		}
		pattern := l.regexLiteral(args[0])
		if pattern == nil {
			return nil
		}
		return l.rtOp("string.replaceRegex", recv, hir.T(hir.String), pattern, with)
	}
	// A dynamic pattern (new RegExp(...)): map through the runtime replace.
	pat := l.expr(args[0])
	if pat != nil && pat.Type.Kind == hir.RegExp {
		return l.rtOp("string.replaceRegex", recv, hir.T(hir.String), pat, with)
	}
	if args[0].Kind == ast.KindStringLiteral {
		needle, ok := l.simpleRegexText(args[0].Text())
		if ok {
			l.diagf(n, "note-regex-mapped", "string replace with literal %q mapped to replaceAll", needle)
			return l.rtOp("string.replaceAll", recv, hir.T(hir.String), hir.L(hir.T(hir.String), needle), with)
		}
	}
	l.diagf(n, "unsupported-regex", "replace with a non-literal pattern is not lowered")
	return nil
}

// simpleRegexText accepts a plain string pattern (any characters, used as a
// literal needle).
func (l *lowerer) simpleRegexText(text string) (string, bool) {
	return text, true
}

// simpleRegex accepts literal patterns with the global flag and decodes escapes.
func (l *lowerer) simpleRegex(n *ast.Node) (string, bool) {
	text := n.Text() // /pattern/flags
	slash := strings.LastIndex(text[1:], "/")
	if slash < 0 {
		l.diagf(n, "unsupported-regex", "malformed regular expression %s", text)
		return "", false
	}
	pattern := text[1 : 1+slash]
	flags := text[2+slash:]
	if flags != "g" {
		l.diagf(n, "unsupported-regex", "regular expression /%s/%s: only the global flag is mapped", pattern, flags)
		return "", false
	}
	var decoded strings.Builder
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' {
			i++
			if i >= len(pattern) {
				break
			}
			if ch, ok := regexCharEscapes[pattern[i]]; ok && !(pattern[i] == '0' && i+1 < len(pattern) && pattern[i+1] >= '0' && pattern[i+1] <= '9') {
				decoded.WriteString(ch)
				continue
			}
		} else if !strings.ContainsRune(".^$*+?()[]{}|", rune(pattern[i])) {
			decoded.WriteByte(pattern[i])
			continue
		}
		l.diagf(n, "unsupported-regex", "regular expression /%s/g is not a literal pattern", pattern)
		return "", false
	}
	if decoded.Len() == 0 {
		l.diagf(n, "unsupported-regex", "empty regular expression is not lowered")
		return "", false
	}
	l.diagf(n, "note-regex-mapped", "regex /%s/g mapped to string.replaceAll", pattern)
	return decoded.String(), true
}

var regexCharEscapes = map[byte]string{
	'n': "\n", 'r': "\r", 't': "\t", 'v': "\v", 'f': "\f", '0': "\x00",
	'\\': "\\", '/': "/", '.': ".", '*': "*", '+': "+", '?': "?", '(': "(", ')': ")",
	'[': "[", ']': "]", '{': "{", '}': "}", '^': "^", '$': "$", '|': "|", '-': "-",
}

// callArgs lowers arguments against the lowered parameter list. A trailing
// variadic parameter collects the remaining arguments (or a spread); omitted
// optional parameters pass undefined.
func (l *lowerer) callArgs(at *ast.Node, args []*ast.Node, params []hir.Param) ([]*hir.Expr, bool) {
	m := &hir.Method{Params: params}
	return l.callArgsMethod(at, args, m, 0)
}

// newExpression lowers allocations of lowered classes; `new Set([..])` is
// only allowed in statement contexts (see collectionInit).
func (l *lowerer) newExpression(n *ast.Node) *hir.Expr {
	if x, ok := l.newRegExp(n); ok {
		return x
	}
	sym := l.resolve(n.Expression())
	if sym != nil && sym.Name == "Error" && l.classOf(sym) == nil {
		f := l.fileOfSymbol(sym)
		if f != nil && strings.Contains(f.FileName(), "lib.") {
			if len(n.Arguments()) > 1 {
				l.diagf(n, "unsupported-new", "Error accepts at most one message in this lowering")
				return nil
			}
			message := hir.L(hir.T(hir.String), "")
			if len(n.Arguments()) == 1 {
				message = l.expr(n.Arguments()[0])
			}
			if message == nil {
				return nil
			}
			return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(l.builtinError()), Args: []*hir.Expr{l.coerce(message, hir.T(hir.Optional, hir.T(hir.String)))}}
		}
	}
	// Construct through descriptor-valued locals as well as computed accesses.
	if l.classOf(sym) == nil && (sym == nil || sym.Name != "Set" && sym.Name != "Map" && sym.Name != "Array") {
		cv := l.expr(n.Expression())
		if cv != nil && cv.Type.Kind == hir.Dynamic {
			cv = l.rtOp("dynamic.asClassValue", cv, hir.T(hir.ClassValue))
		}
		if cv != nil && cv.Type.Kind == hir.Optional && cv.Type.Args[0].Kind == hir.ClassValue {
			cv = &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: hir.T(hir.ClassValue), X: cv}
		}
		if cv != nil && cv.Type.Kind == hir.ClassValue && len(n.Arguments()) == 0 {
			result := l.mapCheckerType(n, l.ck.GetTypeAtLocation(n))
			if result.Kind == hir.Dynamic {
				result = hir.Ref(hir.RootObject)
			}
			if l.hint.Kind == hir.ClassRef || l.hint.Kind == hir.InterfaceRef {
				result = l.hint
			}
			return l.rtOp("classvalue.new", cv, result)
		}
	}
	// Context supplies the element types of an empty collection before
	// trying explicit arguments (which otherwise diagnose missing arguments).
	if sym != nil && l.librarySymbol(sym) && len(n.Arguments()) == 0 && len(n.TypeArguments()) == 0 {
		kind := hir.Void
		switch sym.Name {
		case "Set":
			kind = hir.OrderedSet
		case "Map":
			kind = hir.OrderedMap
		case "Array":
			kind = hir.Array
		}
		if kind != hir.Void {
			typ := l.hint
			if typ.Kind == hir.Optional {
				typ = typ.Args[0]
			}
			if typ.Kind != kind {
				if context := l.ck.GetContextualType(n, 0); context != nil {
					typ = l.mapCheckerType(n, context)
				}
			}
			if typ.Kind == kind {
				return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ}
			}
		}
	}
	// Empty library collections: new Set<T>(), new Map<K,V>().
	if t := n.Expression(); t != nil && t.Kind == ast.KindIdentifier && len(n.Arguments()) == 0 {
		switch t.Text() {
		case "Set":
			if elem, ok := l.typeArgAt(n, 0); ok {
				l.diagf(n, "note-empty-set", "new Set<T>() lowered to an empty ordered set")
				typ := hir.T(hir.OrderedSet, elem)
				return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ}
			}
		case "Map":
			k, ok1 := l.typeArgAt(n, 0)
			v, ok2 := l.typeArgAt(n, 1)
			if ok1 && ok2 {
				l.diagf(n, "note-empty-map", "new Map<K,V>() lowered to an empty ordered map")
				typ := hir.T(hir.OrderedMap, k, v)
				return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ}
			}
		}
	}
	// new Set(existingSet): a copy.
	if t := n.Expression(); t != nil && t.Kind == ast.KindIdentifier && t.Text() == "Set" {
		args := n.Arguments()
		if len(args) == 1 {
			a := l.expr(args[0])
			if a != nil && a.Type.Kind == hir.OrderedSet {
				l.diagf(n, "note-set-copy", "new Set(set) lowered to a copy")
				return l.rtOp("set.copy", a, a.Type, a)
			}
		}
	}
	// new Array<T>(n): n still-undefined slots.
	if t := n.Expression(); t != nil && t.Kind == ast.KindIdentifier && t.Text() == "Array" {
		elem, ok := l.typeArgAt(n, 0)
		if ok && len(n.Arguments()) == 1 {
			sz := l.indexValue(l.expr(n.Arguments()[0]))
			if sz != nil && sz.Type.Kind == hir.I32 {
				l.diagf(n, "note-array-size", "new Array<T>(n) lowered to n undefined slots")
				typ := hir.T(hir.Array, elem)
				return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ, Args: []*hir.Expr{sz}}
			}
		}
	}
	c := l.classOf(l.resolve(n.Expression()))
	if c == nil {
		name := "?"
		if n.Expression().Kind == ast.KindIdentifier {
			name = n.Expression().Text()
		}
		l.diagf(n, "unsupported-new", "new %s is not a lowered class", name)
		return nil
	}
	if c.Abstract {
		l.diagf(n, "unsupported-new", "new abstract class %s", c.Name)
		return nil
	}
	x := &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(c.Name)}
	ctor := l.constructorOf(c)
	if ctor == nil {
		if len(n.Arguments()) > 0 {
			l.diagf(n, "unsupported-new", "new %s with arguments but no lowered constructor", c.Name)
			return nil
		}
		return x
	}
	args, ok := l.callArgs(n, n.Arguments(), ctor.Params)
	if !ok {
		return nil
	}
	x.Args = args
	return x
}

// constructorOf resolves c's own or inherited constructor.
func (l *lowerer) constructorOf(c *hir.Class) *hir.Method {
	for name := c.Name; name != ""; {
		var found *hir.Class
		for _, x := range l.out.Classes {
			if x.Name == name {
				found = x
				break
			}
		}
		if found == nil {
			return nil
		}
		if found.Ctor != nil {
			return found.Ctor
		}
		name = found.Super
	}
	return nil
}

func (l *lowerer) binary(n *ast.Node) *hir.Expr {
	b := n.AsBinaryExpression()
	op := b.OperatorToken.Kind
	switch op {
	case ast.KindEqualsEqualsEqualsToken, ast.KindEqualsEqualsToken:
		return l.equality(n, b, false)
	case ast.KindExclamationEqualsEqualsToken, ast.KindExclamationEqualsToken:
		return l.equality(n, b, true)
	case ast.KindAmpersandAmpersandToken, ast.KindBarBarToken:
		if !l.ck.GetTypeAtLocation(n).IsBooleanLike() {
			x := l.expr(b.Left)
			if x == nil {
				return nil
			}
			x = l.tempInit(n, x.Type, x)
			hint := l.hint
			base := x.Type
			if base.Kind == hir.Optional {
				base = base.Args[0]
			}
			if base.Kind == hir.Array && b.Right.Kind == ast.KindArrayLiteralExpression && len(b.Right.AsArrayLiteralExpression().Elements.Nodes) == 0 {
				l.hint = base
			}
			y := l.expr(b.Right)
			l.hint = hint
			if y == nil {
				return nil
			}
			var typ hir.Type
			if base.Kind == hir.Array && base.Equal(y.Type) {
				typ = base
			} else {
				typ = l.mapCheckerType(n, l.ck.GetTypeAtLocation(n))
			}
			if typ.Kind == hir.Dynamic {
				l.diagf(n, "unsupported-shortcircuit", "mixed tagged operands require value-preserving truthiness")
				return nil
			}
			condition := &hir.Expr{Kind: hir.ToBoolean, Type: hir.T(hir.Bool), X: x}
			left, right := l.coerce(x, typ), l.coerce(y, typ)
			if x.Type.Kind == hir.Optional && x.Type.Args[0].Equal(typ) && op == ast.KindBarBarToken {
				left = &hir.Expr{Kind: hir.Narrow, Type: typ, X: x}
			}
			if !l.acceptsType(typ, left.Type) || !l.acceptsType(typ, right.Type) {
				l.diagf(n, "unsupported-shortcircuit", "operands cannot preserve the result type %s", typ.String())
				return nil
			}
			if op == ast.KindAmpersandAmpersandToken {
				left, right = right, left
			}
			return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: typ, X: condition, Y: left, Z: right}
		}
		hop := "&&"
		if op == ast.KindBarBarToken {
			hop = "||"
		}
		x, y := l.condition(b.Left), l.condition(b.Right)
		if x == nil || y == nil {
			return nil
		}
		return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: hop, Type: hir.T(hir.Bool), X: x, Y: y}
	case ast.KindLessThanToken, ast.KindLessThanEqualsToken, ast.KindGreaterThanToken, ast.KindGreaterThanEqualsToken:
		x, y := l.expr(b.Left), l.expr(b.Right)
		if x == nil || y == nil {
			return nil
		}
		cop := map[ast.Kind]string{
			ast.KindLessThanToken: "<", ast.KindLessThanEqualsToken: "<=",
			ast.KindGreaterThanToken: ">", ast.KindGreaterThanEqualsToken: ">="}[op]
		if x.Type.Kind == hir.Optional || y.Type.Kind == hir.Optional {
			return l.optionalRelation(n, cop, x, y)
		}
		return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: cop, Type: hir.T(hir.Bool), X: x, Y: y}
	case ast.KindPlusToken:
		if l.ck.GetTypeAtLocation(n).IsStringLike() {
			x, y := l.expr(b.Left), l.expr(b.Right)
			if x == nil || y == nil {
				return nil
			}
			x, y = l.primitiveString(n, x), l.primitiveString(n, y)
			if x == nil || y == nil {
				return nil
			}
			return l.rtOp("string.concat", x, hir.T(hir.String), y)
		}
		return l.arithmetic(n, b, "+")
	case ast.KindMinusToken:
		return l.arithmetic(n, b, "-")
	case ast.KindAsteriskToken:
		return l.arithmetic(n, b, "*")
	case ast.KindPercentToken:
		if b.Right.Kind == ast.KindNumericLiteral && b.Right.Text() == "2" {
			x := l.expr(b.Left)
			if x != nil {
				return l.rtOp("number.remainder2", x, hir.T(hir.Number))
			}
			return nil
		}
		l.diagf(n, "unsupported-number", "Number remainder requires the literal divisor 2 in this phase")
		return nil
	case ast.KindSlashToken:
		l.diagf(n, "unsupported-number", "division is not lowered in this phase")
		return nil
	case ast.KindInstanceOfKeyword:
		return l.instanceOf(n)
	case ast.KindQuestionQuestionToken:
		x := l.expr(b.Left)
		if x == nil {
			return nil
		}
		if x.Type.Kind != hir.Optional && x.Type.Kind != hir.Dynamic {
			l.diagf(n, "unsupported-expr", "?? needs an optional or tagged left operand")
			return nil
		}
		x = l.tempInit(n, x.Type, x)
		base := x.Type
		if base.Kind == hir.Optional {
			base = base.Args[0]
		}
		hint := l.hint
		l.hint = base
		y := l.expr(b.Right)
		l.hint = hint
		if y == nil {
			return nil
		}
		left := x
		if x.Type.Kind == hir.Optional {
			left = &hir.Expr{Kind: hir.Narrow, Type: base, X: x}
		}
		result := y.Type
		if !base.Equal(y.Type) {
			result = l.mapCheckerType(n, l.ck.GetTypeAtLocation(n))
		}
		if result.Kind == hir.Void {
			return nil
		}
		test := &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: x}
		if base.Kind == hir.Dynamic {
			test = l.rtOp("dynamic.isNullish", left, hir.T(hir.Bool))
		}
		return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: result, X: test, Y: l.coerce(y, result), Z: l.coerce(left, result)}

	case ast.KindEqualsToken, ast.KindPlusEqualsToken, ast.KindMinusEqualsToken, ast.KindAsteriskEqualsToken:
		l.diagf(n, "unsupported-expr", "assignment inside an expression is not lowered")
		return nil
	}
	l.diagf(n, "unsupported-expr", "binary operator %s is not lowered", op.String())
	return nil
}

// Undefined converts to NaN in a relational comparison, so every relational
// operator returns false when either optional primitive is absent. Capture
// both operands before testing the tags: the RHS still executes in that case.
// Mixed primitive coercion remains blocking until its JS semantics are present.
func (l *lowerer) optionalRelation(n *ast.Node, op string, x, y *hir.Expr) *hir.Expr {
	base := func(t hir.Type) hir.Type {
		if t.Kind == hir.Optional {
			return t.Args[0]
		}
		return t
	}
	t := base(x.Type)
	if !t.Equal(base(y.Type)) || (t.Kind != hir.Number && t.Kind != hir.String) {
		l.diagf(n, "unsupported-optional-comparison", "mixed optional comparison requires JavaScript primitive coercion")
		return nil
	}
	x = l.tempInit(n, x.Type, x)
	y = l.tempInit(n, y.Type, y)
	var guards []*hir.Expr
	narrow := func(v *hir.Expr) *hir.Expr {
		if v.Type.Kind != hir.Optional {
			return v
		}
		guards = append(guards, &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: v}})
		return &hir.Expr{Kind: hir.Narrow, Type: t, X: v}
	}
	result := &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: op, Type: hir.T(hir.Bool), X: narrow(x), Y: narrow(y)}
	for i := len(guards) - 1; i >= 0; i-- {
		result = &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "&&", Type: hir.T(hir.Bool), X: guards[i], Y: result}
	}
	return result
}

func (l *lowerer) arithmetic(n *ast.Node, b *ast.BinaryExpression, op string) *hir.Expr {
	x, y := l.expr(b.Left), l.expr(b.Right)
	if x == nil || y == nil {
		return nil
	}
	return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: op, Type: x.Type, X: x, Y: y}
}

// instanceOf lowers `x instanceof C` to the boolean ancestry test; C may be
// a dynamic class value (a constructor passed around).
func (l *lowerer) instanceOf(n *ast.Node) *hir.Expr {
	b := n.AsBinaryExpression()
	x := l.expr(b.Left)
	if x == nil {
		return nil
	}
	if c := l.classOf(l.resolve(b.Right)); c != nil {
		return &hir.Expr{Kind: hir.InstanceOf, Node: l.node(n), Type: hir.T(hir.Bool), X: x, Owner: c.Name}
	}
	cv := l.expr(b.Right)
	if cv != nil && cv.Type.Kind == hir.ClassValue {
		return &hir.Expr{Kind: hir.InstanceOf, Node: l.node(n), Type: hir.T(hir.Bool), X: x, Y: cv}
	}
	l.diagf(n, "unsupported-expr", "instanceof needs a lowered class operand")
	return nil
}

// equality lowers === and !==, mapping comparisons with undefined to the
// explicit IsUndefined node.
func (l *lowerer) equality(n *ast.Node, b *ast.BinaryExpression, negated bool) *hir.Expr {
	// `typeof x === "string"` dispatches on the tagged box.
	for _, pair := range [2][2]*ast.Node{{b.Left, b.Right}, {b.Right, b.Left}} {
		operand, literal := pair[0], pair[1]
		if operand == nil || literal == nil || operand.Kind != ast.KindTypeOfExpression {
			continue
		}
		if literal.Kind == ast.KindStringLiteral && operand.Kind == ast.KindTypeOfExpression {
			return l.typeofCompare(n, operand.AsTypeOfExpression().Expression, literal.Text(), negated)
		}
	}
	leftUndef := l.isUndefinedType(b.Left)
	rightUndef := l.isUndefinedType(b.Right)
	x, y := l.expr(b.Left), l.expr(b.Right)
	if x == nil || y == nil {
		return nil
	}
	loose := b.OperatorToken.Kind == ast.KindEqualsEqualsToken || b.OperatorToken.Kind == ast.KindExclamationEqualsToken
	if leftUndef != rightUndef {
		// Preserve both operand effects even for a statically undefined call.
		x, y = l.tempInit(n, x.Type, x), l.tempInit(n, y.Type, y)
		operand := x
		if leftUndef {
			operand = y
		}
		test := hir.L(hir.T(hir.Bool), false)
		if operand.Type.Kind == hir.Optional || operand.Type.IsRef() {
			test = &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: operand}
			base := operand.Type
			if base.Kind == hir.Optional {
				base = base.Args[0]
			}
			if loose && base.Kind == hir.Dynamic {
				test = l.rtOp("dynamic.isNullish", l.coerce(operand, hir.T(hir.Dynamic)), hir.T(hir.Bool))
			}
		}
		if negated {
			return &hir.Expr{Kind: hir.Unary, Node: l.node(n), Op: "!", Type: hir.T(hir.Bool), X: test}
		}
		return test
	}
	tagged := func(t hir.Type) bool {
		return t.Kind == hir.Dynamic || t.Kind == hir.Optional && t.Args[0].Kind == hir.Dynamic
	}
	if tagged(x.Type) || tagged(y.Type) {
		if loose {
			l.diagf(n, "unsupported-dynamic-comparison", "loose tagged equality requires JavaScript coercion")
			return nil
		}
		test := l.rtOp("dynamic.strictEquals", l.coerce(x, hir.T(hir.Dynamic)), hir.T(hir.Bool), l.coerce(y, hir.T(hir.Dynamic)))
		if negated {
			return &hir.Expr{Kind: hir.Unary, Node: l.node(n), Op: "!", Type: hir.T(hir.Bool), X: test}
		}
		return test
	}

	if x.Type.Kind == hir.Optional && x.Type.Args[0].Equal(y.Type) {
		y = l.coerce(y, x.Type)
	}
	if y.Type.Kind == hir.Optional && y.Type.Args[0].Equal(x.Type) {
		x = l.coerce(x, y.Type)
	}
	if !x.Type.Equal(y.Type) && x.Type.IsRef() && y.Type.IsRef() {
		if l.acceptsType(x.Type, y.Type) {
			y = &hir.Expr{Kind: hir.Cast, Type: x.Type, X: y}
		} else if l.acceptsType(y.Type, x.Type) {
			x = &hir.Expr{Kind: hir.Cast, Type: y.Type, X: x}
		}
	}
	op := "=="
	if negated {
		op = "!="
	}
	return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: op, Type: hir.T(hir.Bool), X: x, Y: y}
}

func (l *lowerer) isUndefinedType(n *ast.Node) bool {
	t := l.ck.GetTypeAtLocation(n)
	return t != nil && t.Flags()&checker.TypeFlagsUndefined != 0
}

func (l *lowerer) prefixUnary(n *ast.Node) *hir.Expr {
	u := n.AsPrefixUnaryExpression()
	switch u.Operator {
	case ast.KindExclamationToken:
		x := l.condition(u.Operand)
		if x == nil {
			return nil
		}
		return &hir.Expr{Kind: hir.Unary, Node: l.node(n), Op: "!", Type: hir.T(hir.Bool), X: x}
	case ast.KindMinusToken:
		if u.Operand != nil && u.Operand.Kind == ast.KindNumericLiteral {
			return l.numericLiteral(u.Operand, hir.T(hir.Number), -1)
		}
		x := l.expr(u.Operand)
		if x == nil {
			return nil
		}
		return &hir.Expr{Kind: hir.Unary, Node: l.node(n), Op: "-", Type: x.Type, X: x}
	}
	l.diagf(n, "unsupported-expr", "unary %s is not lowered", u.Operator.String())
	return nil
}

// asExpression keeps the checker's proof: the operand is lowered with its
// natural type and viewed as the asserted type. A Dynamic operand unboxes;
// a cast to a vendored but not-lowered class becomes a lazily grown view
// interface through an unchecked Cast.
func (l *lowerer) asExpression(n *ast.Node) *hir.Expr {
	if n.Type() != nil && n.Type().Kind == ast.KindTypeReference && typeRefName(n.Type()) == "const" {
		return l.expr(n.Expression())
	}
	x := l.expr(n.Expression())
	if x == nil {
		return nil
	}
	t := l.ck.GetTypeAtLocation(n)
	if t == nil {
		return x
	}
	before := len(l.diags)
	mapped := l.mapCheckerType(n, t)
	if hasBlocking(l.diags[before:]) {
		l.diags = l.diags[:before]
		return x
	}
	switch {
	case x.Type.Kind == hir.Dynamic && mapped.Kind == hir.String:
		return l.rtOp("dynamic.asString", x, hir.T(hir.String))
	case x.Type.Kind == hir.Dynamic && mapped.Kind == hir.ClassValue:
		return l.rtOp("dynamic.asClassValue", x, hir.T(hir.ClassValue))
	case x.Type.Kind == hir.Dynamic && (mapped.Kind == hir.ClassRef || mapped.Kind == hir.InterfaceRef):
		return l.rtOp("dynamic.asRef", x, mapped)
	case (mapped.Kind == hir.ClassRef || mapped.Kind == hir.InterfaceRef) && !mapped.Equal(x.Type):
		if mapped.Kind == hir.ClassRef && x.Type.Kind == hir.ClassRef && l.classesByQualifiedName(mapped.Name) != nil {
			l.diagf(n, "unsupported-assertion", "type assertion requires checker-proven narrowing of its operand")
			return nil
		}
		l.diagf(n, "note-cast", "cast to %s recorded as an unchecked view", mapped.String())
		return &hir.Expr{Kind: hir.Cast, Node: l.node(n), Type: mapped, X: x}
	}
	return x
}

// objectLiteral lowers { ... } whose contextual type is a named alias shape,
// by constructing the synthesized class; spread properties copy the fields of
// the spread value first.
func (l *lowerer) objectLiteral(n *ast.Node) *hir.Expr {
	hint := l.hint
	if hint.Kind == hir.Optional && len(hint.Args) == 1 {
		hint = hint.Args[0]
	}
	// A record hint builds a map: spreads copy entries, keys set.
	if hint.Kind == hir.OrderedMap {
		return l.recordLiteral(n, hint)
	}
	var target *hir.Class
	if hint.Kind == hir.ClassRef {
		for _, c := range l.out.Classes {
			if c.Name == hint.Name {
				target = c
				break
			}
		}
	}
	if target == nil {
		// Without a hint, derive the shape from the checker's type of the
		// literal (a synthesized shape class or a record map).
		if t := l.ck.GetTypeAtLocation(n); t != nil {
			before := len(l.diags)
			mapped := l.mapCheckerType(n, t)
			if len(l.diags) == before && mapped.Kind != hir.Void {
				l.hint = mapped
				target = shapeClassOf(l.out.Classes, mapped)
			} else {
				l.diags = l.diags[:before]
			}
		}
	}
	if target == nil {
		// An empty literal with a shape hint constructs the shape with
		// absent optionals; a synthesized checker shape otherwise.
		props := n.AsObjectLiteralExpression().Properties.Nodes
		if len(props) == 0 {
			if hint.Kind == hir.ClassRef {
				for _, c := range l.out.Classes {
					if c.Name != hint.Name || c.Ctor == nil {
						continue
					}
					args := []*hir.Expr{}
					ok := true
					for _, p := range c.Ctor.Params {
						if p.Type.Kind != hir.Optional {
							ok = false
							break
						}
						args = append(args, &hir.Expr{Kind: hir.Lit, Type: p.Type})
					}
					if ok {
						return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(c.Name), Args: args}
					}
				}
			}
			if t := l.ck.GetTypeAtLocation(n); t != nil && t.Symbol() == nil {
				if c := l.synthFromProperties(n, l.ck.GetPropertiesOfType(t)); c != nil {
					args := []*hir.Expr{}
					for _, p := range c.Ctor.Params {
						if p.Type.Kind != hir.Optional {
							return nil
						}
						args = append(args, &hir.Expr{Kind: hir.Lit, Type: p.Type})
					}
					return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(c.Name), Args: args}
				}
			}
		}
		l.diagf(n, "unsupported-type", "object literal needs a named lowered shape")
		return nil
	}
	ctor := target.Ctor
	props := n.AsObjectLiteralExpression().Properties.Nodes
	args := make([]*hir.Expr, len(ctor.Params))
	for i := range args {
		if ctor.Params[i].Type.Kind == hir.Optional {
			args[i] = &hir.Expr{Kind: hir.Lit, Type: ctor.Params[i].Type}
		}
	}
	for _, p := range props {
		var value *ast.Node
		var field string
		switch p.Kind {
		case ast.KindShorthandPropertyAssignment:
			value = p.Name()
			field = p.Name().Text()
		case ast.KindPropertyAssignment:
			value = p.Initializer()
			nameNode := propertyNameNode(p)
			if nameNode == nil {
				l.diagf(p, "unsupported-type", "object literal property without a name")
				return nil
			}
			if nameNode.Kind == ast.KindComputedPropertyName {
				// Constant computed keys (enum members) resolve through the
				// checker.
				key, ok := l.constantKeyName(nameNode)
				if !ok {
					l.diagf(p, "unsupported-type", "computed object literal key is not a constant")
					return nil
				}
				field = key
			} else {
				field = nameNode.Text()
			}
		case ast.KindSpreadAssignment:
			// Copy every field of the spread value into its parameter.
			sv := l.expr(p.Expression())
			if sv == nil || sv.Type.Kind != hir.ClassRef {
				l.diagf(p, "unsupported-type", "spread needs a shape value")
				return nil
			}
			l.diagf(p, "note-object-spread", "object spread copies the fields of %s", sv.Type.Name)
			for i, cp := range ctor.Params {
				for _, c := range l.out.Classes {
					if c.Name != sv.Type.Name {
						continue
					}
					for _, f := range c.Fields {
						if f.Name == cp.Name {
							args[i] = l.coerce(&hir.Expr{Kind: hir.FieldGet, Node: l.node(p), Name: f.Name, Type: f.Type, X: sv}, cp.Type)
						}
					}
				}
				// Fields the spread source lacks stay unset here; a later
				// explicit property (or the shape's optionals) fills them.
			}
			continue
		default:
			l.diagf(p, "unsupported-type", "object literal property %s is not lowered", p.Kind.String())
			return nil
		}
		idx := -1
		for i, cp := range ctor.Params {
			if cp.Name == field {
				idx = i
				break
			}
		}
		if idx < 0 {
			l.diagf(p, "unsupported-type", "object literal property %s is not in shape %s", field, target.Name)
			return nil
		}
		l.hint = ctor.Params[idx].Type
		a := l.expr(value)
		l.hint = hint
		if a == nil {
			return nil
		}
		args[idx] = l.coerce(a, ctor.Params[idx].Type)
	}
	for i, a := range args {
		if a == nil {
			l.diagf(n, "unsupported-type", "object literal misses field %s", ctor.Params[i].Name)
			return nil
		}
	}
	return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(target.Name), Args: args}
}

// shapeClassOf finds the lowered class of a shape type reference.
func shapeClassOf(classes []*hir.Class, t hir.Type) *hir.Class {
	if t.Kind != hir.ClassRef {
		return nil
	}
	for _, c := range classes {
		if c.Name == t.Name {
			return c
		}
	}
	return nil
}

func (l *lowerer) indexValue(x *hir.Expr) *hir.Expr {
	if x != nil && x.Type.Kind == hir.Number {
		return &hir.Expr{Kind: hir.RuntimeOp, Node: x.Node, Op: "number.index", Type: hir.T(hir.I32), X: x}
	}
	return x
}

// templateExpr uses cooked scanner literals and evaluates substitutions once,
// left to right. Object/array coercion requires JS ToPrimitive and is rejected.
func (l *lowerer) templateExpr(n *ast.Node) *hir.Expr {
	t := n.AsTemplateExpression()
	result := l.expr(t.Head)
	for _, span := range t.TemplateSpans.Nodes {
		part := l.expr(span.Expression())
		if part == nil {
			return nil
		}
		part = l.primitiveString(span, part)
		if part == nil {
			return nil
		}
		tail := l.expr(span.AsTemplateSpan().Literal)
		if result == nil || tail == nil {
			return nil
		}
		result = l.rtOp("string.concat", l.rtOp("string.concat", result, hir.T(hir.String), part), hir.T(hir.String), tail)
	}
	return result
}

// ToString for supported primitive values. Preserve evaluation at the operand
// position, even when an optional needs both a presence test and a payload read.
func (l *lowerer) primitiveString(at *ast.Node, x *hir.Expr) *hir.Expr {
	s := hir.T(hir.String)
	switch x.Type.Kind {
	case hir.String:
		return x
	case hir.Number:
		return l.rtOp("number.toString", x, s)
	case hir.I32:
		return l.rtOp("i32.toString", x, s)
	case hir.Bool:
		return &hir.Expr{Kind: hir.Conditional, Type: s, X: x, Y: hir.L(s, "true"), Z: hir.L(s, "false")}
	case hir.Dynamic:
		return l.rtOp("dynamic.toString", x, s)
	case hir.Optional:
		if x.Type.Args[0].IsRef() && x.Type.Args[0].Kind != hir.Dynamic {
			break
		}
		saved := l.pend
		l.pend = nil
		v := l.tempInit(at, x.Type, x)
		pre := l.pend
		l.pend = saved
		payload := l.primitiveString(at, &hir.Expr{Kind: hir.Narrow, Type: x.Type.Args[0], X: v})
		if payload == nil {
			return nil
		}
		return &hir.Expr{Kind: hir.Seq, Type: s, Stmt: hir.B(pre...), Y: &hir.Expr{Kind: hir.Conditional, Type: s,
			X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: v}, Y: hir.L(s, "undefined"), Z: payload}}
	}
	l.diagf(at, "unsupported-expr", "string coercion of %s requires JavaScript ToPrimitive", x.Type)
	return nil
}
