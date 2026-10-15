package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

func (l *lowerer) familyAdmission(x *hir.Expr, dst hir.Type) *hir.Expr {
	u := l.out.Union(dst.Name)
	if u == nil {
		return x
	}
	if x.Type.Kind == hir.UnionRef && hir.UnionIncludes(u, l.out.Union(x.Type.Name)) {
		return l.rtOp("union.widen", x, dst)
	}
	for _, a := range u.Alternatives {
		if x.Type.Equal(a.Payload) {
			return &hir.Expr{Kind: hir.RuntimeOp, Node: x.Node, Op: "union.admit", Owner: u.Name, Name: a.ID, Type: dst, X: x}
		}
	}
	return x // A mismatch remains a verifier error, never erased membership.
}

func (l *lowerer) familyWriteDestination(t *checker.Type) bool {
	if t == nil || len(l.out.Unions) == 0 {
		return false
	}
	if typ, ok := l.familyTypes[l.familyAliasKey(t)]; ok && typ.Kind == hir.UnionRef {
		return true
	}
	if _, ok := l.familySubsetType(t); ok {
		return true
	}
	if t.Flags()&(checker.TypeFlagsObject|checker.TypeFlagsIntersection) == 0 {
		return false
	}
	typ, ok := l.familyShapes[l.familyTypeKey(t, false, map[*checker.Type]bool{})]
	if !ok {
		return false
	}
	for _, u := range l.out.Unions {
		for _, a := range u.Alternatives {
			if a.Payload.Equal(typ) {
				return true
			}
		}
	}
	return false
}

func (l *lowerer) familyLiteral(n *ast.Node, hint hir.Type) *hir.Expr {
	u := l.out.Union(hint.Name)
	if u == nil {
		return nil
	}
	t := l.ck.GetTypeAtLocation(n)
	tags := l.familyTags(l.ck.GetTypeOfPropertyOfType(t, u.Discriminant))
	var chosen *hir.UnionAlternative
	for i := range u.Alternatives {
		a := &u.Alternatives[i]
		match := len(tags) > 0
		for _, tag := range tags {
			found := false
			for _, own := range a.Tags {
				found = found || tag == own
			}
			match = match && found
		}
		if match {
			for _, field := range a.Literals {
				actual := l.familyTags(l.ck.GetTypeOfPropertyOfType(t, field.Field))
				if len(actual) == 0 {
					match = false
					break
				}
				for _, value := range actual {
					found := false
					for _, allowed := range field.Values {
						found = found || allowed == value
					}
					if !found {
						match = false
					}
				}
			}
		}
		if match && l.familyChecks[a.Payload.Name] != nil {
			match = l.ck.IsTypeAssignableTo(t, l.familyChecks[a.Payload.Name])
			if !match {
				for _, f := range l.classByName(a.Payload.Name).Fields {
					if f.Type.Kind != hir.Optional && l.ck.GetPropertyOfType(t, f.Name) == nil {
						l.diagf(n, "unsupported-type", "object literal misses field %s", f.Name)
						return nil
					}
				}
			}
		}
		if match {
			if chosen != nil {
				l.diagf(n, "unsupported-type", "recursive union literal has ambiguous payload membership")
				return nil
			}
			chosen = a
		}
	}
	if chosen == nil {
		l.diagf(n, "unsupported-type", "recursive union literal has no admitted tag or compatible payload")
		return nil
	}
	saved := l.hint
	l.hint = chosen.Payload
	payload := l.objectLiteral(n)
	l.hint = saved
	if payload == nil {
		return nil
	}
	return l.familyAdmission(payload, hint)
}

// Common reads retain the family. Unique narrowed reads use union.project in
// narrowed(), then the same live field ABI as other structural records.
func (l *lowerer) familyFieldAccess(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures || len(l.out.Unions) == 0 {
		return nil, false
	}
	p := n.AsPropertyAccessExpression()
	return l.familyFieldRead(n, p.Expression, n.Name().Text())
}

func (l *lowerer) familyIndexedAccess(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	p := n.AsElementAccessExpression()
	if ast.IsAssignmentTarget(n) && l.familyWriteDestination(l.ck.GetTypeAtLocation(p.Expression)) {
		l.diagf(n, "unsupported-assignment", "recursive union payload write requires narrowing and a proven alias-invalidation policy")
		return nil, true
	}
	if p.ArgumentExpression == nil || p.ArgumentExpression.Kind != ast.KindStringLiteral {
		return nil, false
	}
	return l.familyFieldRead(n, p.Expression, p.ArgumentExpression.Text())
}

func (l *lowerer) familyFieldRead(n, expression *ast.Node, name string) (*hir.Expr, bool) {
	t := l.ck.GetTypeAtLocation(expression)
	if t == nil {
		return nil, false
	}
	alias := l.familyAliasKey(t)
	mapped, ok := l.familyTypes[alias]
	if !ok {
		mapped, ok = l.familySubsetType(t)
	}
	if !ok || mapped.Kind != hir.UnionRef {
		return nil, false
	}
	if ast.IsAssignmentTarget(n) {
		l.diagf(n, "unsupported-assignment", "recursive union field writes require a narrowed destination")
		return nil, true
	}
	u := l.out.Union(mapped.Name)
	result := hir.Type{}
	for _, a := range u.Alternatives {
		c := l.classByName(a.Payload.Name)
		found := false
		for _, f := range c.Fields {
			if f.Name == name {
				if result.Kind != "" && !result.Equal(f.Type) {
					if familyScalarRead(result) && familyScalarRead(f.Type) {
						result = hir.T(hir.Dynamic)
					} else {
						l.diagf(n, "unsupported-expr", "recursive union field %s requires narrowing", name)
						return nil, true
					}
				}
				if result.Kind != hir.Dynamic {
					result = f.Type
				}
				found = true
			}
		}
		if !found {
			l.diagf(n, "unsupported-expr", "recursive union field %s requires narrowing", name)
			return nil, true
		}
	}
	saved := l.hint
	l.hint = hir.Type{}
	recv := l.expr(expression)
	l.hint = saved
	if recv == nil {
		return nil, true
	}
	op := "union.read"
	if name == u.Discriminant {
		op = "union.tag"
	}
	return &hir.Expr{Kind: hir.RuntimeOp, Node: l.node(n), Op: op, Name: name, Type: result, X: recv}, true
}

func familyScalarRead(t hir.Type) bool {
	switch t.Kind {
	case hir.String, hir.Number, hir.I32, hir.I64, hir.Bool, hir.Dynamic:
		return true
	}
	return false
}
