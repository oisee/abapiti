package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Parentheses also carry JSDoc assertions in JS input. They do not bind a
// receiver when the enclosed expression is a standalone function value.
func gogenCallee(n *ast.Node) *ast.Node {
	for n != nil && (n.Kind == ast.KindParenthesizedExpression || n.Kind == ast.KindAsExpression || n.Kind == ast.KindTypeAssertionExpression) {
		n = n.Expression()
	}
	return n
}

func (l *lowerer) gogenResolvedData(node *ast.Node) bool {
	if !l.unprunedFeatures || l.ck == nil || node.AsInterfaceDeclaration().HeritageClauses == nil {
		return false
	}
	t := l.ck.GetApparentType(l.ck.GetDeclaredTypeOfSymbol(node.Symbol()))
	for _, p := range l.ck.GetPropertiesOfType(t) {
		for _, d := range p.Declarations {
			// Class storage (including private/protected brands) does not
			// turn a method interface extending that class into a data row.
			if d.Kind == ast.KindPropertySignature {
				return true
			}
		}
	}
	return false
}

// Use the instantiated interface's properties, never the utility declaration's
// generic parameters or the syntactic bases. Register the class before mapping
// fields so recursive references retain the declared shape's identity.
func (l *lowerer) gogenResolvedDataClass(node *ast.Node, c *hir.Class) bool {
	if !l.unprunedFeatures {
		return false
	}
	needed := node.AsInterfaceDeclaration().HeritageClauses != nil || l.gogenResolvedMembers(node)
	for _, m := range node.Members() {
		needed = needed || m.Kind == ast.KindMethodSignature
	}
	if !needed {
		return false
	}
	ctor := &hir.Method{Node: l.node(node), Name: "constructor", Result: hir.T(hir.Void)}
	c.Ctor = ctor
	t := l.ck.GetApparentType(l.ck.GetDeclaredTypeOfSymbol(node.Symbol()))
	var body []*hir.Stmt
	for _, p := range l.ck.GetPropertiesOfType(t) {
		at := node
		if len(p.Declarations) > 0 {
			at = p.Declarations[0]
		}
		saved := l.file
		l.file = ast.GetSourceFileOfNode(at)
		before := len(l.diags)
		pt := l.ck.GetTypeOfSymbol(p)
		var ft hir.Type
		if e, ok := l.overrides[node]; ok && e.Types[p.Name].Kind != "" {
			ft = e.Types[p.Name]
			l.diagf(at, "note-override", "%s: %s", e.ID, e.Rationale)
		} else if len(l.ck.GetSignaturesOfType(pt, checker.SignatureKindCall)) > 0 {
			ft = l.functionType(at, pt)
		} else if annotated, ok := l.propertyAnnotationOverride(p); ok {
			ft = annotated
		} else if family, ok := l.optionalFamilyMember(p, at); ok {
			ft = family
		} else if at.Kind == ast.KindPropertySignature && at.Type() != nil && l.ck.GetTypeFromTypeNode(at.Type()) == pt {
			// Preserve the annotation ABI unless generic instantiation has
			// actually changed the property type.
			ft = l.mapTypeNode(at.Type())
		} else {
			ft = l.mapCheckerType(at, pt)
		}
		if ft.Kind == hir.Void || hasBlocking(l.diags[before:]) {
			l.diags = l.diags[:before]
			if fallback, ok := l.dataMemberFallback(at, pt); ok {
				ft = fallback
				l.markAbsentOnly(c.Name, p.Name, pt)
			}
		}
		if ft.Kind == hir.Optional && ft.Args[0].Kind == hir.Dynamic && !hasBlocking(l.diags[before:]) {
			// `x?: undefined` maps directly but, like `x?: never`, only
			// ever reads undefined.
			l.markAbsentOnly(c.Name, p.Name, pt)
		}
		if ft.Kind == hir.Void || hasBlocking(l.diags[before:]) {
			l.diags = l.diags[:before]
			l.diagf(at, "skipped-interface-member", "data interface %s: member %s has unlowered types (%s)", c.Name, p.Name, l.ck.TypeToString(pt))
			l.file = saved
			continue
		}
		if (p.Flags&ast.SymbolFlagsOptional != 0 || l.literalPropertyAbsent(p)) && ft.Kind != hir.Optional {
			ft = hir.T(hir.Optional, ft)
		}
		field := hir.Field{Node: l.node(at), Name: p.Name, Type: ft, Readonly: shapePropertyReadonly(p)}
		c.Fields = append(c.Fields, field)
		if p.ValueDeclaration == nil || p.ValueDeclaration.Kind != ast.KindMethodDeclaration {
			// A class method inherited into the data view stays a method of
			// its class: registering it as a field would turn every call of
			// that method into a stored-function call.
			l.fields[p] = field
		}
		c.PrimitiveReads = c.PrimitiveReads || l.checkedFields[at] && (ft.Kind == hir.String || ft.Kind == hir.Number || ft.Kind == hir.Bool)
		ctor.Params = append(ctor.Params, hir.Param{Name: p.Name, Type: ft})
		body = append(body, &hir.Stmt{Kind: hir.Assign, Node: l.node(at), X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(at), Name: p.Name, Type: ft, X: &hir.Expr{Kind: hir.This, Node: c.Node, Type: hir.Ref(c.Name)}}, Y: hir.V(p.Name, ft)})
		l.file = saved
	}
	ctor.Body = hir.B(body...)
	l.classesByName[l.file.FileName()+" "+node.Symbol().Name] = c
	l.diagf(node, "note-data-interface", "interface %s with resolved data members lowered to a shape class", c.Name)
	return true
}

// dataMemberFallback types optional data members the general mapping
// cannot: a member that is only ever absent (`x?: never`, undefined) is an
// always-absent tagged slot, and an optional method or function member
// (`isDirty?(): boolean`) is an optional function value.
func (l *lowerer) dataMemberFallback(at *ast.Node, pt *checker.Type) (hir.Type, bool) {
	if !l.unprunedFeatures || pt == nil {
		return hir.Type{}, false
	}
	var present []*checker.Type
	parts := []*checker.Type{pt}
	if pt.IsUnion() {
		parts = pt.AsUnionOrIntersectionType().Types()
	}
	absent := false
	for _, part := range parts {
		if part.Flags()&(checker.TypeFlagsUndefined|checker.TypeFlagsVoid) != 0 {
			absent = true
			continue
		}
		if part.Flags()&checker.TypeFlagsNever != 0 {
			continue
		}
		present = append(present, part)
	}
	if len(present) == 0 && (absent || pt.Flags()&checker.TypeFlagsNever != 0) {
		return hir.T(hir.Optional, hir.T(hir.Dynamic)), true
	}
	if absent && len(present) == 1 && len(l.ck.GetSignaturesOfType(present[0], checker.SignatureKindCall)) > 0 {
		before := len(l.diags)
		fn := l.functionType(at, present[0])
		if fn.Kind == hir.Void || hasBlocking(l.diags[before:]) {
			l.diags = l.diags[:before]
			return hir.Type{}, false
		}
		return hir.T(hir.Optional, fn), true
	}
	return hir.Type{}, false
}

func (l *lowerer) markAbsentOnly(class, field string, pt *checker.Type) {
	if fb, ok := l.dataMemberFallback(nil, pt); ok && fb.Kind == hir.Optional && fb.Args[0].Kind == hir.Dynamic && len(l.ck.GetSignaturesOfType(pt, checker.SignatureKindCall)) == 0 {
		if l.absentOnlyFields == nil {
			l.absentOnlyFields = map[string]bool{}
		}
		l.absentOnlyFields[class+"."+field] = true
	}
}

// optionalFamilyMember maps `x?: F` for a recursive family alias F to
// Optional<F>. The checker's F | undefined spreads F's members into a fresh
// union that no longer names the alias (F = Meta & (A | B) distributes), so
// mapping that union would lose the family to the object root.
func (l *lowerer) optionalFamilyMember(p *ast.Symbol, at *ast.Node) (hir.Type, bool) {
	if !l.unprunedFeatures || p.Flags&ast.SymbolFlagsOptional == 0 || at.Kind != ast.KindPropertySignature || at.Type() == nil || at.Type().Kind != ast.KindTypeReference {
		return hir.Type{}, false
	}
	tt := l.ck.GetTypeFromTypeNode(at.Type())
	if tt == nil || !tt.IsUnion() && tt.Flags()&checker.TypeFlagsIntersection == 0 || l.familyAliasKey(tt) == "" {
		return hir.Type{}, false
	}
	before := len(l.diags)
	mapped, ok := l.recursiveFamilyType(at.Type(), tt)
	if !ok || mapped.Kind != hir.UnionRef || hasBlocking(l.diags[before:]) {
		l.diags = l.diags[:before]
		return hir.Type{}, false
	}
	return hir.T(hir.Optional, mapped), true
}
