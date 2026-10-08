package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
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
		if r.Expression == nil {
			return &hir.Stmt{Kind: hir.Return, Node: l.node(n)}
		}
		l.hint = l.method.Result
		x := l.expr(r.Expression)
		l.hint = hir.Type{}
		if x == nil {
			return nil
		}
		return &hir.Stmt{Kind: hir.Return, Node: l.node(n), X: x}
	case ast.KindBreakStatement:
		return &hir.Stmt{Kind: hir.Break, Node: l.node(n)}
	case ast.KindContinueStatement:
		return &hir.Stmt{Kind: hir.Continue, Node: l.node(n)}
	case ast.KindEmptyStatement:
		return nil
	case ast.KindThrowStatement, ast.KindTryStatement, ast.KindSwitchStatement, ast.KindForInStatement, ast.KindDoStatement, ast.KindLabeledStatement:
		l.diagf(n, "unsupported-statement", "%s is not lowered", n.Kind.String())
		return nil
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
	if d.Name() == nil || d.Name().Kind != ast.KindIdentifier || d.Symbol() == nil {
		l.diagf(d, "unsupported-binding", "variable declaration with a binding pattern")
		return nil
	}
	name := d.Name().Text()
	sym := d.Symbol()
	init := d.Initializer()
	var typ hir.Type
	var stmts []*hir.Stmt
	var x *hir.Expr
	switch {
	case init != nil && l.isNewCollection(init):
		var expr *hir.Expr
		stmts, expr = l.collectionInit(init)
		if expr == nil {
			return nil
		}
		typ = expr.Type
		x = expr
	case init != nil && init.Kind == ast.KindArrayLiteralExpression:
		var expr *hir.Expr
		stmts, expr = l.arrayLiteral(init)
		if expr == nil || expr.Kind != hir.Local {
			return nil
		}
		typ = expr.Type
		x = expr
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
		if init != nil {
			// The contextual type makes `undefined` literals well typed.
			l.hint = typ
			x = l.expr(init)
			l.hint = hir.Type{}
			if x == nil {
				return nil
			}
		}
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
					Y: l.rtOp("string.concat", target, hir.T(hir.String), l.expr(b.Right))}
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
	l.hint = target.Type
	x := l.expr(rhs)
	l.hint = hir.Type{}
	if x == nil {
		return nil
	}
	return &hir.Stmt{Kind: hir.Assign, Node: l.node(at), X: target, Y: x}
}

// assignTarget lowers the left side of an assignment: locals, `this.x` fields
// and static fields.
func (l *lowerer) assignTarget(lhs *ast.Node) *hir.Expr {
	if lhs == nil {
		return nil
	}
	switch lhs.Kind {
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
				var owner *hir.Class
				if p.Expression.Kind == ast.KindThisKeyword {
					owner = l.class
				} else {
					owner = l.classOf(l.resolve(p.Expression))
				}
				if owner != nil {
					return &hir.Expr{Kind: hir.StaticGet, Node: l.node(lhs), Owner: owner.Name, Name: f.Name, Type: f.Type}
				}
				l.diagf(lhs, "unsupported-expr", "static field receiver is not a class")
				return nil
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
		l.diagf(lhs, "unsupported-assignment", "assignment to property %s is not lowered", lhs.Text())
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
	if f.Incrementor != nil && l.containsContinue(f.Statement) {
		l.diagf(n, "unsupported-statement", "continue in a for loop with an update is not lowered")
		return nil
	}
	l.push()
	body := []*hir.Stmt{}
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
	if len(decls) != 1 || decls[0].Name() == nil || decls[0].Name().Kind != ast.KindIdentifier || decls[0].Symbol() == nil {
		l.diagf(n, "unsupported-statement", "for-of needs a single identifier binding")
		return nil
	}
	name := decls[0].Name().Text()
	elem := l.mapCheckerType(decls[0], l.ck.GetTypeOfSymbol(decls[0].Symbol()))
	x := l.expr(f.Expression)
	if x == nil || elem.Kind == hir.Void {
		return nil
	}
	l.push()
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
