package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// collectionReferenceBase joins nominal references without depending on the
// checker's structural union reduction (which can discard sibling classes).
func (l *lowerer) collectionReferenceBase(parts []hir.Type) (hir.Type, bool) {
	if len(parts) == 0 {
		return hir.Type{}, false
	}
	base := parts[0]
	for _, part := range parts {
		if part.Kind != hir.ClassRef && part.Kind != hir.InterfaceRef {
			return hir.Type{}, false
		}
	}
	for _, part := range parts[1:] {
		if base.Equal(part) {
			continue
		}
		if base.Kind != hir.ClassRef || part.Kind != hir.ClassRef {
			base = hir.Type{}
			break
		}
		name := l.commonAncestor(base.Name, part.Name)
		if name == "" {
			base = hir.Type{}
			break
		}
		base = hir.Ref(name)
	}
	if base.Name != "" {
		return base, true
	}
	// An existing reference constituent may itself be the shared interface.
	for _, candidate := range parts {
		accepts := true
		for _, part := range parts {
			accepts = accepts && l.acceptsType(candidate, part)
		}
		if accepts {
			return candidate, true
		}
	}
	if iface, ok := l.commonInterfaceByNames(parts); ok {
		candidate := hir.Type{Kind: hir.InterfaceRef, Name: iface.Name}
		for _, part := range parts {
			if !l.acceptsType(candidate, part) {
				return hir.Type{}, false
			}
		}
		return candidate, true
	}
	return hir.Type{}, false
}

// inferredCollectionType preserves the checker's row layout but joins the
// actual nominal types of every fresh element/slot. Explicit annotations and
// contextual types are handled by callers before invoking this inference.
func (l *lowerer) inferredCollectionType(n *ast.Node, typ hir.Type) hir.Type {
	if !l.unprunedFeatures || n == nil || n.Kind != ast.KindArrayLiteralExpression || typ.Kind != hir.Array {
		return typ
	}
	els := n.AsArrayLiteralExpression().Elements.Nodes
	if len(els) == 0 {
		return typ
	}
	if tuple, ok := l.collectionTupleSlots(n, els); ok {
		return hir.T(hir.Array, tuple)
	}
	rows := true
	for _, el := range els {
		rows = rows && el.Kind == ast.KindObjectLiteralExpression
		if el.Kind == ast.KindObjectLiteralExpression {
			for _, prop := range el.AsObjectLiteralExpression().Properties.Nodes {
				if (prop.Kind != ast.KindPropertyAssignment && prop.Kind != ast.KindShorthandPropertyAssignment) || prop.Name() == nil ||
					(prop.Name().Kind != ast.KindIdentifier && prop.Name().Kind != ast.KindStringLiteral && prop.Name().Kind != ast.KindNumericLiteral) {
					return typ
				}
			}
		}
	}
	if !rows {
		var elements []hir.Type
		for _, el := range els {
			t := l.ck.GetTypeAtLocation(el)
			if el.Kind == ast.KindSpreadElement {
				info := l.ck.GetIndexInfoOfType(l.ck.GetTypeAtLocation(el.Expression()), l.ck.GetNumberType())
				if info == nil {
					return typ
				}
				t = info.ValueType()
			}
			elements = append(elements, l.collectionValueType(el, t))
		}
		if base, ok := l.collectionElementType(n, elements); ok {
			for _, element := range elements {
				if !base.Equal(element) {
					if l.collectionElements == nil {
						l.collectionElements = map[string]bool{}
					}
					l.collectionElements[base.String()] = true
					break
				}
			}
			return hir.T(hir.Array, base)
		}
		return typ
	}
	array := l.ck.GetTypeAtLocation(n)
	info := l.ck.GetIndexInfoOfType(array, l.ck.GetNumberType())
	if info == nil {
		return typ
	}
	row := info.ValueType()
	props := l.ck.GetPropertiesOfType(row)
	if row.IsUnion() {
		props = l.ck.GetAllPossiblePropertiesOfTypes(row.AsUnionOrIntersectionType().Types())
	}
	slots := map[string]hir.Type{}
	for _, prop := range props {
		var values []hir.Type
		for _, el := range els {
			for _, p := range el.AsObjectLiteralExpression().Properties.Nodes {
				if p.Name().Text() == prop.Name {
					value := p.Initializer()
					if p.Kind == ast.KindShorthandPropertyAssignment {
						value = p.Name()
					}
					values = append(values, l.collectionValueType(value, l.ck.GetTypeAtLocation(value)))
				}
			}
		}
		if base, ok := l.collectionElementType(n, values); ok {
			for _, value := range values {
				if !base.Equal(value) {
					slots[prop.Name] = base
					break
				}
			}
		}
	}
	if len(slots) != 0 {
		if shape := l.synthFromPropertyTypes(n, props, slots); shape != nil {
			l.rememberCollectionShape(shape.Name)
			return hir.T(hir.Array, hir.Ref(shape.Name))
		}
	}
	return typ
}

func (l *lowerer) rememberCollectionShape(name string) {
	if l.collectionShapes == nil {
		l.collectionShapes = map[string]bool{}
	}
	l.collectionShapes[name] = true
}

// Tuple rows keep positional storage. Join each slot independently, rather
// than copying the first tuple's concrete class layout to all later rows.
func (l *lowerer) collectionTupleSlots(n *ast.Node, rows []*ast.Node) (hir.Type, bool) {
	var slots [][]hir.Type
	for rowIndex, row := range rows {
		t := l.ck.GetTypeAtLocation(row)
		if t == nil || !t.IsTupleType() {
			return hir.Type{}, false
		}
		count := 0
		for ; ; count++ {
			prop := l.ck.GetPropertyOfType(t, itoa(count))
			if prop == nil {
				break
			}
			if rowIndex == 0 {
				slots = append(slots, nil)
			}
			if count >= len(slots) {
				return hir.Type{}, false
			}
			slots[count] = append(slots[count], l.collectionValueType(row, l.ck.GetTypeOfSymbol(prop)))
		}
		if count != len(slots) {
			return hir.Type{}, false
		}
	}
	var elems []hir.Type
	changed := false
	for _, parts := range slots {
		elem := parts[0]
		if base, ok := l.collectionElementType(n, parts); ok {
			elem = base
		}
		for _, part := range parts {
			if !elem.Equal(part) {
				if !l.acceptsType(elem, part) {
					return hir.Type{}, false
				}
				changed = true
			}
		}
		elems = append(elems, elem)
	}
	if !changed {
		return hir.Type{}, false
	}
	tuple := l.tupleShape(n, elems)
	l.rememberCollectionShape(tuple.Name)
	return tuple, true
}

func (l *lowerer) collectionElementType(n *ast.Node, parts []hir.Type) (hir.Type, bool) {
	optional := false
	var refs []hir.Type
	for _, part := range parts {
		if part.Kind == hir.Optional && len(part.Args) == 1 {
			optional = true
			part = part.Args[0]
			if part.Kind == hir.Void {
				continue // literal undefined contributes absence, not a class
			}
		}
		if part.Kind != hir.ClassRef && part.Kind != hir.InterfaceRef {
			return hir.Type{}, false
		}
		refs = append(refs, part)
	}
	if len(refs) == 0 {
		return hir.Type{}, false
	}
	if base, ok := l.collectionReferenceBase(refs); ok {
		if optional {
			base = hir.T(hir.Optional, base)
		}
		return base, true
	}
	// union also constructs a checked shared method interface when the
	// constituents have no declared common nominal ancestor/interface.
	base := l.union(n, refs, optional)
	return base, base.Kind != hir.Void
}

func (l *lowerer) collectionValueType(n *ast.Node, t *checker.Type) hir.Type {
	if t != nil && t.Flags()&checker.TypeFlagsUndefined != 0 {
		// Inference-only absence marker; it is never used as physical storage.
		return hir.T(hir.Optional, hir.T(hir.Void))
	}
	return l.mapCheckerType(n, t)
}

func (l *lowerer) inferredDeclarationCollection(d *ast.Node, typ hir.Type) hir.Type {
	if d.Type() != nil || d.Initializer() == nil || l.ck.GetContextualType(d.Initializer(), 0) != nil {
		return typ
	}
	return l.inferredCollectionType(d.Initializer(), typ)
}

func (l *lowerer) inferCollectionFields(f *ast.SourceFile) {
	for _, node := range l.statementNodes(f) {
		if node.Kind != ast.KindClassDeclaration {
			continue
		}
		c := l.classOf(node.Symbol())
		if c == nil {
			continue
		}
		for _, member := range node.Members() {
			if member.Kind != ast.KindPropertyDeclaration || member.Name() == nil || member.Name().Kind != ast.KindIdentifier {
				continue
			}
			name := member.Name().Text()
			if override, ok := l.overrides[node]; ok && override.Types[name].Kind != "" {
				continue
			}
			for i := range c.Fields {
				field := &c.Fields[i]
				if field.Name != name {
					continue
				}
				field.Type = l.inferredDeclarationCollection(member, field.Type)
				if sym := member.Symbol(); sym != nil {
					l.fields[sym] = *field
					if sym.Parent != nil {
						l.fieldsBy[f.FileName()+" "+sym.Parent.Name+"."+name] = *field
					}
				}
			}
		}
	}
}
