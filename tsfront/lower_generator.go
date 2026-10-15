package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Generators use ordinary HIR objects. No body instruction runs at creation;
// next() advances a persistent program counter to the next yield. The result
// has an explicit done bit and an optional payload, including on exhaustion.
// The supported protocol is Generator<T, void, undefined>; sending values,
// return()/throw(), and suspension inside exception handlers are refused.
type generatorABI struct {
	element    hir.Type
	iterator   hir.Type
	result     hir.Type
	arrayClass hir.Type
}

func isGeneratorFunction(n *ast.Node) bool {
	switch n.Kind {
	case ast.KindMethodDeclaration:
		return n.AsMethodDeclaration().AsteriskToken != nil
	case ast.KindFunctionDeclaration:
		return n.AsFunctionDeclaration().AsteriskToken != nil
	}
	return false
}

// Workload snapshot adaptations are justified only by their coverage inputs.
// Retain them byte-for-byte for the default build, but lower the actual source
// when measuring the full closure. Signature and Iterable adaptations belong
// to that same snapshot ABI and must be removed together.
func (l *lowerer) useResumableGenerators() {
	for n, e := range l.overrides {
		if !e.GeneratorSnapshot {
			continue
		}
		if e.Patterns != nil {
			copy := *e.Patterns
			copy.Annotations = map[string]hir.Type{}
			for key, value := range e.Patterns.Annotations {
				copy.Annotations[key] = l.ensureGeneratorABI(value.Args[0]).iterator
			}
			e.Patterns = &copy
			l.overrides[n] = e
		} else {
			delete(l.overrides, n)
		}
	}
}

func (l *lowerer) generatorTypeNode(n *ast.Node) hir.Type {
	a := n.TypeArguments()
	if !generatorSyntaxDefaults(a) {
		l.diagf(n, "unsupported-generator", "only Generator<T, void, undefined> is lowered")
		return hir.T(hir.Void)
	}
	return l.ensureGeneratorABI(l.mapTypeNode(a[0])).iterator
}
func (l *lowerer) generatorResultCheckerType(n *ast.Node, t *checker.Type) (hir.Type, bool) {
	if !l.generatorEnabled {
		return hir.Type{}, false
	}
	if alias := t.Alias(); alias != nil && alias.Symbol() != nil && alias.Symbol().Name == "IteratorResult" && l.librarySymbol(alias.Symbol()) {
		args := alias.TypeArguments()
		if len(args) == 2 && args[1].Flags()&(checker.TypeFlagsVoid|checker.TypeFlagsUndefined) != 0 {
			return l.ensureGeneratorABI(l.mapCheckerType(n, args[0])).result, true
		}
	}
	return hir.Type{}, false
}

func (l *lowerer) generatorCheckerType(n *ast.Node, t *checker.Type) (hir.Type, bool) {
	if !l.generatorEnabled {
		return hir.Type{}, false
	}
	if t.Symbol() != nil && (t.Symbol().Name == "Iterable" || t.Symbol().Name == "RegExpStringIterator") && l.librarySymbol(t.Symbol()) {
		args := l.ck.GetTypeArguments(t)
		if len(args) > 0 {
			return l.ensureGeneratorABI(l.mapCheckerType(n, args[0])).iterator, true
		}
	}
	if t.Symbol() == nil || t.Symbol().Name != "Generator" || !l.librarySymbol(t.Symbol()) {
		return hir.Type{}, false
	}
	a := l.ck.GetTypeArguments(t)
	if !generatorCheckerDefaults(a) {
		l.diagf(n, "unsupported-generator", "only Generator<T, void, undefined> is lowered")
		return hir.T(hir.Void), true
	}
	return l.ensureGeneratorABI(l.mapCheckerType(n, a[0])).iterator, true
}
func (l *lowerer) ensureGeneratorABI(element hir.Type) *generatorABI {
	key := element.String()
	if l.generatorABIs == nil {
		l.generatorABIs = map[string]*generatorABI{}
	}
	if a := l.generatorABIs[key]; a != nil {
		return a
	}
	suffix := typeSig(element)[:16]
	a := &generatorABI{element: element, iterator: hir.Type{Kind: hir.InterfaceRef, Name: "generator.iterator." + suffix}, result: hir.Ref("generator.result." + suffix)}
	l.generatorABIs[key] = a
	node := func() hir.Node { return hir.Node{ID: l.nextID(), Source: "generator ABI"} }
	self := &hir.Expr{Kind: hir.This, Type: a.result}
	opt := hir.T(hir.Optional, element)
	result := &hir.Class{Node: node(), Name: a.result.Name, Fields: []hir.Field{{Node: node(), Name: "done", Type: hir.T(hir.Bool)}, {Node: node(), Name: "value", Type: opt}}}
	result.Ctor = &hir.Method{Node: node(), Name: "constructor", Result: hir.T(hir.Void), Params: []hir.Param{{Name: "done", Type: hir.T(hir.Bool)}, {Name: "value", Type: opt}}, Body: hir.B(
		&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Name: "done", Type: hir.T(hir.Bool), X: self}, Y: hir.V("done", hir.T(hir.Bool))},
		&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Name: "value", Type: opt, X: self}, Y: hir.V("value", opt)})}
	l.out.Classes = append(l.out.Classes, result)
	iface := &hir.Interface{Node: node(), Name: a.iterator.Name, Methods: []*hir.Method{{Node: node(), Name: "next", Virtual: true, Result: a.result}, {Node: node(), Name: "close", Virtual: true, Result: hir.T(hir.Void)}, {Node: node(), Name: "iterate", Virtual: true, Result: a.iterator}}}
	l.out.Interfaces = append(l.out.Interfaces, iface)
	if l.ifaceDone == nil {
		l.ifaceDone = map[*hir.Interface]bool{}
	}
	l.ifaceDone[iface] = true
	return a
}
func (l *lowerer) generatorABIFor(t hir.Type) *generatorABI {
	for _, a := range l.generatorABIs {
		if t.Equal(a.iterator) {
			return a
		}
	}
	return nil
}
func (l *lowerer) generatorResultABI(t hir.Type) *generatorABI {
	for _, a := range l.generatorABIs {
		if t.Equal(a.result) {
			return a
		}
	}
	return nil
}
func (l *lowerer) isGeneratorABIValue(x *hir.Expr) bool {
	if l.generatorABIFor(x.Type) != nil || l.generatorResultABI(x.Type) != nil {
		return true
	}
	return x.Kind == hir.FieldGet && x.X != nil && l.generatorResultABI(x.X.Type) != nil
}
func (l *lowerer) generatorNextCall(n *ast.Node) (*hir.Expr, bool) {
	callee := n.Expression()
	if callee == nil || callee.Kind != ast.KindPropertyAccessExpression {
		return nil, false
	}
	p := callee.AsPropertyAccessExpression()
	// Consult the checker first so unrelated calls do not allocate preludes.
	t := l.ck.GetTypeAtLocation(p.Expression)
	if t == nil || t.Symbol() == nil || (t.Symbol().Name != "Generator" && t.Symbol().Name != "RegExpStringIterator") || !l.librarySymbol(t.Symbol()) {
		return nil, false
	}
	recv := l.expr(p.Expression)
	if recv == nil {
		return nil, true
	}
	a := l.generatorABIFor(recv.Type)
	if a == nil {
		return nil, false
	} // source-pinned snapshot ABI
	if p.Name().Text() != "next" || len(n.Arguments()) > 1 || (len(n.Arguments()) == 1 && !isUndefinedKeyword(n.Arguments()[0])) {
		l.diagf(n, "unsupported-generator", "generator supports next() or next(undefined); return/throw and sent values are not lowered")
		return nil, true
	}
	return &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), X: recv, Name: "next", Type: a.result}, true
}
func (l *lowerer) generatorResultProperty(n *ast.Node) (*hir.Expr, bool) {
	p := n.AsPropertyAccessExpression()
	if p.Name().Text() != "done" && p.Name().Text() != "value" {
		return nil, false
	}
	// A generator next result is a library IteratorResult (a union), so match
	// the receiver's resolved HIR ABI, never user-defined property names alone.
	t := l.ck.GetTypeAtLocation(p.Expression)
	if t == nil || !strings.Contains(l.ck.TypeToString(t), "Iterator") {
		return nil, false
	}
	x := l.expr(p.Expression)
	if x == nil {
		return nil, true
	}
	a := l.generatorResultABI(x.Type)
	if a == nil {
		return nil, false
	}
	typ := hir.T(hir.Bool)
	if p.Name().Text() == "value" {
		typ = hir.T(hir.Optional, a.element)
	}
	return &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: p.Name().Text(), Type: typ, X: x}, true
}

const generatorYieldMarker = "frontend.generator.yield"

func (l *lowerer) generatorYieldStatement(n *ast.Node) *hir.Stmt {
	if l.generatorYield == nil {
		l.diagf(n, "unsupported-generator", "yield requires a supported generator body")
		return nil
	}
	y := n.AsYieldExpression()
	if y.Expression == nil {
		l.diagf(n, "unsupported-generator", "yield without a value is not lowered")
		return nil
	}
	x := l.expr(y.Expression)
	if x == nil {
		return nil
	}
	marker := func(v *hir.Expr) *hir.Stmt {
		return &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: &hir.Expr{Kind: hir.RuntimeOp, Op: generatorYieldMarker, Type: hir.T(hir.Void), X: l.coerce(l.presentValue(y.Expression, v, l.generatorYield.Type), l.generatorYield.Type)}}
	}
	if y.AsteriskToken == nil {
		return marker(x)
	}
	l.serial++
	name := "delegatedValue" + itoa(l.serial)
	if a := l.generatorABIFor(x.Type); a != nil {
		return l.generatorLoop(x, name, marker(hir.V(name, a.element)))
	}
	if x.Type.Kind == hir.Array {
		return &hir.Stmt{Kind: hir.ForEach, Name: name, Type: x.Type.Args[0], X: x, Body: marker(hir.V(name, x.Type.Args[0]))}
	}
	l.diagf(n, "unsupported-generator", "yield* requires a supported generator or array")
	return nil
}
