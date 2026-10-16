package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Callable values use ordinary interfaces and classes, shared by both targets.
// Named functions keep their direct-call ABI until they escape as values.
func (l *lowerer) functionType(n *ast.Node, t *checker.Type) hir.Type {
	if l.isThunkType(t) {
		return l.thunkType()
	}
	sigs := l.ck.GetSignaturesOfType(t, checker.SignatureKindCall)
	if len(sigs) != 1 {
		return hir.T(hir.Void)
	}
	sig := sigs[0]
	m := &hir.Method{Name: "call", Virtual: true, Result: l.mapCheckerType(n, l.ck.GetReturnTypeOfSignature(sig))}
	for _, p := range sig.Parameters() {
		typ := l.mapCheckerType(n, l.ck.GetTypeOfSymbol(p))
		if typ.Kind == hir.Void {
			return hir.T(hir.Void)
		}
		param := hir.Param{Name: "arg" + itoa(len(m.Params)), Type: typ}
		if d := p.ValueDeclaration; d != nil && d.Kind == ast.KindParameter {
			param.Variadic = d.AsParameterDeclaration().DotDotDotToken != nil
			if (d.Initializer() != nil || d.QuestionToken() != nil) && param.Type.Kind != hir.Optional {
				param.Type = hir.T(hir.Optional, param.Type)
			}
			if (d.Initializer() != nil || d.QuestionToken() != nil) && param.Type.Kind != hir.Optional {
				param.Type = hir.T(hir.Optional, param.Type)
			}
		}
		m.Params = append(m.Params, param)
	}
	for _, i := range l.out.Interfaces {
		if !strings.HasPrefix(i.Name, "closure.function.") || len(i.Methods) != 1 {
			continue
		}
		old := i.Methods[0]
		if !old.Result.Equal(m.Result) || len(old.Params) != len(m.Params) {
			continue
		}
		equal := true
		for j := range m.Params {
			equal = equal && old.Params[j].Type.Equal(m.Params[j].Type) && old.Params[j].Variadic == m.Params[j].Variadic
		}
		if equal {
			return hir.Type{Kind: hir.InterfaceRef, Name: i.Name}
		}
	}
	name := "closure.function." + itoa(len(l.out.Interfaces))
	m.Node = l.node(n)
	i := &hir.Interface{Node: l.node(n), Name: name, Methods: []*hir.Method{m}}
	l.out.Interfaces = append(l.out.Interfaces, i)
	if l.ifaceDone == nil {
		l.ifaceDone = map[*hir.Interface]bool{}
	}
	l.ifaceDone[i] = true
	return hir.Type{Kind: hir.InterfaceRef, Name: name}
}

func (l *lowerer) callableMethod(t hir.Type) *hir.Method {
	if t.Kind != hir.InterfaceRef || !(strings.HasPrefix(t.Name, "closure.function.") || t.Name == thunkInterfaceName) {
		return nil
	}
	for _, i := range l.out.Interfaces {
		if i.Name == t.Name {
			return i.Methods[0]
		}
	}
	return nil
}

// A cell stores the binding, not its current value. All writes and reads of
// that checked symbol are rewritten together after bodies have been lowered.
func (l *lowerer) captureCell(c localCapture) hir.Type {
	if l.captureCells == nil {
		l.captureCells = map[*ast.Symbol]hir.Type{}
	}
	if t, ok := l.captureCells[c.symbol]; ok {
		return t
	}
	name := "closure.cell." + itoa(len(l.captureCells))
	t := hir.Ref(name)
	self := &hir.Expr{Kind: hir.This, Type: t}
	ctor := &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Params: []hir.Param{{Name: "value", Type: c.typ}},
		Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, Name: "value", Type: c.typ, X: self}, Y: hir.V("value", c.typ)})}
	l.out.Classes = append(l.out.Classes, &hir.Class{Node: l.node(c.node), Name: name, Fields: []hir.Field{{Name: "value", Type: c.typ}}, Ctor: ctor})
	l.captureCells[c.symbol] = t
	if c.symbol.ValueDeclaration != nil && c.symbol.ValueDeclaration.Kind == ast.KindParameter {
		if l.captureParamNames == nil {
			l.captureParamNames = map[*ast.Symbol]string{}
		}
		used := map[string]bool{}
		var walk func(*ast.Node)
		walk = func(n *ast.Node) {
			if n.Kind == ast.KindIdentifier {
				used[n.Text()] = true
			}
			n.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
		}
		walk(l.file.AsNode())
		name := c.name + "_cell"
		for suffix := 1; used[name]; suffix++ {
			name = c.name + "_cell_" + itoa(suffix)
		}
		l.captureParamNames[c.symbol] = name
	}
	return t
}

func (l *lowerer) finishCaptureCells() {
	if len(l.captureCells) == 0 {
		return
	}
	// Declarations and parameters allocate one cell per invocation. Closure
	// transport locals deliberately have no declSymbols entry and share it.
	for decl, symbol := range l.declSymbols {
		if cell, ok := l.captureCells[symbol]; ok {
			if decl.Type.Equal(cell) {
				continue
			}
			args := []*hir.Expr{}
			if decl.X != nil {
				args = append(args, decl.X)
			}
			decl.Type = cell
			decl.X = &hir.Expr{Kind: hir.New, Node: decl.Node, Type: cell, Args: args}
		}
	}
	for method, params := range l.paramSymbols {
		var pre []*hir.Stmt
		for _, p := range method.Params {
			if cell, ok := l.captureCells[params[p.Name]]; ok {
				pre = append(pre, &hir.Stmt{Kind: hir.VarDecl, Name: l.captureParamNames[params[p.Name]], Type: cell, X: &hir.Expr{Kind: hir.New, Type: cell, Args: []*hir.Expr{hir.V(p.Name, p.Type)}}})
			}
		}
		if len(pre) > 0 {
			method.Body = hir.B(append(pre, method.Body)...)
		}
	}
	for expr, symbol := range l.localSymbols {
		cell, ok := l.captureCells[symbol]
		if !ok || expr.Kind != hir.Local {
			continue
		}
		name := expr.Name
		// Source parameters use a distinct cell local to preserve their public ABI.
		for method, params := range l.paramSymbols {
			if params[name] == symbol && l.localSymbolMethods[expr] == method {
				for _, p := range method.Params {
					if p.Name == name && !p.Type.Equal(cell) {
						name = l.captureParamNames[symbol]
						break
					}
				}
				break
			}
		}
		if expr.Type.Equal(cell) {
			expr.Name = name
			continue
		}
		original := *expr
		*expr = hir.Expr{Kind: hir.FieldGet, Node: original.Node, Name: "value", Type: original.Type, X: hir.V(name, cell)}
	}
}

// functionValue wraps a lifted local function or module function. Receiver
// and lexical inputs are evaluated once when the value is formed.
// valueSymbol resolves a value reference; in a shorthand property `{f}` the
// identifier names the property, and the value is the binding it reads.
func (l *lowerer) valueSymbol(n *ast.Node) *ast.Symbol {
	if n != nil && n.Kind == ast.KindIdentifier && n.Parent != nil && n.Parent.Kind == ast.KindShorthandPropertyAssignment {
		if value := l.ck.GetShorthandAssignmentValueSymbol(n.Parent); value != nil {
			if value.Flags&ast.SymbolFlagsAlias != 0 {
				if target, ok := l.ck.ResolveAlias(value); ok {
					return target
				}
			}
			return value
		}
	}
	return l.resolve(n)
}

func (l *lowerer) functionValue(n *ast.Node) *hir.Expr {
	if !l.unprunedFeatures {
		return nil
	}
	symbol := l.resolve(n)
	if symbol != nil && l.refuseUndefinedThis(symbol.ValueDeclaration) {
		return nil
	}
	if binding := l.functionBindings[symbol]; binding != nil && binding.Kind == hir.VarDecl {
		return l.functionBindingRead(n, binding, symbol)
	}
	var method *hir.Method
	var owner string
	var receiver *hir.Expr
	var captures []localCapture
	copyBody := false
	if n.Kind == ast.KindIdentifier {
		name := n.Text()
		lf := l.localFns[name]
		if pending := l.pendingFns[name]; lf == nil && pending != nil {
			delete(l.pendingFns, name)
			lf = l.liftLocalFn(name, pending)
		}
		if lf != nil {
			method, owner, captures = lf.method, lf.owner.Name, lf.captures
			if !method.Static {
				receiver = l.localFnReceiver(lf)
			}
		} else if sym := l.valueSymbol(n); sym != nil {
			if f := l.fileOfSymbol(sym); f != nil {
				if ref, ok := l.funcsBy[f.FileName()+" "+sym.Name]; ok {
					owner = ref.owner
					method = l.methodsBy[owner+"."+ref.method]
				}
			}
		}
	} else if n.Kind == ast.KindPropertyAccessExpression {
		p := n.AsPropertyAccessExpression()
		method = l.methodOf(l.resolve(n))
		if method != nil && method.Static {
			if c := l.classOf(l.resolve(p.Expression)); c != nil {
				owner = c.Name
			} else {
				return nil
			}
		} else if method != nil {
			sym := l.resolve(n)
			if sym.ValueDeclaration == nil || sym.ValueDeclaration.Body() == nil {
				// An interface or abstract method: the implementation that
				// would run without its receiver is not known statically.
				l.diagf(n, "unsupported-closure", "unbound method value has no known implementation to check for this")
				return nil
			}
			if touchesThis(sym.ValueDeclaration.Body()) {
				l.diagf(n, "unsupported-closure", "unbound method value uses this")
				return nil
			}
			recv := l.expr(p.Expression)
			if recv == nil {
				return nil
			}
			l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: recv})
			copyBody = true
		}
	}

	if method == nil {
		return nil
	}
	typ := l.functionType(n, l.ck.GetTypeAtLocation(n))
	signature := l.callableMethod(typ)
	if signature == nil {
		return nil
	}
	l.serial++
	name := "closure.value." + itoa(l.serial)
	c := &hir.Class{Node: l.node(n), Name: name, Implements: []string{typ.Name}}
	ctor := &hir.Method{Name: "constructor", Result: hir.T(hir.Void)}
	self := &hir.Expr{Kind: hir.This, Type: hir.Ref(name)}
	var init []*hir.Stmt
	var args []*hir.Expr
	field := func(name string, t hir.Type, x *hir.Expr) *hir.Expr {
		c.Fields = append(c.Fields, hir.Field{Name: name, Type: t})
		ctor.Params = append(ctor.Params, hir.Param{Name: name, Type: t})
		get := &hir.Expr{Kind: hir.FieldGet, Name: name, Type: t, X: self}
		init = append(init, &hir.Stmt{Kind: hir.Assign, X: get, Y: hir.V(name, t)})
		args = append(args, x)
		return get
	}
	if receiver != nil {
		receiver = field("receiver", receiver.Type, receiver)
	}
	var callArgs []*hir.Expr
	for _, cap := range captures {
		callArgs = append(callArgs, field(cap.name, cap.typ, l.captureTransport(cap, cap.typ)))
	}
	call := *signature
	call.Node = l.node(n)
	call.Params = append([]hir.Param(nil), signature.Params...)
	for _, p := range call.Params {
		callArgs = append(callArgs, hir.V(p.Name, p.Type))
	}
	x := &hir.Expr{Kind: hir.DirectCall, Owner: owner, Name: method.Name, Type: method.Result, Args: callArgs}
	if receiver != nil {
		x.Kind = hir.VirtualCall
		x.X = receiver
	}
	if typ.Name == thunkInterfaceName && call.Result.Kind == hir.Promise {
		effect := x
		if x.Type.Kind == hir.Promise {
			effect = &hir.Expr{Kind: hir.PromiseAwait, Type: x.Type.Args[0], X: x}
		}
		call.Body = &hir.Stmt{Kind: hir.Async, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: effect}, &hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.PromiseFulfill, Type: call.Result}})}
	} else if call.Result.Kind == hir.Void {
		call.Body = hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: x})
	} else {
		call.Body = hir.B(&hir.Stmt{Kind: hir.Return, X: x})
	}
	if copyBody {
		impl := *method
		impl.Name = "invoke"
		impl.Virtual = false
		impl.Static = false
		l.callableBodies = append(l.callableBodies, [2]*hir.Method{&impl, method})
		c.Methods = append(c.Methods, &impl)
		x.Kind = hir.DirectCall
		x.Owner = name
		x.Name = "invoke"
		x.X = self
	}

	ctor.Body = hir.B(init...)
	c.Ctor = ctor
	c.Methods = append(c.Methods, &call)
	l.out.Classes = append(l.out.Classes, c)
	value := &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(name), Args: args}
	if binding := l.functionBindings[symbol]; binding != nil {
		*binding = hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: n.Text(), Type: typ, X: value}
		if l.declSymbols == nil {
			l.declSymbols = map[*hir.Stmt]*ast.Symbol{}
		}
		l.declSymbols[binding] = symbol
		l.declare(n.Text(), typ)
		return l.functionBindingRead(n, binding, symbol)
	}
	return value
}

func (l *lowerer) cellValueType(t hir.Type) hir.Type {
	for _, c := range l.out.Classes {
		if c.Name == t.Name {
			return c.Fields[0].Type
		}
	}
	return hir.T(hir.Void)
}

func (l *lowerer) assignedInFunction(fn *ast.Node, symbol *ast.Symbol) bool {
	found := false
	var walk func(*ast.Node)
	walk = func(n *ast.Node) {
		if n == nil {
			return
		}
		if n.Kind == ast.KindBinaryExpression {
			b := n.AsBinaryExpression()
			if ast.IsAssignmentOperator(b.OperatorToken.Kind) && b.Left.Kind == ast.KindIdentifier && l.ck.GetSymbolAtLocation(b.Left) == symbol {
				found = true
			}
		}
		var updated *ast.Node
		if n.Kind == ast.KindPostfixUnaryExpression {
			updated = n.AsPostfixUnaryExpression().Operand
		} else if n.Kind == ast.KindPrefixUnaryExpression {
			u := n.AsPrefixUnaryExpression()
			if u.Operator == ast.KindPlusPlusToken || u.Operator == ast.KindMinusMinusToken {
				updated = u.Operand
			}
		}
		if updated != nil && updated.Kind == ast.KindIdentifier && l.ck.GetSymbolAtLocation(updated) == symbol {
			found = true
		}
		n.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(fn.Body())
	return found
}

func (l *lowerer) functionValueCall(n, callee *ast.Node) *hir.Expr {
	x := l.callReceiverExpr(callee)
	if x == nil {
		return nil
	}
	if (x.Type.Kind == hir.Dynamic || x.Type.Kind == hir.Optional && x.Type.Args[0].Kind == hir.Dynamic) && l.checkedStoredCallable(callee) != nil {
		typ := l.functionType(callee, l.checkedStoredCallable(callee))
		x = l.coerce(x, typ)
	}
	optional := false
	if l.unprunedFeatures && x.Type.Kind == hir.Optional && len(x.Type.Args) == 1 && l.callableMethod(x.Type.Args[0]) != nil {
		// An optional function member: f?.() yields undefined when absent;
		// a plain f() on an absent member is JavaScript's TypeError, here the
		// checked narrowing that raises.
		optional = n.AsCallExpression().QuestionDotToken != nil
		x = l.tempInit(callee, x.Type, x)
	}
	slot := x
	if x.Type.Kind == hir.Optional && len(x.Type.Args) == 1 {
		slot = &hir.Expr{Kind: hir.Narrow, Node: l.node(callee), Type: x.Type.Args[0], X: x}
	}
	m := l.callableMethod(slot.Type)
	if m == nil {
		l.diagf(n, "unsupported-call", "stored function has no lowered call signature")
		return nil
	}
	recv := l.callReceiver(n, slot, nil)
	args, ok := l.callArgsMethod(n, n.Arguments(), m, 0)
	if !ok {
		return nil
	}
	if recv.Type.Name == thunkInterfaceName {
		return l.invokeThunk(n, recv)
	}
	call := &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), Name: "call", Type: m.Result, X: recv, Args: args}
	if !optional || m.Result.Kind == hir.Void {
		if optional {
			absent := &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: x}
			l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: absent}, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: call})})
			return &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: hir.T(hir.Bool), Stmt: hir.B(), Y: hir.L(hir.T(hir.Bool), false)}
		}
		return call
	}
	result := m.Result
	if result.Kind != hir.Optional {
		result = hir.T(hir.Optional, result)
	}
	return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: result, X: &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: x},
		Y: &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: result}, Z: l.coerce(call, result)}
}

// Nullable callable unions retain tagged storage so null and undefined stay
// distinct. Only a checked declaration can recover their callable ABI.
func (l *lowerer) checkedStoredCallable(n *ast.Node) *checker.Type {
	if !l.unprunedFeatures {
		return nil
	}
	sym := l.resolve(n)
	if sym == nil {
		return nil
	}
	t := l.ck.GetNonNullableType(l.ck.GetTypeOfSymbol(sym))
	if t.Flags()&checker.TypeFlagsAnyOrUnknown != 0 || len(l.ck.GetSignaturesOfType(t, checker.SignatureKindCall)) != 1 {
		return nil
	}
	return t
}

func (l *lowerer) storedCallableType(n *ast.Node, t hir.Type) bool {
	if t.Kind == hir.Optional {
		t = t.Args[0]
	}
	return l.isCallableValueType(t) || t.Kind == hir.Dynamic && l.checkedStoredCallable(n) != nil
}

func (l *lowerer) bindingMutable(symbol *ast.Symbol) bool {
	if symbol == nil || symbol.ValueDeclaration == nil {
		return false
	}
	for parent := symbol.ValueDeclaration.Parent; parent != nil; parent = parent.Parent {
		switch parent.Kind {
		case ast.KindMethodDeclaration, ast.KindConstructor, ast.KindFunctionDeclaration, ast.KindFunctionExpression, ast.KindArrowFunction:
			return l.assignedInFunction(parent, symbol)
		}
	}
	return false
}

func (l *lowerer) captureTransport(c localCapture, typ hir.Type) *hir.Expr {
	name := c.name
	if cell, ok := l.captureCells[c.symbol]; ok && cell.Equal(typ) && l.paramSymbols[l.method][name] == c.symbol {
		for _, p := range l.method.Params {
			if p.Name == name && !p.Type.Equal(cell) {
				name = l.captureParamNames[c.symbol]
				break
			}
		}
	}
	value := hir.V(name, typ)
	if l.localSymbols == nil {
		l.localSymbols = map[*hir.Expr]*ast.Symbol{}
	}
	l.localSymbols[value] = c.symbol
	if l.localSymbolMethods == nil {
		l.localSymbolMethods = map[*hir.Expr]*hir.Method{}
	}
	l.localSymbolMethods[value] = l.method
	return value
}

// Lazy lifted functions gain storage at their declaration when they escape.
// Repeated reads must observe the same JS function object.
func (l *lowerer) functionBindingPlaceholder(name *ast.Node) *hir.Stmt {
	if l.functionBindings == nil {
		l.functionBindings = map[*ast.Symbol]*hir.Stmt{}
	}
	placeholder := hir.B()
	l.functionBindings[l.ck.GetSymbolAtLocation(name)] = placeholder
	return placeholder
}

func (l *lowerer) functionBindingRead(n *ast.Node, binding *hir.Stmt, symbol *ast.Symbol) *hir.Expr {
	value := hir.V(binding.Name, binding.Type)
	value.Node = l.node(n)
	if l.localSymbols == nil {
		l.localSymbols = map[*hir.Expr]*ast.Symbol{}
	}
	l.localSymbols[value] = symbol
	return value
}

func (l *lowerer) pruneFunctionPlaceholders() {
	empty := map[*hir.Stmt]bool{}
	for _, binding := range l.functionBindings {
		if binding.Kind == hir.Block {
			empty[binding] = true
		}
	}
	if len(empty) == 0 {
		return
	}
	for _, c := range l.out.Classes {
		for _, m := range numberMethods(c) {
			walkNumberStmt(m.Body, func(s *hir.Stmt) {
				kept := s.List[:0]
				for _, child := range s.List {
					if !empty[child] {
						kept = append(kept, child)
					}
				}
				s.List = kept
			}, func(e *hir.Expr) {})
		}
	}
}

// Lifted members of a closure use its physical receiver. Source-level this
// may instead denote an outer object, carried through thisOverride.
func (l *lowerer) localFnReceiver(lf *localFn) *hir.Expr {
	if l.unprunedFeatures && strings.HasPrefix(lf.owner.Name, "closure.") {
		return &hir.Expr{Kind: hir.This, Type: hir.Ref(lf.owner.Name)}
	}
	return l.this(lf.owner)
}

func (l *lowerer) isCallableValueType(t hir.Type) bool { return l.callableMethod(t) != nil }
