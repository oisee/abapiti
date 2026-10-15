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
	joined := false
	for _, a := range u.Alternatives {
		if joined {
			break
		}
		c := l.classByName(a.Payload.Name)
		found := false
		for _, f := range c.Fields {
			if f.Name == name {
				if merged, ok := familyOptionalRead(result, f.Type); ok {
					result = merged
				} else if result.Kind != "" && !result.Equal(f.Type) {
					if familyScalarRead(result) && familyScalarRead(f.Type) {
						result = hir.T(hir.Dynamic)
					} else if common, ok := l.familyCommonRead(n, t, name, u); ok {
						result = common
						joined = true
					} else {
						l.diagf(n, "unsupported-expr", "recursive union field %s requires narrowing", name)
						return nil, true
					}
				}
				if result.Kind == "" {
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

// familyCommonRead is the common-field view of a field that every member
// declares with differing reference types (a narrower payload or subset in
// some members). The checker's property type over the whole receiver names
// the joined type; when it maps to a family accepting every member's storage,
// the read validates the stored value against it. Otherwise it refuses.
func (l *lowerer) familyCommonRead(n *ast.Node, recv *checker.Type, name string, u *hir.UnionDecl) (hir.Type, bool) {
	prop := l.ck.GetTypeOfPropertyOfType(recv, name)
	if prop == nil || prop.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
		return hir.Type{}, false
	}
	want, ok := l.familyTypes[l.familyAliasKey(prop)]
	if !ok && prop.IsUnion() {
		var present []*checker.Type
		for _, part := range prop.AsUnionOrIntersectionType().Types() {
			if part.Flags()&checker.TypeFlagsUndefined == 0 {
				present = append(present, part)
			}
		}
		if want, ok = l.familySubsetParts(present); ok && len(present) < len(prop.AsUnionOrIntersectionType().Types()) {
			want = hir.T(hir.Optional, want)
		}
	}
	if !ok {
		before := len(l.diags)
		want = l.mapCheckerType(n, prop)
		blocked := hasBlocking(l.diags[before:])
		l.diags = l.diags[:before]
		if blocked {
			return hir.Type{}, false
		}
	}
	base := want
	if base.Kind == hir.Optional && len(base.Args) == 1 {
		base = base.Args[0]
	}
	if base.Kind != hir.UnionRef {
		return hir.Type{}, false
	}
	optional := false
	for _, a := range u.Alternatives {
		var field *hir.Field
		for i, f := range l.classByName(a.Payload.Name).Fields {
			if f.Name == name {
				field = &l.classByName(a.Payload.Name).Fields[i]
			}
		}
		if field == nil || !hir.UnionReadAccepts(l.out, hir.T(hir.Optional, base), field.Type) {
			return hir.Type{}, false
		}
		optional = optional || field.Type.Kind == hir.Optional
	}
	if optional {
		return hir.T(hir.Optional, base), true
	}
	return base, true
}

func familyScalarRead(t hir.Type) bool {
	switch t.Kind {
	case hir.String, hir.Number, hir.I32, hir.I64, hir.Bool, hir.Dynamic:
		return true
	}
	return false
}

// familyOptionalRead joins T and Optional<T> (an exactly-undefined sibling
// field) into Optional<T>. The read is a dynamic get, so an absent key and a
// present undefined both yield the empty optional, as in JavaScript.
func familyOptionalRead(a, b hir.Type) (hir.Type, bool) {
	if a.Kind == "" || a.Equal(b) {
		return hir.Type{}, false
	}
	if a.Kind == hir.Optional && len(a.Args) == 1 && a.Args[0].Equal(b) {
		return a, true
	}
	if b.Kind == hir.Optional && len(b.Args) == 1 && b.Args[0].Equal(a) {
		return b, true
	}
	return hir.Type{}, false
}
