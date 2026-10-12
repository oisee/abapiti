package tsfront

import "github.com/oisee/abapiti/hir"

// A deliberately invariant, property-only structural compatibility proof.
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
			if !g.Static && f.Name == g.Name && f.Type.Equal(g.Type) {
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
