package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

func (l *lowerer) objectBindingDefault(el *ast.Node, value *hir.Expr) *hir.Expr {
	be := el.AsBindingElement()
	if be.Initializer == nil {
		return value
	}
	saved := l.pend
	l.pend = nil
	read := l.tempInit(el, value.Type, value)
	pre := l.pend
	l.pend = nil
	typ := l.mapCheckerType(be.Name(), l.ck.GetTypeAtLocation(be.Name()))
	hint := l.hint
	l.hint = typ
	def := l.expr(be.Initializer)
	l.hint = hint
	if def != nil {
		def = l.coerce(def, typ)
	}
	if len(l.pend) > 0 && def != nil {
		def = &hir.Expr{Kind: hir.Seq, Type: typ, Stmt: hir.B(l.pend...), Y: def}
	}
	l.pend = saved
	if def == nil {
		return value
	}
	present := read
	if read.Type.Kind == hir.Optional && typ.Kind != hir.Optional && typ.Kind != hir.Dynamic {
		// The default applies exactly when the property is undefined, so
		// this branch reads a present value.
		present = &hir.Expr{Kind: hir.Narrow, Node: l.node(el), Type: read.Type.Args[0], X: read}
	}
	present = l.coerce(present, typ)
	return &hir.Expr{Kind: hir.Seq, Node: l.node(el), Type: typ, Stmt: hir.B(pre...), Y: &hir.Expr{Kind: hir.Conditional, Type: typ, X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: read}, Y: def, Z: present}}
}

// Binding defaults need to retain null at the call boundary so that the
// parameter prologue, rather than a structural view conversion, throws TypeError.
func bindingDefaultParameterType(p *ast.Node, typ hir.Type) hir.Type {
	if p.Name() != nil && p.Name().Kind == ast.KindObjectBindingPattern {
		for _, el := range p.Name().AsBindingPattern().Elements.Nodes {
			if el.AsBindingElement().Initializer != nil {
				return hir.T(hir.Optional, hir.T(hir.Dynamic))
			}
		}
	}
	return typ
}
