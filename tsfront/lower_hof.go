package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
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
	switch name {
	case "map", "filter", "some", "reduce":
	default:
		return nil, false
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
	body, params, ok := l.callbackBody(n, args[0], elem)
	if !ok {
		return nil, true
	}
	l.diagf(n, "note-callback-inline", "%s callback inlined into a loop", name)
	if body.Type.Kind == hir.Void {
		return nil, true
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
	if !ok || len(params) != 2 {
		l.diagf(n, "unsupported-call", "reduce callback must take (accumulator, element)")
		return nil, true
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
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: params[1], Type: elem, X: recv,
		Body: hir.B(
			&hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[0], Type: accT, X: acc},
			&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: acc, Y: body})})
	return acc, true
}

// callbackBody lowers a callback to its value expression and its parameter
// names (the caller binds them). Supported: `x => expr`, `(x, i, a) => expr`,
// `x => { return expr; }` with exactly one return, a plain identifier naming
// a module function, and destructured first parameters.
func (l *lowerer) callbackBody(n *ast.Node, cb *ast.Node, elem hir.Type) (*hir.Expr, []string, bool) {
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
						l.serial++
						v := "e" + itoa(l.serial)
						return &hir.Expr{Kind: hir.DirectCall, Node: l.node(cb), Owner: ref.owner, Name: ref.method,
							Type: l.callbackResultType(ref), Args: []*hir.Expr{hir.V(v, elem)}}, []string{v}, true
					}
				}
			}
		}
	}
	if cb.Kind != ast.KindArrowFunction && cb.Kind != ast.KindFunctionExpression {
		l.diagf(n, "unsupported-callback", "callback %s is not an arrow function", cb.Kind.String())
		return nil, nil, false
	}
	params := cb.Parameters()
	if params == nil {
		l.diagf(n, "unsupported-callback", "callback without parameters")
		return nil, nil, false
	}
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
	if len(names) == 0 {
		l.diagf(n, "unsupported-callback", "callback without parameters")
		return nil, nil, false
	}
	// Parameter types come from the checker (the element for the first
	// parameter of map/filter/some, the accumulator for reduce, the index
	// for the second), falling back to the element type.
	for i, p := range names {
		_ = i
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
		l.declare(p, t)
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
		if len(stmts) != 1 || stmts[0].Kind != ast.KindReturnStatement || stmts[0].AsReturnStatement().Expression == nil {
			l.diagf(n, "unsupported-callback", "callback block must be exactly one return")
			return nil, nil, false
		}
		value = stmts[0].AsReturnStatement().Expression
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
			field := local
			if be.PropertyName != nil && be.PropertyName.Kind == ast.KindIdentifier {
				field = be.PropertyName.Text()
			}
			ft := l.patternFieldType(x.Type, field)
			l.declare(local, ft)
			l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(el), Name: local, Type: ft,
				X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(el), Name: field, Type: ft, X: x}})
		}
	case ast.KindArrayBindingPattern:
		for i, el := range pattern.AsBindingPattern().Elements.Nodes {
			if el.Kind != ast.KindBindingElement || el.Name() == nil || el.Name().Kind != ast.KindIdentifier {
				continue
			}
			local := el.Name().Text()
			var value *hir.Expr
			if x.Type.Kind == hir.Array {
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
			l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(el), Name: local, Type: value.Type, X: value})
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
	args := []*hir.Expr{}
	for _, c := range lf.captures {
		t, ok := l.lookup(c)
		if !ok {
			l.diagf(callee, "unsupported-call", "capture %s is not in scope", c)
			return nil
		}
		args = append(args, hir.V(c, t))
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
		Type: lf.method.Result, X: l.this(l.class), Args: append(args, rest...)}
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
		if i < fixed && arg.Kind == ast.KindSpreadElement {
			return l.tupleSpreadArgs(at, args, params)
		}
	}

	if len(params) > 0 && params[len(params)-1].Variadic {
		rest := params[len(params)-1]
		fixed := params[:len(params)-1]
		if len(args) < len(fixed) {
			// Optional trailing fixed parameters may be omitted.
			for i, p := range fixed {
				if i < len(args) || p.Type.Kind != hir.Optional {
					continue
				}
			}
		}
		fixedArgsIn := args
		if len(fixedArgsIn) > len(fixed) {
			fixedArgsIn = fixedArgsIn[:len(fixed)]
		}
		out, ok := l.fixedArgs(at, fixedArgsIn, fixed)
		if !ok {
			return nil, false
		}
		restArgs := []*ast.Node{}
		if len(args) > len(fixed) {
			restArgs = args[len(fixed):]
		}
		// Evaluate fixed arguments before constructing/evaluating the rest array.
		for i, arg := range out {
			l.serial++
			name := "arg" + itoa(l.serial)
			l.declare(name, arg.Type)
			l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(at), Name: name, Type: arg.Type, X: arg})
			out[i] = hir.V(name, arg.Type)
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
	out, ok := l.fixedArgs(at, args, params)
	return out, ok
}

// fixedArgs lowers arguments for a fixed parameter list.
func (l *lowerer) fixedArgs(at *ast.Node, args []*ast.Node, params []hir.Param) ([]*hir.Expr, bool) {
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
			out = append(out, l.coerce(a, p.Type))
			continue
		}
		if p.Type.Kind == hir.Optional {
			out = append(out, &hir.Expr{Kind: hir.Lit, Type: p.Type})
			continue
		}
		l.diagf(at, "unsupported-call", "missing argument for parameter %s", p.Name)
		return nil, false
	}
	if len(args) > len(params) {
		l.diagf(args[len(params)], "unsupported-call", "too many arguments (%d for %d)", len(args), len(params))
		return nil, false
	}
	return out, true
}

// spreadPush lowers arr.push(...xs) into a loop of pushes, as an expression
// yielding the new length (which the callers ignore).
func (l *lowerer) spreadPush(n *ast.Node, recv *hir.Expr, spread *ast.Node) *hir.Expr {
	xs := l.expr(spread.Expression())
	if xs == nil || xs.Type.Kind != hir.Array || !recv.Type.Equal(xs.Type) {
		l.diagf(n, "unsupported-call", "push spread needs a matching array")
		return nil
	}
	l.serial++
	v := "p" + itoa(l.serial)
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: v, Type: recv.Type.Args[0], X: xs,
		Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n),
			X: l.rtOp("array.push", recv, hir.T(hir.I32), hir.V(v, recv.Type.Args[0]))})})
	return l.rtOp("array.length", recv, hir.T(hir.I32))
}

// Finite tuple spreads provide a checked arity even across fixed parameters.
// Evaluate each source once before distributing its elements to parameters.
func (l *lowerer) tupleSpreadArgs(at *ast.Node, args []*ast.Node, params []hir.Param) ([]*hir.Expr, bool) {
	var values []*hir.Expr
	for _, arg := range args {
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
				values = append(values, &hir.Expr{Kind: hir.FieldGet, Node: l.node(arg), Type: field.Type, Name: field.Name, X: x})
			}
		} else {
			t := l.ck.GetTypeAtLocation(arg.Expression())
			if x.Type.Kind != hir.Array || t == nil || !t.IsTupleType() {
				l.diagf(arg, "unsupported-call", "fixed-parameter spread requires a finite tuple")
				return nil, false
			}
			for i := 0; l.ck.GetPropertyOfType(t, itoa(i)) != nil; i++ {
				values = append(values, &hir.Expr{Kind: hir.IndexGet, Node: l.node(arg), Type: x.Type.Args[0], X: x, Y: hir.L(hir.T(hir.I32), i)})
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
