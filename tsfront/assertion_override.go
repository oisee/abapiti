package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
)

// assertionOverride returns the reviewed target type of an exact `x as T`
// assertion pinned by an enclosing override (Patterns.AssertionTypes).
func (l *lowerer) assertionOverride(n *ast.Node) (hir.Type, bool) {
	text := l.file.Text()[scanner.GetTokenPosOfNode(n, l.file, false):n.End()]
	for parent := n; parent != nil; parent = parent.Parent {
		if e, ok := l.overrides[parent]; ok && e.Patterns != nil {
			if t, ok := e.Patterns.AssertionTypes[text]; ok {
				l.diagf(n, "note-override", "%s: %s", e.ID, e.Rationale)
				return t, true
			}
		}
	}
	return hir.Type{}, false
}

// declaredAssertionOverride: an unannotated local initialized by a corrected
// assertion takes the corrected type (the checker infers the stated one).
func (l *lowerer) declaredAssertionOverride(init *ast.Node) (hir.Type, bool) {
	for init != nil && init.Kind == ast.KindParenthesizedExpression {
		init = init.Expression()
	}
	if init == nil || init.Kind != ast.KindAsExpression {
		return hir.Type{}, false
	}
	return l.assertionOverride(init)
}

// assertedView converts x to the reviewed class: absent stays absent, a
// present object is cast to it (a checked cast: it traps if the object is
// not that class, never silently continues as another one).
func (l *lowerer) assertedView(n *ast.Node, x *hir.Expr, target hir.Type) *hir.Expr {
	class := target
	if class.Kind == hir.Optional {
		class = class.Args[0]
	}
	optional := hir.T(hir.Optional, class)
	if x.Type.Kind != hir.Optional {
		return &hir.Expr{Kind: hir.Cast, Node: l.node(n), Type: class, X: x}
	}
	saved := l.tempInit(n, x.Type, x)
	present := &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: x.Type.Args[0], X: saved}
	return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: optional,
		X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: saved},
		Y: &hir.Expr{Kind: hir.Lit, Type: optional}, Z: &hir.Expr{Kind: hir.Cast, Node: l.node(n), Type: class, X: present}}
}

// unrelatedClassNarrowing: the checker's flow type names a class unrelated to
// the value's lowered class (only after a reviewed assertion correction).
// Such a narrowing is invalid; keep the lowered class, unwrapping presence.
func (l *lowerer) unrelatedClassNarrowing(n *ast.Node, x *hir.Expr, typ hir.Type) (*hir.Expr, bool) {
	base, target := x.Type, typ
	if base.Kind == hir.Optional {
		base = base.Args[0]
	}
	if target.Kind == hir.Optional {
		target = target.Args[0]
	}
	if base.Kind != hir.ClassRef || target.Kind != hir.ClassRef || l.acceptsType(base, target) || l.acceptsType(target, base) {
		return nil, false
	}
	if x.Type.Kind == hir.Optional && typ.Kind != hir.Optional {
		return &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: base, X: x}, true
	}
	return x, true
}
