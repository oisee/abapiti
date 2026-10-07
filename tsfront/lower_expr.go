package tsfront

import (
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Expression lowering. Every expression is lowered to its natural HIR node
// first (locals and fields carry their declared type); the checker's type at
// the node is then compared with it, and flow narrowing (`if (x) ...`,
// `x === undefined` branches, dominating instanceof) becomes a hir.Narrow.

func (l *lowerer) rtOp(op string, x *hir.Expr, t hir.Type, args ...*hir.Expr) *hir.Expr {
	return &hir.Expr{Kind: hir.RuntimeOp, Node: x.Node, Op: op, Type: t, X: x, Args: args}
}

// condition lowers an expression used where a boolean is required, applying
// JavaScript truthiness where the checker type is not boolean.
func (l *lowerer) condition(n *ast.Node) *hir.Expr {
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
	x := l.naturalExpr(n)
	if x == nil {
		return nil
	}
	return l.narrowedAt(n, x)
}

// narrowedAt wraps x when the checker's type at the node is a narrower
// reference type than x's natural type. Mapping failures are quiet here: the
// natural type (from the declaration registry) stands.
func (l *lowerer) narrowedAt(n *ast.Node, x *hir.Expr) *hir.Expr {
	t := l.ck.GetTypeAtLocation(n)
	if t == nil {
		return x
	}
	before := len(l.diags)
	typ := l.mapCheckerType(n, t)
	if len(l.diags) != before {
		l.diags = l.diags[:before] // quiet: narrowing is optional
		return x
	}
	return l.narrowed(n, x, typ)
}

func (l *lowerer) narrowed(n *ast.Node, x *hir.Expr, typ hir.Type) *hir.Expr {
	if typ.Kind == hir.Void || x.Type.Equal(typ) {
		return x
	}
	if typ.Kind == hir.ClassRef || typ.Kind == hir.InterfaceRef {
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
		// tsgo's scanner stores string literals decoded; Text() is the value.
		return &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.String), Value: n.Text()}
	case ast.KindNumericLiteral:
		return l.numericLiteral(n, hir.T(hir.I32), 1)
	case ast.KindTrueKeyword:
		return hir.L(hir.T(hir.Bool), true)
	case ast.KindFalseKeyword:
		return hir.L(hir.T(hir.Bool), false)
	case ast.KindIdentifier:
		return l.identifier(n)
	case ast.KindThisKeyword:
		return l.this(l.class)
	case ast.KindParenthesizedExpression:
		return l.expr(n.Expression())
	case ast.KindPropertyAccessExpression:
		return l.propertyAccess(n)
	case ast.KindCallExpression:
		return l.call(n)
	case ast.KindNewExpression:
		return l.newExpression(n)
	case ast.KindBinaryExpression:
		return l.binary(n)
	case ast.KindPrefixUnaryExpression:
		return l.prefixUnary(n)
	case ast.KindConditionalExpression:
		c := n.AsConditionalExpression()
		x := l.condition(c.Condition)
		y, z := l.expr(c.WhenTrue), l.expr(c.WhenFalse)
		if x == nil || y == nil || z == nil {
			return nil
		}
		return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: y.Type, X: x, Y: y, Z: z}
	case ast.KindAsExpression:
		return l.asExpression(n)
	case ast.KindNonNullExpression:
		return l.expr(n.Expression())
	case ast.KindObjectLiteralExpression:
		return l.objectLiteral(n)
	case ast.KindArrayLiteralExpression:
		if len(n.AsArrayLiteralExpression().Elements.Nodes) == 0 && l.hint.Kind == hir.Array {
			return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: l.hint}
		}
		l.diagf(n, "unsupported-array", "array literal outside a statement context")
		return nil
	case ast.KindRegularExpressionLiteral:
		l.diagf(n, "unsupported-regex", "regular expression outside replace is not lowered")
		return nil
	}
	l.diagf(n, "unsupported-expr", "%s is not lowered", n.Kind.String())
	return nil
}

func (l *lowerer) numericLiteral(n *ast.Node, typ hir.Type, sign float64) *hir.Expr {
	text := n.Text()
	if strings.ContainsAny(text, ".eE") {
		l.diagf(n, "unsupported-number", "numeric literal %s is not an i32 integer", text)
		return nil
	}
	v, err := strconv.ParseInt(text, 0, 64)
	if err != nil {
		l.diagf(n, "unsupported-number", "numeric literal %s: %v", text, err)
		return nil
	}
	l.checkNumberLiteral(n, float64(v)*sign)
	return hir.L(typ, int(v)*int(sign))
}

// identifier lowers a reference by resolving its symbol: locals and
// parameters first, then module values, then `undefined`.
func (l *lowerer) identifier(n *ast.Node) *hir.Expr {
	name := n.Text()
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
		l.diagf(n, "unsupported-expr", "undefined without an optional context")
		return nil
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
	if l.classOf(sym) != nil {
		l.diagf(n, "unsupported-class-value", "class %s used as a value", name)
		return nil
	}
	l.diagf(n, "unsupported-expr", "identifier %s is not a lowered local or module value", name)
	return nil
}

// propertyAccess lowers member reads; call targets are handled in call.
func (l *lowerer) propertyAccess(n *ast.Node) *hir.Expr {
	p := n.AsPropertyAccessExpression()
	sym := l.resolve(n)
	f, hasField := l.fieldOf(sym)
	if sym != nil && hasField {
		if p.Expression.Kind == ast.KindThisKeyword {
			return &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: f.Name, Type: f.Type, X: l.this(l.class)}
		}
		if p.Expression.Kind == ast.KindSuperKeyword {
			l.diagf(n, "unsupported-expr", "super field access is not lowered")
			return nil
		}
		recv := l.expr(p.Expression)
		if recv == nil {
			return nil
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
	// Library property on a primitive or collection receiver.
	recv := l.expr(p.Expression)
	if recv == nil {
		return nil
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
	}
	l.diagf(n, "unsupported-expr", "property %s is not lowered", n.Name().Text())
	return nil
}

// call lowers method calls: lowered class methods (virtual and super) and
// the library operations on strings, numbers and collections.
func (l *lowerer) call(n *ast.Node) *hir.Expr {
	callee := n.Expression()
	if callee == nil {
		l.diagf(n, "unsupported-call", "call without a callee")
		return nil
	}
	if callee.Kind == ast.KindPropertyAccessExpression {
		p := callee.AsPropertyAccessExpression()
		name := callee.Name().Text()
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
			return &hir.Expr{Kind: hir.SuperCall, Node: l.node(n), Name: name, Type: hm.Result, Args: args}
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
			recv := l.expr(p.Expression)
			if recv == nil {
				return nil
			}
			args, ok := l.callArgs(n, n.Arguments(), hm.Params)
			if !ok {
				return nil
			}
			return &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), Name: name, Type: hm.Result, X: recv, Args: args}
		}
		// Library operation, dispatched on the lowered receiver type.
		recv := l.expr(p.Expression)
		if recv == nil {
			return nil
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
		case "charAt":
			return l.rtOp("string.charAt", recv, str, one()), true
		case "charCodeAt":
			return l.rtOp("string.charCodeAt", recv, i32, one()), true
		case "substring":
			a, b := two()
			return l.rtOp("string.substring", recv, str, a, b), true
		case "substr":
			a, b := two()
			return l.rtOp("string.substr", recv, str, a, b), true
		case "trim":
			return l.rtOp("string.trim", recv, str), true
		case "toUpperCase":
			return l.rtOp("string.toUpperCase", recv, str), true
		case "concat":
			return l.rtOp("string.concat", recv, str, one()), true
		case "replace":
			return l.replaceCall(n, recv, args), true
		}
	case hir.I32:
		if name == "toString" {
			return l.rtOp("i32.toString", recv, str), true
		}
	case hir.Array:
		switch name {
		case "push":
			return l.rtOp("array.push", recv, i32, one()), true
		case "length":
			return l.rtOp("array.length", recv, i32), true
		}
	case hir.OrderedSet:
		switch name {
		case "has":
			return l.rtOp("set.has", recv, boolT, one()), true
		case "add":
			return l.rtOp("set.add", recv, recv.Type, one()), true
		case "size":
			return l.rtOp("set.size", recv, i32), true
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

// replaceCall lowers s.replace(regexLiteral, literal): the only regular
// expression shape this phase supports is a single character (optionally
// escaped) with the global flag, mapped to string.replaceAll. Every other
// pattern is reported, never silently dropped.
func (l *lowerer) replaceCall(n *ast.Node, recv *hir.Expr, args []*ast.Node) *hir.Expr {
	if len(args) != 2 || args[0].Kind != ast.KindRegularExpressionLiteral {
		l.diagf(n, "unsupported-regex", "replace with a non-literal pattern is not lowered")
		return nil
	}
	needle, ok := l.simpleRegex(args[0])
	if !ok {
		return nil
	}
	with := l.expr(args[1])
	if with == nil {
		return nil
	}
	return l.rtOp("string.replaceAll", recv, hir.T(hir.String), hir.L(hir.T(hir.String), needle), with)
}

// simpleRegex accepts /\x/g and returns the decoded character.
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
	l.diagf(n, "note-regex-mapped", "regex /%s/g mapped to string.replaceAll", pattern)
	if len(pattern) == 1 && pattern[0] != '\\' && pattern[0] != '.' {
		return pattern, true
	}
	if len(pattern) == 2 && pattern[0] == '\\' {
		if s, ok := regexCharEscapes[pattern[1]]; ok {
			return s, true
		}
	}
	l.diagf(n, "unsupported-regex", "regular expression /%s/g is not a single character", pattern)
	return "", false
}

var regexCharEscapes = map[byte]string{
	'n': "\n", 'r': "\r", 't': "\t", 'v': "\v", 'f': "\f", '0': "\x00",
	'\\': "\\", '/': "/", '.': ".", '*': "*", '+': "+", '?': "?", '(': "(", ')': ")",
	'[': "[", ']': "]", '{': "{", '}': "}", '^': "^", '$': "$", '|': "|", '-': "-",
}

// callArgs lowers arguments against the lowered parameter list, filling
// omitted trailing optional parameters with undefined.
func (l *lowerer) callArgs(at *ast.Node, args []*ast.Node, params []hir.Param) ([]*hir.Expr, bool) {
	out := make([]*hir.Expr, 0, len(params))
	for i, p := range params {
		if i < len(args) {
			l.hint = p.Type
			a := l.expr(args[i])
			l.hint = hir.Type{}
			if a == nil {
				return nil, false
			}
			out = append(out, a)
			continue
		}
		if p.Type.Kind == hir.Optional {
			out = append(out, &hir.Expr{Kind: hir.Lit, Type: p.Type})
			continue
		}
		l.diagf(at, "unsupported-call", "missing argument for parameter %s", p.Name)
		return nil, false
	}
	if len(args) > len(params) {
		l.diagf(args[len(params)], "unsupported-call", "too many arguments (%d for %d)", len(args), len(params))
		return nil, false
	}
	return out, true
}

// newExpression lowers allocations of lowered classes; `new Set([..])` is
// only allowed in statement contexts (see collectionInit).
func (l *lowerer) newExpression(n *ast.Node) *hir.Expr {
	c := l.classOf(l.resolve(n.Expression()))
	if c == nil {
		l.diagf(n, "unsupported-new", "new %s is not a lowered class", n.Expression().Text())
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
		return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: cop, Type: hir.T(hir.Bool), X: x, Y: y}
	case ast.KindPlusToken:
		if l.ck.GetTypeAtLocation(n).IsStringLike() {
			x, y := l.expr(b.Left), l.expr(b.Right)
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
		return l.arithmetic(n, b, "%")
	case ast.KindSlashToken:
		l.diagf(n, "unsupported-number", "division is not lowered under the i32 number policy")
		return nil
	case ast.KindInstanceOfKeyword:
		return l.instanceOf(n)
	case ast.KindEqualsToken, ast.KindPlusEqualsToken, ast.KindMinusEqualsToken, ast.KindAsteriskEqualsToken:
		l.diagf(n, "unsupported-expr", "assignment inside an expression is not lowered")
		return nil
	}
	l.diagf(n, "unsupported-expr", "binary operator %s is not lowered", op.String())
	return nil
}

func (l *lowerer) arithmetic(n *ast.Node, b *ast.BinaryExpression, op string) *hir.Expr {
	x, y := l.expr(b.Left), l.expr(b.Right)
	if x == nil || y == nil {
		return nil
	}
	return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: op, Type: x.Type, X: x, Y: y}
}

// instanceOf lowers `x instanceof C` to the boolean ancestry test.
func (l *lowerer) instanceOf(n *ast.Node) *hir.Expr {
	b := n.AsBinaryExpression()
	x := l.expr(b.Left)
	c := l.classOf(l.resolve(b.Right))
	if x == nil || c == nil {
		l.diagf(n, "unsupported-expr", "instanceof needs a lowered class operand")
		return nil
	}
	return &hir.Expr{Kind: hir.InstanceOf, Node: l.node(n), Type: hir.T(hir.Bool), X: x, Owner: c.Name}
}

// equality lowers === and !==, mapping comparisons with undefined to the
// explicit IsUndefined node.
func (l *lowerer) equality(n *ast.Node, b *ast.BinaryExpression, negated bool) *hir.Expr {
	leftUndef := l.isUndefinedType(b.Left)
	rightUndef := l.isUndefinedType(b.Right)
	if leftUndef != rightUndef {
		operand := b.Left
		if leftUndef {
			operand = b.Right
		}
		x := l.expr(operand)
		if x == nil {
			return nil
		}
		if x.Type.Kind != hir.Optional {
			l.diagf(n, "unsupported-expr", "undefined comparison needs an optional operand")
			return nil
		}
		test := &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: x}
		if negated {
			return &hir.Expr{Kind: hir.Unary, Node: l.node(n), Op: "!", Type: hir.T(hir.Bool), X: test}
		}
		return test
	}
	x, y := l.expr(b.Left), l.expr(b.Right)
	if x == nil || y == nil {
		return nil
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
			return l.numericLiteral(u.Operand, hir.T(hir.I32), -1)
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
// natural type and viewed as the asserted reference type.
func (l *lowerer) asExpression(n *ast.Node) *hir.Expr {
	return l.expr(n.Expression())
}

// objectLiteral lowers { ... } whose contextual type is a named alias shape,
// by constructing the synthesized class.
func (l *lowerer) objectLiteral(n *ast.Node) *hir.Expr {
	hint := l.hint
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
		l.diagf(n, "unsupported-type", "object literal needs a named lowered shape")
		return nil
	}
	ctor := target.Ctor
	props := n.AsObjectLiteralExpression().Properties.Nodes
	if len(props) != len(ctor.Params) {
		l.diagf(n, "unsupported-type", "object literal has %d properties, shape %s has %d", len(props), target.Name, len(ctor.Params))
		return nil
	}
	args := []*hir.Expr{}
	for i, p := range props {
		var value *ast.Node
		switch p.Kind {
		case ast.KindShorthandPropertyAssignment:
			value = p.Name()
		case ast.KindPropertyAssignment:
			value = p.Initializer()
		default:
			l.diagf(p, "unsupported-type", "object literal property %s is not lowered", p.Kind.String())
			return nil
		}
		l.hint = ctor.Params[i].Type
		a := l.expr(value)
		l.hint = hint
		if a == nil {
			return nil
		}
		args = append(args, a)
	}
	return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(target.Name), Args: args}
}

// decodeStringLiteral decodes a TypeScript string literal's raw text
// (including the quotes). The second result is an error message.
func decodeStringLiteral(raw string) (string, string) {
	if len(raw) < 2 {
		return "", "too short"
	}
	quote := raw[0]
	if quote != '"' && quote != '\'' && quote != '`' || raw[len(raw)-1] != quote {
		return "", "not a quoted literal"
	}
	body := raw[1 : len(raw)-1]
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(body) {
			return "", "trailing backslash"
		}
		switch e := body[i]; e {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'v':
			b.WriteByte('\v')
		case 'f':
			b.WriteByte('\f')
		case 'b':
			b.WriteByte('\b')
		case '0':
			b.WriteByte(0)
		case '\\', '/', '\'', '"', '`':
			b.WriteByte(e)
		case '\n':
			// line continuation: nothing
		case 'x':
			if i+2 >= len(body) {
				return "", "short \\x escape"
			}
			v, err := strconv.ParseUint(body[i+1:i+3], 16, 8)
			if err != nil {
				return "", "bad \\x escape"
			}
			b.WriteByte(byte(v))
			i += 2
		case 'u':
			if i+1 < len(body) && body[i+1] == '{' {
				end := strings.IndexByte(body[i+2:], '}')
				if end < 0 {
					return "", "unterminated \\u{ escape"
				}
				v, err := strconv.ParseUint(body[i+2:i+2+end], 16, 32)
				if err != nil {
					return "", "bad \\u{ escape"
				}
				b.WriteRune(rune(v))
				i += 2 + end
				continue
			}
			if i+4 >= len(body) {
				return "", "short \\u escape"
			}
			v, err := strconv.ParseUint(body[i+1:i+5], 16, 32)
			if err != nil {
				return "", "bad \\u escape"
			}
			b.WriteRune(rune(v))
			i += 4
		default:
			return "", "unsupported escape \\" + string(e)
		}
	}
	return b.String(), ""
}
