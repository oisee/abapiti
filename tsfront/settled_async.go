package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

func (l *lowerer) finishAsync(n *ast.Node, m *hir.Method, body *hir.Stmt) *hir.Stmt {
	if !l.asyncMethods[m] {
		return body
	}
	if m.Result.Kind != hir.Promise {
		l.diagf(n, "unsupported-async", "async result requires the settled Promise ABI")
		return nil
	}
	// A void async body still produces a fulfillment, not an ordinary void ABI.
	if m.Result.Args[0].Kind == hir.Void {
		body = hir.B(body, &hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.PromiseFulfill, Type: m.Result}})
	}
	return &hir.Stmt{Kind: hir.Async, Node: l.node(n), Body: body}
}

func (l *lowerer) asyncReturn(n, value *ast.Node) *hir.Stmt {
	result := l.method.Result
	if result.Kind != hir.Promise {
		return nil
	}
	var x *hir.Expr
	if value != nil && !(result.Args[0].Kind == hir.Void && isUndefinedKeyword(value)) {
		l.hint = result.Args[0]
		x = l.expr(value)
		l.hint = hir.Type{}
		if x == nil {
			return nil
		}
		if x.Type.Kind == hir.Promise {
			if !l.certifiedSettled(value, map[*ast.Node]bool{}) {
				l.diagf(value, "unsupported-async", "async return provider is not certified already-settled (pending promises and arbitrary thenables need a scheduler)")
				return nil
			}
			// Async adoption drains into a fresh promise. Unlike Promise.resolve,
			// an async call must not return the input promise identity.
			if !x.Type.Equal(result) {
				l.diagf(value, "unsupported-async", "settled promise adoption requires the same fulfillment type")
				return nil
			}
			drain := &hir.Expr{Kind: hir.PromiseAwait, Node: l.node(n), Type: result.Args[0], X: x}
			return &hir.Stmt{Kind: hir.Return, Node: l.node(n), X: &hir.Expr{Kind: hir.PromiseFulfill, Node: l.node(n), Type: result, X: drain}}
		}
		if l.mayThenable(value) {
			l.diagf(value, "unsupported-async", "async return may be an arbitrary thenable; assimilation is refused")
			return nil
		}
		if x.Type.Kind == hir.Void && !l.certifiedUndefinedValue(value, map[*ast.Node]bool{}) {
			l.diagf(value, "unsupported-async", "void fulfillment expression is not certified undefined; a value-preserving void ABI is outside the settled model")
			return nil
		}
		x = l.coerce(x, result.Args[0])
	}
	if x == nil && result.Args[0].Kind != hir.Void {
		l.diagf(n, "unsupported-async", "bare async return requires a void fulfillment")
		return nil
	}
	return &hir.Stmt{Kind: hir.Return, Node: l.node(n), X: &hir.Expr{Kind: hir.PromiseFulfill, Node: l.node(n), Type: result, X: x}}
}

func (l *lowerer) awaitSettled(n *ast.Node) *hir.Expr {
	operand := n.AsAwaitExpression().Expression
	if !l.settledEnabled {
		l.diagf(n, "unsupported-async", "await requires unpruned settled-promise lowering")
		return nil
	}
	l.settlementFailure = ""
	if !l.certifiedSettled(operand, map[*ast.Node]bool{}) {
		if l.settlementFailure != "" {
			l.diagf(n, "unsupported-async", "settlement proof failed (not certified already-settled): %s", l.settlementFailure)
			return nil
		}
		l.diagf(n, "unsupported-async", "await provider is not certified already-settled (open progress providers, pending promises and arbitrary thenables need a scheduler)")
		return nil
	}
	x := l.expr(operand)
	if x == nil {
		return nil
	}
	if x.Type.Kind != hir.Promise {
		if x.Type.Kind == hir.Optional && x.Type.Args[0].Kind == hir.Promise {
			if x.Type.Args[0].Args[0].Kind != hir.Void || !l.closedProgressCall(operand, map[*ast.Node]bool{}) {
				l.diagf(n, "unsupported-async", "optional Promise await requires a closed-world IProgress.tick settlement proof")
				return nil
			}
			return &hir.Expr{Kind: hir.PromiseAwait, Node: l.node(n), Type: x.Type.Args[0].Args[0], X: x}
		}
		return x
	}
	return &hir.Expr{Kind: hir.PromiseAwait, Node: l.node(n), Type: x.Type.Args[0], X: x}
}

func (l *lowerer) mayThenable(n *ast.Node) bool {
	t := l.ck.GetTypeAtLocation(n)
	if t == nil || t.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
		return true
	}
	return l.ck.GetPropertyOfType(t, "then") != nil
}

// Certification follows producers, never a Promise annotation or an interface
// signature. Dispatch and transitive source bodies are re-derived per Lower.
func (l *lowerer) certifiedSettled(n *ast.Node, seen map[*ast.Node]bool) bool {
	if n == nil || seen[n] && n.Kind != ast.KindCallExpression {
		return false
	}
	// Call cycles are handled by the body graph. Value/alias cycles still
	// refuse: revisiting a Promise annotation is not a producer proof.
	if n.Kind != ast.KindCallExpression {
		seen[n] = true
		defer delete(seen, n)
	}
	if n.Kind == ast.KindParenthesizedExpression {
		return l.certifiedSettled(n.Expression(), seen)
	}
	t := l.ck.GetTypeAtLocation(n)
	if t != nil && t.Flags()&(checker.TypeFlagsStringLike|checker.TypeFlagsNumberLike|checker.TypeFlagsBooleanLike|checker.TypeFlagsVoid|checker.TypeFlagsUndefined|checker.TypeFlagsNull) != 0 {
		return true
	}
	if n.Kind == ast.KindIdentifier {
		sym := l.resolve(n)
		if sym != nil && sym.ValueDeclaration != nil {
			d := sym.ValueDeclaration
			if d.Kind == ast.KindVariableDeclaration && d.Parent != nil && d.Parent.Flags&ast.NodeFlagsConst != 0 {
				return l.certifiedSettled(d.Initializer(), seen)
			}
		}
	}
	if n.Kind != ast.KindCallExpression {
		return false
	}
	if name, ok := l.builtinPromiseCall(n); ok {
		args := n.Arguments()
		switch name {
		case "reject":
			return len(args) <= 1
		case "resolve":
			return len(args) == 0 || len(args) == 1 && (!l.mayThenable(args[0]) || l.certifiedSettled(args[0], seen))
		}
		return false
	}
	if l.isProgressDispatch(n) {
		return l.closedProgressCall(n, seen)
	}
	targets, reason := l.settledDispatch(n)
	if reason != "" {
		l.settlementFailure = reason
		return false
	}
	if len(targets) == 0 {
		return false
	}
	for _, d := range targets {
		if !l.certifiedSettledBody(d, seen) {
			if l.settlementFailure == "" {
				l.settlementFailure = l.settledLabel(d) + " is not provably already-settled"
			}
			return false
		}
	}
	return true
}

func (l *lowerer) builtinPromiseCall(n *ast.Node) (string, bool) {
	callee := n.Expression()
	if callee == nil || callee.Kind != ast.KindPropertyAccessExpression {
		return "", false
	}
	p := callee.AsPropertyAccessExpression()
	if p.Expression.Kind != ast.KindIdentifier || p.Expression.Text() != "Promise" || !l.librarySymbol(l.resolve(p.Expression)) {
		return "", false
	}
	return p.Name().Text(), true
}

func (l *lowerer) promiseStatic(n *ast.Node) (*hir.Expr, bool) {
	callee := n.Expression()
	if l.settledEnabled && callee != nil && callee.Kind == ast.KindIdentifier {
		switch callee.Text() {
		case "setTimeout", "setInterval", "queueMicrotask":
			sym := l.resolve(callee)
			external := sym == nil || l.librarySymbol(sym)
			if sym != nil {
				for _, d := range sym.Declarations {
					f := ast.GetSourceFileOfNode(d)
					if f != nil && f.IsDeclarationFile {
						external = true
					}
				}
			}
			if external {
				l.diagf(n, "unsupported-async", "%s is refused: pending work and observable microtask interleaving require a scheduler", callee.Text())
				return nil, true
			}
		}
	}
	name, ok := l.builtinPromiseCall(n)
	if !ok {
		return nil, false
	}
	if !l.settledEnabled {
		l.diagf(n, "unsupported-async", "Promise operations require unpruned settled-promise lowering")
		return nil, true
	}
	if name != "resolve" && name != "reject" {
		l.diagf(n, "unsupported-async", "Promise.%s is refused: pending combinators and microtask interleaving need a scheduler", name)
		return nil, true
	}
	args := n.Arguments()
	if len(args) > 1 {
		l.diagf(n, "unsupported-async", "Promise.%s expects at most one argument", name)
		return nil, true
	}
	typ := l.mapCheckerType(n, l.ck.GetTypeAtLocation(n))
	if typ.Kind != hir.Promise {
		return nil, true
	}
	if name == "reject" {
		value := &hir.Expr{Kind: hir.Lit, Type: hir.T(hir.Optional, hir.T(hir.Dynamic))}
		if len(args) == 1 {
			value = l.expr(args[0])
		}
		if value == nil {
			return nil, true
		}
		return &hir.Expr{Kind: hir.PromiseReject, Node: l.node(n), Type: typ, X: value}, true
	}
	if len(args) == 0 {
		return &hir.Expr{Kind: hir.PromiseFulfill, Node: l.node(n), Type: typ}, true
	}
	if l.mayThenable(args[0]) && !l.certifiedSettled(args[0], map[*ast.Node]bool{}) {
		l.diagf(n, "unsupported-async", "Promise.resolve argument may be pending or an arbitrary thenable; assimilation is refused")
		return nil, true
	}
	value := l.expr(args[0])
	if value == nil {
		return nil, true
	}
	if value.Type.Kind == hir.Promise {
		return value, true
	}
	if value.Type.Kind == hir.Void && !l.certifiedUndefinedValue(args[0], map[*ast.Node]bool{}) {
		l.diagf(n, "unsupported-async", "void fulfillment expression is not certified undefined; a value-preserving void ABI is outside the settled model")
		return nil, true
	}
	return &hir.Expr{Kind: hir.PromiseFulfill, Node: l.node(n), Type: typ, X: l.coerce(value, typ.Args[0])}, true
}

func containsAwait(n *ast.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind == ast.KindAwaitExpression {
		return true
	}
	if n.Kind == ast.KindFunctionDeclaration || n.Kind == ast.KindFunctionExpression || n.Kind == ast.KindArrowFunction {
		return false
	}
	found := false
	n.ForEachChild(func(c *ast.Node) bool { found = found || containsAwait(c); return found })
	return found
}

// A call with await sites may not escape and race subsequent source effects.
// This drain has no microtask queue: conservatively require immediate await or
// result adoption for such producers, even when their promises are settled.
func (l *lowerer) settledCallUse(n *ast.Node, x *hir.Expr) bool {
	if !l.settledEnabled || n.Kind != ast.KindCallExpression || (x.Type.Kind != hir.Promise && !(x.Type.Kind == hir.Optional && x.Type.Args[0].Kind == hir.Promise)) {
		return true
	}
	suspends := l.producerHasAwait(n, map[*ast.Node]bool{})
	if !suspends {
		return true
	}
	parent := n.Parent
	for parent != nil && parent.Kind == ast.KindParenthesizedExpression {
		parent = parent.Parent
	}
	if parent != nil && (parent.Kind == ast.KindAwaitExpression || parent.Kind == ast.KindReturnStatement) {
		return true
	}
	l.diagf(n, "unsupported-async", "unawaited call with await sites may expose microtask interleaving; immediate settled drain is required")
	return false
}

// Follow Promise-returning wrappers too; hiding an await behind a synchronous
// wrapper cannot establish absence of observable microtask interleaving.
func (l *lowerer) producerHasAwait(n *ast.Node, seen map[*ast.Node]bool) bool {
	if n == nil {
		return false
	}
	if seen[n] {
		return true
	}
	seen[n] = true
	defer delete(seen, n)
	if _, ok := l.builtinPromiseCall(n); ok {
		return false
	}
	// Include the await sites of every closed-world dispatch target.
	if l.progressCallHasAwait(n, seen) {
		return true
	}
	targets, _ := l.settledDispatch(n)
	for _, d := range targets {
		if l.settledBodyHasAwait(d, seen) {
			return true
		}
	}
	return false
}

// The existing ordinary void ABI is owned elsewhere. Use its expression effects
// only when the source proves the fulfillment really is undefined.
func (l *lowerer) certifiedUndefinedValue(n *ast.Node, seen map[*ast.Node]bool) bool {
	if n == nil || isUndefinedKeyword(n) {
		return true
	}
	if seen[n] {
		return false
	}
	seen[n] = true
	defer delete(seen, n)
	if n.Kind == ast.KindParenthesizedExpression {
		return l.certifiedUndefinedValue(n.Expression(), seen)
	}
	if n.Kind == ast.KindAwaitExpression {
		return l.certifiedSettled(n.AsAwaitExpression().Expression, map[*ast.Node]bool{})
	}
	if n.Kind != ast.KindCallExpression {
		return false
	}
	sym := l.resolve(n.Expression())
	if sym == nil {
		return false
	}
	for _, d := range sym.Declarations {
		if d.Body() == nil {
			continue
		}
		if seen[d] {
			return false
		}
		if d.Kind == ast.KindMethodDeclaration && d.ModifierFlags()&(ast.ModifierFlagsStatic|ast.ModifierFlagsPrivate) == 0 {
			callee := n.Expression()
			if callee.Kind != ast.KindPropertyAccessExpression || callee.AsPropertyAccessExpression().Expression.Kind != ast.KindNewExpression {
				return false
			}
		}
		seen[d] = true
		safe := true
		var walk func(*ast.Node)
		walk = func(x *ast.Node) {
			if x == nil || !safe {
				return
			}
			if x.Kind == ast.KindFunctionDeclaration || x.Kind == ast.KindArrowFunction || x.Kind == ast.KindFunctionExpression {
				return
			}
			if x.Kind == ast.KindReturnStatement {
				safe = l.certifiedUndefinedValue(x.AsReturnStatement().Expression, seen)
				return
			}
			x.ForEachChild(func(c *ast.Node) bool { walk(c); return !safe })
		}
		walk(d.Body())
		delete(seen, d)
		return safe
	}
	return false
}

func (l *lowerer) checkSettledBody(d *ast.Node, seen map[*ast.Node]bool, returning bool) bool {
	if d == nil || d.Body() == nil {
		return false
	}
	if d.Body().Kind != ast.KindBlock {
		if returning && l.mayThenable(d.Body()) && !l.certifiedSettled(d.Body(), seen) {
			return false
		}
	}
	safe := true
	var walk func(*ast.Node)
	walk = func(x *ast.Node) {
		if x == nil || !safe {
			return
		}
		if x != d && (x.Kind == ast.KindArrowFunction || x.Kind == ast.KindFunctionExpression || x.Kind == ast.KindFunctionDeclaration) {
			return
		}
		if x.Kind == ast.KindAwaitExpression && !l.certifiedSettled(x.AsAwaitExpression().Expression, seen) {
			safe = false
			return
		}
		if x.Kind == ast.KindReturnStatement && x.AsReturnStatement().Expression != nil {
			v := x.AsReturnStatement().Expression
			if returning && l.mayThenable(v) && !l.certifiedSettled(v, seen) {
				safe = false
				return
			}
		}
		if x.Kind == ast.KindNewExpression && !l.settledConstruction(x, seen) {
			safe = false
			return
		}
		if x.Kind == ast.KindCallExpression && !l.settledBodyCall(x, seen) {
			safe = false
			return
		}
		x.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(d.Body())
	return safe
}

func (l *lowerer) settledBodyHasAwait(d *ast.Node, seen map[*ast.Node]bool) bool {
	if containsAwait(d.Body()) {
		return true
	}
	found := false
	var walk func(*ast.Node)
	walk = func(x *ast.Node) {
		if x == nil || found {
			return
		}
		if x.Kind == ast.KindArrowFunction || x.Kind == ast.KindFunctionExpression || x.Kind == ast.KindFunctionDeclaration {
			return
		}
		if x.Kind == ast.KindCallExpression {
			t := l.ck.GetTypeAtLocation(x)
			if t != nil && t.Symbol() != nil && t.Symbol().Name == "Promise" && l.librarySymbol(t.Symbol()) {
				found = l.producerHasAwait(x, seen)
				if found {
					return
				}
			}
		}
		x.ForEachChild(func(c *ast.Node) bool { walk(c); return found })
	}
	walk(d.Body())
	if found {
		return true
	}
	return false
}
