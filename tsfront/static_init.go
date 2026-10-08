package tsfront

import (
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/stringutil"
)

// pureInitializer is deliberately a syntax whitelist, not an effect inference.
// Class reads are limited to earlier pure fields of the same class: another
// class's mutable static may have changed before ABAP lazily initializes us.
func (l *lowerer) pureInitializer(n, owner *ast.Node, visiting map[*ast.Node]bool) bool {
	if n == nil || visiting[n] {
		return false
	}
	visiting[n] = true
	defer delete(visiting, n)
	pure := func(x *ast.Node) bool { return l.pureInitializer(x, owner, visiting) }
	switch n.Kind {
	case ast.KindStringLiteral, ast.KindNoSubstitutionTemplateLiteral, ast.KindNumericLiteral, ast.KindTrueKeyword, ast.KindFalseKeyword:
		return true
	case ast.KindParenthesizedExpression:
		return pure(n.Expression())
	case ast.KindPrefixUnaryExpression:
		p := n.AsPrefixUnaryExpression()
		switch p.Operator {
		case ast.KindPlusToken, ast.KindMinusToken, ast.KindExclamationToken, ast.KindTildeToken:
			return pure(p.Operand)
		}
	case ast.KindBinaryExpression:
		b := n.AsBinaryExpression()
		switch b.OperatorToken.Kind {
		case ast.KindPlusToken, ast.KindMinusToken, ast.KindAsteriskToken, ast.KindSlashToken, ast.KindPercentToken,
			ast.KindLessThanToken, ast.KindGreaterThanToken, ast.KindLessThanEqualsToken, ast.KindGreaterThanEqualsToken,
			ast.KindEqualsEqualsToken, ast.KindExclamationEqualsToken, ast.KindEqualsEqualsEqualsToken, ast.KindExclamationEqualsEqualsToken,
			ast.KindAmpersandAmpersandToken, ast.KindBarBarToken, ast.KindQuestionQuestionToken,
			ast.KindAmpersandToken, ast.KindBarToken, ast.KindCaretToken, ast.KindLessThanLessThanToken, ast.KindGreaterThanGreaterThanToken, ast.KindGreaterThanGreaterThanGreaterThanToken:
			return pure(b.Left) && pure(b.Right)
		}
	case ast.KindConditionalExpression:
		c := n.AsConditionalExpression()
		return pure(c.Condition) && pure(c.WhenTrue) && pure(c.WhenFalse)
	case ast.KindIdentifier:
		sym := l.resolve(n)
		if sym == nil || sym.ValueDeclaration == nil {
			return false
		}
		d := sym.ValueDeclaration
		if d.Kind != ast.KindVariableDeclaration || d.Parent == nil || d.Parent.Kind != ast.KindVariableDeclarationList || d.Parent.Flags&ast.NodeFlagsConst == 0 {
			return false
		}
		// Only module constants, not arbitrary local bindings.
		if d.Parent.Parent == nil || d.Parent.Parent.Parent == nil || d.Parent.Parent.Parent.Kind != ast.KindSourceFile {
			return false
		}
		return pure(d.Initializer())
	case ast.KindPropertyAccessExpression:
		if l.enumOf(l.resolve(n.Expression())) != nil {
			return true
		}
		if recv := l.resolve(n.Expression()); recv != nil && recv.ValueDeclaration != nil {
			d := recv.ValueDeclaration
			if d.Kind == ast.KindVariableDeclaration && d.Parent != nil && d.Parent.Flags&ast.NodeFlagsConst != 0 && d.Parent.Parent != nil && d.Parent.Parent.Parent != nil && d.Parent.Parent.Parent.Kind == ast.KindSourceFile {
				return l.moduleInitializer(d.Initializer())
			}
		}
		sym := l.resolve(n.Name())
		if sym == nil || sym.ValueDeclaration == nil || owner == nil {
			return false
		}
		d := sym.ValueDeclaration
		if d.Kind != ast.KindPropertyDeclaration || d.Parent != owner.Parent || d.Pos() >= owner.Pos() || d.ModifierFlags()&ast.ModifierFlagsStatic == 0 {
			return false
		}
		return l.pureInitializer(d.Initializer(), d, visiting)
	case ast.KindObjectLiteralExpression:
		return len(n.AsObjectLiteralExpression().Properties.Nodes) == 0
	case ast.KindArrayLiteralExpression:
		for _, el := range n.AsArrayLiteralExpression().Elements.Nodes {
			if !pure(el) {
				return false
			}
		}
		return true
	case ast.KindNewExpression:
		// Empty standard collections allocate no observable module/class state.
		// isNewCollection intentionally selects only literal-population lowering.
		if expr := n.Expression(); expr != nil && expr.Kind == ast.KindIdentifier && len(n.Arguments()) == 0 {
			sym := l.resolve(expr)
			if l.librarySymbol(sym) && l.classOf(sym) == nil && (sym.Name == "Map" || sym.Name == "Set" || sym.Name == "Array") {
				return true
			}
		}
		if !l.isNewCollection(n) {
			return false
		}
		for _, arg := range n.Arguments() {
			if !pure(arg) {
				return false
			}
		}
		return true
	}
	return false
}

// Inspect scanner strings as JS code units, before Go's rune decoding can
// replace CESU-8 surrogate sentinels with U+FFFD. Include literals in syntax
// which is otherwise rejected or skipped, not just lowered expressions.
func (l *lowerer) checkStringLiterals(n *ast.Node) {
	switch n.Kind {
	case ast.KindStringLiteral, ast.KindNoSubstitutionTemplateLiteral, ast.KindTemplateHead, ast.KindTemplateMiddle, ast.KindTemplateTail:
		if hasLoneSurrogate(stringutil.CombineSurrogatePairs(n.Text())) {
			l.diagf(n, "unsupported-lone-surrogate", "string literal contains a lone UTF-16 surrogate")
		}
	}
	n.ForEachChild(func(child *ast.Node) bool { l.checkStringLiterals(child); return false })
}

func hasLoneSurrogate(s string) bool {
	for len(s) > 0 {
		r, size := stringutil.DecodeJSStringRune(s)
		if stringutil.IsSurrogate(r) {
			return true
		}
		s = s[size:]
	}
	return false
}

// Module factory calls used by descriptor registries may allocate objects, but
// must not observe a class static whose ABAP initialization is lazy.
func (l *lowerer) moduleInitializer(n *ast.Node) bool {
	safe := true
	var walk func(*ast.Node)
	walk = func(x *ast.Node) {
		if x == nil {
			return
		}
		if x.Kind == ast.KindPropertyAccessExpression {
			p := x.AsPropertyAccessExpression()
			if l.classOf(l.resolve(p.Expression)) != nil {
				safe = false
			}
		}
		x.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(n)
	return safe
}
