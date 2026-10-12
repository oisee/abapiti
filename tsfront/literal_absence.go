package tsfront

import (
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// A contextual primitive declaration is no presence proof when a literal
// initializes it from erased data. Decide the storage layout before signatures
// and bodies, so every constructor and alias sees the same optional field ABI.
func (l *lowerer) scanLiteralAbsence(n *ast.Node) {
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
				typ := l.ck.GetTypeOfSymbol(prop)
				if typ == nil || typ.Flags()&(checker.TypeFlagsStringLike|checker.TypeFlagsNumberLike|checker.TypeFlagsBooleanLike) == 0 {
					continue
				}
				for _, decl := range prop.Declarations {
					l.absentFields[decl] = true
				}
			}
		}
	}
	n.ForEachChild(func(child *ast.Node) bool { l.scanLiteralAbsence(child); return false })
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
