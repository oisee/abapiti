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
	errorType := hir.Ref(l.builtinTypeError())
	check := l.rtOp("dynamic.isNullish", l.rtOp("dynamic.of", receiver, hir.T(hir.Dynamic)), hir.T(hir.Bool))
	check.Node = l.node(p)
	out = append(out, &hir.Stmt{Kind: hir.If, Node: l.node(p), X: check,
		Body: hir.B(&hir.Stmt{Kind: hir.Throw, Node: l.node(p), X: &hir.Expr{Kind: hir.New, Node: l.node(p), Type: errorType,
			Args: []*hir.Expr{l.coerce(hir.L(hir.T(hir.String), "Cannot destructure null or undefined"), hir.T(hir.Optional, hir.T(hir.String)))}}})})
	if receiver.Type.Kind == hir.Optional {
		receiver = &hir.Expr{Kind: hir.Narrow, Node: l.node(p), Type: receiver.Type.Args[0], X: receiver}
	}
	if receiver.Type.Kind == hir.Dynamic && len(p.Name().AsBindingPattern().Elements.Nodes) > 0 {
		target := l.mapCheckerType(p, l.ck.GetNonNullableType(l.ck.GetTypeAtLocation(p)))
		receiver = l.coerce(receiver, target)
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
		if receiver.Type.Kind == hir.Dynamic {
			value = l.rtOp("dynamic.get", receiver, hir.T(hir.Dynamic), hir.L(hir.T(hir.String), field))
			value.Node = l.node(el)
		}
		if sym := l.ck.GetPropertyOfType(l.ck.GetTypeAtLocation(p), field); sym != nil {
			for _, decl := range sym.Declarations {
				if decl.Kind == ast.KindGetAccessor {
					if getter := l.methodOf(sym); getter != nil {
						value = &hir.Expr{Kind: hir.VirtualCall, Node: l.node(el), X: receiver, Name: getter.Name, Type: getter.Result}
					}
				}
			}
		}
		value = l.objectBindingDefault(el, value)
		l.declare(local, value.Type)
		out = append(out, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(el), Name: local, Type: value.Type, X: value})
	}
	return out
}
