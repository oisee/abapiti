package tsfront

import (
	"context"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

func (l *lowerer) gogenInheritedData(node *ast.Node) bool {
	if !l.unprunedFeatures || node.AsInterfaceDeclaration().HeritageClauses == nil || l.ck == nil {
		return false
	}
	props := l.ck.GetPropertiesOfType(l.ck.GetDeclaredTypeOfSymbol(node.Symbol()))
	needed := false
	for _, p := range props {
		needed = needed || l.gogenExtractedCallable(l.ck.GetTypeOfSymbol(p))
	}
	if !needed {
		return false
	}
	for _, p := range props {
		for _, d := range p.Declarations {
			if d.Kind == ast.KindPropertySignature || d.Kind == ast.KindPropertyDeclaration {
				return true
			}
		}
	}
	return false
}

// Method signatures in a structural data record are function-valued slots.
// Extracting one copies the function value and does not bind the record as this.
func (l *lowerer) gogenDataCallable(m *ast.Node, c *hir.Class, ctor *hir.Method, list *[]*hir.Stmt) bool {
	if !l.unprunedFeatures || m.Kind != ast.KindMethodSignature || m.Symbol() == nil || !l.gogenExtractedCallable(l.ck.GetTypeOfSymbol(m.Symbol())) {
		return false
	}
	if m.Name() == nil || m.Name().Kind != ast.KindIdentifier || m.Symbol() == nil {
		return false
	}
	typ := l.functionType(m, l.ck.GetTypeOfSymbol(m.Symbol()))
	if l.callableMethod(typ) == nil {
		l.diagf(m, "unsupported-callable", "data method needs one supported call signature")
		return true
	}
	if m.QuestionToken() != nil {
		typ = hir.T(hir.Optional, typ)
	}
	name := m.Name().Text()
	field := hir.Field{Node: l.node(m), Name: name, Type: typ}
	c.Fields = append(c.Fields, field)
	ctor.Params = append(ctor.Params, hir.Param{Name: name, Type: typ})
	*list = append(*list, &hir.Stmt{Kind: hir.Assign, Node: l.node(m), X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(m), Name: name, Type: typ, X: &hir.Expr{Kind: hir.This, Type: hir.Ref(c.Name)}}, Y: hir.V(name, typ)})
	l.fields[m.Symbol()] = field
	return true
}

// Ordinary functions need a call-site receiver, including when first lifted
// into a local. Arrows keep lexical this; test callbacks have a separate ABI.
func (l *lowerer) gogenRefuseReceiver(n *ast.Node) bool {
	if l.unprunedFeatures && (n.Kind == ast.KindFunctionExpression || n.Kind == ast.KindFunctionDeclaration) && !l.isTestCallback(n) && touchesThis(n.Body()) {
		l.diagf(n, "unsupported-closure", "ordinary function reads call-site this; callable receiver ABI is not lowered")
		return true
	}
	return false
}

// Only directly invoked function slots need an additional callable ABI. Keep
// unrelated structural signatures, including the pinned registry deployment.
func (l *lowerer) scanGogenCallables(files []string) {
	if !l.unprunedFeatures {
		return
	}
	l.gogenCallables = map[*ast.Node]bool{}
	for _, name := range files {
		f, ok := l.prog.File(name)
		if !ok {
			continue
		}
		ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		var visit func(*ast.Node) bool
		visit = func(n *ast.Node) bool {
			if n.Kind == ast.KindCallExpression && n.Expression().Kind == ast.KindIdentifier {
				sym := l.resolve(n.Expression())
				if sym != nil && sym.ValueDeclaration != nil {
					d := sym.ValueDeclaration
					extracted := d.Kind == ast.KindParameter || d.Kind == ast.KindBindingElement || d.Kind == ast.KindVariableDeclaration
					if extracted {
						for _, sig := range ck.GetSignaturesOfType(ck.GetTypeAtLocation(n.Expression()), checker.SignatureKindCall) {
							if sig.Declaration() != nil {
								l.gogenCallables[sig.Declaration()] = true
							}
						}
					}
				}
			}
			n.ForEachChild(visit)
			return false
		}
		visit(f.AsNode())
		done()
	}
}

func (l *lowerer) gogenExtractedCallable(t *checker.Type) bool {
	if t == nil {
		return false
	}
	sigs := l.ck.GetSignaturesOfType(t, checker.SignatureKindCall)
	return len(sigs) == 1 && l.gogenCallables[sigs[0].Declaration()]
}

func (l *lowerer) gogenCallableNeeded(n *ast.Node, t *checker.Type) bool {
	return len(l.ck.GetSignaturesOfType(l.ck.GetTypeAtLocation(n), checker.SignatureKindCall)) > 0 || l.gogenExtractedCallable(t)
}

// A callable is not the empty structural row, even when a recursive data
// graph has already interned an empty row with the same property surface.
func (l *lowerer) gogenDirectCallable(n *ast.Node, t *checker.Type) (hir.Type, bool) {
	if !l.unprunedFeatures || t == nil || !l.gogenExtractedCallable(t) || l.mappingTypes[t] {
		return hir.Type{}, false
	}
	if l.isThunkType(t) {
		return l.thunkType(), true
	}
	if l.mappingTypes == nil {
		l.mappingTypes = map[*checker.Type]bool{}
	}
	l.mappingTypes[t] = true
	defer delete(l.mappingTypes, t)
	return l.functionType(n, t), true
}
