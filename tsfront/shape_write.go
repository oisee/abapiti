package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Property symbols can describe a structural checker view rather than the
// receiver's nominal layout. Existing fields keep their typed setter; erased
// properties use the original record's checked dynamic storage adapter.
func (l *lowerer) shapeWriteTarget(at *ast.Node, recv *hir.Expr, name string) *hir.Expr {
	if name == "__proto__" {
		return nil
	}
	if recv.Type.Kind == hir.Optional {
		recv = &hir.Expr{Kind: hir.Narrow, Node: l.node(at), Type: recv.Type.Args[0], X: recv}
	}
	if recv.Type.Kind == hir.OrderedMap && recv.Type.Args[0].Kind == hir.String {
		return l.rtOp("map.set", recv, recv.Type, hir.L(hir.T(hir.String), name), nil)
	}
	if recv.Type.Kind == hir.ClassRef {
		for c := l.classByName(recv.Type.Name); c != nil; c = l.classByName(c.Super) {
			for _, f := range c.Fields {
				if f.Name == name && !f.Static {
					return &hir.Expr{Kind: hir.FieldGet, Node: l.node(at), Name: name, Type: f.Type, X: recv}
				}
			}
		}
		c := l.classByName(recv.Type.Name)
		if c == nil && recv.Type.Name != hir.RootObject {
			return nil
		}
	} else if recv.Type.Kind != hir.Dynamic {
		return nil
	}
	return l.rtOp("dynamic.put", l.coerce(recv, hir.T(hir.Dynamic)), hir.T(hir.Void), hir.L(hir.T(hir.String), name), nil)
}
