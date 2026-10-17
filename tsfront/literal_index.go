package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// objectPrototypeKeys are the properties every plain object literal inherits.
var objectPrototypeKeys = []string{"constructor", "__proto__", "hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable", "toLocaleString", "toString", "valueOf", "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__"}

// literalFamilyIndex lowers `{K1: v1, K2: v2, ...}[key]` (OSG namedType's
// builtin type table) when every value is a member of one complete family:
// the values are evaluated in source order, then the key; an own key selects
// its value (the last duplicate wins), an inherited Object.prototype key
// traps, any other key is undefined. The checker types the read as any, so
// without this the untyped nested literals lose their family membership.
func (l *lowerer) literalFamilyIndex(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	e := n.AsElementAccessExpression()
	if e.ArgumentExpression == nil || e.QuestionDotToken != nil {
		return nil, false
	}
	lit := ast.SkipParentheses(e.Expression)
	if lit.Kind != ast.KindObjectLiteralExpression {
		return nil, false
	}
	props := lit.AsObjectLiteralExpression().Properties
	if props == nil || len(props.Nodes) == 0 {
		return nil, false
	}
	var names []string
	var values []*ast.Node
	for _, p := range props.Nodes {
		var value *ast.Node
		switch p.Kind {
		case ast.KindPropertyAssignment:
			value = p.AsPropertyAssignment().Initializer
		case ast.KindShorthandPropertyAssignment:
			value = p.Name()
		default:
			return nil, false
		}
		name := p.Name()
		if name == nil || name.Kind != ast.KindIdentifier && name.Kind != ast.KindStringLiteral {
			return nil, false
		}
		for _, k := range objectPrototypeKeys {
			if name.Text() == k {
				return nil, false
			}
		}
		names = append(names, name.Text())
		values = append(values, value)
	}
	family, familyType := l.literalIndexFamily(values)
	if family.Kind != hir.UnionRef {
		return nil, false
	}
	result := hir.T(hir.Optional, family)
	temps := make([]*hir.Expr, len(values))
	for i, v := range values {
		if inner := ast.SkipParentheses(v); inner.Kind == ast.KindObjectLiteralExpression {
			// Without a contextual type the checker widens the tag to
			// string; in the family's context it keeps its literal type.
			if l.contextLiteralTypes == nil {
				l.contextLiteralTypes = map[*ast.Node]*checker.Type{}
			}
			l.contextLiteralTypes[inner] = l.ck.CheckExpressionWithContextualType(inner, familyType)
		}
		saved := l.hint
		l.hint = family
		x := l.expr(v)
		l.hint = saved
		if x == nil {
			return nil, true
		}
		x = l.coerce(x, family)
		if !x.Type.Equal(family) {
			l.diagf(v, "unsupported-expr", "indexed literal value %s is not a member of %s", names[i], family.Name)
			return nil, true
		}
		temps[i] = l.tempInit(v, family, x)
	}
	key := l.expr(e.ArgumentExpression)
	if key == nil {
		return nil, true
	}
	if key.Type.Kind != hir.String {
		l.diagf(e.ArgumentExpression, "unsupported-expr", "indexed literal key must be a string")
		return nil, true
	}
	key = l.tempInit(e.ArgumentExpression, key.Type, key)
	out := l.tempInit(n, result, &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: result})
	str := hir.T(hir.String)
	eq := func(k string) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "==", Type: hir.T(hir.Bool), X: key, Y: hir.L(str, k)}
	}
	var chain *hir.Stmt
	inherited := hir.B(&hir.Stmt{Node: l.node(n), Kind: hir.Trap, Name: "inherited Object.prototype property of an indexed literal"})
	for i := len(objectPrototypeKeys) - 1; i >= 0; i-- {
		s := &hir.Stmt{Kind: hir.If, Node: l.node(n), X: eq(objectPrototypeKeys[i]), Body: inherited}
		if chain != nil {
			s.Else = hir.B(chain)
		}
		chain = s
	}
	seen := map[string]bool{}
	for i := len(names) - 1; i >= 0; i-- {
		if seen[names[i]] {
			continue
		}
		seen[names[i]] = true
		assign := &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: out, Y: l.coerce(temps[i], result)}
		chain = &hir.Stmt{Kind: hir.If, Node: l.node(n), X: eq(names[i]), Body: hir.B(assign), Else: hir.B(chain)}
	}
	l.pendStmt(chain)
	return out, true
}

// literalIndexFamily is the single complete family that some value's own
// type already names (a member alias such as Extract<Type, {k: "i"}>), with
// that family's checker type.
func (l *lowerer) literalIndexFamily(values []*ast.Node) (hir.Type, *checker.Type) {
	var family hir.Type
	var familyType *checker.Type
	for _, v := range values {
		t := l.ck.GetTypeAtLocation(v)
		if t == nil {
			continue
		}
		before := len(l.diags)
		mapped := l.mapCheckerType(v, t)
		l.diags = l.diags[:before]
		if mapped.Kind == hir.Optional {
			continue
		}
		candidate := hir.Type{}
		if mapped.Kind == hir.UnionRef {
			candidate = mapped
		} else if mapped.Kind == hir.ClassRef {
			// The payload's widest family: every other complete family that
			// admits it (a subset) is included in it.
			var owners []*hir.UnionDecl
			for _, u := range l.out.Unions {
				if u == nil || !u.Complete {
					continue
				}
				for _, a := range u.Alternatives {
					if a.Payload.Equal(mapped) {
						owners = append(owners, u)
						break
					}
				}
			}
			for _, u := range owners {
				widest := true
				for _, o := range owners {
					widest = widest && hir.UnionIncludes(u, o)
				}
				if widest {
					candidate = hir.Type{Kind: hir.UnionRef, Name: u.Name}
					break
				}
			}
		}
		if candidate.Kind != hir.UnionRef {
			continue
		}
		if u := l.out.Union(candidate.Name); u == nil || !u.Complete {
			return hir.Type{}, nil
		}
		if family.Kind == hir.UnionRef && family.Name != candidate.Name {
			return hir.Type{}, nil
		}
		family = candidate
	}
	if family.Kind != hir.UnionRef {
		return hir.Type{}, nil
	}
	// The family's checker type: the union of its members' checked layouts.
	var members []*checker.Type
	for _, a := range l.out.Union(family.Name).Alternatives {
		check := l.familyChecks[a.Payload.Name]
		if check == nil {
			return hir.Type{}, nil
		}
		members = append(members, check)
	}
	familyType = l.ck.GetUnionType(members)
	if familyType == nil {
		return hir.Type{}, nil
	}
	return family, familyType
}
