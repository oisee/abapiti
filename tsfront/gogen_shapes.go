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
			if d.Kind == ast.KindPropertySignature || d.Kind == ast.KindPropertyDeclaration {
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
		} else {
			ft = l.mapCheckerType(at, pt)
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
