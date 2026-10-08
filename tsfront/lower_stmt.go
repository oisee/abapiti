package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
)

// Statement lowering. l.scope holds the declared type of every local in
// scope; reads carry the checker's narrowed type through hir.Narrow.

func (l *lowerer) push() { l.scope = append(l.scope, map[string]hir.Type{}) }
func (l *lowerer) pop()  { l.scope = l.scope[:len(l.scope)-1] }
func (l *lowerer) declare(n string, t hir.Type) {
	if len(l.scope) == 0 {
		l.push()
	}
	l.scope[len(l.scope)-1][n] = t
}

// lookup returns the declared type of a local, searching inner scopes first.
func (l *lowerer) lookup(n string) (hir.Type, bool) {
	for i := len(l.scope) - 1; i >= 0; i-- {
		if t, ok := l.scope[i][n]; ok {
			return t, true
		}
	}
	return hir.Type{}, false
}

func (l *lowerer) block(n *ast.Node) *hir.Stmt {
	if e, ok := l.overrides[n]; ok && e.Body != nil {
		return e.Body()
	}
	l.push()
	s := l.stmt(n)
	l.pop()
	return s
}

// stmts lowers one statement to a list: variable statements contribute their
// declarations to the enclosing scope, everything else is a single entry.
func (l *lowerer) stmts(n *ast.Node) []*hir.Stmt {
	if n != nil && n.Kind == ast.KindVariableStatement {
		list := n.AsVariableStatement().DeclarationList
		if list == nil || list.Kind != ast.KindVariableDeclarationList {
			return nil
		}
		var out []*hir.Stmt
		for _, d := range list.AsVariableDeclarationList().Declarations.Nodes {
			out = append(out, l.varDecl(d)...)
		}
		return out
	}
	s := l.stmt(n)
	if s == nil {
		return nil
	}
	return []*hir.Stmt{s}
}

func (l *lowerer) stmt(n *ast.Node) *hir.Stmt {
	for parent := n.Parent; parent != nil; parent = parent.Parent {
		if e, ok := l.overrides[parent]; ok && len(e.Statements) > 0 {
			span := l.file.Text()[scanner.GetTokenPosOfNode(n, l.file, false):n.End()]
			if build := e.Statements[span]; build != nil {
				l.diagf(n, "note-override", "%s: %s", e.ID, e.Rationale)
				return build()
			}
		}
	}
	switch n.Kind {
	case ast.KindBlock:
		l.push()
		list := []*hir.Stmt{}
		for _, s := range n.AsBlock().Statements.Nodes {
			list = append(list, l.stmts(s)...)
		}
		l.pop()
		return hir.B(list...)
	case ast.KindVariableStatement:
		if list := l.stmts(n); len(list) == 1 {
			return list[0]
		} else if len(list) > 1 {
			return hir.B(list...)
		}
		return nil
	case ast.KindExpressionStatement:
		return l.expressionStatement(n.Expression())
	case ast.KindIfStatement:
		ifs := n.AsIfStatement()
		s := &hir.Stmt{Kind: hir.If, Node: l.node(n), X: l.condition(ifs.Expression)}
		if value, ok := constantBool(s.X); ok {
			// A branch the constant condition never selects is dead code in
			// JavaScript too; it is not lowered.
			l.diagf(n, "note-dead-branch", "if with a constant condition keeps only the live branch")
			var live *hir.Stmt
			if value {
				live = l.scopeBlock(ifs.ThenStatement)
			} else if ifs.ElseStatement != nil {
				live = l.scopeBlock(ifs.ElseStatement)
			}
			if s.X.Kind == hir.Seq {
				return hir.B(s.X.Stmt, live)
			}
			return live
		}
		s.Body = l.scopeBlock(ifs.ThenStatement)
		if ifs.ElseStatement != nil {
			s.Else = l.scopeBlock(ifs.ElseStatement)
		}
		return s
	case ast.KindWhileStatement:
		w := n.AsWhileStatement()
		return &hir.Stmt{Kind: hir.While, Node: l.node(n), X: l.condition(w.Expression), Body: l.scopeBlock(w.Statement)}
	case ast.KindForStatement:
		return l.forStatement(n)
	case ast.KindForOfStatement:
		return l.forOfStatement(n)
	case ast.KindReturnStatement:
		r := n.AsReturnStatement()
		if r.Expression == nil || (l.method != nil && l.method.Result.Kind == hir.Void && isUndefinedKeyword(r.Expression)) {
			if l.method != nil && l.method.Result.Kind == hir.Optional {
				return &hir.Stmt{Kind: hir.Return, Node: l.node(n), X: &hir.Expr{Kind: hir.Lit, Type: l.method.Result}}
			}
			return &hir.Stmt{Kind: hir.Return, Node: l.node(n)}
		}
		l.hint = l.method.Result
		x := l.expr(r.Expression)
		l.hint = hir.Type{}
		if x == nil {
			return nil
		}
		return &hir.Stmt{Kind: hir.Return, Node: l.node(n), X: l.coerce(x, l.method.Result)}
	case ast.KindBreakStatement:
		if flag, ok := l.breakViaFlag[n]; ok {
			return hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: flag, Y: hir.L(hir.T(hir.Bool), true)}, &hir.Stmt{Kind: hir.Break, Node: l.node(n)})
		}
		return &hir.Stmt{Kind: hir.Break, Node: l.node(n)}
	case ast.KindContinueStatement:
		if l.continueAsBreak[n] {
			return &hir.Stmt{Kind: hir.Break, Node: l.node(n)}
		}
		return &hir.Stmt{Kind: hir.Continue, Node: l.node(n)}
	case ast.KindEmptyStatement:
		return nil
	case ast.KindThrowStatement:
		t := n.AsThrowStatement()
		if t.Expression == nil {
			l.diagf(n, "unsupported-statement", "rethrow without an expression")
			return nil
		}
		l.hint = hir.Type{}
		x := l.expr(t.Expression)
		if x == nil {
			return nil
		}
		return &hir.Stmt{Kind: hir.Throw, Node: l.node(n), X: x}
	case ast.KindTryStatement:
		return l.tryStatement(n)
	case ast.KindSwitchStatement:
		return l.switchStatement(n)
	case ast.KindForInStatement:
		return l.forInStatement(n)
	}
	l.diagf(n, "unsupported-statement", "%s is not lowered", n.Kind.String())
	return nil
}

func (l *lowerer) scopeBlock(n *ast.Node) *hir.Stmt {
	if n == nil {
		return nil
	}
	if n.Kind == ast.KindBlock {
		return l.stmt(n) // stmt(Block) manages the scope itself
	}
	l.push()
	s := l.stmt(n)
	l.pop()
	return s
}

// varDecl lowers one variable declaration to one or more statements (a
// collection initializer contributes its construction statements); the
// declaration itself lands in the enclosing scope, never in a nested block.
// Destructuring is rejected.
func (l *lowerer) varDecl(d *ast.Node) []*hir.Stmt {
	if d.Name() != nil && (d.Name().Kind == ast.KindObjectBindingPattern || d.Name().Kind == ast.KindArrayBindingPattern) {
		return l.destructureDecl(d)
	}
	if d.Name() == nil || d.Name().Kind != ast.KindIdentifier || d.Symbol() == nil {
		l.diagf(d, "unsupported-binding", "variable declaration with a binding pattern")
		return nil
	}
	name := d.Name().Text()
	sym := d.Symbol()
	init := d.Initializer()
	if init != nil && (init.Kind == ast.KindArrowFunction || init.Kind == ast.KindFunctionExpression) {
		if _, lifted := l.pendingFns[name]; lifted {
			// Lifted to a method; calls resolve through localFns.
			l.declare(name, hir.T(hir.Void))
			return nil
		}
	}
	var typ hir.Type
	var stmts []*hir.Stmt
	var x *hir.Expr
	switch {
	case init != nil && (d.Type() == nil || d.Type().Kind == ast.KindAnyKeyword) && l.isNamespaceValue(init):
		x, _ = l.namespaceRef(init)
		typ = x.Type
		l.diagf(d, "note-any-namespace-alias", "namespace alias retains its registry type")
	case init != nil && l.isNewCollection(init):
		var expr *hir.Expr
		stmts, expr = l.collectionInit(init)
		if expr == nil {
			return nil
		}
		typ = expr.Type
		x = expr
	case init != nil && init.Kind == ast.KindArrayLiteralExpression && d.Type() == nil:
		typ = l.mapCheckerType(d, l.ck.GetTypeAtLocation(d))
		if typ.Kind == hir.Void {
			return nil
		}
		l.hint = typ
		x = l.expr(init)
		l.hint = hir.Type{}
		if x == nil {
			return nil
		}
		typ = x.Type
	default:
		switch {
		case d.Type() != nil:
			typ = l.mapTypeNode(d.Type())
		case init != nil:
			typ = l.mapCheckerType(d, l.ck.GetTypeOfSymbol(sym))
		case sym != nil:
			typ = l.mapCheckerType(d, l.ck.GetTypeOfSymbol(sym))
		default:
			l.diagf(d, "unsupported-binding", "variable %s has no type", name)
			return nil
		}
		if typ.Kind == hir.Void {
			return nil
		}
		if typ.Kind != hir.Optional && init != nil {
			// An initializer that may be undefined makes the local optional.
			if it := l.ck.GetTypeAtLocation(init); it != nil && it.IsUnion() {
				optional := false
				for _, c := range it.AsUnionOrIntersectionType().Types() {
					if c.Flags()&checker.TypeFlagsUndefined != 0 {
						optional = true
					}
				}
				if optional {
					typ = hir.T(hir.Optional, typ)
				}
			}
		}
		if d.Type() == nil && init != nil && d.Parent != nil && d.Parent.Flags&ast.NodeFlagsConst == 0 {
			typ = l.widenLet(d, sym, typ)
		}
		if init != nil {
			// The contextual type makes `undefined` literals well typed.
			l.hint = typ
			x = l.expr(init)
			l.hint = hir.Type{}
			if x == nil {
				return nil
			}
			if d.Type() == nil && l.ck.GetTypeOfSymbol(sym).Flags()&checker.TypeFlagsAny != 0 {
				typ = x.Type
			}
			// An initializer that yields an optional (map lookups, optional
			// chains) makes the local optional even when the declared type
			// does not spell it out.
			if x.Type.Kind == hir.Optional && typ.Kind != hir.Optional {
				typ = x.Type
			}
		}
	}
	if x != nil {
		x = l.coerce(x, typ)
	}
	decl := &hir.Stmt{Kind: hir.VarDecl, Node: l.node(d), Name: name, Type: typ, X: x}
	l.declare(name, typ)
	return append(stmts, decl)
}

// arrayLiteral lowers [a, b, ...] in a statement context into a fresh array
// plus pushes; the element type is the checker type of the first element.
func (l *lowerer) arrayLiteral(n *ast.Node) ([]*hir.Stmt, *hir.Expr) {
	els := n.AsArrayLiteralExpression().Elements.Nodes
	if len(els) == 0 {
		l.diagf(n, "unsupported-array", "empty array literal needs a contextual type")
		return nil, nil
	}
	elem := l.mapCheckerType(n, l.ck.GetTypeAtLocation(els[0]))
	if elem.Kind == hir.Void {
		return nil, nil
	}
	typ := hir.T(hir.Array, elem)
	l.serial++
	tmp := "a" + itoa(l.serial)
	l.declare(tmp, typ)
	stmts := []*hir.Stmt{{Kind: hir.VarDecl, Node: l.node(n), Name: tmp, Type: typ,
		X: &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ}}}
	_ = tmp
	for _, el := range els {
		v := l.expr(el)
		if v == nil {
			return nil, nil
		}
		stmts = append(stmts, &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(el),
			X: l.rtOp("array.push", hir.V(tmp, typ), hir.T(hir.I32), v)})
	}
	return stmts, hir.V(tmp, typ)
}

// expressionStatement handles assignments (including compound) and calls.
func (l *lowerer) expressionStatement(n *ast.Node) *hir.Stmt {
	if n == nil {
		return nil
	}
	if n.Kind == ast.KindPostfixUnaryExpression {
		u := n.AsPostfixUnaryExpression()
		if u.Operator == ast.KindPlusPlusToken || u.Operator == ast.KindMinusMinusToken {
			target := l.assignTarget(u.Operand)
			if target == nil {
				return nil
			}
			op := "+"
			if u.Operator == ast.KindMinusMinusToken {
				op = "-"
			}
			return &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: target, Y: &hir.Expr{Kind: hir.Binary, Node: l.node(n), Type: target.Type, Op: op, X: target, Y: hir.L(target.Type, 1)}}
		}
	}
	if n.Kind == ast.KindBinaryExpression {
		b := n.AsBinaryExpression()
		switch b.OperatorToken.Kind {
		case ast.KindEqualsToken:
			return l.assignment(n, b.Left, b.Right)
		case ast.KindPlusEqualsToken, ast.KindMinusEqualsToken, ast.KindAsteriskEqualsToken:
			op := map[ast.Kind]string{
				ast.KindPlusEqualsToken: "+", ast.KindMinusEqualsToken: "-", ast.KindAsteriskEqualsToken: "*"}[b.OperatorToken.Kind]
			target := l.assignTarget(b.Left)
			if target == nil {
				return nil
			}
			// `x += s` on strings concatenates.
			if l.ck.GetTypeAtLocation(b.Left).IsStringLike() {
				return &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: target,
					Y: l.rtOp("string.concat", l.expr(b.Left), hir.T(hir.String), l.expr(b.Right))}
			}
			return &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: target,
				Y: &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: op, Type: target.Type,
					X: target, Y: l.expr(b.Right)}}
		}
	}
	x := l.expr(n)
	if x == nil {
		return nil
	}
	return &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: x}
}

func (l *lowerer) assignment(at *ast.Node, lhs, rhs *ast.Node) *hir.Stmt {
	target := l.assignTarget(lhs)
	if target == nil {
		return nil
	}
	if target.Kind == hir.RuntimeOp && target.Op == "map.set" {
		// m[k] = v lowers to map.set(m, k, v).
		l.hint = target.X.Type.Args[1]
		v := l.expr(rhs)
		l.hint = hir.Type{}
		if v == nil {
			return nil
		}
		set := l.rtOp("map.set", target.X, target.X.Type, target.Args[0], v)
		return &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(at), X: set}
	}
	l.hint = target.Type
	x := l.expr(rhs)
	l.hint = hir.Type{}
	if x == nil {
		return nil
	}
	x = l.coerce(x, target.Type)
	return &hir.Stmt{Kind: hir.Assign, Node: l.node(at), X: target, Y: x}
}

// assignTarget lowers the left side of an assignment: locals, `this.x`
// fields, static fields, map slots and array indexes (including a field of
// an indexed element).
func (l *lowerer) assignTarget(lhs *ast.Node) *hir.Expr {
	if lhs == nil {
		return nil
	}
	switch lhs.Kind {
	case ast.KindElementAccessExpression:
		e := lhs.AsElementAccessExpression()
		recv := l.expr(e.Expression)
		if recv == nil {
			return nil
		}
		arg := l.expr(e.ArgumentExpression)
		if arg == nil {
			return nil
		}
		switch recv.Type.Kind {
		case hir.OrderedMap:
			return &hir.Expr{Kind: hir.RuntimeOp, Node: l.node(lhs), Op: "map.set", Type: recv.Type,
				X: recv, Args: []*hir.Expr{arg, nil}} // filled by the assignment lowering
		case hir.Array:
			return &hir.Expr{Kind: hir.IndexGet, Node: l.node(lhs), Type: recv.Type.Args[0], X: recv, Y: l.indexValue(arg)}
		}
		l.diagf(lhs, "unsupported-assignment", "element assignment on %s is not lowered", recv.Type.Kind)
		return nil
	case ast.KindIdentifier:
		name := lhs.Text()
		if t, ok := l.lookup(name); ok {
			return hir.V(name, t)
		}
		if sym := l.resolve(lhs); sym != nil {
			if owner, fieldName, ok := l.modvarOf(sym); ok {
				for _, mod := range l.out.Classes {
					if mod.Name != owner {
						continue
					}
					for _, f := range mod.Fields {
						if f.Name == fieldName {
							return &hir.Expr{Kind: hir.StaticGet, Node: l.node(lhs), Owner: mod.Name, Name: fieldName, Type: f.Type}
						}
					}
				}
			}
		}
		l.diagf(lhs, "unsupported-assignment", "assignment to %s is not lowered", name)
	case ast.KindPropertyAccessExpression:
		p := lhs.AsPropertyAccessExpression()
		sym := l.resolve(lhs)
		f, hasField := l.fieldOf(sym)
		if sym != nil && hasField {
			if f.Static {
				owner := l.classOf(sym.Parent)
				if p.Expression.Kind == ast.KindThisKeyword {
					owner = l.class
				}
				if owner != nil {
					return &hir.Expr{Kind: hir.StaticGet, Node: l.node(lhs), Owner: owner.Name, Name: f.Name, Type: f.Type}
				}
			}
			if p.Expression.Kind == ast.KindThisKeyword {
				return &hir.Expr{Kind: hir.FieldGet, Node: l.node(lhs), Name: f.Name, Type: f.Type, X: l.this(l.class)}
			}
			recv := l.expr(p.Expression)
			if recv == nil {
				return nil
			}
			return &hir.Expr{Kind: hir.FieldGet, Node: l.node(lhs), Name: f.Name, Type: f.Type, X: recv}
		}
		l.diagf(lhs, "unsupported-assignment", "assignment to property %s is not lowered", p.Name().Text())
	default:
		l.diagf(lhs, "unsupported-assignment", "assignment target %s is not lowered", lhs.Kind.String())
	}
	return nil
}

// forStatement lowers a classic for loop into initializer statements plus a
// while loop whose body ends with the update. `continue` inside a loop with
// an update would skip it, so that combination is rejected.
func (l *lowerer) forStatement(n *ast.Node) *hir.Stmt {
	f := n.AsForStatement()
	out := []*hir.Stmt{}
	if f.Initializer != nil {
		if f.Initializer.Kind == ast.KindVariableDeclarationList {
			for _, d := range f.Initializer.AsVariableDeclarationList().Declarations.Nodes {
				out = append(out, l.varDecl(d)...)
			}
		} else {
			out = append(out, l.expressionStatement(f.Initializer))
		}
	}
	cond := &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.Bool), Value: true}
	if f.Condition != nil {
		cond = l.condition(f.Condition)
	}
	l.push()
	body := []*hir.Stmt{}
	if f.Incrementor != nil && l.containsContinue(f.Statement) {
		wrapped := l.forBodyWithContinue(n, f.Statement, f.Incrementor)
		l.pop()
		if wrapped == nil {
			return nil
		}
		out = append(out, &hir.Stmt{Kind: hir.While, Node: l.node(n), X: cond, Body: wrapped})
		return hir.B(out...)
	}
	if f.Statement != nil {
		body = append(body, l.scopeBlock(f.Statement))
	}
	if f.Incrementor != nil {
		body = append(body, l.expressionStatement(f.Incrementor))
	}
	l.pop()
	out = append(out, &hir.Stmt{Kind: hir.While, Node: l.node(n), X: cond, Body: hir.B(body...)})
	return hir.B(out...)
}

func (l *lowerer) forOfStatement(n *ast.Node) *hir.Stmt {
	f := n.AsForInOrOfStatement()
	if f.Initializer == nil || f.Initializer.Kind != ast.KindVariableDeclarationList {
		l.diagf(n, "unsupported-statement", "for-of needs a variable declaration")
		return nil
	}
	decls := f.Initializer.AsVariableDeclarationList().Declarations.Nodes
	if len(decls) != 1 || decls[0].Name() == nil {
		l.diagf(n, "unsupported-statement", "for-of needs a single binding")
		return nil
	}
	var elemT *checker.Type
	if sym := decls[0].Symbol(); sym != nil {
		elemT = l.ck.GetTypeOfSymbol(sym)
	}
	if elemT == nil {
		elemT = l.ck.GetTypeAtLocation(decls[0])
	}
	if elemT == nil {
		l.diagf(n, "unsupported-statement", "for-of binding without a type")
		return nil
	}
	elem := l.mapCheckerType(decls[0], elemT)
	x := l.expr(f.Expression)
	if x == nil {
		return nil
	}
	if x.Type.Kind == hir.Array {
		elem = x.Type.Args[0]
	}
	if elem.Kind == hir.Void {
		return nil
	}
	l.push()
	if decls[0].Name().Kind != ast.KindIdentifier {
		// A destructuring binding: loop over a fresh name, then declare the
		// pattern's fields from it.
		l.serial++
		name := "f" + itoa(l.serial)
		l.destructurePattern(decls[0].Name(), hir.V(name, elem))
		body := l.scopeBlock(f.Statement)
		l.pop()
		// The pattern's field declarations were appended to the prelude;
		// splice them into the loop body head.
		pre := l.pend
		l.pend = nil
		return &hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: name, Type: elem, X: x,
			Body: hir.B(append(pre, body)...)}
	}
	name := decls[0].Name().Text()
	l.declare(name, elem)
	body := l.scopeBlock(f.Statement)
	l.pop()
	return &hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: name, Type: elem, X: x, Body: body}
}

func (l *lowerer) containsContinue(n *ast.Node) bool {
	found := false
	var walk func(*ast.Node)
	walk = func(x *ast.Node) {
		if x == nil || found {
			return
		}
		if x.Kind == ast.KindContinueStatement {
			found = true
			return
		}
		if x.Kind == ast.KindFunctionExpression || x.Kind == ast.KindArrowFunction || x.Kind == ast.KindFunctionDeclaration {
			return
		}
		x.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(n)
	return found
}

// fileOfSymbol locates the source file that declares a module symbol.
func (l *lowerer) fileOfSymbol(sym *ast.Symbol) *ast.SourceFile {
	decl := sym.ValueDeclaration
	if decl == nil && len(sym.Declarations) > 0 {
		decl = sym.Declarations[0]
	}
	if decl == nil {
		return nil
	}
	return ast.GetSourceFileOfNode(decl)
}

func (l *lowerer) isNamespaceValue(n *ast.Node) bool { _, ok := l.namespaceRef(n); return ok }
