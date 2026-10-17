package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"strings"
)

// Phase-2 class lowering: module functions, interface variants (partial
// method interfaces, data interfaces as synthesized classes), default and
// variadic parameters, derived classes without constructors, implicit
// (structural) implements, and local arrow functions lifted to methods.

// functionToMethod lowers a module-level function declaration to a static
// method.
func (l *lowerer) functionToMethod(fn *ast.Node, owner *hir.Class) *hir.Method {
	return l.functionBodyToMethod(fn, owner, fn.Name().Text())
}

func (l *lowerer) functionBodyToMethod(fn *ast.Node, owner *hir.Class, name string) *hir.Method {
	hm := l.methodsBy[owner.Name+"."+name]
	if hm == nil {
		return nil
	}
	l.class = owner
	l.method = hm
	defer l.trapBodyDiagnostics(fn, hm, len(l.diags))
	if l.trapUnexecuted(fn, hm) {
		return hm
	}
	if e, ok := l.overrides[fn]; ok && e.Method != nil {
		return hm
	}
	if fn.ModifierFlags()&ast.ModifierFlagsAsync != 0 && !l.settledEnabled {
		l.diagf(fn, "unsupported-async", "async body must be excluded by validated workload coverage")
		return nil
	}
	if l.methodFailed(hm) {
		return nil
	}
	if fn.Body() == nil {
		return nil
	}
	l.push()
	for _, p := range hm.Params {
		l.declare(p.Name, p.Type)
	}
	l.asyncMethods[hm] = fn.ModifierFlags()&ast.ModifierFlagsAsync != 0
	if isGeneratorFunction(fn) {
		l.lowerGeneratorBody(fn, hm)
		l.pop()
		return hm
	}
	pre := l.applyDefaults(fn, hm)
	restore := l.withLocalFns(fn.Body())
	if fn.Body().Kind == ast.KindBlock {
		hm.Body = hir.B(append(pre, l.block(fn.Body()))...)
	} else {
		hm.Body = hir.B(append(pre, l.functionBody(fn.Body()))...)
	}
	restore()
	hm.Body = l.finishAsync(fn, hm, hm.Body)
	l.pop()
	return hm
}

// methodFailed reports whether a signature mapping produced unusable parts.
func (l *lowerer) methodFailed(hm *hir.Method) bool {
	if hm.Result.Kind == hir.Void && len(hm.Params) == 0 {
		return false
	}
	for _, p := range hm.Params {
		if p.Type.Kind == hir.Void {
			return true
		}
	}
	return false
}

// applyDefaults assigns default parameter values in the body prologue when
// the parameter is absent.
func (l *lowerer) applyDefaults(fn *ast.Node, hm *hir.Method) []*hir.Stmt {
	var out []*hir.Stmt
	i := 0
	for _, p := range fn.Parameters() {
		if ast.IsThisParameter(p) {
			continue
		}
		if p.Initializer() != nil && i < len(hm.Params) && hm.Params[i].Type.Kind == hir.Optional {
			hp := hm.Params[i]
			l.hint = hp.Type.Args[0]
			def := l.expr(p.Initializer())
			l.hint = hir.Type{}
			if def != nil {
				out = append(out, &hir.Stmt{Kind: hir.If, Node: l.node(p),
					X:    &hir.Expr{Kind: hir.IsUndefined, Node: l.node(p), Type: hir.T(hir.Bool), X: hir.V(hp.Name, hp.Type)},
					Body: hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(p), X: hir.V(hp.Name, hp.Type), Y: l.coerce(def, hp.Type)})})
			}
		}
		out = append(out, l.bindObjectParameter(p, hm.Params[i])...)
		i++
	}
	return out
}

// withLocalFns scans a method body for arrow functions assigned to local
// constants and records them for lazy lifting: the lift happens at the first
// call, when the declarations the closure captures already carry their types
// in the scope (a const is initialized before any use in straight-line
// code). The declaration statement itself is skipped by varDecl.
func (l *lowerer) withLocalFns(body *ast.Node) func() {
	if body == nil {
		return func() {}
	}
	saved := l.localFns
	savedPending := l.pendingFns
	l.localFns = map[string]*localFn{}
	l.pendingFns = map[string]*ast.Node{}
	for name, lf := range saved {
		l.localFns[name] = lf
	}
	for name, fn := range savedPending {
		l.pendingFns[name] = fn
	}

	var walk func(*ast.Node)
	walk = func(x *ast.Node) {
		if x == nil {
			return
		}
		if x.Kind == ast.KindFunctionDeclaration {
			if x.Name() != nil && x.Body() != nil {
				l.pendingFns[x.Name().Text()] = x
			}
			return
		}
		if x.Kind == ast.KindFunctionExpression || x.Kind == ast.KindArrowFunction {
			return // do not descend into nested closures
		}
		if x.Kind == ast.KindVariableStatement {
			list := x.AsVariableStatement().DeclarationList
			if list != nil && list.Kind == ast.KindVariableDeclarationList {
				for _, d := range list.AsVariableDeclarationList().Declarations.Nodes {
					if d.Name() != nil && d.Name().Kind == ast.KindIdentifier && d.Initializer() != nil &&
						(d.Initializer().Kind == ast.KindArrowFunction || d.Initializer().Kind == ast.KindFunctionExpression) {
						if !l.unprunedFeatures || list.Flags&ast.NodeFlagsConst != 0 {
							l.pendingFns[d.Name().Text()] = d.Initializer()
						}
					}
				}
			}
		}
		x.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(body)
	return func() { l.localFns = saved; l.pendingFns = savedPending }
}

// Keep existing output names when unique. Separate lexical callbacks in
// different enclosing methods may use the same local name; their lifted
// class members must still be distinct. A source position makes collisions
// deterministic without renaming the default workload's unique callbacks.
func (l *lowerer) localFnMemberName(local string, fn *ast.Node) string {
	base := "fn_" + local
	used := func(name string) bool {
		for _, m := range l.class.Methods {
			if m.Name == name {
				return true
			}
		}
		for _, f := range l.class.Fields {
			if f.Name == name {
				return true
			}
		}
		return false
	}
	if !used(base) {
		return base
	}
	candidate := base + "_" + itoa(fn.Pos())
	for i := 1; used(candidate); i++ {
		candidate = base + "_" + itoa(fn.Pos()) + "_" + itoa(i)
	}
	return candidate
}

// liftLocalFn creates the private method for one local arrow function. The
// method body is lowered on demand, when the enclosing method's scope still
// holds the captured locals.
func (l *lowerer) liftLocalFn(name string, fn *ast.Node) *localFn {
	undefinedThis := l.refuseUndefinedThis(fn)
	receiver := l.gogenRefuseReceiver(fn)
	if undefinedThis || receiver {
		return nil
	}
	if l.localFns == nil {
		l.localFns = map[string]*localFn{}
	}
	if lf, ok := l.localFns[name]; ok {
		return lf
	}
	// Captures are lexical inputs; mutable bindings travel in shared cells.
	captures := []localCapture{}
	receiverFree := !touchesThis(fn.Body())
	seen := map[*ast.Symbol]bool{}
	var selfSymbol *ast.Symbol
	if fn.Parent != nil && fn.Parent.Kind == ast.KindVariableDeclaration && fn.Parent.Initializer() == fn {
		selfSymbol = l.ck.GetSymbolAtLocation(fn.Parent.Name())
	} else if fn.Name() != nil {
		selfSymbol = l.ck.GetSymbolAtLocation(fn.Name())
	}
	var free []localCapture
	if l.unprunedFeatures {
		free = l.recursiveFreeLocals(fn)
	} else {
		free = l.freeLocals(fn)
	}
	for _, capture := range free {
		if l.unprunedFeatures && selfSymbol != nil && capture.symbol == selfSymbol {
			continue
		}
		if capture.symbol == nil {
			l.diagf(capture.node, "unsupported-call", "capture %s has no checked binding", capture.name)
			return nil
		}
		lf := l.localFns[capture.name]
		if l.isCallableValueType(capture.typ) {
			lf = nil
		}
		if pending := l.pendingFns[capture.name]; !l.unprunedFeatures && lf == nil && pending != nil && !l.isCallableValueType(capture.typ) {
			delete(l.pendingFns, capture.name)
			lf = l.liftLocalFn(capture.name, pending)
		}
		// Known lifted callees remain direct calls. Their actual lexical inputs
		// must be carried through the caller, rather than capturing a void value.
		if lf != nil {
			receiverFree = receiverFree && lf.method.Static
			for _, transitive := range lf.captures {
				if !seen[transitive.symbol] {
					captures = append(captures, transitive)
					seen[transitive.symbol] = true
				}
			}
			continue
		}
		if capture.typ.Kind == hir.Void {
			l.diagf(capture.node, "unsupported-call", "capture %s has no checked binding", capture.name)
			return nil
		}
		if l.unprunedFeatures && l.bindingMutable(capture.symbol) {
			capture.typ = l.captureCell(capture)
		}
		if !seen[capture.symbol] {
			captures = append(captures, capture)
			seen[capture.symbol] = true
		}
	}
	m := &hir.Method{Node: l.node(fn), Name: l.localFnMemberName(name, fn), Virtual: true, Result: hir.T(hir.Void)}
	defer l.trapBodyDiagnostics(fn, m, len(l.diags))
	if l.method != nil && (l.method.Static || (receiverFree && l.testRegistrationHelper(fn))) {
		// A suite thunk has no module instance. Receiver-free registration
		// helpers use the existing static/capture-parameter lowering, rather
		// than fabricating a module-typed `this` in the thunk's body.
		m.Static = true
		m.Virtual = false
	}
	for _, c := range captures {
		m.Params = append(m.Params, hir.Param{Name: c.name, Type: c.typ})
	}
	sigParams := fn.Parameters()
	if l.unprunedFeatures {
		if l.paramSymbols == nil {
			l.paramSymbols = map[*hir.Method]map[string]*ast.Symbol{}
		}
		l.paramSymbols[m] = map[string]*ast.Symbol{}
		for _, p := range sigParams {
			if p.Name() != nil && p.Name().Kind == ast.KindIdentifier {
				l.paramSymbols[m][p.Name().Text()] = l.ck.GetSymbolAtLocation(p.Name())
			}
		}
	}
	l.push()
	for i, c := range captures {
		typ := m.Params[i].Type
		if cell, ok := l.captureCells[c.symbol]; ok && cell.Equal(typ) {
			typ = l.cellValueType(cell)
		}
		l.declare(c.name, typ)
	}
	for _, p := range sigParams {
		if p.Name() == nil || p.Name().Kind != ast.KindIdentifier {
			l.diagf(p, "unsupported-callback", "lifted function parameter with a binding pattern")
			l.pop()
			return nil
		}
		var typ hir.Type
		if ps := p.Symbol(); ps != nil {
			typ = l.mapCheckerType(p, l.ck.GetTypeOfSymbol(ps))
		} else {
			typ = hir.T(hir.Void)
		}
		if p.Initializer() != nil || p.QuestionToken() != nil {
			if typ.Kind != hir.Optional {
				typ = hir.T(hir.Optional, typ)
			}
		}
		m.Params = append(m.Params, hir.Param{Name: p.Name().Text(), Type: typ, Variadic: p.AsParameterDeclaration().DotDotDotToken != nil})
		l.declare(p.Name().Text(), typ)
	}
	if fn.Type() != nil {
		m.Result = l.mapTypeNode(fn.Type())
	} else if t := l.ck.GetTypeAtLocation(fn); t != nil {
		if sig := l.ck.GetSignaturesOfType(t, checker.SignatureKindCall); len(sig) > 0 {
			m.Result = l.mapCheckerType(fn, l.ck.GetReturnTypeOfSignature(sig[0]))
		}
	}
	if t := l.ck.GetContextualType(fn, 0); t != nil {
		if sigs := l.ck.GetSignaturesOfType(t, checker.SignatureKindCall); len(sigs) == 1 && isIntrinsicVoid(l.ck.GetReturnTypeOfSignature(sigs[0])) {
			m.Result = hir.T(hir.Void)
		}
	}
	// Distinct lexical functions may share a source name in this class.
	baseName := m.Name
	for suffix := 2; ; suffix++ {
		used := false
		for _, existing := range l.class.Methods {
			if existing.Name == m.Name {
				used = true
				break
			}
		}
		if !used {
			break
		}
		m.Name = baseName + "_" + itoa(suffix)
	}
	// Lower the body now, while the captures are in scope.
	savedMethod := l.method
	l.method = m
	body := fn.Body()
	if body == nil {
		l.pop()
		return nil
	}
	// Publish the complete signature before lowering the body. Recursive
	// calls must target this same lexical method, not an absent/stale member.
	lf := &localFn{method: m, captures: captures, owner: l.class, fn: fn}
	if l.unprunedFeatures {
		l.localFns[name] = lf
		l.class.Methods = append(l.class.Methods, m)
	}
	restoreFns := l.withLocalFns(body)
	defaults := *m
	defaults.Params = m.Params[len(captures):]
	pre := l.applyDefaults(fn, &defaults)
	inner := l.functionBody(body)
	restoreFns()
	m.Body = inner
	if len(pre) > 0 {
		m.Body = hir.B(append(pre, inner)...)
	}
	l.method = savedMethod
	l.pop()
	if !l.unprunedFeatures {
		l.localFns[name] = lf
		l.class.Methods = append(l.class.Methods, m)
	}
	return lf
}

// functionBody keeps expression-body preludes inside the function. Void
// arrows evaluate their expression for effects and have no return value.
func (l *lowerer) functionBody(body *ast.Node) *hir.Stmt {
	saved := l.pend
	l.pend = nil
	defer func() { l.pend = saved }()
	if body.Kind == ast.KindBlock {
		return l.block(body)
	}
	var stmt *hir.Stmt
	if l.method.Result.Kind == hir.Void {
		stmt = l.expressionStatement(body)
	} else {
		// A concise body is `return body`: the declared result type is its
		// contextual type, as for a return statement.
		l.hint = l.method.Result
		x := l.expr(body)
		l.hint = hir.Type{}
		if x != nil {
			x = l.coerce(l.presentValue(body, x, l.method.Result), l.method.Result)
		}
		stmt = &hir.Stmt{Kind: hir.Return, Node: l.node(body), X: x}
	}
	return hir.B(append(l.pend, stmt)...)
}

// recursiveFreeLocals completes the lexical inputs of every reachable local
// callee before publishing any signature. Walking checked bindings with a visited
// set handles cycles without consuming a half-built callee ABI. Bodies are still
// lowered lazily, after the caller has published its complete signature.
func (l *lowerer) recursiveFreeLocals(fn *ast.Node) []localCapture {
	visited := map[*ast.Node]bool{}
	seen := map[*ast.Symbol]bool{}
	var out []localCapture
	var walk func(*ast.Node)
	walk = func(current *ast.Node) {
		if visited[current] {
			return
		}
		visited[current] = true
		for _, c := range l.freeLocals(current) {
			if pending := l.pendingFns[c.name]; pending != nil && !l.isCallableValueType(c.typ) {
				var symbol *ast.Symbol
				if pending.Name() != nil {
					symbol = l.ck.GetSymbolAtLocation(pending.Name())
				} else if pending.Parent != nil && pending.Parent.Kind == ast.KindVariableDeclaration {
					symbol = l.ck.GetSymbolAtLocation(pending.Parent.Name())
				}
				if symbol == c.symbol {
					walk(pending)
					continue
				}
			}
			if lf := l.localFns[c.name]; lf != nil && !l.isCallableValueType(c.typ) {
				for _, input := range lf.captures {
					if !seen[input.symbol] {
						seen[input.symbol] = true
						out = append(out, input)
					}
				}
				continue
			}
			// Self recursion is represented by a direct call, never a captured value.
			if current == fn && c.symbol != nil && (c.symbol.ValueDeclaration == fn || c.symbol.ValueDeclaration == fn.Parent) {
				continue
			}
			if !seen[c.symbol] {
				seen[c.symbol] = true
				out = append(out, c)
			}
		}
	}
	walk(fn)
	return out
}

// freeLocals lists the enclosing-method local names an arrow function reads
// including writes, resolved by checked binding identity across nested scopes.
func (l *lowerer) freeLocals(fn *ast.Node) []localCapture {
	if !l.unprunedFeatures {
		return l.legacyFreeLocals(fn)
	}
	declared := map[*ast.Symbol]bool{}
	used := map[string][]*ast.Node{}
	var walk func(*ast.Node)
	walk = func(x *ast.Node) {
		if x == nil {
			return
		}
		switch x.Kind {
		case ast.KindPropertyAccessExpression:
			walk(x.Expression())
			return
		case ast.KindParameter, ast.KindVariableDeclaration, ast.KindFunctionDeclaration, ast.KindFunctionExpression:
			if x.Name() != nil && x.Name().Kind == ast.KindIdentifier {
				declared[l.ck.GetSymbolAtLocation(x.Name())] = true
			}
		case ast.KindIdentifier:
			if _, isLocal := l.lookup(x.Text()); isLocal || l.pendingFns[x.Text()] != nil {
				used[x.Text()] = append(used[x.Text()], x)
			}
			return
		}
		x.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(fn)
	var names []string
	for name := range used {
		names = append(names, name)
	}
	sortStrings(names)
	var out []localCapture
	for _, name := range names {
		for _, node := range used[name] {
			symbol := l.ck.GetSymbolAtLocation(node)
			if node.Parent != nil && node.Parent.Kind == ast.KindShorthandPropertyAssignment {
				// {go, type} reads the binding named type, not the property.
				if value := l.ck.GetShorthandAssignmentValueSymbol(node.Parent); value != nil {
					symbol = value
				}
			}
			if declared[symbol] {
				continue
			}
			typ, checked := l.checkedBindingType(symbol)
			if !checked || typ.Kind == hir.Void {
				typ, _ = l.lookup(name)
			}
			out = append(out, localCapture{name: name, typ: typ, symbol: symbol, node: node})
			break
		}
	}
	return out
}

func (l *lowerer) legacyFreeLocals(fn *ast.Node) []localCapture {
	declared := map[string]bool{}
	assigned := map[string]bool{}
	used := map[string][]*ast.Node{}
	var walk func(*ast.Node, bool)
	walkChildren := func(x *ast.Node, inFn bool) {
		x.ForEachChild(func(c *ast.Node) bool { walk(c, inFn); return false })
	}
	walk = func(x *ast.Node, inFn bool) {
		if x == nil {
			return
		}
		switch x.Kind {
		case ast.KindPropertyAccessExpression:
			walk(x.Expression(), inFn)
			return
		case ast.KindParameter:
			if x.Name() != nil && x.Name().Kind == ast.KindIdentifier {
				declared[x.Name().Text()] = true
			}
			if x.Initializer() != nil {
				walk(x.Initializer(), inFn)
			}
			return
		case ast.KindVariableDeclaration:
			if x.Name() != nil && x.Name().Kind == ast.KindIdentifier {
				declared[x.Name().Text()] = inFn
				if init := x.Initializer(); init != nil {
					walk(init, inFn)
				}
				return
			}
		case ast.KindIdentifier:
			name := x.Text()
			if _, isLocal := l.lookup(name); isLocal {
				if inFn {
					used[name] = append(used[name], x)
				}
			}
			return
		case ast.KindBinaryExpression:
			b := x.AsBinaryExpression()
			switch b.OperatorToken.Kind {
			case ast.KindEqualsToken, ast.KindPlusEqualsToken, ast.KindMinusEqualsToken, ast.KindAsteriskEqualsToken:
				if b.Left.Kind == ast.KindIdentifier {
					if inFn {
						assigned[b.Left.Text()] = true
					}
				}
				// Property/index assignment evaluates its receiver too; capturing
				// that reference does not assign the local binding itself.
				if b.Left.Kind != ast.KindIdentifier {
					walk(b.Left, inFn)
				}
				walk(b.Right, inFn)
				return
			}
		case ast.KindArrowFunction, ast.KindFunctionExpression, ast.KindFunctionDeclaration:
			if x.Name() != nil && x.Name().Kind == ast.KindIdentifier {
				declared[x.Name().Text()] = true
			}
			x.ForEachChild(func(c *ast.Node) bool { walk(c, true); return false })
			return
		}
		walkChildren(x, inFn)
	}
	walk(fn, true)
	var names []string
	for name := range used {
		if !declared[name] && !assigned[name] {
			names = append(names, name)
		}
	}
	sortStrings(names)
	out := make([]localCapture, 0, len(names))
	for _, name := range names {
		node := used[name][0]
		symbol := l.ck.GetSymbolAtLocation(node)
		typ, checked := l.checkedBindingType(symbol)
		if !checked || typ.Kind == hir.Void {
			if fallback, ok := l.lookup(name); ok {
				typ = fallback
			}
		}
		out = append(out, localCapture{name: name, typ: typ, symbol: symbol, node: node})
	}
	return out
}

func (l *lowerer) checkedBindingType(symbol *ast.Symbol) (hir.Type, bool) {
	if symbol == nil {
		return hir.Type{}, false
	}
	for declaration, declarationSymbol := range l.declSymbols {
		if declarationSymbol == symbol {
			return declaration.Type, true
		}
	}
	for method, params := range l.paramSymbols {
		for _, param := range method.Params {
			if params[param.Name] == symbol {
				return param.Type, true
			}
		}
	}
	return hir.Type{}, false
}

// covariantImplements applies the structural interface edges discovered
// while lowering (a class value flowed into an interface-typed position).
func (l *lowerer) covariantImplements() {
	for _, c := range l.out.Classes {
		ifaces := l.covariants[c.Name]
		if len(ifaces) == 0 {
			continue
		}
		names := []string{}
		for name := range ifaces {
			names = append(names, name)
		}
		sortStrings(names)
		for _, name := range names {
			exists := false
			for _, i := range c.Implements {
				if i == name {
					exists = true
				}
			}
			if !exists {
				c.Implements = append(c.Implements, name)
			}
		}
	}
}

// recordImplements notes that a value of class src flows into an
// interface-typed position.
func (l *lowerer) recordImplements(src hir.Type, dst hir.Type) {
	if src.Kind != hir.ClassRef || dst.Kind != hir.InterfaceRef {
		return
	}
	if l.covariants[src.Name] == nil {
		l.covariants[src.Name] = map[string]bool{}
	}
	l.covariants[src.Name][dst.Name] = true
}

// coerce adapts a value flowing into a target type: boxing into Dynamic,
// recording implicit interface implementations.
func (l *lowerer) coerce(x *hir.Expr, dst hir.Type) *hir.Expr {
	if x != nil && x.Type.Kind == hir.Array && l.generatorABIFor(dst) != nil {
		return l.generatorArrayAdapter(x, dst)
	}
	if x == nil {
		return nil
	}
	if (x.Type.Kind == hir.InterfaceRef || x.Type.Kind == hir.ClassRef && strings.HasPrefix(x.Type.Name, "closure.")) && dst.Kind == hir.InterfaceRef {
		if adapted := l.closureAdapter(nil, x, dst); adapted != nil {
			return adapted
		}
	}
	if (x.Type.Kind == hir.InterfaceRef || x.Type.Kind == hir.ClassRef && strings.HasPrefix(x.Type.Name, "closure.")) && dst.Kind == hir.Optional && len(dst.Args) == 1 && dst.Args[0].Kind == hir.InterfaceRef {
		// A present function value stored into an optional function slot.
		if adapted := l.closureAdapter(nil, x, dst.Args[0]); adapted != nil {
			return l.coerce(adapted, dst)
		}
	}
	if l.unprunedFeatures && dst.Kind == hir.UnionRef && x.Type.Kind == hir.Optional && len(x.Type.Args) == 1 && (x.Type.Args[0].Kind == hir.UnionRef || x.Type.Args[0].Kind == hir.ClassRef) {
		// A present-only use of an optional family value (a JavaScript partial
		// field read the checker types as any): checked narrowing, absent raises.
		x = &hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: x.Type.Args[0], X: x}
	}
	if l.unprunedFeatures && dst.Kind == hir.UnionRef && x.Type.Kind == hir.Dynamic {
		// A boxed value (a flow-narrowed `F | null` local) stored into a
		// family slot: the family shares the boxed storage and the projection
		// validates membership; a mismatch raises, never retags.
		if u := l.out.Union(dst.Name); u != nil && u.Complete {
			return l.rtOp("union.project", x, dst)
		}
	}
	if l.unprunedFeatures && dst.Kind == hir.UnionRef && !x.Type.Equal(dst) {
		return l.familyAdmission(x, dst)
	}
	if l.unprunedFeatures && dst.Kind == hir.Optional && len(dst.Args) == 1 && dst.Args[0].Kind == hir.UnionRef && x.Type.Kind == hir.Dynamic {
		saved := l.tempInit(nil, x.Type, x)
		return &hir.Expr{Kind: hir.Conditional, Node: x.Node, Type: dst, X: &hir.Expr{Kind: hir.IsUndefined, Node: x.Node, Type: hir.T(hir.Bool), X: saved}, Y: &hir.Expr{Kind: hir.Lit, Node: x.Node, Type: dst}, Z: l.familyAdmission(saved, dst.Args[0])}
	}
	if l.unprunedFeatures && dst.Kind == hir.Optional && len(dst.Args) == 1 && dst.Args[0].Kind == hir.UnionRef && (x.Type.Kind == hir.ClassRef || x.Type.Kind == hir.UnionRef && !x.Type.Equal(dst.Args[0])) {
		// A flow-narrowed payload (or a subset family value) stored into
		// `F | undefined` is admitted to (widened into) F first; the optional
		// wrapping is the ordinary present value.
		if admitted := l.familyAdmission(x, dst.Args[0]); admitted != x {
			return l.coerce(admitted, dst)
		}
	}
	if l.mapStorageView(dst, x.Type) {
		view := l.rtOp("map.view", x, dst)
		if view.Source == "" && l.method != nil {
			view.Node = l.method.Node
		}
		return view
	}
	if l.unprunedFeatures && l.dataShapeSubtype(dst, x.Type) {
		// Property-only interfaces are nominal storage layouts in HIR, not
		// nominal TS types. Reuse the live data-class view: never copy fields
		// or lose extra keys, identity, or mutations through the original.
		return l.dynamicReference(l.coerce(x, hir.T(hir.Dynamic)), dst)
	}
	if l.unprunedFeatures && x.Kind == hir.Lit && x.Value == nil && x.Type.Kind == hir.Optional && dst.Kind == hir.Optional {
		return &hir.Expr{Kind: hir.Lit, Node: x.Node, Type: dst}
	}
	if dst.Kind == hir.Optional && dst.Args[0].Kind == hir.Dynamic && !x.Type.Equal(dst) {
		boxed := l.coerce(x, hir.T(hir.Dynamic))
		return &hir.Expr{Kind: hir.Conditional, Type: dst, X: hir.L(hir.T(hir.Bool), true), Y: boxed, Z: &hir.Expr{Kind: hir.Lit, Type: dst}}
	}
	if x.Type.Kind == hir.Dynamic && dst.Kind == hir.Optional && dst.Args[0].Kind != hir.Dynamic {
		base := dst.Args[0]
		if base.Kind == hir.String || base.Kind == hir.Number || base.Kind == hir.Bool || l.unprunedFeatures && base.IsRef() {
			x = l.tempInit(nil, x.Type, x)
			return &hir.Expr{Kind: hir.Conditional, Type: dst, X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: x}, Y: &hir.Expr{Kind: hir.Lit, Type: dst}, Z: l.coerce(x, base)}
		}
	}
	if x.Type.Kind == hir.Optional && x.Type.Args[0].Kind == hir.Dynamic && dst.Kind == hir.Optional {
		base := dst.Args[0]
		if base.Kind == hir.String || base.Kind == hir.Number || base.Kind == hir.Bool || l.unprunedFeatures && base.IsRef() {
			x = l.tempInit(nil, x.Type, x)
			present := &hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: hir.T(hir.Dynamic), X: x}
			return &hir.Expr{Kind: hir.Conditional, Type: dst, X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: x}, Y: &hir.Expr{Kind: hir.Lit, Type: dst}, Z: l.coerce(present, base)}
		}
	}
	if l.unprunedFeatures && x.Type.Kind == hir.Array && dst.Kind == hir.Array && x.Type.Args[0].Kind == hir.Dynamic && dst.Args[0].Kind == hir.ClassRef {
		return l.dynamicReference(l.coerce(x, hir.T(hir.Dynamic)), dst)
	}
	if x.Type.Kind == hir.Dynamic && dst.Kind == hir.ClassValue {
		return l.rtOp("dynamic.asClassValue", x, dst)
	}
	if x.Type.Kind == hir.Dynamic && dst.Kind == hir.BigInt {
		return l.rtOp("dynamic.asBigInt", x, dst)
	}
	if x.Type.Kind == hir.Dynamic && (dst.Kind == hir.String || dst.Kind == hir.Number || dst.Kind == hir.Bool) {
		op := map[hir.Kind]string{hir.String: "dynamic.asString", hir.Number: "dynamic.asNumber", hir.Bool: "dynamic.asBoolean"}[dst.Kind]
		return l.rtOp(op, x, dst)
	}
	if x.Type.Kind == hir.Dynamic && (dst.Kind == hir.ClassRef || dst.Kind == hir.InterfaceRef || dst.Kind == hir.Array || dst.Kind == hir.OrderedMap || dst.Kind == hir.OrderedSet) {
		// TypeScript typed the tagged value as the reference: the checked
		// unboxing raises for any other payload.
		return l.dynamicReference(x, dst)
	}
	if x.Type.Kind == hir.Optional && x.Type.Args[0].Kind == hir.Dynamic && (dst.Kind == hir.String || dst.Kind == hir.Number || dst.Kind == hir.Bool) {
		// TypeScript typed the tagged value as the primitive: absence raises.
		return l.coerce(&hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: hir.T(hir.Dynamic), X: x}, dst)
	}
	if l.unprunedFeatures && l.inJSFile() && x.Type.Kind == hir.Optional && len(x.Type.Args) == 1 && x.Type.Args[0].Kind == hir.Dynamic && (dst.Kind == hir.ClassRef || dst.Kind == hir.InterfaceRef) {
		// An any value (obj?.getMainABAPFile()?.getStructure() on an erased
		// receiver) passed where JSDoc declares a present reference: absence
		// reaches the use as `x!` does, a present payload is unboxed checked.
		return l.coerce(&hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: hir.T(hir.Dynamic), X: x}, dst)
	}
	if l.unprunedFeatures && l.inJSFile() && x.Type.Kind == hir.Optional && len(x.Type.Args) == 1 && x.Type.Args[0].IsRef() && x.Type.Args[0].Kind != hir.Dynamic && dst.IsRef() && dst.Kind != hir.Dynamic {
		// Unchecked JavaScript passes a possibly-undefined reference where the
		// JSDoc declares a present one (`f(map.get(k))`): the value flows on
		// unchanged, as `x!` does; absence reaches the use, as in JavaScript.
		return l.coerce(&hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: x.Type.Args[0], X: x}, dst)
	}
	if l.unprunedFeatures && l.inJSFile() && x.Type.Kind == hir.UnionRef && dst.Kind == hir.ClassRef {
		// Unchecked JavaScript passes a family value where JSDoc declares one
		// of its members (sameSort(a, b) after a.k === b.k narrowed only a):
		// the checked projection keeps the object and raises on another member.
		if u := l.out.Union(x.Type.Name); u != nil && u.Complete {
			for _, a := range u.Alternatives {
				if a.Payload.Equal(dst) {
					return &hir.Expr{Kind: hir.RuntimeOp, Node: x.Node, Op: "union.project", Owner: u.Name, Name: a.ID, Type: dst, X: x}
				}
			}
		}
	}
	if l.unprunedFeatures && l.inJSFile() && x.Type.Kind == hir.Optional && len(x.Type.Args) == 1 && dst.Kind == hir.Optional && len(dst.Args) == 1 {
		// The same for a possibly-undefined wider reference stored where
		// JSDoc declares `Subtype | undefined`: undefined stays undefined.
		from, to := x.Type.Args[0], dst.Args[0]
		if (from.Kind == hir.ClassRef || from.Kind == hir.InterfaceRef) && (to.Kind == hir.ClassRef || to.Kind == hir.InterfaceRef) && to.Name != hir.RootObject && !l.refAccepts(to, from) && l.refSharedSubtype(from, to) {
			x = l.tempInit(nil, x.Type, x)
			present := &hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: to, X: &hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: from, X: x}}
			return &hir.Expr{Kind: hir.Conditional, Node: x.Node, Type: dst, X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: x}, Y: &hir.Expr{Kind: hir.Lit, Type: dst}, Z: l.coerce(present, dst)}
		}
	}
	if l.unprunedFeatures && l.inJSFile() && (x.Type.Kind == hir.ClassRef || x.Type.Kind == hir.InterfaceRef) {
		// Unchecked JavaScript passes a wider reference where JSDoc declares a
		// subtype (an INode argument for an ExpressionNode parameter): the
		// object flows on through a checked cast, which raises only when no
		// declared subtype holds it.
		target := dst
		if target.Kind == hir.Optional && len(target.Args) == 1 {
			target = target.Args[0]
		}
		if (target.Kind == hir.ClassRef || target.Kind == hir.InterfaceRef) && target.Name != hir.RootObject && !l.refAccepts(target, x.Type) && l.refSharedSubtype(x.Type, target) {
			return l.coerce(&hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: target, X: x}, dst)
		}
	}
	// A present-only use of an optional primitive (TypeScript already typed
	// it as the base): checked narrowing, absent raises.
	if x.Type.Kind == hir.Optional && x.Type.Args[0].Equal(dst) && !dst.IsRef() && dst.Kind != hir.Optional && dst.Kind != hir.Dynamic {
		return &hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: dst, X: x}
	}
	if dst.Kind == hir.Optional && x.Type.Equal(dst.Args[0]) {
		return &hir.Expr{Kind: hir.Conditional, Type: dst, X: hir.L(hir.T(hir.Bool), true), Y: x, Z: &hir.Expr{Kind: hir.Lit, Type: dst}}
	}
	if l.unprunedFeatures && dst.Kind == hir.Dynamic && x.Kind == hir.RuntimeOp && x.Op == "dynamic.asString" {
		// A TS assertion cannot convert erased storage at an erased destination.
		return x.X
	}
	if dst.Kind == hir.Dynamic && x.Type.Kind != hir.Dynamic {
		return l.rtOp("dynamic.of", x, hir.T(hir.Dynamic))
	}
	if dst.Kind == hir.Optional && dst.Args[0].Kind == hir.Dynamic {
		switch {
		case x.Type.Kind == hir.Dynamic || x.Type.Equal(dst):
		case x.Type.Kind == hir.Optional && x.Type.Args[0].Kind != hir.Dynamic:
			// An absent value stays absent; a present one is boxed.
			present := l.tempInit(nil, x.Type, x)
			absent := &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: present}
			boxed := l.rtOp("dynamic.of", &hir.Expr{Kind: hir.Narrow, Type: x.Type.Args[0], X: present}, hir.T(hir.Dynamic))
			return &hir.Expr{Kind: hir.Conditional, Type: dst, X: absent, Y: &hir.Expr{Kind: hir.Lit, Type: dst}, Z: boxed}
		case x.Type.Kind != hir.Optional && x.Type.Kind != hir.Void:
			return l.rtOp("dynamic.of", x, hir.T(hir.Dynamic))
		}
	}
	if dst.Kind == hir.Optional && dst.Args[0].Kind == hir.ClassRef {
		if x.Type.Kind == hir.InterfaceRef {
			converted := l.coerce(x, dst.Args[0])
			if converted.Type.Equal(dst.Args[0]) {
				return l.coerce(converted, dst)
			}
		}
		if x.Type.Kind == hir.Optional && x.Type.Args[0].Kind == hir.InterfaceRef {
			source := x.Type.Args[0]
			for base := l.classByName(l.ifaceClassBaseOf[source.Name]); base != nil; base = l.classByName(base.Super) {
				if base.Name != dst.Args[0].Name {
					continue
				}
				saved := l.tempInit(nil, x.Type, x)
				present := &hir.Expr{Kind: hir.Narrow, Type: source, X: saved}
				converted := &hir.Expr{Kind: hir.Cast, Type: dst.Args[0], X: present}
				return &hir.Expr{Kind: hir.Conditional, Type: dst, X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: saved}, Y: &hir.Expr{Kind: hir.Lit, Type: dst}, Z: converted}
			}
		}
	}
	if x.Type.Kind == hir.InterfaceRef && dst.Kind == hir.ClassRef {
		for base := l.classByName(l.ifaceClassBaseOf[x.Type.Name]); base != nil; base = l.classByName(base.Super) {
			if base.Name == dst.Name {
				return &hir.Expr{Kind: hir.Cast, Node: x.Node, Type: dst, X: x}
			}
		}
	}
	// An interface value flowing into a class its declaration extends: the
	// checked cast raises only for an implementer outside the declared
	// hierarchy, which TypeScript's structural typing would not admit.
	{
		src, target := x.Type, dst
		if src.Kind == hir.Optional {
			src = src.Args[0]
		}
		if target.Kind == hir.Optional {
			target = target.Args[0]
		}
		if src.Kind == hir.InterfaceRef && target.Kind == hir.InterfaceRef && src.Name != target.Name && l.acceptsType(target, src) {
			// Structurally compatible interfaces are distinct ABAP interface
			// types: the assignment needs the checked cast.
			narrowTo := dst
			if x.Type.Kind == hir.Optional && dst.Kind != hir.Optional {
				narrowTo = hir.T(hir.Optional, dst)
			}
			return &hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: narrowTo, X: x}
		}
		if src.Kind == hir.InterfaceRef && target.Kind == hir.ClassRef && target.Name != hir.RootObject && (l.interfaceHasClassBase(src.Name, target) || l.unionPartsAccepted(src.Name, target)) {
			narrowTo := dst
			if x.Type.Kind == hir.Optional && dst.Kind != hir.Optional {
				narrowTo = hir.T(hir.Optional, dst)
			}
			return &hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: narrowTo, X: x}
		}
	}
	if x.Type.Kind == hir.ClassRef && dst.Kind == hir.InterfaceRef && !l.acceptsType(dst, x.Type) {
		l.recordImplements(x.Type, dst)
	}
	return x
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// lookupPendingAllowed reports whether lazy lifting is active in this body.
func (l *lowerer) lookupPendingAllowed() bool { return l.pendingFns != nil }

// inJSFile reports lowering inside a .js/.mjs/.cjs source (no checkJs).
func (l *lowerer) inJSFile() bool {
	return l.file != nil && ast.IsInJSFile(l.file.AsNode())
}

// refAccepts mirrors the verifier's reference upcast: the class chain and its
// declared interfaces, or an interface whose methods src provides alike.
func (l *lowerer) refAccepts(dst, src hir.Type) bool {
	if dst.Equal(src) {
		return true
	}
	if dst.Kind == hir.InterfaceRef && src.Kind == hir.InterfaceRef {
		for _, i := range l.out.Interfaces {
			if i.Name != dst.Name {
				continue
			}
			for _, m := range i.Methods {
				other := l.interfaceMethod(src.Name, m.Name)
				if other == nil || !m.Result.Equal(other.Result) || len(m.Params) != len(other.Params) {
					return false
				}
				for k, p := range m.Params {
					if !p.Type.Equal(other.Params[k].Type) {
						return false
					}
				}
			}
			return true
		}
		return false
	}
	if src.Kind != hir.ClassRef {
		return false
	}
	for c := l.classByName(src.Name); c != nil; c = l.classByName(c.Super) {
		if dst.Kind == hir.ClassRef && c.Name == dst.Name {
			return true
		}
		for _, i := range c.Implements {
			if dst.Kind == hir.InterfaceRef && dst.Name == i {
				return true
			}
		}
	}
	return false
}

// refSharedSubtype reports a lowered class both references accept, so a
// checked cast between them can succeed.
func (l *lowerer) refSharedSubtype(a, b hir.Type) bool {
	for _, c := range l.out.Classes {
		ref := hir.Ref(c.Name)
		if l.refAccepts(a, ref) && l.refAccepts(b, ref) {
			return true
		}
	}
	return false
}
