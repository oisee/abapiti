package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Predicate/probe overloads describe one JS function. When no declared
// signature covers the set, a tagged argument ABI preserves every argument;
// the optional boolean result retains undefined from the probe overload.
func (l *lowerer) booleanInterfaceOverload(node, member *ast.Node) (*hir.Method, bool) {
	if !l.unprunedFeatures || member.Symbol() == nil {
		return nil, false
	}
	var group []*ast.Node
	for _, m := range node.Members() {
		if (m.Kind == ast.KindMethodSignature || m.Kind == ast.KindMethodDeclaration) && m.Symbol() == member.Symbol() {
			group = append(group, m)
		}
	}
	if len(group) < 2 {
		return nil, false
	}
	arity := len(group[0].Parameters())
	optional, presentOnly := false, false
	for _, m := range group {
		if len(m.Parameters()) != arity {
			return nil, false
		}
		for _, p := range m.Parameters() {
			if ast.IsThisParameter(p) || p.Name() == nil || p.Name().Kind != ast.KindIdentifier || p.AsParameterDeclaration().DotDotDotToken != nil || p.QuestionToken() != nil || p.Initializer() != nil {
				return nil, false
			}
		}
		sig := l.ck.GetSignatureFromDeclaration(m)
		if sig == nil {
			return nil, false
		}
		ret := l.ck.GetReturnTypeOfSignature(sig)
		if ret == nil {
			return nil, false
		}
		parts := []*checker.Type{ret}
		if ret.IsUnion() {
			parts = ret.AsUnionOrIntersectionType().Types()
		}
		hasBoolean, hasUndefined := false, false
		for _, part := range parts {
			if part.Flags()&checker.TypeFlagsUndefined != 0 {
				hasUndefined = true
			} else if part.Flags()&checker.TypeFlagsBooleanLike != 0 {
				hasBoolean = true
			} else {
				return nil, false
			}
		}
		if !hasBoolean {
			return nil, false
		}
		optional = optional || hasUndefined
		presentOnly = presentOnly || !hasUndefined
	}
	if !optional || !presentOnly {
		return nil, false
	}
	if member != group[0] {
		return nil, true
	}
	name, ok := l.memberName(member, nil)
	if !ok {
		return nil, true
	}
	hm := &hir.Method{Node: l.node(member), Name: name, Virtual: true, Result: hir.T(hir.Optional, hir.T(hir.Bool))}
	for _, p := range member.Parameters() {
		hm.Params = append(hm.Params, hir.Param{Name: p.Name().Text(), Type: hir.T(hir.Dynamic)})
	}
	return hm, true
}
