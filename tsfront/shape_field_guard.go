package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// A property guard refines a read without changing the object's declared
// storage. An intersection parameter can require that refined property; give
// it the existing live view, preserving identity and checking required reads.
func (l *lowerer) guardedShapeArgument(at *ast.Node, x *hir.Expr, dst hir.Type) *hir.Expr {
	if !l.unprunedFeatures || x == nil || x.Type.Kind != hir.ClassRef || dst.Kind != hir.ClassRef || x.Type.Equal(dst) || !l.shapeLike[x.Type.Name] || !l.shapeLike[dst.Name] {
		return x
	}
	source, target := l.classByName(x.Type.Name), l.classByName(dst.Name)
	if source == nil || target == nil || len(source.Methods) != 0 || len(target.Methods) != 0 || source.Super != "" || target.Super != "" {
		return x
	}
	refined := false
	for _, f := range target.Fields {
		if f.Static {
			return x
		}
		var member *hir.Field
		for i := range source.Fields {
			if source.Fields[i].Name == f.Name {
				member = &source.Fields[i]
				break
			}
		}
		if member == nil {
			if l.gogenAbsentShapeField(f, source) {
				continue
			}
			return x
		}
		g := *member
		if g.Static {
			return x
		}
		if f.Type.Equal(g.Type) || f.Type.Kind == hir.Dynamic || f.Type.Kind == hir.Optional && f.Type.Args[0].Equal(g.Type) || f.Readonly && g.Readonly && l.readonlyShapeRead(f.Type, g.Type, map[string]bool{}) {
			continue
		}
		if g.Type.Kind != hir.Optional || !f.Type.Equal(g.Type.Args[0]) || !l.guardedPropertyPresent(at, f.Name) {
			return x
		}
		refined = true
	}
	if !refined {
		return x
	}
	return l.dynamicReference(l.coerce(x, hir.T(hir.Dynamic)), dst)
}

func (l *lowerer) guardedPropertyPresent(at *ast.Node, name string) bool {
	receiver := gogenCallee(at)
	if receiver == nil || receiver.Kind != ast.KindIdentifier {
		return false
	}
	evaluationEnd := at.End()
	if at.Parent != nil && at.Parent.Kind == ast.KindCallExpression {
		// Later arguments also run before the callee can read this view.
		evaluationEnd = at.Parent.End()
	}
	sym := l.resolve(receiver)
	if sym == nil {
		return false
	}
	var proves func(*ast.Node) bool
	proves = func(n *ast.Node) bool {
		n = gogenCallee(n)
		if n == nil || n.Kind != ast.KindBinaryExpression {
			return false
		}
		b := n.AsBinaryExpression()
		if b.OperatorToken.Kind == ast.KindAmpersandAmpersandToken {
			// Only the last conjunct can establish a proof without a later
			// condition effect invalidating it.
			return proves(b.Right)
		}
		if b.OperatorToken.Kind != ast.KindExclamationEqualsEqualsToken && b.OperatorToken.Kind != ast.KindExclamationEqualsToken {
			return false
		}
		var property *ast.Node
		absent := func(n *ast.Node) bool {
			return isUndefinedKeyword(n) && l.isUndefinedType(n)
		}
		if absent(b.Right) {
			property = gogenCallee(b.Left)
		} else if absent(b.Left) {
			property = gogenCallee(b.Right)
		}
		if property == nil || property.Kind != ast.KindPropertyAccessExpression || property.Name().Text() != name {
			return false
		}
		object := gogenCallee(property.Expression())
		return object != nil && object.Kind == ast.KindIdentifier && l.resolve(object) == sym
	}
	ancestors := map[*ast.Node]bool{}
	for child := at; child != nil && child.Parent != nil; child = child.Parent {
		ancestors[child] = true
		parent := child.Parent
		if ast.IsFunctionLike(parent) {
			break
		}
		var condition, branch *ast.Node
		switch parent.Kind {
		case ast.KindIfStatement:
			s := parent.AsIfStatement()
			if child == s.ThenStatement {
				condition, branch = s.Expression, s.ThenStatement
			}
		case ast.KindConditionalExpression:
			c := parent.AsConditionalExpression()
			if child == c.WhenTrue {
				condition, branch = c.Condition, c.WhenTrue
			}
		case ast.KindBinaryExpression:
			b := parent.AsBinaryExpression()
			if child == b.Right && b.OperatorToken.Kind == ast.KindAmpersandAmpersandToken {
				condition, branch = b.Left, b.Right
			}
		}
		if !proves(condition) {
			continue
		}
		// Do not carry a property proof across a call, assignment, or suspension.
		// Ancestor calls have not executed yet: their arguments evaluate first.
		safe := true
		var walk func(*ast.Node) bool
		walk = func(n *ast.Node) bool {
			if n == nil || n.End() <= condition.End() || n.Pos() >= evaluationEnd {
				return false
			}
			if !ancestors[n] {
				switch n.Kind {
				case ast.KindCallExpression, ast.KindNewExpression, ast.KindDeleteExpression, ast.KindAwaitExpression, ast.KindYieldExpression, ast.KindPostfixUnaryExpression, ast.KindPrefixUnaryExpression:
					safe = false
					return true
				case ast.KindBinaryExpression:
					if ast.IsAssignmentOperator(n.AsBinaryExpression().OperatorToken.Kind) {
						safe = false
						return true
					}
				}
			}
			return n.ForEachChild(walk)
		}
		walk(branch)
		if safe {
			return true
		}
	}
	return false
}
