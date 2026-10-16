package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"strings"
)

// Array higher-order calls, inlined into loops at the call site (ADR-0006).
// A callback may be a single expression, a block with exactly one `return e`,
// or a plain module-function reference; anything else is a diagnostic. The
// receiver is hoisted into a temporary through the prelude mechanism, so
// chains (filter().map()) desugar into consecutive loops.

// hofCall lowers arr.<name>(...) for the inlined operations; the second
// result reports whether name was one of them.
func (l *lowerer) hofCall(n *ast.Node, name string, recv *hir.Expr, args []*ast.Node) (*hir.Expr, bool) {
	if recv.Type.Kind != hir.Array {
		return nil, false
	}
	elem := recv.Type.Args[0]
	if len(args) > 0 && (args[0].Kind == ast.KindIdentifier || args[0].Kind == ast.KindPropertyAccessExpression || args[0].Kind == ast.KindCallExpression) {
		switch name {
		case "map", "filter", "some", "reduce", "find", "forEach", "every", "sort":
			// The receiver expression precedes callback lookup/bind. Keeping
			// its live reference also prevents a second evaluation for arg 3.
			recv = l.tempInit(n, recv.Type, recv)
		}
	}
	switch name {
	case "map", "filter", "some", "reduce", "find":
	case "forEach", "every":
		if len(args) == 1 && args[0].Kind == ast.KindFunctionExpression && callbackUsesThis(args[0].Body()) {
			l.diagf(args[0], "unsupported-callback", "%s function callback this binding is not lowered", name)
			return nil, true
		}
		if l.pinnedDenseCallback(n) || l.pinnedTestRows(n) {
			return l.denseCallbackLoop(n, name, recv, args), true
		}
		return l.arrayCallbackLoop(n, name, recv, args, true), true
	default:
		return l.syntaxHofCall(n, name, recv, args)
	}
	if len(args) > 0 && args[0].Kind == ast.KindIdentifier {
		if check := l.moduleCallableCall(n, args[0], hir.L(hir.T(hir.Bool), true)); check.Kind == hir.Seq {
			l.pendStmt(check.Stmt)
		}
	}
	if name == "reduce" && len(args) == 1 && l.unprunedFeatures {
		return l.reduceNoInitial(n, recv, elem, args[0])
	}
	if name == "reduce" {
		if len(args) != 2 {
			l.diagf(n, "unsupported-call", "reduce needs a callback and an initial value")
			return nil, true
		}
		x, ok := l.reduceLoop(n, recv, elem, args[0], args[1])
		return x, ok
	}
	if len(args) != 1 {
		l.diagf(n, "unsupported-call", "%s needs one callback", name)
		return nil, true
	}
	if name == "find" && !elem.IsRef() {
		if l.pinnedDenseCallback(n) {
			return l.denseCallbackLoop(n, name, recv, args), true
		}
		// Reuse the dense ABI with a checked Get at every visit. Its
		// Optional result distinguishes a match of ""/0 from no match;
		// a missing primitive slot refuses before callback invocation.
		return l.arrayCallbackLoop(n, name, recv, args, true), true
	}
	body, params, ok := l.callbackBody(n, args[0], elem)
	if !ok {
		return nil, true
	}
	l.diagf(n, "note-callback-inline", "%s callback inlined into a loop", name)
	if body.Type.Kind == hir.Void {
		return nil, true
	}
	if name == "find" {
		return l.findReferenceLoop(n, recv, elem, body, params), true
	}

	// Loop shape: ForEach binds params[0]; a second parameter is the index
	// (a counter incremented at the top of the body), a third the array.
	var idxInit *hir.Stmt
	if len(params) >= 2 {
		l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[1], Type: hir.T(hir.Number),
			X: hir.L(hir.T(hir.Number), -1)})
		idxInit = &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: hir.V(params[1], hir.T(hir.Number)),
			Y: &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "+", Type: hir.T(hir.Number),
				X: hir.V(params[1], hir.T(hir.Number)), Y: hir.L(hir.T(hir.Number), 1)}}
	}
	if len(params) >= 3 {
		l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[2], Type: recv.Type, X: recv})
	}
	loopBody := func(inner ...*hir.Stmt) *hir.Stmt {
		if idxInit != nil {
			return hir.B(append([]*hir.Stmt{idxInit}, inner...)...)
		}
		return hir.B(inner...)
	}

	switch name {
	case "map":
		out := l.tempVar(n, hir.T(hir.Array, body.Type))
		l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: params[0], Type: elem, X: recv,
			Body: loopBody(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n),
				X: l.rtOp("array.push", out, hir.T(hir.I32), body)})})
		return out, true
	case "filter":
		out := l.tempVar(n, hir.T(hir.Array, elem))
		cond := l.toBool(n, body)
		l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: params[0], Type: elem, X: recv,
			Body: loopBody(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: cond,
				Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n),
					X: l.rtOp("array.push", out, hir.T(hir.I32), hir.V(params[0], elem))})})})
		return out, true
	case "some":
		out := l.tempInit(n, hir.T(hir.Bool), hir.L(hir.T(hir.Bool), false))
		cond := l.toBool(n, body)
		l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: params[0], Type: elem, X: recv,
			Body: loopBody(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: cond,
				Body: hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: out, Y: hir.L(hir.T(hir.Bool), true)},
					&hir.Stmt{Kind: hir.Break, Node: l.node(n)})})})
		return out, true
	}
	return nil, false
}

// find captures length once, but reads the current slot for each callback.
// Unlike a table LOOP this ignores appended slots and observes removals and
// replacements. Reference slots preserve undefined as an initial reference;
// primitive slots need a separate callback ABI and remain blocked.
func (l *lowerer) findReferenceLoop(n *ast.Node, receiver *hir.Expr, elem hir.Type, predicate *hir.Expr, params []string) *hir.Expr {
	recv := l.tempInit(n, receiver.Type, receiver)
	limit := l.tempInit(n, hir.T(hir.Number), l.rtOp("array.length", recv, hir.T(hir.I32)))
	index := l.tempInit(n, hir.T(hir.Number), hir.L(hir.T(hir.Number), 0))
	optional := hir.T(hir.Optional, elem)
	out := l.tempInit(n, optional, &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: optional, Value: nil})
	var body []*hir.Stmt
	body = append(body, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[0], Type: elem,
		X: &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: elem, X: l.rtOp("array.get", recv, optional, index)}})
	if len(params) >= 2 {
		body = append(body, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[1], Type: hir.T(hir.Number), X: l.coerce(index, hir.T(hir.Number))})
	}
	if len(params) >= 3 {
		body = append(body, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[2], Type: recv.Type, X: recv})
	}
	body = append(body, &hir.Stmt{Kind: hir.If, Node: l.node(n), X: l.toBool(n, predicate), Body: hir.B(
		&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: out, Y: l.coerce(hir.V(params[0], elem), optional)},
		&hir.Stmt{Kind: hir.Break, Node: l.node(n)},
	)}, &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: index, Y: &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "+", Type: index.Type, X: index, Y: hir.L(index.Type, 1)}})
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "<", Type: hir.T(hir.Bool), X: index, Y: limit}, Body: hir.B(body...)})
	return out
}

// toBool applies JavaScript truthiness when the expression is not boolean.
func (l *lowerer) toBool(n *ast.Node, x *hir.Expr) *hir.Expr {
	if x.Type.Kind == hir.Bool {
		return x
	}
	return &hir.Expr{Kind: hir.ToBoolean, Node: l.node(n), Type: hir.T(hir.Bool), X: x}
}

// tempInit declares a fresh local initialized with x.
func (l *lowerer) tempInit(at *ast.Node, t hir.Type, x *hir.Expr) *hir.Expr {
	l.serial++
	name := "t" + itoa(l.serial)
	l.declare(name, t)
	l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(at), Name: name, Type: t, X: x})
	return hir.V(name, t)
}

// reduceLoop lowers arr.reduce((acc, e) => ..., init): the callback's first
// parameter is the accumulator, the second the element.
func (l *lowerer) reduceLoop(n *ast.Node, recv *hir.Expr, elem hir.Type, cb, init *ast.Node) (*hir.Expr, bool) {
	body, params, ok := l.callbackBody(n, cb, elem)
	if !ok || len(params) < 1 || len(params) > 4 {
		l.diagf(n, "unsupported-call", "reduce callback needs at most four parameters")
		return nil, true
	}
	if len(params) == 1 {
		l.serial++
		params = append(params, "e"+itoa(l.serial))
	}
	accT := l.lookupTypeOf(params[0])
	if accT.Kind == hir.Void {
		accT = body.Type
	}
	l.hint = accT
	acc0 := l.expr(init)
	l.hint = hir.Type{}
	if acc0 == nil {
		return nil, true
	}
	acc := l.tempInit(n, accT, acc0)
	l.declare(params[0], accT)
	l.declare(params[1], elem)
	var pre []*hir.Stmt
	if len(params) >= 3 {
		index := hir.V(params[2], hir.T(hir.Number))
		l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: index.Name, Type: index.Type, X: hir.L(index.Type, -1)})
		pre = append(pre, &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: index, Y: &hir.Expr{Kind: hir.Binary, Op: "+", Type: index.Type, X: index, Y: hir.L(index.Type, 1)}})
	}
	if len(params) >= 4 {
		l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[3], Type: recv.Type, X: recv})
	}
	pre = append(pre, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[0], Type: accT, X: acc},
		&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: acc, Y: body})
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: params[1], Type: elem, X: recv, Body: hir.B(pre...)})
	return acc, true
}

// reduceNoInitial lowers arr.reduce(cb) as ECMAScript does: the length is
// read once, holes are skipped, the first present element is the initial
// accumulator, and an array without one throws a TypeError.
func (l *lowerer) reduceNoInitial(n *ast.Node, recv *hir.Expr, elem hir.Type, cb *ast.Node) (*hir.Expr, bool) {
	if recv.Type.Kind != hir.Array {
		l.diagf(n, "unsupported-call", "reduce without an initial value needs an array")
		return nil, true
	}
	arr := l.tempInit(n, recv.Type, recv)
	body, params, ok := l.callbackBody(n, cb, elem)
	if !ok || len(params) < 1 || len(params) > 4 {
		l.diagf(n, "unsupported-call", "reduce callback needs at most four parameters")
		return nil, true
	}
	if len(params) == 1 {
		l.serial++
		params = append(params, "e"+itoa(l.serial))
	}
	i, b := hir.T(hir.I32), hir.T(hir.Bool)
	bin := func(op string, x, y *hir.Expr, t hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: op, Type: t, X: x, Y: y}
	}
	has := func(k *hir.Expr) *hir.Expr { return l.rtOp("array.has", arr, b, k) }
	at := func(k *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: elem, X: l.rtOp("array.get", arr, hir.T(hir.Optional, elem), k)}
	}
	step := func(k *hir.Expr) *hir.Stmt {
		return &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: k, Y: bin("+", k, hir.L(i, 1), i)}
	}
	length := l.tempInit(n, i, l.rtOp("array.length", arr, i))
	k := l.tempInit(n, i, hir.L(i, 0))
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: bin("&&", bin("<", k, length, b), &hir.Expr{Kind: hir.Unary, Op: "!", Type: b, X: has(k)}, b), Body: hir.B(step(k))})
	errorType := hir.Ref(l.builtinTypeError())
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: bin(">=", k, length, b), Body: hir.B(&hir.Stmt{Kind: hir.Throw, Node: l.node(n),
		X: &hir.Expr{Kind: hir.New, Node: l.node(n), Type: errorType, Args: []*hir.Expr{l.coerce(hir.L(hir.T(hir.String), "Reduce of empty array with no initial value"), hir.T(hir.Optional, hir.T(hir.String)))}}})})
	acc := l.tempInit(n, elem, at(k))
	l.pendStmt(step(k))
	l.declare(params[0], elem)
	l.declare(params[1], elem)
	inner := []*hir.Stmt{
		{Kind: hir.VarDecl, Node: l.node(n), Name: params[0], Type: elem, X: acc},
		{Kind: hir.VarDecl, Node: l.node(n), Name: params[1], Type: elem, X: at(k)},
	}
	if len(params) >= 3 {
		inner = append(inner, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[2], Type: hir.T(hir.Number), X: l.rtOp("number.fromI32", k, hir.T(hir.Number))})
	}
	if len(params) >= 4 {
		inner = append(inner, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[3], Type: arr.Type, X: arr})
	}
	value := l.coerce(body, elem)
	if !value.Type.Equal(elem) {
		l.diagf(n, "unsupported-call", "reduce without an initial value needs a callback returning the element type")
		return nil, true
	}
	inner = append(inner, &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: acc, Y: value})
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: bin("<", k, length, b), Body: hir.B(
		&hir.Stmt{Kind: hir.If, Node: l.node(n), X: has(k), Body: hir.B(inner...)},
		step(k))})
	return acc, true
}

// callbackBody lowers a callback to its value expression and its parameter
// names (the caller binds them). Supported: `x => expr`, `(x, i, a) => expr`,
// `x => { return expr; }` with exactly one return, a plain identifier naming
// a module function, and destructured first parameters.
func (l *lowerer) callbackBody(n *ast.Node, cb *ast.Node, elem hir.Type) (*hir.Expr, []string, bool) {
	if cb.Kind == ast.KindIdentifier {
		if sym := l.resolve(cb); sym != nil && l.refuseUndefinedThis(sym.ValueDeclaration) {
			return nil, nil, false
		}
	}
	if cb.Kind == ast.KindIdentifier {
		if x, names, ok := l.localValueCallback(n, cb, elem); ok {
			return x, names, x != nil
		}
	}
	if body, params, ok := l.booleanCallback(n, cb, elem); ok {
		return body, params, true
	}
	// Preparing a method reference (especially bind's receiver) belongs
	// outside the loop. The callee adapter itself runs once per visit.
	if cb.Kind == ast.KindPropertyAccessExpression || cb.Kind == ast.KindCallExpression {
		return l.methodCallbackBody(n, cb, elem)
	}
	savedPending := l.pend
	l.pend = nil
	l.push()
	defer func() { l.pend = savedPending; l.pop() }()

	if cb.Kind == ast.KindIdentifier {
		name := cb.Text()
		if _, ok := l.lookup(name); !ok {
			if sym := l.resolve(cb); sym != nil {
				if f := l.fileOfSymbol(sym); f != nil {
					if ref, ok := l.funcsBy[f.FileName()+" "+sym.Name]; ok {
						return l.namedCallbackBody(n, cb, elem, ref)
					}
				}
			}
		}
	}
	if cb.Kind != ast.KindArrowFunction && cb.Kind != ast.KindFunctionExpression {
		l.diagf(n, "unsupported-callback", "callback %s is not an arrow function", cb.Kind.String())
		return nil, nil, false
	}
	// Defaults, rest and omitted tails need a callee prologue and an
	// explicit argument adapter, just like ordinary calls. Keep simple
	// callbacks inlined to preserve existing output.
	limit := 3
	if callee := n.Expression(); callee != nil && callee.Kind == ast.KindPropertyAccessExpression {
		switch callee.Name().Text() {
		case "sort":
			limit = 2
		case "reduce":
			limit = 4
		}
	}
	adapt := len(cb.Parameters()) > limit
	for _, p := range cb.Parameters() {
		adapt = adapt || p.Initializer() != nil || p.AsParameterDeclaration().DotDotDotToken != nil
	}
	if adapt {
		l.serial++
		lf := l.liftLocalFn("callback_"+itoa(l.serial), cb)
		if lf == nil {
			return nil, nil, false
		}
		ref := funcRef{owner: lf.owner.Name, method: lf.method.Name}
		return l.callbackInvocation(n, cb, elem, ref, lf, nil)
	}
	params := cb.Parameters()

	var names []string
	for _, pn := range params {
		p := pn
		if p.Name() == nil || p.Name().Kind != ast.KindIdentifier {
			// A destructured parameter binds to the element and declares the
			// pattern's fields.
			l.serial++
			v := "d" + itoa(l.serial)
			l.destructurePattern(p.Name(), hir.V(v, elem))
			names = append(names, v)
			continue
		}
		names = append(names, p.Name().Text())
	}
	// Parameter types come from the checker (the element for the first
	// parameter of map/filter/some, the accumulator for reduce, the index
	// for the second), falling back to the element type.
	for i, p := range names {
		if i >= len(params) {
			break
		}
		t := elem
		if ps := params[i].Symbol(); ps != nil {
			before := len(l.diags)
			mapped := l.mapCheckerType(n, l.ck.GetTypeOfSymbol(ps))
			if len(l.diags) == before && mapped.Kind != hir.Void {
				t = mapped
			} else {
				l.diags = l.diags[:before]
			}
		}
		if i == 0 && (l.collectionShapes[elem.Name] || l.collectionElements[elem.String()]) && params[i].Type() == nil {
			// Inferred callback types can repeat the checker's first-row
			// structural reduction. Bind the original collection's slot layout.
			t = elem
			if l.widenedLets == nil {
				l.widenedLets = map[*ast.Symbol]bool{}
			}
			l.widenedLets[params[i].Symbol()] = true
		}
		if i == 0 && n.Expression() != nil && n.Expression().Kind == ast.KindPropertyAccessExpression && n.Expression().Name().Text() == "from" && l.isGlobalArrayExpr(n.Expression().Expression()) {
			// Array-like holes supply undefined even when TS inferred a
			// non-optional index signature. Bind the actual iteration value.
			t = elem
			if l.widenedLets == nil {
				l.widenedLets = map[*ast.Symbol]bool{}
			}
			l.widenedLets[params[i].Symbol()] = true
		}
		l.declare(p, t)
	}
	if len(names) == 0 {
		l.serial++
		names = append(names, "e"+itoa(l.serial))
		l.declare(names[0], elem)
	}
	body := cb.Body()
	if body == nil {
		return nil, nil, false
	}
	var value *ast.Node
	if body.Kind != ast.KindBlock {
		value = body
	} else {
		stmts := body.AsBlock().Statements.Nodes
		last := len(stmts) - 1
		if last < 0 || stmts[last].Kind != ast.KindReturnStatement || stmts[last].AsReturnStatement().Expression == nil {
			if l.containsReturn(body) {
				return l.callbackWithReturns(cb, names)
			}
			l.diagf(n, "unsupported-callback", "callback block must end with a return of a value")
			return nil, nil, false
		}
		for _, s := range stmts[:last] {
			if l.containsReturn(s) {
				return l.callbackWithReturns(cb, names)
			}
		}
		// Leading statements run per element, before the result value.
		keep := l.hint
		for _, s := range stmts[:last] {
			lowered := l.stmts(s)
			l.pend = append(l.pend, lowered...)
		}
		l.hint = keep
		value = stmts[last].AsReturnStatement().Expression
	}
	if l.hint.Kind == hir.Array {
		l.hint = l.hint.Args[0]
	} else {
		l.hint = hir.Type{}
	}
	x := l.expr(value)
	if x == nil {
		return nil, nil, false
	}
	if len(l.pend) > 0 {
		x = &hir.Expr{Kind: hir.Seq, Node: l.node(cb), Type: x.Type, Stmt: hir.B(l.pend...), Y: x}
	}
	return x, names, true
}

// A callback with early returns uses an ordinary lifted method, so a return
// exits that invocation and cannot escape the surrounding array operation.
func (l *lowerer) callbackWithReturns(cb *ast.Node, names []string) (*hir.Expr, []string, bool) {
	l.serial++
	lf := l.liftLocalFn("callback_"+itoa(l.serial), cb)
	if lf == nil || lf.method.Result.Kind == hir.Void {
		return nil, nil, false
	}
	var args []*hir.Expr
	for _, capture := range lf.captures {
		value := l.captureTransport(capture, capture.typ)
		if l.localSymbols == nil {
			l.localSymbols = map[*hir.Expr]*ast.Symbol{}
		}
		if _, shared := l.captureCells[capture.symbol]; !shared {
			l.localSymbols[value] = capture.symbol
		}
		args = append(args, value)
	}
	for index, name := range names {
		if len(lf.captures)+index >= len(lf.method.Params) {
			break
		}
		args = append(args, hir.V(name, lf.method.Params[len(lf.captures)+index].Type))
	}
	call := &hir.Expr{Kind: hir.VirtualCall, Node: l.node(cb), X: l.localFnReceiver(lf), Name: lf.method.Name, Type: lf.method.Result, Args: args}
	if lf.method.Static {
		call.Kind, call.Owner, call.X = hir.DirectCall, lf.owner.Name, nil
	}
	return call, names, true
}

// destructurePattern declares the fields of an object or array binding
// pattern read from x.
func (l *lowerer) destructurePattern(pattern *ast.Node, x *hir.Expr) {
	if pattern == nil {
		return
	}
	switch pattern.Kind {
	case ast.KindObjectBindingPattern:
		for _, el := range pattern.AsBindingPattern().Elements.Nodes {
			if el.Kind != ast.KindBindingElement {
				continue
			}
			be := el.AsBindingElement()
			if be.Name() == nil || be.Name().Kind != ast.KindIdentifier {
				continue
			}
			local := be.Name().Text()
			if be.DotDotDotToken != nil {
				// `...rest` is a fresh object of the remaining fields.
				l.objectRest(el, local, x)
				continue
			}
			field := local
			if be.PropertyName != nil && be.PropertyName.Kind == ast.KindIdentifier {
				field = be.PropertyName.Text()
			}
			ft := l.patternFieldType(x.Type, field)
			lt := ft
			if sym := el.Symbol(); sym != nil && ft.IsRef() {
				// A required physical field may be an optional binding.
				before := len(l.diags)
				mapped := l.mapCheckerType(el, l.ck.GetTypeOfSymbol(sym))
				if hasBlocking(l.diags[before:]) {
					l.diags = l.diags[:before]
				} else if mapped.Kind == hir.Optional && mapped.Args[0].Equal(ft) {
					lt = mapped
				}
			}
			l.declare(local, lt)
			decl := &hir.Stmt{Kind: hir.VarDecl, Node: l.node(el), Name: local, Type: lt,
				X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(el), Name: field, Type: ft, X: x}}
			l.recordPatternBinding(el.Name(), decl)
			l.pendStmt(decl)
		}
	case ast.KindArrayBindingPattern:
		if x.Type.Kind == hir.Dynamic {
			// A union of array storages is boxed rather than retagged as one
			// generic array. Read each checked binding through that live box.
			var arrays func(*checker.Type) bool
			arrays = func(t *checker.Type) bool {
				if t == nil {
					return false
				}
				if t.IsUnion() {
					for _, part := range t.AsUnionOrIntersectionType().Types() {
						if !arrays(part) {
							return false
						}
					}
					return true
				}
				return l.ck.IsArrayType(t) || t.IsTupleType()
			}
			if pattern.Parent == nil || pattern.Parent.Initializer() == nil || !arrays(l.ck.GetTypeAtLocation(pattern.Parent.Initializer())) {
				l.diagf(pattern, "unsupported-binding", "dynamic array destructuring requires checked array storage")
				return
			}
		}
		for i, el := range pattern.AsBindingPattern().Elements.Nodes {
			if el.Kind != ast.KindBindingElement || el.Name() == nil || el.Name().Kind != ast.KindIdentifier {
				continue
			}
			local := el.Name().Text()
			var value *hir.Expr
			if x.Type.Kind == hir.Dynamic {
				if el.AsBindingElement().DotDotDotToken != nil {
					l.diagf(el, "unsupported-binding", "rest binding of boxed array storage")
					return
				}
				typ := l.mapCheckerType(el, l.ck.GetTypeAtLocation(el.Name()))
				view := l.rtOp("dynamic.arrayView", x, hir.T(hir.Array, hir.T(hir.Dynamic)))
				view.Node = l.node(el)
				value = l.coerce(&hir.Expr{Kind: hir.IndexGet, Node: l.node(el), Type: hir.T(hir.Dynamic), X: view, Y: hir.L(hir.T(hir.I32), i)}, typ)
			} else if x.Type.Kind == hir.Array {
				if el.AsBindingElement().DotDotDotToken != nil {
					value = l.rtOp("array.slice1", x, x.Type, hir.L(hir.T(hir.I32), i))
				} else {
					value = &hir.Expr{Kind: hir.IndexGet, Node: l.node(el), Type: x.Type.Args[0], X: x, Y: hir.L(hir.T(hir.I32), i)}
				}
			} else if el.AsBindingElement().DotDotDotToken != nil {
				typ := l.mapCheckerType(el, l.ck.GetTypeAtLocation(el.Name()))
				if typ.Kind != hir.Array {
					l.diagf(el, "unsupported-binding", "tuple rest must be an array")
					return
				}
				value = l.tempVar(el, typ)
				c := l.classByName(x.Type.Name)
				for _, field := range c.Fields[i:] {
					v := &hir.Expr{Kind: hir.FieldGet, Node: l.node(el), Name: field.Name, Type: field.Type, X: x}
					l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(el), X: l.rtOp("array.push", value, hir.T(hir.I32), l.coerce(v, typ.Args[0]))})
				}
			} else {
				field := "f" + itoa(i)
				value = &hir.Expr{Kind: hir.FieldGet, Node: l.node(el), Type: l.patternFieldType(x.Type, field), Name: field, X: x}
			}
			l.declare(local, value.Type)
			decl := &hir.Stmt{Kind: hir.VarDecl, Node: l.node(el), Name: local, Type: value.Type, X: value}
			l.recordPatternBinding(el.Name(), decl)
			l.pendStmt(decl)
		}

	}
}

// patternFieldType reads the lowered field type of a synthesized shape.
func (l *lowerer) patternFieldType(t hir.Type, field string) hir.Type {
	if t.Kind != hir.ClassRef {
		return hir.T(hir.Void)
	}
	for _, c := range l.out.Classes {
		if c.Name != t.Name {
			continue
		}
		for _, f := range c.Fields {
			if f.Name == field {
				return f.Type
			}
		}
	}
	return hir.T(hir.Void)
}

// callbackResultType resolves a module function's result type by reference.
func (l *lowerer) callbackResultType(ref funcRef) hir.Type {
	for _, c := range l.out.Classes {
		if c.Name != ref.owner {
			continue
		}
		for _, m := range c.Methods {
			if m.Name == ref.method {
				return m.Result
			}
		}
	}
	return hir.T(hir.Void)
}

// lookupTypeOf returns the declared HIR type of a local name.
func (l *lowerer) lookupTypeOf(name string) hir.Type {
	if t, ok := l.lookup(name); ok {
		return t
	}
	return hir.T(hir.Void)
}

// localFnCall builds a call to a lifted local arrow function: the captured
// constants are passed as leading arguments, re-read at the call site (they
// are never assigned).
func (l *lowerer) localFnCall(n *ast.Node, lf *localFn, callee *ast.Node) *hir.Expr {
	if l.integerOptions.TrapOnFailure {
		if sym := l.resolve(callee); sym != nil && sym.ValueDeclaration != nil {
			d := sym.ValueDeclaration
			if d.Kind == ast.KindVariableDeclaration && d.Initializer() != nil && d.Initializer().Kind == ast.KindArrowFunction && l.testRegistrationHelper(d.Initializer()) && n.Pos() < d.End() {
				// Suite const arrows need a source-order initialization proof.
				// Refuse before argument preludes; do not fabricate a ready
				// function binding or a catchable JavaScript ReferenceError.
				l.diagf(n, "unsupported-call", "suite arrow %s is used before its initialization", sym.Name)
				return nil
			}
		}
	}
	args := []*hir.Expr{}
	for _, c := range lf.captures {
		if c.typ.Kind == hir.Void {
			l.diagf(callee, "unsupported-call", "capture %s is not in scope", c.name)
			return nil
		}
		capture := hir.V(c.name, c.typ)
		if cell, shared := l.captureCells[c.symbol]; shared {
			capture = l.captureTransport(c, cell)
		}
		if l.localSymbols == nil {
			l.localSymbols = map[*hir.Expr]*ast.Symbol{}
		}
		if _, cell := l.captureCells[c.symbol]; !cell {
			l.localSymbols[capture] = c.symbol
		}
		args = append(args, capture)
	}
	rest, ok := l.callArgsMethod(n, n.Arguments(), lf.method, len(args))
	if !ok {
		return nil
	}
	if lf.method.Static {
		return &hir.Expr{Kind: hir.DirectCall, Node: l.node(n), Owner: lf.owner.Name,
			Name: lf.method.Name, Type: lf.method.Result, Args: append(args, rest...)}
	}
	return &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), Name: lf.method.Name,
		Type: lf.method.Result, X: l.localFnReceiver(lf), Args: append(args, rest...)}
}

// callArgsStatic lowers call arguments against a module function's method.
func (l *lowerer) callArgsStatic(n *ast.Node, args []*ast.Node, ref funcRef) ([]*hir.Expr, bool) {
	for _, c := range l.out.Classes {
		if c.Name != ref.owner {
			continue
		}
		for _, m := range c.Methods {
			if m.Name == ref.method {
				return l.callArgsMethod(n, args, m, 0)
			}
		}
	}
	return nil, false
}

// callArgsMethod lowers arguments against a method signature, starting at
// parameter position skip (lifted captures). A trailing variadic parameter
// collects the remaining arguments into an array; a spread fills it at once.
func (l *lowerer) callArgsMethod(at *ast.Node, args []*ast.Node, m *hir.Method, skip int) ([]*hir.Expr, bool) {
	params := m.Params[skip:]
	fixed := len(params)
	if fixed > 0 && params[fixed-1].Variadic {
		fixed--
	}
	for i, arg := range args {
		if arg.Kind == ast.KindSpreadElement && (i < fixed || fixed == len(params)) {
			return l.tupleSpreadArgs(at, args, params)
		}
	}

	if len(params) > 0 && params[len(params)-1].Variadic {
		rest := params[len(params)-1]
		fixed := params[:len(params)-1]
		fixedArgsIn := args
		if len(fixedArgsIn) > len(fixed) {
			fixedArgsIn = fixedArgsIn[:len(fixed)]
		}
		out, ok := l.fixedArgs(at, fixedArgsIn, fixed, true)
		if !ok {
			return nil, false
		}
		restArgs := []*ast.Node{}
		if len(args) > len(fixed) {
			restArgs = args[len(fixed):]
		}
		arr := l.tempVar(at, rest.Type)
		for _, ra := range restArgs {
			if ra.Kind == ast.KindSpreadElement {
				v := l.expr(ra.Expression())
				if v != nil && v.Type.Kind == hir.ClassRef && strings.HasPrefix(v.Type.Name, "tuple.") {
					v = l.tempInit(ra, v.Type, v)
					for _, f := range l.classByName(v.Type.Name).Fields {
						value := &hir.Expr{Kind: hir.FieldGet, Name: f.Name, Type: f.Type, X: v}
						l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, X: l.rtOp("array.push", arr, hir.T(hir.I32), l.coerce(value, rest.Type.Args[0]))})
					}
					continue
				}
				if v == nil || v.Type.Kind != hir.Array {
					l.diagf(ra, "unsupported-call", "spread must match the rest array type")
					return nil, false
				}
				l.serial++
				row := hir.V("spread"+itoa(l.serial), v.Type.Args[0])
				l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(ra), Name: row.Name, Type: row.Type, X: v, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(ra), X: l.rtOp("array.push", arr, hir.T(hir.I32), l.coerce(row, rest.Type.Args[0]))})})
				continue
			}
			l.hint = rest.Type.Args[0]
			v := l.expr(ra)
			l.hint = hir.Type{}
			if v == nil {
				return nil, false
			}
			l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(ra), X: l.rtOp("array.push", arr, hir.T(hir.I32), l.coerce(v, rest.Type.Args[0]))})
		}
		return append(out, arr), true
	}
	out, ok := l.fixedArgs(at, args, params, false)
	return out, ok
}

// fixedArgs lowers arguments for a fixed parameter list.
func (l *lowerer) fixedArgs(at *ast.Node, args []*ast.Node, params []hir.Param, restPrelude bool) ([]*hir.Expr, bool) {
	snapshot := func(value *hir.Expr) *hir.Expr {
		if restPrelude {
			l.serial++
			name := "arg" + itoa(l.serial)
			l.declare(name, value.Type)
			l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(at), Name: name, Type: value.Type, X: value})
			return hir.V(name, value.Type)
		}
		if len(args) > len(params) {
			return l.tempInit(at, value.Type, value)
		}
		return value
	}
	out := make([]*hir.Expr, 0, len(params))
	for i, p := range params {
		if i < len(args) {
			if args[i].Kind == ast.KindSpreadElement {
				l.diagf(at, "unsupported-call", "spread against a fixed parameter is not lowered")
				return nil, false
			}
			l.hint = p.Type
			a := l.expr(args[i])
			l.hint = hir.Type{}
			if a == nil {
				return nil, false
			}
			if p.Type.Kind == hir.Optional || p.Type.Kind == hir.Dynamic {
				a = l.observedArrayIndex(a)
			}
			a = l.presentValue(args[i], a, p.Type)
			out = append(out, snapshot(l.coerce(a, p.Type)))
			continue
		}
		if p.Type.Kind == hir.Optional {
			out = append(out, snapshot(&hir.Expr{Kind: hir.Lit, Type: p.Type}))
			continue
		}
		l.diagf(at, "unsupported-call", "missing argument for parameter %s", p.Name)
		return nil, false
	}
	if len(args) > len(params) {
		// All supplied arguments run left to right, including arguments
		// ignored by the callee. Save accepted values before extra effects.
		for _, arg := range args[len(params):] {
			if arg.Kind == ast.KindSpreadElement {
				l.diagf(arg, "unsupported-call", "extra spread requires a finite tuple")
				return nil, false
			}
			value := l.expr(arg)
			if value == nil {
				return nil, false
			}
			l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(arg), X: value})
		}
	}
	return out, true
}

// spreadPush lowers arr.push(...xs) into a loop of pushes, as an expression
// yielding the new length (which the callers ignore).
func (l *lowerer) spreadPush(n *ast.Node, recv *hir.Expr, spread *ast.Node) *hir.Expr {
	xs := l.expr(spread.Expression())
	if xs != nil && xs.Type.Kind == hir.OrderedSet {
		xs = l.rtOp("set.values", xs, hir.T(hir.Array, xs.Type.Args[0]))
	}
	if xs == nil || xs.Type.Kind != hir.Array || !l.acceptsType(recv.Type.Args[0], xs.Type.Args[0]) {
		have := "nil"
		if xs != nil {
			have = xs.Type.String()
		}
		l.diagf(n, "unsupported-call", "push spread needs a matching array (%s <- %s)", recv.Type.String(), have)
		return nil
	}
	l.serial++
	v := "p" + itoa(l.serial)
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: v, Type: xs.Type.Args[0], X: xs,
		Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n),
			X: l.rtOp("array.push", recv, hir.T(hir.I32), l.coerce(hir.V(v, xs.Type.Args[0]), recv.Type.Args[0]))})})
	return l.rtOp("array.length", recv, hir.T(hir.I32))
}

// Finite tuple spreads provide a checked arity even across fixed parameters.
// Evaluate each source once before distributing its elements to parameters.
func (l *lowerer) tupleSpreadArgs(at *ast.Node, args []*ast.Node, params []hir.Param) ([]*hir.Expr, bool) {
	var values []*hir.Expr
	for argIndex, arg := range args {
		if arg.Kind != ast.KindSpreadElement {
			x := l.expr(arg)
			if x == nil {
				return nil, false
			}
			values = append(values, l.tempInit(arg, x.Type, x))
			continue
		}
		x := l.expr(arg.Expression())
		if x == nil {
			return nil, false
		}
		x = l.tempInit(arg, x.Type, x)
		if x.Type.Kind == hir.ClassRef && strings.HasPrefix(x.Type.Name, "tuple.") {
			c := l.classByName(x.Type.Name)
			for _, field := range c.Fields {
				value := &hir.Expr{Kind: hir.FieldGet, Node: l.node(arg), Type: field.Type, Name: field.Name, X: x}
				if argIndex+1 < len(args) {
					value = l.tempInit(arg, value.Type, value)
				}
				values = append(values, value)
			}
		} else {
			t := l.ck.GetTypeAtLocation(arg.Expression())
			if x.Type.Kind != hir.Array || t == nil || !t.IsTupleType() {
				l.diagf(arg, "unsupported-call", "fixed-parameter spread requires a finite tuple")
				return nil, false
			}
			for i := 0; l.ck.GetPropertyOfType(t, itoa(i)) != nil; i++ {
				value := &hir.Expr{Kind: hir.IndexGet, Node: l.node(arg), Type: x.Type.Args[0], X: x, Y: hir.L(hir.T(hir.I32), i)}
				if argIndex+1 < len(args) {
					value = l.tempInit(arg, value.Type, value)
				}
				values = append(values, value)
			}
		}
	}
	var out []*hir.Expr
	for i, p := range params {
		if p.Variadic {
			array := l.tempVar(at, p.Type)
			if i < len(values) {
				for _, value := range values[i:] {
					l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(at), X: l.rtOp("array.push", array, hir.T(hir.I32), l.coerce(value, p.Type.Args[0]))})
				}
			}
			out = append(out, array)
			break
		}
		if i < len(values) {
			out = append(out, l.coerce(values[i], p.Type))
		} else if p.Type.Kind == hir.Optional {
			out = append(out, &hir.Expr{Kind: hir.Lit, Type: p.Type})
		} else {
			l.diagf(at, "unsupported-call", "tuple spread is missing parameter %s", p.Name)
			return nil, false
		}
	}
	return out, true
}

// Named callbacks use the implementation's physical ABI, rather than making
// a one-argument HIR call regardless of its declared parameters. Array
// callbacks receive element, index, array; absent optional tails stay absent.
func (l *lowerer) namedCallbackBody(at, cb *ast.Node, elem hir.Type, ref funcRef) (*hir.Expr, []string, bool) {
	return l.callbackInvocation(at, cb, elem, ref, nil, nil)
}

func (l *lowerer) callbackInvocation(at, cb *ast.Node, elem hir.Type, ref funcRef, lf *localFn, receiver *hir.Expr) (*hir.Expr, []string, bool) {
	var method *hir.Method
	for _, c := range l.out.Classes {
		if c.Name == ref.owner {
			for _, m := range c.Methods {
				if m.Name == ref.method {
					method = m
					break
				}
			}
		}
	}
	if method == nil {
		return nil, nil, false
	}
	params := method.Params
	var args []*hir.Expr
	if lf != nil {
		params = params[len(lf.captures):]
		for _, capture := range lf.captures {
			value := l.captureTransport(capture, capture.typ)
			if l.localSymbols == nil {
				l.localSymbols = map[*hir.Expr]*ast.Symbol{}
			}
			if _, shared := l.captureCells[capture.symbol]; !shared {
				l.localSymbols[value] = capture.symbol
			}
			args = append(args, value)
		}
	}
	types := []hir.Type{elem, hir.T(hir.Number), hir.T(hir.Array, elem)}
	minimum := 1
	if callee := at.Expression(); callee != nil && callee.Kind == ast.KindPropertyAccessExpression {
		switch callee.Name().Text() {
		case "sort":
			types, minimum = []hir.Type{elem, elem}, 2
		case "reduce":
			acc := elem
			if len(at.Arguments()) > 1 {
				acc = l.mapCheckerType(at, l.ck.GetTypeAtLocation(at.Arguments()[1]))
			}
			types, minimum = []hir.Type{acc, elem, hir.T(hir.Number), hir.T(hir.Array, elem)}, 2
		}
	}
	count := len(params)
	if count > 0 && params[count-1].Variadic {
		count = len(types)
	}
	if count > len(types) {
		count = len(types)
	}
	if count < minimum {
		count = minimum
	}
	var names []string
	var supplied []*hir.Expr
	for i := 0; i < count; i++ {
		l.serial++
		names = append(names, "e"+itoa(l.serial))
		supplied = append(supplied, hir.V(names[i], types[i]))
	}
	for i, p := range params {
		if p.Variadic {
			arr := l.tempVar(cb, p.Type)
			tail := supplied
			if i < len(tail) {
				tail = tail[i:]
			} else {
				tail = nil
			}
			for _, value := range tail {
				l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(cb), X: l.rtOp("array.push", arr, hir.T(hir.I32), l.coerce(value, p.Type.Args[0]))})
			}
			args = append(args, arr)
			break
		}
		if i < len(supplied) {
			args = append(args, l.coerce(supplied[i], p.Type))
		} else if p.Type.Kind == hir.Optional {
			args = append(args, &hir.Expr{Kind: hir.Lit, Type: p.Type})
		} else {
			l.diagf(cb, "unsupported-callback", "missing callback argument for parameter %s", p.Name)
			return nil, nil, false
		}
	}
	call := &hir.Expr{Kind: hir.DirectCall, Node: l.node(cb), Owner: ref.owner, Name: ref.method, Type: method.Result, Args: args}
	if receiver != nil {
		call.Kind, call.Owner, call.X = hir.VirtualCall, "", receiver
	} else if lf != nil && !method.Static {
		call.Kind, call.Owner, call.X = hir.VirtualCall, "", l.localFnReceiver(lf)
	}
	if len(l.pend) > 0 {
		call = &hir.Expr{Kind: hir.Seq, Node: l.node(cb), Type: call.Type, Stmt: hir.B(l.pend...), Y: call}
	}
	return call, names, true
}

// Pattern bindings participate in the same cell rewrite as ordinary locals.
func (l *lowerer) recordPatternBinding(name *ast.Node, decl *hir.Stmt) {
	if !l.unprunedFeatures {
		return
	}
	if l.declSymbols == nil {
		l.declSymbols = map[*hir.Stmt]*ast.Symbol{}
	}
	l.declSymbols[decl] = l.ck.GetSymbolAtLocation(name)
}
