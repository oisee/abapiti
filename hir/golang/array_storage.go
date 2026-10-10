package golang

import "github.com/oisee/abapiti/hir"

// The first emission uses erased reference arrays and records every value
// boundary. A leaf class can use pointer element storage only if no array of
// that class crosses a different static type or dynamic boundary anywhere.
// Otherwise all arrays of that class retain the existing shared any ABI.
func (e *emitter) blockArrayTypes(t hir.Type) {
	if t.Kind == hir.Array {
		elem := t.Args[0]
		if elem.Kind == hir.Optional {
			elem = elem.Args[0]
		}
		if elem.Kind == hir.ClassRef {
			e.arrayBlocked[elem.Name] = true
		}
	}
	for _, arg := range t.Args {
		e.blockArrayTypes(arg)
	}
}
func (e *emitter) arrayBoundary(src, dst hir.Type) {
	for src.Kind == hir.Optional {
		src = src.Args[0]
	}
	for dst.Kind == hir.Optional {
		dst = dst.Args[0]
	}
	if src.Equal(dst) {
		return
	}
	e.blockArrayTypes(src)
	e.blockArrayTypes(dst)
}
func (e *emitter) arrayStorage(t hir.Type) hir.Type {
	if t.Kind == hir.Array && t.Args[0].IsRef() {
		elem := t.Args[0]
		if elem.Kind == hir.Optional {
			elem = elem.Args[0]
		}
		if elem.Kind == hir.ClassRef && e.typedArrays[elem.Name] {
			return hir.T(hir.Array, elem)
		}
		return hir.T(hir.Array, hir.Ref(hir.RootObject))
	}
	return t
}
func (e *emitter) arrayMethodBoundary(a, b *hir.Method) {
	if a == nil || b == nil {
		return
	}
	e.arrayBoundary(a.Result, b.Result)
	for i := range a.Params {
		if i < len(b.Params) {
			e.arrayBoundary(a.Params[i].Type, b.Params[i].Type)
		}
	}
}
func (e *emitter) arrayABIBoundaries() {
	for _, c := range e.p.Classes {
		for _, iface := range c.Implements {
			for _, in := range e.p.Interfaces {
				if in.Name != iface {
					continue
				}
				for _, m := range in.Methods {
					actual, _ := e.method(c, m.Name)
					e.arrayMethodBoundary(actual, m)
				}
			}
		}
		if c.Super != "" {
			for _, m := range c.Methods {
				inherited, _ := e.method(e.classBy(c.Super), m.Name)
				e.arrayMethodBoundary(m, inherited)
			}
		}
	}
}
