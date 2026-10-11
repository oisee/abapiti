package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Labels are lowered to ordinary structured HIR. A jump out of nested loops
// sets a completion flag; each intervening loop propagates it immediately.
// This keeps ABAP EXIT/CONTINUE and Go break/continue on the same path.
func (l *lowerer) labeledStatement(n *ast.Node) *hir.Stmt {
	labeled := n.AsLabeledStatement()
	target := labeled.Statement
	switch target.Kind {
	case ast.KindForStatement, ast.KindForOfStatement, ast.KindWhileStatement:
	default:
		l.diagf(n, "unsupported-statement", "label requires a loop")
		return nil
	}
	flag := l.tempInit(n, hir.T(hir.I32), hir.L(hir.T(hir.I32), 0))
	if l.labeledControl == nil {
		l.labeledControl = map[*ast.Node]*hir.Stmt{}
	}
	var jumps []*ast.Node
	var walk func(*ast.Node, int)
	walk = func(x *ast.Node, depth int) {
		if x == nil {
			return
		}
		var label *ast.Node
		kind := hir.Break
		value := 1
		switch x.Kind {
		case ast.KindBreakStatement:
			label = x.AsBreakStatement().Label
		case ast.KindContinueStatement:
			label = x.AsContinueStatement().Label
			kind = hir.Continue
			value = 2
		case ast.KindFunctionDeclaration, ast.KindFunctionExpression, ast.KindArrowFunction:
			return
		case ast.KindForStatement, ast.KindForOfStatement, ast.KindForInStatement, ast.KindWhileStatement, ast.KindDoStatement:
			if x != target {
				depth++
			}
		}
		if label != nil && label.Text() == labeled.Label.Text() {
			control := &hir.Stmt{Kind: kind, Node: l.node(x)}
			if depth > 0 {
				control = hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(x), X: flag, Y: hir.L(flag.Type, value)}, &hir.Stmt{Kind: hir.Break, Node: l.node(x)})
			}
			l.labeledControl[x] = control
			jumps = append(jumps, x)
			return
		}
		x.ForEachChild(func(c *ast.Node) bool { walk(c, depth); return false })
	}
	walk(target, 0)
	lowered := l.stmt(target)
	for _, jump := range jumps {
		delete(l.labeledControl, jump)
	}
	loop := lowered
	if loop != nil && loop.Kind == hir.Block && len(loop.List) > 0 {
		loop = loop.List[len(loop.List)-1]
	}
	if loop == nil || (loop.Kind != hir.While && loop.Kind != hir.ForEach) {
		return lowered
	}
	var update *hir.Stmt
	if target.Kind == ast.KindForStatement && target.AsForStatement().Incrementor != nil && loop.Body != nil && len(loop.Body.List) > 0 {
		update = loop.Body.List[len(loop.Body.List)-1]
	}
	test := func(value int) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Type: hir.T(hir.Bool), Op: "==", X: flag, Y: hir.L(flag.Type, value)}
	}
	var rewrite func(*hir.Stmt, int) *hir.Stmt
	rewrite = func(s *hir.Stmt, depth int) *hir.Stmt {
		if s == nil {
			return nil
		}
		if s.Kind == hir.While || s.Kind == hir.ForEach {
			s.Body = rewrite(s.Body, depth+1)
			var guard *hir.Stmt
			if depth == 0 {
				cont := hir.B(&hir.Stmt{Kind: hir.Assign, X: flag, Y: hir.L(flag.Type, 0)}, &hir.Stmt{Kind: hir.Continue})
				cont = continueWithUpdate(cont, update)
				guard = hir.B(&hir.Stmt{Kind: hir.If, X: test(1), Body: hir.B(&hir.Stmt{Kind: hir.Break})}, &hir.Stmt{Kind: hir.If, X: test(2), Body: cont})
			} else {
				guard = &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Binary, Type: hir.T(hir.Bool), Op: "!=", X: flag, Y: hir.L(flag.Type, 0)}, Body: hir.B(&hir.Stmt{Kind: hir.Break})}
			}
			return hir.B(s, guard)
		}
		s.Body = rewrite(s.Body, depth)
		s.Else = rewrite(s.Else, depth)
		for i, child := range s.List {
			s.List[i] = rewrite(child, depth)
		}
		return s
	}
	loop.Body = rewrite(loop.Body, 0)
	return lowered
}
