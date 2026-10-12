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

// unionFieldAccess proves a shared data slot using the flow type, rather than
// trusting a transient union property symbol or casting to one constituent.
// The existing live data-class ABI reads the original storage through the view.
func (l *lowerer) unionFieldAccess(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	p := n.AsPropertyAccessExpression()
	t := l.ck.GetTypeAtLocation(p.Expression)
	if t == nil || !t.IsUnion() {
		return nil, false
	}
	for _, part := range t.AsUnionOrIntersectionType().Types() {
		if part.Flags()&checker.TypeFlagsObject == 0 {
			return nil, false
		}
		// Fresh literal collection rows have a separate proof and physical
		// slot layout. A common-field read must neither replace that layout
		// nor make an unproven registration initializer executable.
		if sym := part.Symbol(); sym != nil && sym.ValueDeclaration != nil && sym.ValueDeclaration.Kind == ast.KindObjectLiteralExpression {
			return nil, false
		}
	}
	name := n.Name().Text()
	if name == "constructor" {
		// Native constructor metadata is inherited, not a common data slot.
		native := true
		for _, part := range t.AsUnionOrIntersectionType().Types() {
			typ := l.mapCheckerType(n, part)
			c := l.classByName(typ.Name)
			if typ.Kind == hir.InterfaceRef {
				if strings.HasPrefix(typ.Name, "closure.") {
					l.diagf(n, "unsupported-expr", "constructor metadata of callable unions is unsupported")
					return nil, true
				}
				// An explicitly declared constructor method is not the inherited
				// native descriptor. Otherwise method interfaces carry the original
				// runtime object; the descriptor operation refuses nonnative values.
				for _, iface := range l.out.Interfaces {
					if iface.Name == typ.Name {
						for _, method := range iface.Methods {
							if method.Name == name {
								native = false
							}
						}
					}
				}
				continue
			}
			// Structural is finalized after bodies are lowered; the existing
			// shape registry is authoritative at this earlier phase.
			if typ.Kind != hir.ClassRef || c == nil || c.Structural || l.shapeLike[c.Name] || strings.HasPrefix(c.Name, "shape.") {
				native = false
				break
			}
			for base := c; base != nil; base = l.classByName(base.Super) {
				for _, f := range base.Fields {
					if f.Name == name && !f.Static {
						native = false
					}
				}
			}
		}
		if native {
			return nil, false
		}
	}
	var fields []hir.Field
	var classes []*hir.Class
	structural := 0
	for _, part := range t.AsUnionOrIntersectionType().Types() {
		prop := l.ck.GetPropertyOfType(part, name)
		if prop == nil {
			l.diagf(n, "unsupported-expr", "union field %s requires narrowing: not present in every member", name)
			return nil, true
		}
		if prop.Flags&ast.SymbolFlagsMethod != 0 {
			return nil, false
		}
		typ := l.mapCheckerType(n, part)
		c := l.classByName(typ.Name)
		if typ.Kind != hir.ClassRef || c == nil {
			return nil, false
		}
		classes = append(classes, c)
		if l.shapeLike[c.Name] || strings.HasPrefix(c.Name, "shape.") {
			structural++
		}
		var field *hir.Field
		for base := c; base != nil; base = l.classByName(base.Super) {
			for i := range base.Fields {
				f := &base.Fields[i]
				if f.Name == name && !f.Static && !f.Private {
					field = f
					break
				}
			}
			if field != nil {
				break
			}
		}
		if field == nil {
			l.diagf(n, "unsupported-expr", "union field %s requires a public data slot in every member", name)
			return nil, true
		}
		fields = append(fields, *field)
	}
	if len(fields) < 2 {
		return nil, false
	}
	if ast.IsAssignmentTarget(n) {
		l.diagf(n, "unsupported-assignment", "union field %s write requires narrowing", name)
		return nil, true
	}
	result := hir.T(hir.Void)
	for _, candidate := range fields {
		compatible := true
		for _, f := range fields {
			compatible = compatible && l.unionFieldReadAccepts(candidate.Type, f.Type)
		}
		if compatible {
			result = candidate.Type
			break
		}
	}
	if result.Kind == hir.Void {
		l.diagf(n, "unsupported-expr", "union field %s has incompatible member types", name)
		return nil, true
	}
	if structural == 0 {
		return l.nativeUnionField(n, classes, fields, result), true
	}
	if structural != len(classes) {
		l.diagf(n, "unsupported-expr", "union field %s mixes native and structural storage; requires narrowing", name)
		return nil, true
	}
	// A common read type need not be a safe mutable type for every member.
	// Keep this view read-only; writes require checker narrowing to a real slot.
	key := fmt.Sprintf("union.fields.%x", sha256.Sum256([]byte(name+":"+result.String())))
	view := l.classByName(key)
	if view == nil {
		view = &hir.Class{Node: l.node(n), Name: key, Structural: true,
			Fields: []hir.Field{{Node: l.node(n), Name: name, Type: result, Readonly: true}}}
		l.out.Classes = append(l.out.Classes, view)
		if l.shapeLike == nil {
			l.shapeLike = map[string]bool{}
		}
		l.shapeLike[key] = true
	}
	oldHint := l.hint
	l.hint = hir.Type{}
	recv := l.expr(p.Expression)
	l.hint = oldHint
	if recv == nil {
		return nil, true
	}
	recv = l.dynamicReference(l.coerce(recv, hir.T(hir.Dynamic)), hir.Ref(key))
	return &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: name, Type: result, X: recv}, true
}

// Container element widening would also expose writes through the returned
// alias. Retain invariant container storage; optional presence may still widen.
func (l *lowerer) unionFieldReadAccepts(dst, src hir.Type) bool {
	a, b := dst, src
	if a.Kind == hir.Optional {
		a = a.Args[0]
	}
	if b.Kind == hir.Optional {
		b = b.Args[0]
	}
	switch a.Kind {
	case hir.Array, hir.OrderedMap, hir.OrderedSet:
		if !a.Equal(b) {
			return false
		}
	}
	return l.acceptsType(dst, src) || l.dataShapeSubtype(dst, src)
}

// Native objects share a nominal getter interface. Each forwarder reads its
// real field; there is no structural projection or fabricated class instance.
func (l *lowerer) nativeUnionField(n *ast.Node, classes []*hir.Class, fields []hir.Field, result hir.Type) *hir.Expr {
	name := n.Name().Text()
	names := make([]string, len(classes))
	for i, c := range classes {
		names[i] = c.Name
	}
	sort.Strings(names)
	key := fmt.Sprintf("union.field.read.%x", sha256.Sum256([]byte(strings.Join(names, "|")+":"+name+":"+result.String())))
	view := hir.Type{Kind: hir.InterfaceRef, Name: key}
	method := key + ".get"
	exists := false
	for _, i := range l.out.Interfaces {
		if i.Name == key {
			exists = true
			break
		}
	}
	if !exists {
		l.out.Interfaces = append(l.out.Interfaces, &hir.Interface{Node: l.node(n), Name: key, Methods: []*hir.Method{{Node: l.node(n), Name: method, Result: result}}})
		seen := map[string]bool{}
		for i, c := range classes {
			if seen[c.Name] {
				continue
			}
			seen[c.Name] = true
			value := &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: name, Type: fields[i].Type, X: &hir.Expr{Kind: hir.This, Node: l.node(n), Type: hir.Ref(c.Name)}}
			c.Methods = append(c.Methods, &hir.Method{Node: l.node(n), Name: method, Virtual: true, Result: result, Body: hir.B(&hir.Stmt{Kind: hir.Return, Node: l.node(n), X: l.coerce(value, result)})})
			l.recordImplements(hir.Ref(c.Name), view)
		}
		if l.abiReady {
			l.covariantImplements()
		}
	}
	oldHint := l.hint
	l.hint = hir.Type{}
	recv := l.expr(n.AsPropertyAccessExpression().Expression)
	l.hint = oldHint
	if recv == nil {
		return nil
	}
	return &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), Name: method, Type: result, X: &hir.Expr{Kind: hir.Cast, Node: l.node(n), Type: view, X: recv}}
}
