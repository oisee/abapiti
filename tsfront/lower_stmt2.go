package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Phase-2 statement lowering: try/catch/throw, switch, for-in, destructuring
// declarations and element assignment targets.

// tryStatement lowers try { } catch (e: any) { }: the catch variable is
// typed as the common base of the thrown values (the builtin Error), which
// `instanceof` then narrows exactly like the original.
func (l *lowerer) tryStatement(n *ast.Node) *hir.Stmt {
	t := n.AsTryStatement()
	if t.FinallyBlock != nil && t.CatchClause == nil {
		if l.containsReturn(t.TryBlock) || l.containsLoopControl(t.TryBlock) {
			l.diagf(t.FinallyBlock, "unsupported-statement", "finally with return, break or continue in the try block is not lowered")
			return nil
		}
		l.diagf(n, "note-finally", "try/finally lowered as catch-all, finally block, re-raise")
		return &hir.Stmt{Kind: hir.Finally, Node: l.node(n), Body: l.scopeBlock(t.TryBlock), Else: l.scopeBlock(t.FinallyBlock)}
	}
	if t.FinallyBlock != nil {
		l.diagf(t.FinallyBlock, "unsupported-statement", "finally with catch is not lowered")
		return nil
	}
	if t.CatchClause == nil {
		l.diagf(n, "unsupported-statement", "try without catch is not lowered")
		return nil
	}
	cc := t.CatchClause.AsCatchClause()
	name := "err"
	if cc.VariableDeclaration != nil && cc.VariableDeclaration.Name() != nil && cc.VariableDeclaration.Name().Kind == ast.KindIdentifier {
		name = cc.VariableDeclaration.Name().Text()
	}
	typ := l.throwBaseType(n)
	if l.settledEnabled && (l.asyncMethods[l.method] || containsAwait(t.TryBlock)) {
		typ = hir.T(hir.Dynamic)
	}
	s := &hir.Stmt{Kind: hir.Try, Node: l.node(n), Name: name, Type: typ}
	if cc.VariableDeclaration != nil && cc.VariableDeclaration.Name() != nil && cc.VariableDeclaration.Name().Kind == ast.KindIdentifier {
		if l.declSymbols == nil {
			l.declSymbols = map[*hir.Stmt]*ast.Symbol{}
		}
		l.declSymbols[s] = l.ck.GetSymbolAtLocation(cc.VariableDeclaration.Name())
	}
	s.Body = l.scopeBlock(t.TryBlock)
	l.push()
	l.declare(name, typ)
	s.Else = l.scopeBlock(cc.Block)
	l.pop()
	return s
}

// throwBaseType picks the catch payload type: the builtin Error covers the
// thrown values of the closure (Error and its subclasses, and payload throws
// of other types fall back to the first found).
func (l *lowerer) throwBaseType(n *ast.Node) hir.Type {
	base := ""
	var walk func(*ast.Node)
	walk = func(x *ast.Node) {
		if x == nil || base != "" {
			return
		}
		if x.Kind == ast.KindThrowStatement && x.AsThrowStatement().Expression != nil {
			t := l.ck.GetTypeAtLocation(x.AsThrowStatement().Expression)
			if t != nil && t.Symbol() != nil {
				before := len(l.diags)
				mapped := l.mapCheckerType(x, t)
				if len(l.diags) != before {
					l.diags = l.diags[:before]
				} else if mapped.Kind == hir.ClassRef {
					base = mapped.Name
				}
			}
		}
		x.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(n)
	return hir.Ref(l.builtinError())
}

// builtinError returns the HIR name of the synthesized Error class,
// creating it once.
func (l *lowerer) builtinError() string {
	const name = "builtin.Error"
	for _, c := range l.out.Classes {
		if c.Name == name {
			return name
		}
	}
	msg := hir.T(hir.String)
	c := &hir.Class{Node: hir.Node{ID: l.nextID(), Source: name}, Name: name,
		Fields: []hir.Field{{Node: hir.Node{ID: l.nextID(), Source: name}, Name: "message", Type: msg}},
		Ctor: &hir.Method{Node: hir.Node{ID: l.nextID(), Source: name}, Name: "constructor",
			Params: []hir.Param{{Name: "message", Type: hir.T(hir.Optional, msg)}}, Result: hir.T(hir.Void),
			Body: hir.B(&hir.Stmt{Kind: hir.Assign, Node: hir.Node{ID: l.nextID(), Source: name},
				X: &hir.Expr{Kind: hir.FieldGet, Node: hir.Node{ID: l.nextID(), Source: name}, Name: "message", Type: msg,
					X: &hir.Expr{Kind: hir.This, Node: hir.Node{ID: l.nextID(), Source: name}, Type: hir.Ref(name)}},
				Y: &hir.Expr{Kind: hir.Conditional, Type: msg, X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: hir.V("message", hir.T(hir.Optional, msg))}, Y: hir.L(msg, ""), Z: &hir.Expr{Kind: hir.Narrow, Type: msg, X: hir.V("message", hir.T(hir.Optional, msg))}}})}}
	l.out.Classes = append(l.out.Classes, c)
	if l.unprunedFeatures {
		// Native parser failures retain their Error ancestry and distinct
		// constructor identity; they are not host traps or plain Error values.
		l.out.Classes = append(l.out.Classes, &hir.Class{Node: c.Node, Name: "builtin.SyntaxError", Super: name})
	}
	l.diagf(nil, "note-builtin-error", "TypeScript Error mapped to the synthesized class builtin.Error")
	return name
}

// builtinTypeError returns the HIR name of builtin.TypeError, creating it on
// first use so programs that never raise or name it keep their class set.
func (l *lowerer) builtinTypeError() string {
	const name = "builtin.TypeError"
	super := l.builtinError()
	for _, c := range l.out.Classes {
		if c.Name == name {
			return name
		}
	}
	l.out.Classes = append(l.out.Classes, &hir.Class{Node: hir.Node{ID: l.nextID(), Source: name}, Name: name, Super: super})
	return name
}

// switchStatement lowers terminated clauses to an if/else chain. Empty labels
// fall through even with noFallthroughCasesInSwitch enabled. Nonempty
// fallthrough and nested switch-targeting breaks refuse rather than silently
// skipping statements or breaking an enclosing loop.
func (l *lowerer) switchStatement(n *ast.Node) *hir.Stmt {
	sw := n.AsSwitchStatement()
	if sw.CaseBlock == nil {
		return nil
	}
	subject := l.expr(sw.Expression)
	if subject == nil {
		return nil
	}
	allowFallthrough := l.unprunedFeatures && subject.Kind == hir.RuntimeOp && subject.Op == "union.tag"
	var clauses []*ast.Node
	for _, c := range sw.CaseBlock.AsCaseBlock().Clauses.Nodes {
		clauses = append(clauses, c)
	}
	// Resolve each label to the next nonempty clause, including default labels.
	bodies := make([]*ast.Node, len(clauses))
	var next *ast.Node
	for i := len(clauses) - 1; i >= 0; i-- {
		c := clauses[i]
		statements := c.AsCaseOrDefaultClause().Statements.Nodes
		for _, s := range statements {
			switch s.Kind {
			case ast.KindVariableStatement, ast.KindClassDeclaration, ast.KindFunctionDeclaration, ast.KindEnumDeclaration:
				// JS has one lexical environment for the entire case block;
				// even case expressions see its uninitialized bindings (TDZ).
				// Per-body HIR scopes cannot model that. Explicit blocks are OK.
				l.diagf(s, "unsupported-statement", "switch-scoped declarations require a shared lexical environment")
				return nil
			}
		}
		if len(statements) > 0 {
			if switchNestedBreak(c) {
				l.diagf(c, "unsupported-statement", "nested break targeting switch is not lowered")
				return nil
			}
			terminated := false
			for _, s := range statements {
				if switchClauseEnds(s) {
					terminated = true
					break
				}
			}
			if i+1 < len(clauses) && !terminated && !allowFallthrough {
				l.diagf(c, "unsupported-statement", "nonempty switch fallthrough is not lowered")
				return nil
			}
			next = c
		}
		bodies[i] = next
	}
	values := make([]*hir.Expr, len(clauses))
	stable := subject.Kind == hir.Lit || subject.Kind == hir.Local
	for i, c := range clauses {
		if c.Kind == ast.KindDefaultClause {
			continue
		}
		value := l.expr(c.AsCaseOrDefaultClause().Expression)
		if value == nil {
			return nil
		}
		stable = stable && value.Kind == hir.Lit
		if subject.Type.Kind == hir.Optional && subject.Type.Args[0].Equal(value.Type) {
			value = l.optionalView(value, subject.Type)
		}
		values[i] = value
	}
	// A simple local compared only to literals cannot change during selection.
	// Otherwise snapshot the discriminant once, before any case expressions.
	var decl *hir.Stmt
	if !stable {
		l.serial++
		name := "t" + itoa(l.serial)
		decl = &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: name, Type: subject.Type, X: subject}
		subject = hir.V(name, subject.Type)
	}
	body := func(c *ast.Node) *hir.Stmt {
		if c == nil {
			return hir.B()
		}
		l.push()
		defer l.pop()
		if allowFallthrough {
			return l.familySwitchBody(c, clauses)
		}
		return hir.B(l.stmtList(clauseStatements(c))...)
	}
	// Default is the fallback after *all* case tests, wherever it appears.
	var chain *hir.Stmt
	for i, c := range clauses {
		if c.Kind == ast.KindDefaultClause {
			chain = body(bodies[i])
		}
	}
	for i := len(clauses) - 1; i >= 0; i-- {
		c := clauses[i]
		if c.Kind == ast.KindDefaultClause {
			continue
		}
		cond := &hir.Expr{Kind: hir.Binary, Node: l.node(c), Op: "==", Type: hir.T(hir.Bool), X: subject, Y: values[i]}
		chain = &hir.Stmt{Kind: hir.If, Node: l.node(c), X: cond, Body: body(bodies[i]), Else: chain}
	}
	if decl != nil {
		return hir.B(decl, chain)
	}
	return chain
}

// forInStatement lowers `for (const k in obj)`: records and namespace maps
// are OrderedMaps, so this is a loop over map.keys().
func (l *lowerer) forInStatement(n *ast.Node) *hir.Stmt {
	f := n.AsForInOrOfStatement()
	if f.Initializer == nil || f.Initializer.Kind != ast.KindVariableDeclarationList {
		l.diagf(n, "unsupported-statement", "for-in needs a variable declaration")
		return nil
	}
	decls := f.Initializer.AsVariableDeclarationList().Declarations.Nodes
	if len(decls) != 1 || decls[0].Name() == nil || decls[0].Name().Kind != ast.KindIdentifier {
		l.diagf(n, "unsupported-statement", "for-in needs a single identifier binding")
		return nil
	}
	name := decls[0].Name().Text()
	recv := l.expr(f.Expression)
	if recv != nil && recv.Type.Kind == hir.Optional && recv.Type.Args[0].Kind == hir.OrderedMap && recv.Type.Args[0].Args[0].Kind == hir.String {
		// for-in over undefined iterates nothing. The receiver is evaluated
		// once into a local declared by the returned block.
		l.serial++
		present := hir.V("t"+itoa(l.serial), recv.Type)
		decl := &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: present.Name, Type: recv.Type, X: recv}
		keys := l.rtOp("map.keys", &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: recv.Type.Args[0], X: present}, hir.T(hir.Array, hir.T(hir.String)))
		l.push()
		l.declare(name, hir.T(hir.String))
		body := l.scopeBlock(f.Statement)
		l.pop()
		loop := &hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: name, Type: hir.T(hir.String), X: keys, Body: body}
		absent := &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: present}
		return hir.B(decl, &hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Node: l.node(n), Op: "!", Type: hir.T(hir.Bool), X: absent}, Body: hir.B(loop)})
	}
	if recv == nil {
		return nil
	}
	if l.unprunedFeatures && (recv.Type.Kind == hir.ClassRef || recv.Type.Kind == hir.Dynamic) {
		keys := l.rtOp("dynamic.keys", l.coerce(recv, hir.T(hir.Dynamic)), hir.T(hir.Array, hir.T(hir.String)))
		l.push()
		l.declare(name, hir.T(hir.String))
		body := l.scopeBlock(f.Statement)
		l.pop()
		return &hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: name, Type: hir.T(hir.String), X: keys, Body: body}
	}
	if recv.Type.Kind != hir.OrderedMap || recv.Type.Args[0].Kind != hir.String {
		l.diagf(n, "unsupported-statement", "for-in needs a string-keyed map (%s)", recv.Type.String())
		return nil
	}
	keys := l.rtOp("map.keys", recv, hir.T(hir.Array, hir.T(hir.String)))
	l.push()
	l.declare(name, hir.T(hir.String))
	body := l.scopeBlock(f.Statement)
	l.pop()
	return &hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: name, Type: hir.T(hir.String), X: keys, Body: body}
}

// switchClauseEnds proves an unconditional abrupt completion. Conditional
// switch breaks are rejected separately because HIR Break means loop exit.
func switchClauseEnds(n *ast.Node) bool {
	switch n.Kind {
	case ast.KindBreakStatement, ast.KindContinueStatement, ast.KindReturnStatement, ast.KindThrowStatement:
		return true
	case ast.KindBlock:
		for _, s := range n.AsBlock().Statements.Nodes {
			if switchClauseEnds(s) {
				return true
			}
		}
	case ast.KindIfStatement:
		x := n.AsIfStatement()
		return x.ElseStatement != nil && switchClauseEnds(x.ThenStatement) && switchClauseEnds(x.ElseStatement)
	}
	return false
}

func switchNestedBreak(c *ast.Node) bool {
	var walk func(*ast.Node) bool
	walk = func(n *ast.Node) bool {
		switch n.Kind {
		case ast.KindBreakStatement:
			return n.AsBreakStatement().Label == nil
		case ast.KindSwitchStatement, ast.KindForStatement, ast.KindForOfStatement, ast.KindForInStatement,
			ast.KindWhileStatement, ast.KindDoStatement, ast.KindFunctionDeclaration, ast.KindFunctionExpression, ast.KindArrowFunction:
			return false // a break here targets a different construct
		}
		return n.ForEachChild(walk)
	}
	for _, s := range clauseStatements(c) {
		if walk(s) {
			return true
		}
	}
	return false
}

// clauseStatements returns a case clause's statements up to its direct
// `break` (the if/else chain ends the clause anyway); statements after
// the break are unreachable.
func clauseStatements(c *ast.Node) []*ast.Node {
	var out []*ast.Node
	for _, s := range c.AsCaseOrDefaultClause().Statements.Nodes {
		if s.Kind == ast.KindBreakStatement && s.AsBreakStatement().Label == nil {
			break
		}
		out = append(out, s)
	}
	return out
}

// destructureDecl lowers `const {a: x, b} = expr` and `const [x, y] = expr`.
func (l *lowerer) destructureDecl(d *ast.Node) []*hir.Stmt {
	if d.Name() == nil || (d.Name().Kind != ast.KindObjectBindingPattern && d.Name().Kind != ast.KindArrayBindingPattern) {
		return nil
	}
	if d.Initializer() == nil {
		l.diagf(d, "unsupported-binding", "destructuring declaration without initializer")
		return nil
	}
	t := l.mapCheckerType(d, l.ck.GetTypeAtLocation(d.Initializer()))
	if t.Kind == hir.Void {
		return nil
	}
	l.serial++
	tmp := "s" + itoa(l.serial)
	l.declare(tmp, t)
	decl := &hir.Stmt{Kind: hir.VarDecl, Node: l.node(d), Name: tmp, Type: t, X: l.expr(d.Initializer())}
	out := []*hir.Stmt{decl}
	// destructurePattern appends to the prelude; collect around it.
	saved := l.pend
	l.pend = nil
	l.destructurePattern(d.Name(), hir.V(tmp, t))
	out = append(out, l.pend...)
	l.pend = saved
	return out
}

// stmtList lowers a list of statements (clauses' statement lists).
func (l *lowerer) stmtList(nodes []*ast.Node) []*hir.Stmt {
	var out []*hir.Stmt
	for _, s := range nodes {
		out = append(out, l.stmts(s)...)
	}
	return out
}
