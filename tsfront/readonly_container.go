package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Readonly is conversion provenance, not a physical layout distinction. Retain
// the checker types as well: HIR's nominal element names cannot prove TS
// covariance (especially admission into a structurally compatible union arm).
func (l *lowerer) rememberContainer(t *checker.Type, mapped hir.Type) hir.Type {
	if !l.unprunedFeatures || t == nil {
		return mapped
	}
	if mapped.Kind != hir.Array && mapped.Kind != hir.OrderedMap && mapped.Kind != hir.OrderedSet {
		return mapped
	}
	if sym := t.Symbol(); sym != nil && l.librarySymbol(sym) {
		mapped.Readonly = sym.Name == "ReadonlyArray" || sym.Name == "ReadonlyMap" || sym.Name == "ReadonlySet"
	}
	if l.containerCheckerTypes == nil {
		l.containerCheckerTypes = map[string][]*checker.Type{}
	}
	key := mapped.String()
	for _, old := range l.containerCheckerTypes[key] {
		if old == t {
			return mapped
		}
	}
	l.containerCheckerTypes[key] = append(l.containerCheckerTypes[key], t)
	return mapped
}

func (l *lowerer) markFreshContainer(x *hir.Expr) {
	if x == nil || x.Type.Kind != hir.Array && x.Type.Kind != hir.OrderedMap && x.Type.Kind != hir.OrderedSet {
		return
	}
	if l.freshContainers == nil {
		l.freshContainers = map[*hir.Expr]bool{}
	}
	l.freshContainers[x] = true
}

func (l *lowerer) isFreshContainer(x *hir.Expr) bool {
	if x == nil {
		return false
	}
	if l.freshContainers[x] {
		return true
	}
	if x.Kind == hir.Seq {
		return l.isFreshContainer(x.Y)
	}
	return x.Kind == hir.Local && l.localSymbols[x] != nil && l.freshArraySymbols[l.localSymbols[x]]
}

func (l *lowerer) freshContainerConversion(x *hir.Expr, dst hir.Type) *hir.Expr {
	if !l.unprunedFeatures || !l.isFreshContainer(x) || dst.Kind != x.Type.Kind {
		return nil
	}
	for _, target := range l.containerCheckerTypes[dst.String()] {
		for _, source := range l.containerCheckerTypes[x.Type.String()] {
			if l.ck.IsTypeAssignableTo(source, target) {
				return l.dynamicReference(l.coerce(x, hir.T(hir.Dynamic)), dst)
			}
		}
	}
	return nil
}
func (l *lowerer) readonlyContainerAssignable(dst, src hir.Type) bool {
	if !l.unprunedFeatures || !dst.Readonly || dst.Kind != src.Kind || dst.Equal(src) {
		return false
	}
	if dst.Kind != hir.Array && dst.Kind != hir.OrderedMap && dst.Kind != hir.OrderedSet {
		return false
	}
	if dst.Kind == hir.OrderedMap && !dst.Args[0].Equal(src.Args[0]) {
		return false
	}
	for _, d := range l.containerCheckerTypes[dst.String()] {
		if d.Symbol() == nil || (d.Symbol().Name != "ReadonlyMap" && d.Symbol().Name != "ReadonlySet" && d.Symbol().Name != "ReadonlyArray") {
			continue
		}
		for _, s := range l.containerCheckerTypes[src.String()] {
			if l.ck.IsTypeAssignableTo(s, d) {
				return true
			}
		}
	}
	return false
}

func (l *lowerer) hasReadonlyCovariance(dst, src hir.Type, seen map[string]bool) bool {
	if dst.Equal(src) {
		return false
	}
	if dst.Kind == hir.Optional {
		dst = dst.Args[0]
		if src.Kind == hir.Optional {
			src = src.Args[0]
		}
	}
	if l.readonlyContainerAssignable(dst, src) {
		if l.dataArraySubtype(dst, src) {
			return l.hasReadonlyCovariance(dst.Args[0], src.Args[0], seen)
		}
		return true
	}
	if dst.Kind != hir.ClassRef || src.Kind != hir.ClassRef {
		return false
	}
	key := dst.String() + "/" + src.String()
	if seen[key] {
		return false
	}
	seen[key] = true
	d, s := l.classByName(dst.Name), l.classByName(src.Name)
	if d == nil || s == nil {
		return false
	}
	for _, f := range d.Fields {
		for _, g := range s.Fields {
			if f.Name == g.Name && l.hasReadonlyCovariance(f.Type, g.Type, seen) {
				return true
			}
		}
	}
	return false
}

func (l *lowerer) recordedReadonlyContainerAssignable(dst, src hir.Type) bool {
	if dst.Kind != src.Kind || dst.Equal(src) {
		return false
	}
	if dst.Kind != hir.Array && dst.Kind != hir.OrderedMap && dst.Kind != hir.OrderedSet {
		return false
	}
	for _, target := range l.containerCheckerTypes[dst.String()] {
		if target.Symbol() == nil || (target.Symbol().Name != "ReadonlyMap" && target.Symbol().Name != "ReadonlySet" && target.Symbol().Name != "ReadonlyArray") {
			continue
		}
		for _, source := range l.containerCheckerTypes[src.String()] {
			if l.ck.IsTypeAssignableTo(source, target) {
				return true
			}
		}
	}
	return false
}

// The new checked covariance is read-only. Stop older array/Set subtype
// fallbacks from giving a writable target a narrower backing element domain.
// Identical structural layouts retain their existing invariant adapters.
func (l *lowerer) mutableContainerCovariance(dst, src hir.Type, seen map[string]bool) bool {
	if dst.Equal(src) {
		return false
	}
	if dst.Kind == hir.Optional {
		dst = dst.Args[0]
		if src.Kind == hir.Optional {
			src = src.Args[0]
		}
	}
	if dst.Kind != src.Kind {
		return false
	}
	switch dst.Kind {
	case hir.Array, hir.OrderedSet, hir.OrderedMap:
		a, b := dst.Args[len(dst.Args)-1], src.Args[len(src.Args)-1]
		// Existing erased and method-interface carriers already share storage;
		// this guard concerns new views between different data element domains.
		if a.Kind == hir.Dynamic || b.Kind == hir.Dynamic || a.Equal(hir.Ref(hir.RootObject)) || b.Equal(hir.Ref(hir.RootObject)) {
			return false
		}
		if a.Kind == hir.InterfaceRef && len(l.out.StructuralUnion(a.Name)) == 0 || b.Kind == hir.InterfaceRef && len(l.out.StructuralUnion(b.Name)) == 0 {
			return false
		}
		for _, d := range l.containerCheckerTypes[dst.String()] {
			for _, s := range l.containerCheckerTypes[src.String()] {
				if l.ck.IsTypeAssignableTo(d, s) && l.ck.IsTypeAssignableTo(s, d) {
					return false
				}
			}
		}
		if dst.Args[len(dst.Args)-1].Equal(src.Args[len(src.Args)-1]) {
			return false
		}
		if l.recordedReadonlyContainerAssignable(dst, src) {
			return false
		}
		if !dst.Readonly && !dst.Equal(src) && !hir.StructuralStorageEquivalent(l.out, dst.Args[len(dst.Args)-1], src.Args[len(src.Args)-1], l.shapeLike) {
			return true
		}
		return false
	case hir.ClassRef:
		key := dst.String() + "/" + src.String()
		if seen[key] {
			return false
		}
		seen[key] = true
		d, s := l.classByName(dst.Name), l.classByName(src.Name)
		if d == nil || s == nil {
			return false
		}
		for _, f := range d.Fields {
			for _, g := range s.Fields {
				if f.Name == g.Name && l.mutableContainerCovariance(f.Type, g.Type, seen) {
					return true
				}
			}
		}
	}
	return false
}

func (l *lowerer) readonlyContainerConversion(x *hir.Expr, dst hir.Type) *hir.Expr {
	if dst.Kind == hir.Optional && len(dst.Args) == 1 {
		src := x.Type
		if src.Kind == hir.Optional {
			src = src.Args[0]
		}
		if l.readonlyContainerAssignable(dst.Args[0], src) && !l.dataArraySubtype(dst.Args[0], src) {
			if x.Type.Kind != hir.Optional {
				return l.coerce(l.coerce(x, dst.Args[0]), dst)
			}
			saved := l.tempInit(nil, x.Type, x)
			present := &hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: src, X: saved}
			return &hir.Expr{Kind: hir.Conditional, Node: x.Node, Type: dst,
				X: &hir.Expr{Kind: hir.IsUndefined, Node: x.Node, Type: hir.T(hir.Bool), X: saved},
				Y: &hir.Expr{Kind: hir.Lit, Node: x.Node, Type: dst}, Z: l.coerce(l.coerce(present, dst.Args[0]), dst)}
		}
	}
	if l.readonlyContainerAssignable(dst, x.Type) && !l.dataArraySubtype(dst, x.Type) {
		return l.rtOp("container.readonlyView", x, dst)
	}
	return nil
}
