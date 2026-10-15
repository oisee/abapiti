package tsfront

import "github.com/oisee/abapiti/hir"

// Property-only structural compatibility. Container storage remains invariant;
// a present primitive or reference may widen to its optional slot through the live
// adapter. Absent writes into required backing storage still explicitly refuse.
// Native classes/configs and nested row/array covariance are not admitted.
func (l *lowerer) dataShapeSubtype(dst, src hir.Type) bool {
	if dst.Kind != hir.ClassRef || src.Kind != hir.ClassRef || dst.Equal(src) || !l.shapeLike[dst.Name] || !l.shapeLike[src.Name] {
		return false
	}
	d, s := l.classByName(dst.Name), l.classByName(src.Name)
	if d == nil || s == nil || len(d.Methods) != 0 || len(s.Methods) != 0 || d.Super != "" || s.Super != "" {
		return false
	}
	for _, f := range d.Fields {
		if f.Static {
			return false
		}
		found := false
		for _, g := range s.Fields {
			compatible := f.Type.Equal(g.Type)
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
		if !found {
			return false
		}
	}
	return true
}
