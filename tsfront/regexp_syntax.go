package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// An unclosed character class is definitely invalid ECMAScript syntax. Raise
// its source SyntaxError before entering the checked regex language boundary.
func (l *lowerer) guardRegexpSyntax(n *ast.Node, args []*hir.Expr) {
	if !l.unprunedFeatures || args[1].Kind != hir.Lit {
		return
	}
	if args[0].Kind == hir.Lit {
		text, ok := args[0].Value.(string)
		if !ok {
			return
		}
		inClass, escaped := false, false
		for _, c := range text {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
			} else if c == '[' {
				inClass = true
			} else if c == ']' {
				inClass = false
			}
		}
		if !inClass {
			return
		}
	}
	str, integer, boolean := hir.T(hir.String), hir.T(hir.I32), hir.T(hir.Bool)
	pattern := l.tempInit(n, str, args[0])
	args[0] = pattern
	index := l.tempInit(n, integer, hir.L(integer, 0))
	inClass := l.tempInit(n, boolean, hir.L(boolean, false))
	escaped := l.tempInit(n, boolean, hir.L(boolean, false))
	char := l.tempInit(n, str, hir.L(str, ""))
	assign := func(x, y *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: x, Y: y} }
	equal := func(s string) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "==", Type: boolean, X: char, Y: hir.L(str, s)}
	}
	step := &hir.Stmt{Kind: hir.If, Node: l.node(n), X: escaped, Body: hir.B(assign(escaped, hir.L(boolean, false))), Else: hir.B(
		&hir.Stmt{Kind: hir.If, Node: l.node(n), X: equal("\\"), Body: hir.B(assign(escaped, hir.L(boolean, true))), Else: hir.B(
			&hir.Stmt{Kind: hir.If, Node: l.node(n), X: equal("["), Body: hir.B(assign(inClass, hir.L(boolean, true))), Else: hir.B(
				&hir.Stmt{Kind: hir.If, Node: l.node(n), X: equal("]"), Body: hir.B(assign(inClass, hir.L(boolean, false)))})})})}
	cond := &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "<", Type: boolean, X: index, Y: l.rtOp("string.length", pattern, integer)}
	next := &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "+", Type: integer, X: index, Y: hir.L(integer, 1)}
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: cond, Body: hir.B(assign(char, l.rtOp("string.charAt", pattern, str, index)), step, assign(index, next))})
	l.builtinError()
	message := l.rtOp("string.concat", hir.L(str, "Invalid regular expression: /"), str, pattern)
	message = l.rtOp("string.concat", message, str, hir.L(str, "/"))
	message = l.rtOp("string.concat", message, str, args[1])
	message = l.rtOp("string.concat", message, str, hir.L(str, ": Unterminated character class"))
	errorValue := &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref("builtin.SyntaxError"), Args: []*hir.Expr{l.coerce(message, hir.T(hir.Optional, str))}}
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: inClass, Body: hir.B(&hir.Stmt{Kind: hir.Throw, Node: l.node(n), X: errorValue})})
}
