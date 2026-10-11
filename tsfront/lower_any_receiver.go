package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Recover a nominal receiver from a private producer annotated any. Inspect
// every return, excluding nested functions; never infer from a method name or
// the consumer's desired type. Public virtual producers retain their ABI.
func (l *lowerer) anyCallReceiver(n *ast.Node, value *hir.Expr) *hir.Expr {
	if value == nil || value.Type.Kind != hir.Dynamic || n.Kind != ast.KindCallExpression {
		return value
	}
	sym := l.resolve(n.Expression())
	if sym == nil {
		return value
	}
	var declaration *ast.Node
	for _, d := range sym.Declarations {
		if d.Kind == ast.KindMethodDeclaration && d.ModifierFlags()&ast.ModifierFlagsPrivate != 0 && d.Body() != nil {
			declaration = d
			break
		}
	}
	if declaration == nil {
		return value
	}
	before := len(l.diags)
	var result hir.Type
	valid := true
	var visit func(*ast.Node)
	visit = func(node *ast.Node) {
		if node == nil {
			return
		}
		if node != declaration.Body() && (node.Kind == ast.KindFunctionDeclaration || node.Kind == ast.KindFunctionExpression || node.Kind == ast.KindArrowFunction || node.Kind == ast.KindClassDeclaration) {
			return
		}
		if node.Kind == ast.KindReturnStatement {
			x := node.Expression()
			if x == nil || isUndefinedKeyword(x) {
				return
			}
			typ := l.ck.GetTypeAtLocation(x)
			if typ == nil || typ.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
				valid = false
				return
			}
			mapped := l.mapCheckerType(x, typ)
			if mapped.Kind == hir.Optional {
				mapped = mapped.Args[0]
			}
			if mapped.Kind != hir.ClassRef && mapped.Kind != hir.InterfaceRef {
				valid = false
				return
			}
			if result.Kind != "" && !result.Equal(mapped) {
				valid = false
				return
			}
			result = mapped
			return
		}
		node.ForEachChild(func(child *ast.Node) bool { visit(child); return false })
	}
	visit(declaration.Body())
	valid = valid && !hasBlocking(l.diags[before:])
	l.diags = l.diags[:before]
	if !valid || result.Kind == "" {
		return value
	}
	// Preserve an absent return and evaluate the producer exactly once. The
	// checked nominal cast refuses any value outside the proven return domain.
	saved := l.pend
	l.pend = nil
	local := l.tempInit(n, value.Type, value)
	pre := l.pend
	l.pend = saved
	optional := hir.T(hir.Optional, result)
	converted := l.rtOp("dynamic.asRef", local, result)
	test := l.rtOp("dynamic.isNullish", local, hir.T(hir.Bool))
	return &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: optional, Stmt: hir.B(pre...), Y: &hir.Expr{Kind: hir.Conditional, Type: optional, X: test, Y: &hir.Expr{Kind: hir.Lit, Type: optional}, Z: converted}}
}
