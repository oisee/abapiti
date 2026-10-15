package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// HIR Finally has exception-only exits. Stage a return's value in an outer
// local, finish the region, then return it. A finally throw naturally bypasses
// the staged return. Guard only the remaining suffix, retaining local scopes.
func (l *lowerer) returningFinally(n *ast.Node) *hir.Stmt {
	t := n.AsTryStatement()
	if l.containsLoopControl(t.TryBlock) || returnInLoop(t.TryBlock, false) {
		l.diagf(n, "unsupported-statement", "finally with loop exits in the try block is not lowered")
		return nil
	}
	body := l.scopeBlock(t.TryBlock)
	final := l.scopeBlock(t.FinallyBlock)
	l.serial++
	suffix := itoa(l.serial)
	done := hir.V("finallyReturned"+suffix, hir.T(hir.Bool))
	value := hir.V("finallyValue"+suffix, l.method.Result)
	falls := hir.CanFallThrough(body)
	body, _ = stageFinallyReturns(body, done, value)
	out := []*hir.Stmt{{Kind: hir.VarDecl, Node: l.node(n), Name: done.Name, Type: done.Type, X: hir.L(done.Type, false)}}
	if value.Type.Kind != hir.Void {
		out = append(out, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: value.Name, Type: value.Type})
	}
	out = append(out, &hir.Stmt{Kind: hir.Finally, Node: l.node(n), Body: body, Else: final})
	ret := &hir.Stmt{Kind: hir.Return, Node: l.node(n)}
	if value.Type.Kind != hir.Void {
		ret.X = value
	}
	if falls {
		out = append(out, &hir.Stmt{Kind: hir.If, Node: l.node(n), X: done, Body: hir.B(ret)})
	} else {
		out = append(out, ret)
	}
	return hir.B(out...)
}
func returnInLoop(n *ast.Node, loop bool) bool {
	if n == nil {
		return false
	}
	if ast.IsFunctionLike(n) {
		return false
	}
	switch n.Kind {
	case ast.KindForStatement, ast.KindForOfStatement, ast.KindForInStatement, ast.KindWhileStatement, ast.KindDoStatement:
		loop = true
	case ast.KindReturnStatement:
		return loop
	}
	found := false
	n.ForEachChild(func(c *ast.Node) bool { found = returnInLoop(c, loop); return found })
	return found
}
func stageFinallyReturns(s *hir.Stmt, done, value *hir.Expr) (*hir.Stmt, bool) {
	if s == nil {
		return nil, false
	}
	if s.Kind == hir.Return {
		var out []*hir.Stmt
		if s.X != nil {
			out = append(out, &hir.Stmt{Kind: hir.Assign, Node: s.Node, X: value, Y: s.X})
		}
		out = append(out, &hir.Stmt{Kind: hir.Assign, Node: s.Node, X: done, Y: hir.L(done.Type, true)})
		return hir.B(out...), true
	}
	y := *s
	var a, b bool
	y.Body, a = stageFinallyReturns(s.Body, done, value)
	y.Else, b = stageFinallyReturns(s.Else, done, value)
	changed := a || b
	if len(s.List) > 0 {
		var tail []*hir.Stmt
		for i := len(s.List) - 1; i >= 0; i-- {
			next, returns := stageFinallyReturns(s.List[i], done, value)
			if returns && len(tail) > 0 {
				tail = []*hir.Stmt{{Kind: hir.If, Node: s.Node, X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: done.Type, X: done}, Body: hir.B(tail...)}}
			}
			tail = append([]*hir.Stmt{next}, tail...)
			changed = changed || returns
		}
		y.List = tail
	}
	return &y, changed
}
