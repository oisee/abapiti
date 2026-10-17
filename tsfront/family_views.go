package tsfront

import "github.com/oisee/abapiti/hir"

// A structural family view must accept every source tag and literal domain.
// Readonly recursive fields are compared coinductively; mutable Map/Set slots
// keep their invariant layouts. Admission still validates the live value.
func (l *lowerer) familyViewSubtype(dst, src hir.Type, active map[string]bool) bool {
	d := l.out.Union(dst.Name)
	if dst.Kind != hir.UnionRef || d == nil || !d.Complete {
		return false
	}
	key := "family:" + dst.Name + "\x00" + src.String()
	if active[key] {
		return true
	}
	active[key] = true
	defer delete(active, key)
	var alternatives []hir.UnionAlternative
	if src.Kind == hir.UnionRef {
		s := l.out.Union(src.Name)
		if s == nil || !s.Complete || d.Discriminant != s.Discriminant {
			return false
		}
		alternatives = s.Alternatives
	} else if src.Kind == hir.ClassRef {
		for _, s := range l.out.Unions {
			if !s.Complete || s.Discriminant != d.Discriminant {
				continue
			}
			for _, a := range s.Alternatives {
				if a.Payload.Equal(src) {
					alternatives = append(alternatives, a)
				}
			}
		}
	} else {
		return false
	}
	if len(alternatives) == 0 {
		return false
	}
	for _, a := range alternatives {
		found := false
		for _, b := range d.Alternatives {
			if !familyViewDomain(b.Tags, a.Tags) {
				continue
			}
			literals := true
			for _, want := range b.Literals {
				match := false
				for _, have := range a.Literals {
					if want.Field == have.Field && familyViewDomain(want.Values, have.Values) {
						match = true
						break
					}
				}
				if !match {
					literals = false
					break
				}
			}
			if literals && l.readonlyShapeRead(b.Payload, a.Payload, active) {
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
func familyViewDomain(want, have []string) bool {
	for _, v := range have {
		found := false
		for _, w := range want {
			if v == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return len(have) > 0
}
