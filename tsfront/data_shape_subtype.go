package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Property-only structural compatibility. Container storage remains invariant;
// a present primitive or reference may widen to its optional slot through the live
// adapter. Absent writes into required backing storage still explicitly refuse.
// Native classes/configs remain nominal. Readonly nested slots may use an
// equivalent structural layout through the existing live array/data views.
func (l *lowerer) dataShapeSubtype(dst, src hir.Type) bool {
	return l.dataShapeSubtypeSeen(dst, src, map[string]bool{})
}

func (l *lowerer) dataShapeSubtypeSeen(dst, src hir.Type, active map[string]bool) bool {
	if dst.Kind != hir.ClassRef || src.Kind != hir.ClassRef || dst.Equal(src) || !l.shapeLike[dst.Name] || !l.shapeLike[src.Name] {
		return false
	}
	d, s := l.classByName(dst.Name), l.classByName(src.Name)
	if d == nil || s == nil || len(d.Methods) != 0 || len(s.Methods) != 0 || d.Super != "" || s.Super != "" {
		return false
	}
	key := dst.Name + "\x00" + src.Name
	if active[key] {
		return true
	}
	active[key] = true
	defer delete(active, key)
	for _, f := range d.Fields {
		if f.Static {
			return false
		}
		found := false
		for _, g := range s.Fields {
			if g.Static || f.Name != g.Name {
				continue
			}
			compatible := f.Type.Equal(g.Type) || l.gogenOptionalDynamicField(f.Type, g.Type)
			if g.Type.Kind == hir.Optional && len(g.Type.Args) == 1 && f.Type.Equal(g.Type.Args[0]) && l.widenedRequiredProperty(s, g.Name) {
				compatible = true
			}
			if f.Readonly && (hir.UnionReadAccepts(l.out, f.Type, g.Type) || hir.StructuralStorageEquivalent(l.out, f.Type, g.Type, l.shapeLike) || l.readonlyShapeRead(f.Type, g.Type, active)) {
				compatible = true
			}
			if l.dataShapeSubtypeSeen(f.Type, g.Type, active) {
				// A nested record of a property-only subtype (QueryContext's
				// program: QueryProgram over CompilationContext's
				// FrontendProgram): reads view the same live nested record.
				compatible = true
			}
			if f.Type.Kind == hir.Optional && len(f.Type.Args) == 1 && l.dataShapeSubtypeSeen(f.Type.Args[0], g.Type, active) {
				// A present nested record viewed through an optional slot of a
				// property-only supertype: the same live nested view.
				compatible = true
			}
			if f.Type.Kind == hir.Dynamic || f.Type.Kind == hir.Optional && f.Type.Args[0].Equal(g.Type) {
				if f.Type.Kind == hir.Optional && g.Type.IsRef() {
					compatible = true
				}
				switch g.Type.Kind {
				case hir.String, hir.Number, hir.I32, hir.I64, hir.Bool:
					compatible = true
				}
			}
			if !g.Static && f.Name == g.Name && compatible {
				found = true
				break
			}
		}
		if !found && !l.gogenAbsentShapeField(f, s) {
			return false
		}
	}
	return true
}

func (l *lowerer) mapStorageView(dst, src hir.Type) bool {
	if !l.unprunedFeatures || dst.Kind != hir.OrderedMap || src.Kind != hir.OrderedMap || len(dst.Args) != 2 || len(src.Args) != 2 {
		return false
	}
	if dst.Args[0].Kind == hir.Dynamic && src.Args[0].Kind == hir.String && dst.Args[1].Equal(src.Args[1]) {
		return true
	}
	if dst.Args[1].Kind != hir.ClassRef || src.Args[1].Kind != hir.ClassRef || !dst.Args[0].Equal(src.Args[0]) || dst.Args[1].Equal(src.Args[1]) {
		return false
	}
	return hir.StructuralStorageEquivalent(l.out, dst.Args[1], src.Args[1], l.shapeLike)
}

func (l *lowerer) structuralEqualityTypes(a, b hir.Type) bool {
	if a.Kind == hir.Optional {
		a = a.Args[0]
	}
	if b.Kind == hir.Optional {
		b = b.Args[0]
	}
	return l.dataShapeSubtype(a, b) || l.dataShapeSubtype(b, a) || l.mapStorageView(a, b) || l.mapStorageView(b, a)
}

// Required checker properties may carry optional primitive storage because an
// erased initializer can supply undefined. A live required view checks the
// primitive when read, without copying or inventing a value for that slot.
func (l *lowerer) widenedRequiredProperty(c *hir.Class, name string) bool {
	for sym, cls := range l.classes {
		if cls != c {
			continue
		}
		prop := l.ck.GetPropertyOfType(l.ck.GetDeclaredTypeOfSymbol(sym), name)
		return prop != nil && prop.Flags&ast.SymbolFlagsOptional == 0 && l.literalPropertyAbsent(prop)
	}
	return false
}

// Readonly slots may recursively widen data properties. Mutable nested
// containers retain their existing invariant storage compatibility checks.
func (l *lowerer) readonlyShapeRead(dst, src hir.Type, active map[string]bool) bool {
	if dst.Equal(src) {
		return true
	}
	if dst.Kind == hir.Optional && len(dst.Args) == 1 {
		if src.Kind == hir.Optional && len(src.Args) == 1 {
			src = src.Args[0]
		}
		return l.readonlyShapeRead(dst.Args[0], src, active)
	}
	if dst.Kind == hir.Array && src.Kind == hir.Array && len(dst.Args) == 1 && len(src.Args) == 1 {
		return l.readonlyShapeRead(dst.Args[0], src.Args[0], active)
	}
	if dst.Kind == hir.UnionRef {
		return l.familyViewSubtype(dst, src, active)
	}
	return l.dataShapeSubtypeSeen(dst, src, active)
}
