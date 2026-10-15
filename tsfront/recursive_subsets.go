package tsfront

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

func (l *lowerer) familyLiterals(t *checker.Type) []hir.UnionLiteral {
	var result []hir.UnionLiteral
	for _, p := range l.ck.GetPropertiesOfType(t) {
		if tags := l.familyTags(l.ck.GetTypeOfPropertyOfType(t, p.Name)); len(tags) > 0 {
			result = append(result, hir.UnionLiteral{Field: p.Name, Values: tags})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Field < result[j].Field })
	return result
}

// Subsets keep the original storage layouts. Flow refinements restrict literal
// domains without manufacturing a different record or expanding recursive edges.
func (l *lowerer) familySubsetType(t *checker.Type) (hir.Type, bool) {
	if t == nil || !t.IsUnion() || len(l.familyShapes) == 0 {
		return hir.Type{}, false
	}
	return l.familySubsetParts(t.AsUnionOrIntersectionType().Types())
}

// familySubsetParts is familySubsetType over explicit union members (for
// example a union with undefined removed).
func (l *lowerer) familySubsetParts(parts []*checker.Type) (hir.Type, bool) {
	if len(parts) == 0 || len(l.familyShapes) == 0 {
		return hir.Type{}, false
	}
	// Only registered families supply membership; never infer one from tag text.
	for _, u := range l.out.Unions {
		var alternatives []hir.UnionAlternative
		valid := true
		for _, part := range parts {
			found := false
			for _, a := range u.Alternatives {
				original := l.familyChecks[a.Payload.Name]
				if original == nil || !l.familyRefinesLayout(part, original) || !l.ck.IsTypeAssignableTo(part, original) {
					continue
				}
				tags := l.familyTags(l.ck.GetTypeOfPropertyOfType(part, u.Discriminant))
				if len(tags) == 0 {
					continue
				}
				refined := a
				refined.Tags = tags
				refined.Literals = l.familyLiterals(part)
				alternatives = append(alternatives, refined)
				found = true
				break
			}
			if !found {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		sort.Slice(alternatives, func(i, j int) bool { return alternatives[i].ID < alternatives[j].ID })
		key := u.Discriminant
		for _, a := range alternatives {
			key += "/" + a.ID + ":" + strings.Join(a.Tags, ",")
			for _, f := range a.Literals {
				key += ";" + f.Field + ":" + strings.Join(f.Values, ",")
			}
		}
		name := fmt.Sprintf("family.subset.%x", sha256.Sum256([]byte(key)))
		candidate := &hir.UnionDecl{Node: u.Node, Name: name, Discriminant: u.Discriminant, Complete: true, Alternatives: alternatives}
		if hir.UnionIncludes(candidate, u) && hir.UnionIncludes(u, candidate) {
			return hir.Type{Kind: hir.UnionRef, Name: u.Name}, true
		}
		if l.out.Union(name) == nil {
			l.out.Unions = append(l.out.Unions, candidate)
		}
		return hir.Type{Kind: hir.UnionRef, Name: name}, true
	}
	return hir.Type{}, false
}

// Discover the retained family before mapping a utility's resolved result. In
// particular Extract's single-member result no longer carries the source alias.
func (l *lowerer) familyUtilitySources(n *ast.Node, seen map[*ast.Node]bool) {
	if n == nil || seen[n] {
		return
	}
	seen[n] = true
	switch n.Kind {
	case ast.KindTypeReference:
		sym := l.resolve(n.AsTypeReferenceNode().TypeName)
		if sym == nil {
			return
		}
		if sym.Name == "Extract" && l.librarySymbol(sym) && len(n.TypeArguments()) == 2 {
			source := n.TypeArguments()[0]
			l.recursiveFamilyType(source, l.ck.GetTypeFromTypeNode(source))
			return
		}
		for _, d := range sym.Declarations {
			if d.Kind == ast.KindTypeAliasDeclaration {
				l.familyUtilitySources(d.Type(), seen)
			}
		}
	case ast.KindIndexedAccessType:
		l.familyUtilitySources(n.AsIndexedAccessTypeNode().ObjectType, seen)
	}
}

// Intersections adding payload fields need their own completed layout. Only
// literal-domain refinements may reuse an existing field's physical storage.
func (l *lowerer) familyRefinesLayout(part, original *checker.Type) bool {
	a, b := l.ck.GetPropertiesOfType(part), l.ck.GetPropertiesOfType(original)
	if len(a) != len(b) {
		return false
	}
	for _, p := range b {
		if l.ck.GetPropertyOfType(part, p.Name) == nil {
			return false
		}
		x, y := l.ck.GetTypeOfPropertyOfType(part, p.Name), l.ck.GetTypeOfPropertyOfType(original, p.Name)
		if x == y {
			continue
		}
		if x.Flags()&checker.TypeFlagsStringLike != 0 && y.Flags()&checker.TypeFlagsStringLike != 0 {
			continue
		}
		if l.familyTypeKey(x, true, map[*checker.Type]bool{}) != l.familyTypeKey(y, true, map[*checker.Type]bool{}) {
			return false
		}
	}
	return true
}
