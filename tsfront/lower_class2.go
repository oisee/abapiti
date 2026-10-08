package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Phase-2 class lowering: module functions, interface variants (partial
// method interfaces, data interfaces as synthesized classes), default and
// variadic parameters, derived classes without constructors, implicit
// (structural) implements, and local arrow functions lifted to methods.

// functionToMethod lowers a module-level function declaration to a static
// method.
func (l *lowerer) functionToMethod(fn *ast.Node, owner *hir.Class) *hir.Method {
	name := fn.Name().Text()
	hm := l.methodsBy[owner.Name+"."+name]
	if hm == nil {
		return nil
	}
	l.class = owner
	l.method = hm
	if l.trapUnexecuted(fn, hm) {
		return hm
	}
	if e, ok := l.overrides[fn]; ok && e.Method != nil {
		return hm
	}
	if fn.ModifierFlags()&ast.ModifierFlagsAsync != 0 {
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
	pre := l.applyDefaults(fn, hm)
	restore := l.withLocalFns(fn.Body())
	hm.Body = hir.B(append(pre, l.block(fn.Body()))...)
	restore()
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
					Body: hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(p), X: hir.V(hp.Name, hp.Type), Y: def})})
			}
		}
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
		if x.Kind == ast.KindFunctionExpression || x.Kind == ast.KindArrowFunction {
			return // do not descend into nested closures
		}
		if x.Kind == ast.KindVariableStatement {
			list := x.AsVariableStatement().DeclarationList
			if list != nil && list.Kind == ast.KindVariableDeclarationList {
				for _, d := range list.AsVariableDeclarationList().Declarations.Nodes {
					if d.Name() != nil && d.Name().Kind == ast.KindIdentifier && d.Initializer() != nil &&
						(d.Initializer().Kind == ast.KindArrowFunction || d.Initializer().Kind == ast.KindFunctionExpression) {
						l.pendingFns[d.Name().Text()] = d.Initializer()
					}
				}
			}
		}
		x.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(body)
	return func() { l.localFns = saved; l.pendingFns = savedPending }
}

// liftLocalFn creates the private method for one local arrow function. The
// method body is lowered on demand, when the enclosing method's scope still
// holds the captured locals.
func (l *lowerer) liftLocalFn(name string, fn *ast.Node) *localFn {
	if lf, ok := l.localFns[name]; ok {
		return lf
	}
	// Captures: free identifiers that resolve to locals of the enclosing
	// method and are never assigned.
	captures := []string{}
	seen := map[string]bool{}
	for _, name := range l.freeLocals(fn) {
		lf := l.localFns[name]
		if pending := l.pendingFns[name]; lf == nil && pending != nil {
			delete(l.pendingFns, name)
			lf = l.liftLocalFn(name, pending)
		}
		names := []string{name}
		// Known lifted callees remain direct calls. Their actual lexical inputs
		// must be carried through the caller, rather than capturing a void value.
		if lf != nil {
			names = lf.captures
		}
		for _, c := range names {
			if !seen[c] {
				captures = append(captures, c)
				seen[c] = true
			}
		}
	}
	sortStrings(captures)
	m := &hir.Method{Node: l.node(fn), Name: "fn_" + name, Virtual: true, Result: hir.T(hir.Void)}
	if l.method != nil && l.method.Static {
		m.Static = true
		m.Virtual = false
	}
	for _, c := range captures {
		t, _ := l.lookup(c)
		m.Params = append(m.Params, hir.Param{Name: c, Type: t})
	}
	sigParams := fn.Parameters()
	l.push()
	for i, c := range captures {
		l.declare(c, m.Params[i].Type)
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
		m.Params = append(m.Params, hir.Param{Name: p.Name().Text(), Type: typ})
		l.declare(p.Name().Text(), typ)
	}
	if fn.Type() != nil {
		m.Result = l.mapTypeNode(fn.Type())
	} else if t := l.ck.GetTypeAtLocation(fn); t != nil {
		if sig := l.ck.GetSignaturesOfType(t, checker.SignatureKindCall); len(sig) > 0 {
			m.Result = l.mapCheckerType(fn, l.ck.GetReturnTypeOfSignature(sig[0]))
		}
	}
	// Lower the body now, while the captures are in scope.
	savedMethod := l.method
	l.method = m
	body := fn.Body()
	if body == nil {
		l.pop()
		return nil
	}
	restoreFns := l.withLocalFns(body)
	var inner *hir.Stmt
	if body.Kind == ast.KindBlock {
		inner = l.block(body)
	} else {
		inner = hir.B(&hir.Stmt{Kind: hir.Return, Node: l.node(body), X: l.expr(body)})
	}
	restoreFns()
	l.applyFnDefaults(sigParams, m)
	m.Body = inner
	l.method = savedMethod
	l.pop()
	lf := &localFn{method: m, captures: captures, owner: l.class}
	l.localFns[name] = lf
	l.class.Methods = append(l.class.Methods, m)
	return lf
}

// Lifted defaults are rejected until their capture offsets and evaluation
// order can be represented faithfully in a prologue.
func (l *lowerer) applyFnDefaults(params []*ast.ParameterDeclarationNode, m *hir.Method) {
	for _, p := range params {
		if p.Initializer() != nil {
			l.diagf(p, "unsupported-lifted-default", "default of lifted function parameter %s is not lowered", p.Name().Text())
		}
	}
}

// freeLocals lists the enclosing-method local names an arrow function reads
// without assigning them (and without declaring them itself).
func (l *lowerer) freeLocals(fn *ast.Node) []string {
	declared := map[string]bool{}
	assigned := map[string]bool{}
	used := map[string]bool{}
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
					used[name] = true
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
				walk(b.Right, inFn)
				return
			}
		case ast.KindArrowFunction, ast.KindFunctionExpression, ast.KindFunctionDeclaration:
			x.ForEachChild(func(c *ast.Node) bool { walk(c, true); return false })
			return
		}
		walkChildren(x, inFn)
	}
	walk(fn, true)
	var out []string
	for name := range used {
		if !declared[name] && !assigned[name] {
			out = append(out, name)
		}
	}
	sortStrings(out)
	return out
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
	if x == nil {
		return nil
	}
	if dst.Kind == hir.Optional && dst.Args[0].Kind == hir.Dynamic && !x.Type.Equal(dst) {
		boxed := l.coerce(x, hir.T(hir.Dynamic))
		return &hir.Expr{Kind: hir.Conditional, Type: dst, X: hir.L(hir.T(hir.Bool), true), Y: boxed, Z: &hir.Expr{Kind: hir.Lit, Type: dst}}
	}
	if x.Type.Kind == hir.Dynamic && dst.Kind == hir.Optional && dst.Args[0].Kind != hir.Dynamic {
		base := dst.Args[0]
		if base.Kind == hir.String || base.Kind == hir.Number || base.Kind == hir.Bool {
			x = l.tempInit(nil, x.Type, x)
			return &hir.Expr{Kind: hir.Conditional, Type: dst, X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: x}, Y: &hir.Expr{Kind: hir.Lit, Type: dst}, Z: l.coerce(x, base)}
		}
	}
	if x.Type.Kind == hir.Dynamic && (dst.Kind == hir.String || dst.Kind == hir.Number || dst.Kind == hir.Bool) {
		op := map[hir.Kind]string{hir.String: "dynamic.asString", hir.Number: "dynamic.asNumber", hir.Bool: "dynamic.asBoolean"}[dst.Kind]
		return l.rtOp(op, x, dst)
	}
	if dst.Kind == hir.Optional && x.Type.Equal(dst.Args[0]) {
		return &hir.Expr{Kind: hir.Conditional, Type: dst, X: hir.L(hir.T(hir.Bool), true), Y: x, Z: &hir.Expr{Kind: hir.Lit, Type: dst}}
	}
	if dst.Kind == hir.Dynamic && x.Type.Kind != hir.Dynamic {
		return l.rtOp("dynamic.of", x, hir.T(hir.Dynamic))
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
			for base := l.classByName(l.ifaceClassBases[source.Name]); base != nil; base = l.classByName(base.Super) {
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
		for base := l.classByName(l.ifaceClassBases[x.Type.Name]); base != nil; base = l.classByName(base.Super) {
			if base.Name == dst.Name {
				return &hir.Expr{Kind: hir.Cast, Node: x.Node, Type: dst, X: x}
			}
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
