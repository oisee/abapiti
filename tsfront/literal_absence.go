package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// A contextual primitive declaration is no presence proof when a literal
// initializes it from erased data. Decide the storage layout before signatures
// and bodies, so every constructor and alias sees the same optional field ABI.
func (l *lowerer) scanLiteralAbsence(n *ast.Node) {
	if n.Kind == ast.KindPropertyAccessExpression && truthinessUse(n) {
		// A condition observes JS truthiness, including an omitted field on a
		// live view. Keep the value until ToBoolean performs that observation.
		if prop := l.resolve(n); prop != nil && l.ck.GetTypeOfSymbol(prop).IsBooleanLike() {
			parameterProperty := false
			for _, decl := range prop.Declarations {
				parameterProperty = parameterProperty || decl.Kind == ast.KindParameter
			}
			if !parameterProperty {
				l.markPrimitiveAbsence(prop)
			}
		}
	}
	if n.Kind == ast.KindBinaryExpression {
		b := n.AsBinaryExpression()
		switch b.OperatorToken.Kind {
		case ast.KindEqualsEqualsToken, ast.KindEqualsEqualsEqualsToken, ast.KindExclamationEqualsToken, ast.KindExclamationEqualsEqualsToken:
			var property *ast.Node
			if l.isUndefinedType(b.Left) {
				property = b.Right
			} else if l.isUndefinedType(b.Right) {
				property = b.Left
			}
			if property != nil && property.Kind == ast.KindPropertyAccessExpression && property.Flags&ast.NodeFlagsOptionalChain == 0 && property.AsPropertyAccessExpression().QuestionDotToken == nil {
				prop := l.resolve(property)
				if l.markPrimitiveAbsence(prop) {
					for _, decl := range prop.Declarations {
						l.checkedFields[decl] = true
					}
				}
			}
		}
	}
	if n.Kind == ast.KindObjectLiteralExpression {
		context := l.ck.GetContextualType(n, checker.ContextFlagsNone)
		if context != nil {
			for _, p := range n.AsObjectLiteralExpression().Properties.Nodes {
				var value *ast.Node
				if p.Kind == ast.KindPropertyAssignment {
					value = p.Initializer()
				} else if p.Kind == ast.KindShorthandPropertyAssignment {
					value = p.Name()
				} else {
					continue
				}
				name := p.Name()
				if name == nil || (name.Kind != ast.KindIdentifier && name.Kind != ast.KindStringLiteral) || !l.checkerMayBeUndefined(value) {
					continue
				}
				prop := l.ck.GetPropertyOfType(context, name.Text())
				if prop == nil {
					continue
				}
				l.markPrimitiveAbsence(prop)
			}
		}
	}
	n.ForEachChild(func(child *ast.Node) bool { l.scanLiteralAbsence(child); return false })
}

func truthinessUse(n *ast.Node) bool {
	p := n.Parent
	for p != nil && p.Kind == ast.KindParenthesizedExpression {
		n, p = p, p.Parent
	}
	if p == nil {
		return false
	}
	switch p.Kind {
	case ast.KindIfStatement:
		return p.AsIfStatement().Expression == n
	case ast.KindBinaryExpression:
		b := p.AsBinaryExpression()
		return b.Left == n && (b.OperatorToken.Kind == ast.KindAmpersandAmpersandToken || b.OperatorToken.Kind == ast.KindBarBarToken)
	case ast.KindPrefixUnaryExpression:
		return p.AsPrefixUnaryExpression().Operator == ast.KindExclamationToken
	}
	return false
}

// A parameter copied into a field checked for undefined must reach the source
// guard unchanged. This scan follows the already certified field storage; it
// does not supply the constructor's default at the call site.
func (l *lowerer) scanAbsenceParameters(n *ast.Node) {
	if n.Kind == ast.KindBinaryExpression {
		b := n.AsBinaryExpression()
		if b.OperatorToken.Kind == ast.KindEqualsToken && b.Left.Kind == ast.KindPropertyAccessExpression && b.Right.Kind == ast.KindIdentifier && l.literalPropertyAbsentSymbol(l.resolve(b.Left)) {
			if sym := l.resolve(b.Right); sym != nil {
				for _, decl := range sym.Declarations {
					if decl.Kind == ast.KindParameter && primitivePresenceType(l.ck.GetTypeOfSymbol(sym)) {
						l.absentParams[decl] = true
					}
				}
			}
		}
	}
	n.ForEachChild(func(child *ast.Node) bool { l.scanAbsenceParameters(child); return false })
}

func (l *lowerer) parameterMayBeAbsent(sym *ast.Symbol) bool {
	if sym != nil {
		for _, decl := range sym.Declarations {
			if l.absentParams[decl] {
				return true
			}
		}
	}
	return false
}

func primitivePresenceType(t *checker.Type) bool {
	if t == nil {
		return false
	}
	if t.Flags()&(checker.TypeFlagsStringLike|checker.TypeFlagsNumberLike|checker.TypeFlagsBooleanLike|checker.TypeFlagsUndefined) != 0 {
		return true
	}
	if !t.IsUnion() {
		return false
	}
	for _, part := range t.AsUnionOrIntersectionType().Types() {
		if !primitivePresenceType(part) {
			return false
		}
	}
	return true
}

func (l *lowerer) markPrimitiveAbsence(prop *ast.Symbol) bool {
	if prop == nil {
		return false
	}
	typ := l.ck.GetTypeOfSymbol(prop)
	if !primitivePresenceType(typ) || typeMayBeUndefined(typ) {
		return false
	}
	for _, decl := range prop.Declarations {
		l.absentFields[decl] = true
	}
	return true
}

func (l *lowerer) literalPropertyAbsent(p *ast.Symbol) bool {
	for _, decl := range p.Declarations {
		if l.absentFields[decl] {
			return true
		}
	}
	return false
}

func (l *lowerer) literalPropertyAbsentSymbol(p *ast.Symbol) bool {
	return p != nil && l.literalPropertyAbsent(p)
}

// Call/constructor arguments retain their checked primitive boundary when
// the checker requires a present value. Optional/dynamic construction contexts
// still store or box undefined. This does not change any constructor intrinsic.
func (l *lowerer) requiredFieldUse(n *ast.Node) bool {
	if l.hint.Kind == hir.Dynamic || l.hint.Kind == hir.Optional {
		return false
	}
	parent := n.Parent
	for parent != nil && parent.Kind == ast.KindParenthesizedExpression {
		parent = parent.Parent
	}
	if parent == nil {
		return false
	}
	if parent.Kind == ast.KindNewExpression {
		return true
	}
	if parent.Kind == ast.KindCallExpression {
		context := l.ck.GetContextualType(n, checker.ContextFlagsNone)
		return primitivePresenceType(context) && !typeMayBeUndefined(context)
	}
	return false
}
