package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Weak collections deliberately expose only identity lookup. Strong retention
// is unobservable in this subset (there is no WeakRef/finalization API).
func (l *lowerer) gogenCheckerType(n *ast.Node, t *checker.Type) (hir.Type, bool) {
	if !l.unprunedFeatures || t == nil {
		return hir.Type{}, false
	}
	if mapped, ok := l.arrayRecordType(n, t); ok {
		return mapped, true
	}
	if mapped, ok := l.genericPropertyType(t); ok {
		return mapped, true
	}
	if mapped, ok := l.gogenResolvedArray(n, t); ok {
		return mapped, true
	}
	if mapped, ok := l.gogenGenericShape(n, t); ok {
		return mapped, true
	}
	if mapped, ok := l.gogenIndexedType(t); ok {
		return mapped, true
	}
	if t.Flags()&checker.TypeFlagsIntersection != 0 && l.gogenMapValue(n, t) {
		if c := l.synthFromProperties(n, l.ck.GetPropertiesOfType(t)); c != nil {
			return hir.Ref(c.Name), true
		}
	}
	sym := t.Symbol()
	if sym == nil || !l.librarySymbol(sym) {
		return hir.Type{}, false
	}
	if sym.Name == "TypeError" {
		return hir.Ref(l.gogenTypeError()), true
	}
	if sym.Name != "WeakMap" && sym.Name != "WeakSet" {
		return hir.Type{}, false
	}
	args := l.ck.GetTypeArguments(t)
	value := hir.T(hir.Bool)
	if sym.Name == "WeakMap" {
		value = hir.T(hir.Dynamic)
		if len(args) == 2 {
			value = l.mapCheckerType(n, args[1])
		}
	}
	name := "builtin." + sym.Name + "." + value.String()
	for _, c := range l.out.Classes {
		if c.Name == name {
			return hir.Ref(name), true
		}
	}
	mapType := hir.T(hir.OrderedMap, hir.Ref(hir.RootObject), value)
	c := &hir.Class{Node: l.node(n), Name: name, Fields: []hir.Field{{Name: "entries", Type: mapType}}}
	c.Ctor = &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Type: mapType, Name: "entries", X: &hir.Expr{Kind: hir.This, Type: hir.Ref(name)}}, Y: &hir.Expr{Kind: hir.New, Type: mapType}})}
	l.out.Classes = append(l.out.Classes, c)
	return hir.Ref(name), true
}

func (l *lowerer) gogenTypeError() string {
	l.builtinError()
	const name = "builtin.TypeError"
	for _, c := range l.out.Classes {
		if c.Name == name {
			return name
		}
	}
	l.out.Classes = append(l.out.Classes, &hir.Class{Name: name, Super: "builtin.Error"})
	return name
}

func (l *lowerer) gogenNew(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	sym := l.resolve(n.Expression())
	if sym == nil || !l.librarySymbol(sym) || (sym.Name != "WeakMap" && sym.Name != "WeakSet") {
		return nil, false
	}
	if len(n.Arguments()) != 0 {
		l.diagf(n, "unsupported-new", "%s iterable constructor is unsupported", sym.Name)
		return nil, true
	}
	typ, ok := l.gogenCheckerType(n, l.ck.GetTypeAtLocation(n))
	if !ok {
		l.diagf(n, "unsupported-new", "%s has no checked collection type", sym.Name)
		return nil, true
	}
	return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ}, true
}

func (l *lowerer) gogenCall(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	callee := n.Expression()
	if callee == nil {
		return nil, false
	}
	if callee.Kind == ast.KindIdentifier && callee.Text() == "BigInt" && l.librarySymbol(l.resolve(callee)) {
		if len(n.Arguments()) != 1 {
			l.diagf(n, "unsupported-bigint", "BigInt requires one decimal string argument")
			return nil, true
		}
		x := l.expr(n.Arguments()[0])
		if x == nil {
			return nil, true
		}
		if t := l.ck.GetTypeAtLocation(n.Arguments()[0]); t != nil && t.Flags()&checker.TypeFlagsStringLike != 0 {
			x = l.coerce(x, hir.T(hir.String))
		}
		if x.Type.Kind != hir.String {
			l.diagf(n, "unsupported-bigint", "BigInt requires a decimal string; number/boolean coercion is not lowered (got %s)", x.Type)
			return nil, true
		}
		return l.rtOp("bigint.decimal", x, hir.T(hir.BigInt)), true
	}
	if callee.Kind != ast.KindPropertyAccessExpression || callee.Name().Text() != "now" {
		return nil, false
	}
	recv := callee.Expression()
	if recv.Kind != ast.KindIdentifier || recv.Text() != "Date" || !l.librarySymbol(l.resolve(recv)) {
		return nil, false
	}
	if len(n.Arguments()) != 0 {
		l.diagf(n, "unsupported-call", "Date.now accepts no arguments in this lowering")
		return nil, true
	}
	clock := l.rtOp("clock.epochMilliseconds", hir.L(hir.T(hir.I32), int32(0)), hir.T(hir.Number))
	clock.Node = l.node(n)
	return clock, true
}

func (l *lowerer) gogenWeakCall(n *ast.Node, name string, recv *hir.Expr) (*hir.Expr, bool) {
	if recv.Type.Kind != hir.ClassRef || !(strings.HasPrefix(recv.Type.Name, "builtin.WeakMap.") || strings.HasPrefix(recv.Type.Name, "builtin.WeakSet.")) {
		return nil, false
	}
	set := strings.HasPrefix(recv.Type.Name, "builtin.WeakSet.")
	write := name == "set" && !set || name == "add" && set
	if !write && name != "get" && name != "has" && name != "delete" || set && name == "get" {
		l.diagf(n, "unsupported-call", "weak collections have no %s operation (no iteration or size)", name)
		return nil, true
	}
	arity := 1
	if write && !set {
		arity = 2
	}
	if len(n.Arguments()) != arity {
		l.diagf(n, "unsupported-call", "weak collection %s requires %d arguments", name, arity)
		return nil, true
	}
	recv = l.tempInit(n, recv.Type, recv)
	c := l.classNamed(recv.Type.Name)
	mt := c.Fields[0].Type
	values := make([]*hir.Expr, 0, arity)
	for i, a := range n.Arguments() {
		if a.Kind == ast.KindSpreadElement {
			l.diagf(a, "unsupported-call", "weak collection spread arguments are unsupported")
			return nil, true
		}
		target := hir.T(hir.Dynamic)
		if i == 1 {
			target = mt.Args[1]
		}
		old := l.hint
		l.hint = target
		x := l.expr(a)
		l.hint = old
		if x == nil {
			return nil, true
		}
		values = append(values, l.tempInit(a, target, l.coerce(x, target)))
	}
	op := "weak.lookupKey"
	if write {
		op = "weak.writeKey"
		l.gogenTypeError()
	}
	key := l.rtOp(op, values[0], hir.Ref(hir.RootObject))
	key.Node = l.node(n)
	entries := &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Type: mt, Name: "entries", X: recv}
	switch name {
	case "get":
		return l.mapRead(entries, key), true
	case "has", "delete":
		return l.rtOp("map."+name, entries, hir.T(hir.Bool), key), true
	default:
		value := hir.L(hir.T(hir.Bool), true)
		if !set {
			value = values[1]
		}
		l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("map.set", entries, mt, key, value)})
		return recv, true
	}
}

func (l *lowerer) classNamed(name string) *hir.Class {
	for _, c := range l.out.Classes {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (l *lowerer) gogenBuiltinClass(sym *ast.Symbol) *hir.Class {
	if l.unprunedFeatures && sym != nil && sym.Name == "TypeError" && l.librarySymbol(sym) {
		return l.classNamed(l.gogenTypeError())
	}
	return nil
}

// Resolve library collection aliases before the data-alias shape synthesizer:
// Map's method surface is not a record of application fields.
func (l *lowerer) gogenAnnotationType(n *ast.Node) (hir.Type, bool) {
	if !l.unprunedFeatures {
		return hir.Type{}, false
	}
	if mapped, ok := l.gogenResolvedAnnotation(n); ok {
		return mapped, true
	}
	t := l.ck.GetTypeFromTypeNode(n)
	if mapped, ok := l.gogenValueType(t); ok {
		return mapped, true
	}
	if l.gogenExtractedCallable(t) {
		return l.mapCheckerType(n, t), true
	}
	if t == nil || t.Symbol() == nil || !l.librarySymbol(t.Symbol()) {
		return hir.Type{}, false
	}
	switch t.Symbol().Name {
	case "WeakMap", "WeakSet", "TypeError":
		return l.mapCheckerType(n, t), true
	case "Map", "ReadonlyMap", "Set", "ReadonlySet":
		if t.Alias() != nil {
			return l.mapCheckerType(n, t), true
		}
	}
	return hir.Type{}, false
}

// Only flatten an intersection that is the declared Map value. Other
// intersections retain the existing family and narrowing policies.
func (l *lowerer) gogenMapValue(n *ast.Node, t *checker.Type) bool {
	if l.gogenActiveMapValue(t) {
		return true
	}
	source := l.ck.GetTypeAtLocation(n)
	if ast.IsTypeNode(n) {
		source = l.ck.GetTypeFromTypeNode(n)
	}
	if source == nil || source.Symbol() == nil || !l.librarySymbol(source.Symbol()) {
		return false
	}
	if source.Symbol().Name != "Map" && source.Symbol().Name != "ReadonlyMap" {
		return false
	}
	args := l.ck.GetTypeArguments(source)
	return len(args) == 2 && args[1] == t
}
