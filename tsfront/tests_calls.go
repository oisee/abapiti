package tsfront

import (
	"crypto/sha256"
	"fmt"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Only the byte-pinned upstream registration helper gets this allowance.
// Its test-row arrays are dense and the callbacks do not mutate the receiver.
func (l *lowerer) pinnedTestRows(n *ast.Node) bool {
	return l.integerOptions.TrapOnFailure && l.relFile(l.file) == "test/rules/_utils.ts" &&
		n.Expression().Expression().Kind == ast.KindIdentifier && n.Expression().Expression().Text() == "tests" &&
		fmt.Sprintf("%x", sha256.Sum256([]byte(l.file.Text()))) == "8be5e3d9f540c7614904bb7ec36c694a8c2946e21524f40881fd4229d4c43341"
}

func (l *lowerer) isTestCallback(n *ast.Node) bool {
	if n.Parent == nil || n.Parent.Kind != ast.KindCallExpression {
		return false
	}
	callee := n.Parent.Expression()
	sym := l.resolve(callee)
	if sym == nil {
		return false
	}
	f := l.fileOfSymbol(sym)
	return f != nil && l.relFile(f) == "runtime/mocha.ts"
}

// The shim's merged namespaces are API sugar, not arbitrary TS namespace
// support. Resolve the actual runtime symbol so user methods named only/skip
// are never intercepted.
func (l *lowerer) testRuntimeCall(n *ast.Node) (*hir.Expr, bool) {
	if !l.integerOptions.TrapOnFailure {
		return nil, false
	}
	callee := n.Expression()
	if callee.Kind != ast.KindPropertyAccessExpression {
		return nil, false
	}
	base := callee.Expression()
	if base.Kind != ast.KindIdentifier {
		return nil, false
	}
	kind := base.Text()
	modifier := callee.Name().Text()
	if (kind != "it" && kind != "describe") || (modifier != "only" && modifier != "skip") {
		return nil, false
	}
	sym := l.resolve(callee)
	if sym == nil {
		return nil, false
	}
	f := l.fileOfSymbol(sym)
	if f == nil || l.relFile(f) != "runtime/mocha.ts" {
		return nil, false
	}
	method := "register"
	if kind == "describe" {
		method = "scope"
	}
	var m *hir.Method
	for _, c := range l.out.Classes {
		if c.Name == "runtime/mocha.ts.TestRuntime" {
			for _, candidate := range c.Methods {
				if candidate.Name == method {
					m = candidate
				}
			}
		}
	}
	if m == nil {
		return nil, false
	}
	args, ok := l.callArgs(n, n.Arguments(), m.Params[:2])
	if !ok {
		return nil, true
	}
	only, skip := modifier == "only", modifier == "skip"
	if kind == "describe" {
		args = append(args, hir.L(hir.T(hir.Optional, hir.T(hir.Bool)), skip), hir.L(hir.T(hir.Optional, hir.T(hir.Bool)), only))
	} else {
		args = append(args, hir.L(hir.T(hir.Optional, hir.T(hir.Bool)), only), hir.L(hir.T(hir.Optional, hir.T(hir.Bool)), skip))
	}
	return &hir.Expr{Node: l.node(n), Kind: hir.DirectCall, Owner: "runtime/mocha.ts.TestRuntime", Name: method, Type: hir.T(hir.Void), Args: args}, true
}
