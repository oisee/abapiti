package tsfront

import "github.com/oisee/abapiti/hir"

// Reference Set views share the original key storage. Reads use the same
// checked data views as covariant reference arrays; membership keeps identity.
func (l *lowerer) setReferenceView(dst, src hir.Type) bool {
	if !l.unprunedFeatures || dst.Kind != hir.OrderedSet || src.Kind != hir.OrderedSet || len(dst.Args) != 1 || len(src.Args) != 1 || dst.Args[0].Kind != hir.ClassRef || !l.shapeLike[dst.Args[0].Name] {
		return false
	}
	if l.dataShapeSubtype(dst.Args[0], src.Args[0]) {
		return true
	}
	if src.Args[0].Kind == hir.InterfaceRef {
		for _, u := range l.unions {
			if u.iface.Name != src.Args[0].Name {
				continue
			}
			for _, part := range u.parts {
				if !l.dataShapeSubtype(dst.Args[0], part) && !dst.Args[0].Equal(part) {
					return false
				}
			}
			return len(u.parts) > 0
		}
	}
	return false
}
