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
	s := &hir.Stmt{Kind: hir.Try, Node: l.node(n), Name: name, Type: typ}
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
	l.diagf(nil, "note-builtin-error", "TypeScript Error mapped to the synthesized class builtin.Error")
	return name
}

// switchStatement lowers a switch to an if/else chain (no fallthrough in the
// sources: noFallthroughCasesInSwitch is on).
func (l *lowerer) switchStatement(n *ast.Node) *hir.Stmt {
	sw := n.AsSwitchStatement()
	if sw.CaseBlock == nil {
		return nil
	}
	subject := l.expr(sw.Expression)
	if subject == nil {
		return nil
	}
	var clauses []*ast.Node
	for _, c := range sw.CaseBlock.AsCaseBlock().Clauses.Nodes {
		clauses = append(clauses, c)
	}
	// Lower from the last clause backwards, building else-if chains.
	var chain *hir.Stmt
	for i := len(clauses) - 1; i >= 0; i-- {
		c := clauses[i]
		if c.Kind == ast.KindDefaultClause {
			chain = hir.B(l.stmtList(clauseStatements(c))...)
			continue
		}
		value := l.expr(c.AsCaseOrDefaultClause().Expression)
		if value == nil {
			return nil
		}
		if subject.Type.Kind == hir.Optional && subject.Type.Args[0].Equal(value.Type) {
			value = l.optionalView(value, subject.Type)
		}
		cond := &hir.Expr{Kind: hir.Binary, Node: l.node(c), Op: "==", Type: hir.T(hir.Bool), X: subject, Y: value}
		body := hir.B(l.stmtList(clauseStatements(c))...)
		chain = &hir.Stmt{Kind: hir.If, Node: l.node(c), X: cond, Body: body, Else: chain}
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
		// for-in over undefined iterates nothing.
		present := l.tempInit(n, recv.Type, recv)
		keys := l.rtOp("map.keys", &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: recv.Type.Args[0], X: present}, hir.T(hir.Array, hir.T(hir.String)))
		l.push()
		l.declare(name, hir.T(hir.String))
		body := l.scopeBlock(f.Statement)
		l.pop()
		loop := &hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: name, Type: hir.T(hir.String), X: keys, Body: body}
		absent := &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: present}
		return &hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Node: l.node(n), Op: "!", Type: hir.T(hir.Bool), X: absent}, Body: hir.B(loop)}
	}
	if recv == nil || recv.Type.Kind != hir.OrderedMap || recv.Type.Args[0].Kind != hir.String {
		l.diagf(n, "unsupported-statement", "for-in needs a string-keyed map")
		return nil
	}
	keys := l.rtOp("map.keys", recv, hir.T(hir.Array, hir.T(hir.String)))
	l.push()
	l.declare(name, hir.T(hir.String))
	body := l.scopeBlock(f.Statement)
	l.pop()
	return &hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: name, Type: hir.T(hir.String), X: keys, Body: body}
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
