package hir

// MapViewCompatible permits equivalent value layouts and boxed queries over
// string-keyed storage. Other key changes and nested containers stay invariant.
// Backends must separately implement value-based boxed-key hashing. String-key
// query views keep their original storage and cannot accept non-string writes.
func MapViewCompatible(p *Program, dst, src Type) bool {
	if dst.Kind != OrderedMap || src.Kind != OrderedMap || len(dst.Args) != 2 || len(src.Args) != 2 {
		return false
	}
	if dst.Args[0].Kind == Dynamic && src.Args[0].Kind == String && dst.Args[1].Equal(src.Args[1]) {
		return true
	}
	if !dst.Args[0].Equal(src.Args[0]) {
		return false
	}
	return dst.Args[1].Kind == ClassRef && src.Args[1].Kind == ClassRef && StructuralStorageEquivalent(p, dst.Args[1], src.Args[1], nil)
}

// StructuralStorageEquivalent compares finite storage graphs coinductively.
// Only data layouts and arrays of equivalent slots may change names; Map/Set
// storage and primitive representations stay invariant. During lowering the
// registry supplies structural layouts whose final flags are not set yet.
func StructuralStorageEquivalent(p *Program, a, b Type, layouts map[string]bool) bool {
	if p == nil {
		return false
	}
	classes := map[string]*Class{}
	for _, c := range p.Classes {
		classes[c.Name] = c
	}
	type pair struct{ a, b string }
	seen := map[pair]bool{}
	var same func(Type, Type) bool
	same = func(a, b Type) bool {
		if a.Equal(b) {
			return true
		}
		if a.Kind != b.Kind {
			return false
		}
		if (a.Kind == Array || a.Kind == Optional) && len(a.Args) == 1 && len(b.Args) == 1 {
			return same(a.Args[0], b.Args[0])
		}
		if a.Kind == UnionRef {
			return UnionIncludes(p.Union(a.Name), p.Union(b.Name)) && UnionIncludes(p.Union(b.Name), p.Union(a.Name))
		}
		if a.Kind != ClassRef {
			return false
		}
		d, s := classes[a.Name], classes[b.Name]
		if d == nil || s == nil || d.Super != "" || s.Super != "" || len(d.Methods) != 0 || len(s.Methods) != 0 || len(d.Fields) != len(s.Fields) {
			return false
		}
		structural := d.Structural && s.Structural
		if layouts != nil {
			structural = layouts[d.Name] && layouts[s.Name]
		}
		if !structural || d.ViewUnsafe || s.ViewUnsafe {
			return false
		}
		key := pair{a.Name, b.Name}
		if seen[key] {
			return true
		}
		seen[key] = true
		for _, f := range d.Fields {
			if f.Static || f.Private {
				return false
			}
			found := false
			for _, g := range s.Fields {
				if f.Name == g.Name && !g.Static && !g.Private && same(f.Type, g.Type) {
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
	return same(a, b)
}
