package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Escaping thunks: a parameterless arrow function with a void result that
// is stored (the syntax checker's deferred FORM parameter checks) lowers to
// a closure object. Immutable inputs are copied; mutable bindings share
// lexical cells. Invocation `d()` is the thunk interface's call method.
// Parameterized and value-returning functions use typed callable interfaces.

const thunkInterfaceName = "closure.thunk"

// thunkType returns (creating once) the interface every thunk implements.
func (l *lowerer) thunkType() hir.Type {
	for _, i := range l.out.Interfaces {
		if i.Name == thunkInterfaceName {
			return hir.Type{Kind: hir.InterfaceRef, Name: i.Name}
		}
	}
	i := &hir.Interface{Node: hir.Node{ID: l.nextID(), Source: thunkInterfaceName}, Name: thunkInterfaceName,
		Methods: []*hir.Method{{Node: hir.Node{ID: l.nextID(), Source: thunkInterfaceName}, Name: "call", Virtual: true, Result: l.thunkResult()}}}
	l.out.Interfaces = append(l.out.Interfaces, i)
	if l.ifaceDone == nil {
		l.ifaceDone = map[*hir.Interface]bool{}
	}
	l.ifaceDone[i] = true
	return hir.Type{Kind: hir.InterfaceRef, Name: i.Name}
}

// isThunkType reports a checker type with exactly one call signature that
// takes no parameters and returns void (or never, an always-throwing thunk).
// The test host additionally accepts Promise results; its boundary drains them.
func (l *lowerer) isThunkType(t *checker.Type) bool {
	if t == nil {
		return false
	}
	sigs := l.ck.GetSignaturesOfType(t, checker.SignatureKindCall)
	if len(sigs) != 1 || len(sigs[0].Parameters()) != 0 {
		return false
	}
	ret := l.ck.GetReturnTypeOfSignature(sigs[0])
	return ret != nil && (ret.Flags()&(checker.TypeFlagsVoid|checker.TypeFlagsNever) != 0 ||
		(l.integerOptions.TrapOnFailure && l.settledEnabled && ret.Symbol() != nil && ret.Symbol().Name == "Promise" && l.librarySymbol(ret.Symbol())))
}

// closureValue lowers an arrow function used as a value.
func (l *lowerer) closureValue(n *ast.Node) *hir.Expr {
	before := len(l.diags)
	x := l.closureValueInner(n)
	if !l.integerOptions.TrapOnFailure || x != nil {
		return x
	}
	if !l.isThunkType(l.ck.GetTypeAtLocation(n)) && !l.isThunkType(l.ck.GetContextualType(n, 0)) {
		return nil
	}
	l.serial++
	name := "closure." + itoa(l.serial)
	m := &hir.Method{Node: l.node(n), Name: "call", Virtual: true, Result: l.thunkResult()}
	l.trapBodyDiagnostics(n, m, before)
	if m.Body == nil {
		m.Body = failureBody(m.Node, l.locOf(n)+": unsupported callback")
	}
	l.out.Classes = append(l.out.Classes, &hir.Class{Node: l.node(n), Name: name, Implements: []string{l.thunkType().Name}, Methods: []*hir.Method{m}})
	return &hir.Expr{Node: l.node(n), Kind: hir.New, Type: hir.Ref(name)}
}

func (l *lowerer) closureValueInner(n *ast.Node) *hir.Expr {
	typ := hir.Type{Kind: hir.InterfaceRef, Name: thunkInterfaceName}
	if l.integerOptions.TrapOnFailure && len(n.Parameters()) > 0 && l.isTestCallback(n) {
		l.diagf(n, "unsupported-test-done", "done/parameterized callback is not run by the synchronous test host")
		return nil
	}
	general := len(n.Parameters()) > 0 || !l.isThunkType(l.ck.GetTypeAtLocation(n)) && !l.isThunkType(l.ck.GetContextualType(n, 0))
	if general {
		if l.integerOptions.TrapOnFailure && l.isTestCallback(n) {
			l.diagf(n, "unsupported-test-done", "done/parameterized callback is not run by the synchronous test host")
			return nil
		}
		typ = l.functionType(n, l.ck.GetTypeAtLocation(n))
		if l.callableMethod(typ) == nil {
			return nil
		}
	}
	body := n.Body()
	if body == nil {
		return nil
	}
	if l.gogenRefuseReceiver(n) {
		return nil
	}
	captures := l.testFixtureCaptures(n, l.freeLocals(n))
	usesThis := touchesThis(body)
	contextThis := l.integerOptions.TrapOnFailure && n.Kind == ast.KindFunctionExpression && usesThis && l.isTestCallback(n)
	if contextThis {
		usesThis = false
	}
	if usesThis && (l.method == nil || l.method.Static || l.class == nil) {
		l.diagf(n, "unsupported-closure", "closure uses this outside an instance method")
		return nil
	}
	l.serial++
	name := "closure." + itoa(l.serial)
	c := &hir.Class{Node: l.node(n), Name: name, Implements: []string{typ.Name}}
	if !general {
		l.thunkType()
	}
	// Preserve the legacy serial-based target name, but identify the thunk by
	// its enclosing qualified method and original TS location for certificates.
	owner := l.relFile(l.file)
	if l.class != nil {
		owner = l.class.Name
		if l.class.SiteOwner != "" {
			owner = l.class.SiteOwner
		}
	}
	if l.method != nil {
		if l.method.SiteOwner != "" {
			owner = l.method.SiteOwner
		} else {
			owner += "." + l.method.Name
		}
	}
	c.SiteOwner = owner + ".[closure@" + l.prog.siteSource(l.locOf(n)) + "]"
	ctor := &hir.Method{Node: l.node(n), Name: "constructor", Result: hir.T(hir.Void)}
	var init []*hir.Stmt
	var args []*hir.Expr
	field := func(fname string, t hir.Type, value *hir.Expr) {
		c.Fields = append(c.Fields, hir.Field{Node: l.node(n), Name: fname, Type: t})
		ctor.Params = append(ctor.Params, hir.Param{Name: fname, Type: t})
		init = append(init, &hir.Stmt{Kind: hir.Assign, Node: l.node(n),
			X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: fname, Type: t, X: &hir.Expr{Kind: hir.This, Node: l.node(n), Type: hir.Ref(name)}},
			Y: hir.V(fname, t)})
		args = append(args, value)
	}
	captureTypes := map[string]hir.Type{}
	for _, captured := range captures {
		cap := captured.name
		t, ok := l.lookup(cap)
		value := hir.V(cap, t)
		if t.Kind == hir.Void {
			value = l.functionValue(captured.node)
			if value != nil {
				t = l.functionType(captured.node, l.ck.GetTypeAtLocation(captured.node))
				captureTypes[cap] = t
			}
		}
		if !ok || t.Kind == hir.Void || value == nil {
			l.diagf(n, "unsupported-closure", "captured %s has no lowered type", cap)
			return nil
		}
		captureTypes[cap] = t
		if l.bindingMutable(captured.symbol) && captured.symbol != nil && t.Kind != hir.Void {
			captured.typ = t
			l.captureCell(captured)
		}
		if cell, shared := l.captureCells[captured.symbol]; shared {
			field(cap, cell, l.captureTransport(captured, cell))
		} else {
			field(cap, t, value)
		}
	}
	outerType := hir.Type{}
	if usesThis {
		outerType = hir.Ref(l.class.Name)
		field("outer", outerType, l.this(l.class))
	}
	if contextThis {
		outerType = hir.Ref("runtime/mocha.ts.TestContext")
		field("context", outerType, &hir.Expr{Node: l.node(n), Kind: hir.New, Type: outerType})
	}
	ctor.Body = hir.B(init...)
	c.Ctor = ctor
	call := &hir.Method{Node: l.node(n), Name: "call", Virtual: true, Result: l.thunkResult()}
	if general {
		signature := l.callableMethod(typ)
		call.Result = signature.Result
		call.Params = append([]hir.Param(nil), signature.Params...)
		for _, p := range n.Parameters() {
			if p.Name() == nil || p.Name().Kind != ast.KindIdentifier {
				l.diagf(p, "unsupported-closure", "function value parameter requires an identifier")
				return nil
			}
			if ast.IsThisParameter(p) {
				l.diagf(p, "unsupported-closure", "function value with explicit this parameter")
				return nil
			}
		}
	}
	// The test boundary always returns a settled Promise<void>. Keep the
	// producer's fulfillment type in a separate method, then drain it before
	// reporting success. Ordinary compilation retains its void thunk ABI.
	var invoke *hir.Method
	if !general && l.integerOptions.TrapOnFailure && l.settledEnabled {
		ret := l.ck.GetReturnTypeOfSignature(l.ck.GetSignaturesOfType(l.ck.GetTypeAtLocation(n), checker.SignatureKindCall)[0])
		result := hir.T(hir.Void)
		if ret.Symbol() != nil && ret.Symbol().Name == "Promise" && l.librarySymbol(ret.Symbol()) {
			result = l.mapCheckerType(n, ret)
			if !l.synchronousAsyncSuite(n) && !l.certifiedSettledBody(n, map[*ast.Node]bool{}) {
				l.diagf(n, "unsupported-async", "test callback is not certified already-settled; pending promises require a scheduler")
				return nil
			}
		} else if l.testMayThenable(ret) {
			l.diagf(n, "unsupported-async", "test callback may return an arbitrary thenable; settled assimilation is not certified")
			return nil
		}
		invoke = call
		invoke.Name, invoke.Virtual, invoke.Result = "invoke", false, result
		if n.ModifierFlags()&ast.ModifierFlagsAsync != 0 {
			l.asyncMethods[invoke] = true
		}
	}
	call.SiteOwner = c.SiteOwner + ".call"
	c.Methods = append(c.Methods, call)
	l.out.Classes = append(l.out.Classes, c)

	// Lower the body in a scope that sees only the captures (reads of the
	// fields) and the captured this.
	savedScope, savedMethod, savedPend, savedThis, savedClass := l.scope, l.method, l.pend, l.thisOverride, l.class
	l.scope = nil
	l.push()
	self := &hir.Expr{Kind: hir.This, Node: l.node(n), Type: hir.Ref(name)}
	var prologue []*hir.Stmt
	for _, captured := range captures {
		cap := captured.name
		t := captureTypes[cap]
		l.declare(cap, t)
		if cell, shared := l.captureCells[captured.symbol]; shared {
			t = cell
		}
		decl := &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: cap, Type: t, X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: cap, Type: t, X: self}}
		if general {
			if l.declSymbols == nil {
				l.declSymbols = map[*hir.Stmt]*ast.Symbol{}
			}
			l.declSymbols[decl] = captured.symbol
		}
		prologue = append(prologue, decl)
	}
	if usesThis {
		l.thisOverride = &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: "outer", Type: outerType, X: self}
	} else {
		l.thisOverride = nil
	}
	if contextThis {
		l.thisOverride = &hir.Expr{Node: l.node(n), Kind: hir.FieldGet, Name: "context", Type: outerType, X: self}
	}
	l.method = call
	if l.unprunedFeatures {
		l.class = c
	}
	var defaults *hir.Method
	if general {
		aliases := *call
		aliases.Params = append([]hir.Param(nil), call.Params...)
		for i, p := range n.Parameters() {
			sourceName := p.Name().Text()
			param := call.Params[i]
			l.declare(sourceName, param.Type)
			alias := &hir.Stmt{Kind: hir.VarDecl, Node: l.node(p), Name: sourceName, Type: param.Type, X: hir.V(param.Name, param.Type)}
			if l.declSymbols == nil {
				l.declSymbols = map[*hir.Stmt]*ast.Symbol{}
			}
			l.declSymbols[alias] = l.ck.GetSymbolAtLocation(p.Name())
			prologue = append(prologue, alias)
			aliases.Params[i].Name = sourceName
		}
		defaults = &aliases
	}
	before := len(l.diags)
	l.pend = nil
	restore := l.withLocalFns(body)
	var inner *hir.Stmt
	if l.asyncMethods[call] && body.Kind != ast.KindBlock {
		inner = l.asyncReturn(body, body)
	} else {
		inner = l.functionBody(body)
	}
	if general {
		pre := l.applyDefaults(n, defaults)
		symbols := map[string]*ast.Symbol{}
		for _, p := range n.Parameters() {
			symbols[p.Name().Text()] = l.ck.GetSymbolAtLocation(p.Name())
		}
		for _, stmt := range pre {
			walkNumberStmt(stmt, func(s *hir.Stmt) {}, func(e *hir.Expr) {
				if e.Kind == hir.Local && symbols[e.Name] != nil {
					if l.localSymbols == nil {
						l.localSymbols = map[*hir.Expr]*ast.Symbol{}
					}
					l.localSymbols[e] = symbols[e.Name]
				}
			})
		}
		prologue = append(prologue, pre...)
	}
	restore()
	l.pop()
	l.scope, l.method, l.pend, l.thisOverride, l.class = savedScope, savedMethod, savedPend, savedThis, savedClass
	if inner == nil {
		return nil
	}
	call.Body = l.finishAsync(n, call, hir.B(append(prologue, inner)...))
	l.trapBodyDiagnostics(n, call, before)
	if invoke != nil {
		x := &hir.Expr{Kind: hir.DirectCall, Node: l.node(n), Owner: name, Name: "invoke", Type: invoke.Result, X: self}
		if invoke.Result.Kind == hir.Promise {
			x = &hir.Expr{Kind: hir.PromiseAwait, Node: l.node(n), Type: invoke.Result.Args[0], X: x}
		}
		c.Methods = append(c.Methods, &hir.Method{Node: l.node(n), Name: "call", Virtual: true, Result: l.thunkResult(),
			Body: &hir.Stmt{Kind: hir.Async, Node: l.node(n), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: x}, &hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.PromiseFulfill, Type: l.thunkResult()}})}})
	}
	l.diagf(n, "note-closure", "thunk lowered as closure %s capturing %d locals", name, len(captures))
	return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(name), Args: args}
}

// thunkCall lowers a parameterless thunk held in a local or object field.
func (l *lowerer) thunkCall(n *ast.Node, callee *ast.Node) (*hir.Expr, bool) {
	if l.integerOptions.TrapOnFailure && l.relFile(l.file) == "runtime/mocha.ts" && callee.Kind == ast.KindPropertyAccessExpression && callee.Name().Text() == "call" {
		x := l.expr(callee.Expression())
		if x != nil && x.Type.Name == thunkInterfaceName {
			return l.invokeThunk(n, x), true
		}
	}
	// Named functions have their own direct-call path; only stored values
	// use the thunk interface. Do not re-evaluate a method's receiver here.
	if callee.Kind == ast.KindIdentifier {
		t, ok := l.lookup(callee.Text())
		if !ok || t.Kind != hir.InterfaceRef || t.Name != thunkInterfaceName {
			return nil, false
		}
	} else if callee.Kind == ast.KindPropertyAccessExpression {
		if sym := l.resolve(callee); sym != nil && sym.Flags&ast.SymbolFlagsMethod != 0 {
			return nil, false
		}
		if !l.isThunkType(l.ck.GetTypeAtLocation(callee)) {
			return nil, false
		}
	} else {
		return nil, false
	}
	x := l.expr(callee)
	if x == nil {
		return nil, true
	}
	if x.Type.Kind != hir.InterfaceRef || x.Type.Name != thunkInterfaceName {
		return nil, false
	}
	x = l.callReceiver(n, x, nil)
	if _, ok := l.callArgs(n, n.Arguments(), nil); !ok {
		return nil, true
	}
	return l.invokeThunk(n, x), true
}

// Only the trap-on-failure test host normalizes thunks to settled promises.
func (l *lowerer) thunkResult() hir.Type {
	if l.integerOptions.TrapOnFailure && l.settledEnabled {
		return hir.T(hir.Promise, hir.T(hir.Void))
	}
	return hir.T(hir.Void)
}
func (l *lowerer) invokeThunk(n *ast.Node, x *hir.Expr) *hir.Expr {
	call := &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), Name: "call", Type: l.thunkResult(), X: x}
	if call.Type.Kind == hir.Promise {
		return &hir.Expr{Kind: hir.PromiseAwait, Node: l.node(n), Type: hir.T(hir.Void), X: call}
	}
	return call
}

// A void contextual callback type must not erase a possible Promise return.
func (l *lowerer) testMayThenable(t *checker.Type) bool {
	if t.IsUnion() {
		for _, part := range t.AsUnionOrIntersectionType().Types() {
			if l.testMayThenable(part) {
				return true
			}
		}
		return false
	}
	return t.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 || l.ck.GetPropertyOfType(t, "then") != nil
}
