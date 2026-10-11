package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Empty literals allocate the same array as nonempty literals. Their element
// type comes from the surrounding expression, never from a nonexistent element.
func (l *lowerer) emptyArrayLiteral(n *ast.Node) *hir.Expr {
	typ := l.hint
	if typ.Kind == hir.Optional && len(typ.Args) == 1 {
		typ = typ.Args[0]
	}
	if typ.Kind != hir.Array && typ.Kind != hir.OrderedSet {
		if t := l.ck.GetContextualType(n, 0); t != nil {
			if arr := l.singleArrayConstituent(t); arr != nil {
				t = arr
			}
			typ = l.mapCheckerType(n, t)
		}
		if typ.Kind == hir.Optional && len(typ.Args) == 1 {
			typ = typ.Args[0]
		}
	}
	if typ.Kind == hir.Dynamic {
		// An any/unknown context accepts arrays of tagged elements. Construct
		// a real fresh array; the enclosing expression handles its boxing.
		typ = hir.T(hir.Array, hir.T(hir.Dynamic))
	}
	if typ.Kind == hir.Array || typ.Kind == hir.OrderedSet {
		return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ}
	}
	l.diagf(n, "unsupported-array", "empty array literal needs an inferable contextual element type")
	return nil
}
