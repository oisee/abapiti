package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Validate the whole pattern before admitting its signature. Unsupported
// elements must never silently disappear from the entry prologue.
func (l *lowerer) objectParameter(p *ast.Node) bool {
	pattern := p.Name()
	if pattern == nil || pattern.Kind != ast.KindObjectBindingPattern {
		return false
	}
	for _, el := range pattern.AsBindingPattern().Elements.Nodes {
		be := el.AsBindingElement()
		switch {
		case be.DotDotDotToken != nil:
			l.diagf(el, "unsupported-param", "rest element in object parameter")
		case be.Initializer != nil:
			l.diagf(el, "unsupported-param", "default value in object parameter")
		case be.Name() == nil || be.Name().Kind != ast.KindIdentifier:
			l.diagf(el, "unsupported-param", "nested binding pattern in object parameter")
		case be.PropertyName != nil && be.PropertyName.Kind != ast.KindIdentifier:
			l.diagf(el, "unsupported-param", "non-identifier property in object parameter")
		}
	}
	return true
}

func (l *lowerer) bindObjectParameter(p *ast.Node, hp hir.Param) []*hir.Stmt {
	if p.Name() == nil || p.Name().Kind != ast.KindObjectBindingPattern {
		return nil
	}
	var out []*hir.Stmt
	receiver := hir.V(hp.Name, hp.Type)
	if receiver.Type.Kind == hir.Optional {
		receiver = &hir.Expr{Kind: hir.Narrow, Node: l.node(p), Type: receiver.Type.Args[0], X: receiver}
	}
	for _, el := range p.Name().AsBindingPattern().Elements.Nodes {
		be := el.AsBindingElement()
		if be.Name() == nil || be.Name().Kind != ast.KindIdentifier {
			continue
		}
		local, field := be.Name().Text(), be.Name().Text()
		if be.PropertyName != nil {
			field = be.PropertyName.Text()
		}
		ft := l.patternFieldType(receiver.Type, field)
		value := &hir.Expr{Kind: hir.FieldGet, Node: l.node(el), Name: field, Type: ft, X: receiver}
		if sym := l.ck.GetPropertyOfType(l.ck.GetTypeAtLocation(p), field); sym != nil {
			for _, decl := range sym.Declarations {
				if decl.Kind == ast.KindGetAccessor {
					if getter := l.methodOf(sym); getter != nil {
						value = &hir.Expr{Kind: hir.VirtualCall, Node: l.node(el), X: receiver, Name: getter.Name, Type: getter.Result}
					}
				}
			}
		}
		l.declare(local, value.Type)
		out = append(out, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(el), Name: local, Type: value.Type, X: value})
	}
	return out
}
