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
	if !l.integerOptions.TrapOnFailure || n.Expression().Expression().Kind != ast.KindIdentifier {
		return false
	}
	pins := map[string]string{
		"test/rules/_utils.ts":        "8be5e3d9f540c7614904bb7ec36c694a8c2946e21524f40881fd4229d4c43341",
		"test/rules/method_length.ts": "d125afed5be8cd022ba0c1ce89783ffea4ee8c37139d1f08fe9b3aabb42f658e",
		"test/abap/_utils.ts":         "47db2783b0c9dbabe3e62461ac53f07809ff1544718c23652b9a0ee346ca3f2d",
		"test/abap/lexer/count.ts":    "4f04d8d8c6f58d522d5f4b757eca33814e9bd248e9d1ca0d42209f8a4f6b9c9c",
		"test/abap/lexer/type.ts":     "80f81847a173ad3d38e2d47a15dc4e92d43356a930500633519a4574b3e4a737",
	}
	name := n.Expression().Expression().Text()
	return (name == "tests" || name == "cases") &&
		fmt.Sprintf("%x", sha256.Sum256([]byte(l.file.Text()))) == pins[l.relFile(l.file)]
}

// The pinned helper only reads these dense arrays; refine its otherwise-any
// registration parameters, not arbitrary user/core any values.
func (l *lowerer) testParameterType(method, param string) (hir.Type, bool) {
	if !l.integerOptions.TrapOnFailure || param != "tests" {
		return hir.Type{}, false
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(l.file.Text())))
	if l.relFile(l.file) == "test/abap/_utils.ts" && hash == "47db2783b0c9dbabe3e62461ac53f07809ff1544718c23652b9a0ee346ca3f2d" {
		if method == "statementType" {
			return hir.T(hir.Array, hir.T(hir.String)), true
		}
		if method == "statementVersion" {
			for _, s := range l.file.Statements.Nodes {
				if s.Kind == ast.KindFunctionDeclaration && s.Name().Text() == "statementVersionOk" {
					return l.mapTypeNode(s.Parameters()[0].Type()), true
				}
			}
		}
	}
	return hir.Type{}, false
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

// Only local helpers immediately enclosed by an original Mocha describe
// callback receive receiver-free static lowering. This does not recover an
// escaping callable or change captures in nested test bodies.
func (l *lowerer) testRegistrationHelper(fn *ast.Node) bool {
	if !l.integerOptions.TrapOnFailure {
		return false
	}
	for p := fn.Parent; p != nil; p = p.Parent {
		switch p.Kind {
		case ast.KindArrowFunction, ast.KindFunctionExpression, ast.KindFunctionDeclaration, ast.KindMethodDeclaration, ast.KindConstructor:
			return l.isTestCallback(p) && p.Parent.Expression().Kind == ast.KindIdentifier && p.Parent.Expression().Text() == "describe"
		}
	}
	return false
}

// Mocha ignores describe's return value. An async suite containing only
// synchronous registration calls and inert constants has no suspension; its
// async it callbacks are separately certified and drained by the test host.
func (l *lowerer) synchronousAsyncSuite(n *ast.Node) bool {
	if !l.isTestCallback(n) || n.Parent.Expression().Kind != ast.KindIdentifier || n.Parent.Expression().Text() != "describe" || n.Body() == nil || n.Body().Kind != ast.KindBlock {
		return false
	}
	for _, stmt := range n.Body().Statements() {
		if stmt.Kind == ast.KindVariableStatement && stmt.AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsConst != 0 {
			for _, d := range stmt.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
				if !trivialInitializer(d.Initializer()) {
					return false
				}
			}
			continue
		}
		if stmt.Kind != ast.KindExpressionStatement || stmt.Expression().Kind != ast.KindCallExpression {
			return false
		}
		callee := stmt.Expression().Expression()
		if callee.Kind != ast.KindIdentifier || (callee.Text() != "it" && callee.Text() != "describe") {
			return false
		}
		if sym := l.resolve(callee); sym == nil {
			return false
		} else if f := l.fileOfSymbol(sym); f == nil || l.relFile(f) != "runtime/mocha.ts" {
			return false
		}
	}
	return true
}

// The shim's merged namespaces are API sugar, not arbitrary TS namespace
// support. Resolve the actual runtime symbol so user methods named only/skip
// are never intercepted.
func (l *lowerer) testRuntimeCall(n *ast.Node) (*hir.Expr, bool) {
	if !l.integerOptions.TrapOnFailure {
		return nil, false
	}
	callee := n.Expression()
	if sym := l.resolve(callee); sym != nil && sym.Name == "chaiThrowMatch" {
		if f := l.fileOfSymbol(sym); f != nil && l.relFile(f) == "runtime/chai.ts" {
			ref := l.funcsBy[f.FileName()+" chaiThrowMatch"]
			if m := l.methodsBy[ref.owner+"."+ref.method]; m != nil {
				args, ok := l.callArgs(n, n.Arguments(), m.Params)
				if !ok {
					return nil, true
				}
				return l.rtOp("chai.throwMatch", args[0], hir.T(hir.Bool), args[1]), true
			}
		}
	}
	if sym := l.resolve(callee); sym != nil {
		if f := l.fileOfSymbol(sym); f != nil && l.relFile(f) == "runtime/chai.ts" && (sym.Name == "chaiDeepEqual" || sym.Name == "chaiDeepInclude" || sym.Name == "chaiArrayInclude") {
			args := n.Arguments()
			if len(args) != 2 {
				return nil, false
			}
			x, y := l.expr(args[0]), l.expr(args[1])
			if x == nil || y == nil {
				return nil, true
			}
			op, result := "chai.deepEqual", hir.T(hir.Bool)
			if sym.Name == "chaiDeepInclude" {
				op, result = "chai.deepInclude", hir.T(hir.I32)
			} else if sym.Name == "chaiArrayInclude" {
				op = "chai.arrayInclude"
			}
			return l.rtOp(op, l.rtOp("chai.box", x, hir.T(hir.Dynamic)), result, l.rtOp("chai.box", y, hir.T(hir.Dynamic))), true
		}
	}
	if callee.Kind != ast.KindPropertyAccessExpression {
		return nil, false
	}
	base := callee.Expression()
	if base.Kind != ast.KindIdentifier {
		return nil, false
	}
	// Function/namespace merging does not require a function-object receiver
	// for the shim's fail method. Certify both resolved symbols, including
	// aliased imports; arbitrary user objects named expect are untouched.
	if sym := l.resolve(callee); sym != nil && sym.Name == "fail" {
		if f := l.fileOfSymbol(sym); f != nil && l.relFile(f) == "runtime/chai.ts" {
			if parent := l.resolve(base); parent != nil && parent.Name == "expect" && l.fileOfSymbol(parent) == f {
				if len(n.Arguments()) != 1 || n.Arguments()[0].Kind == ast.KindSpreadElement {
					l.diagf(n, "unsupported-call", "chai expect.fail supports only one message argument")
					return nil, true
				}
				ref, ok := l.funcsBy[f.FileName()+" chaiFail"]
				if ok {
					if m := l.methodsBy[ref.owner+"."+ref.method]; m != nil {
						args, valid := l.callArgs(n, n.Arguments(), m.Params)
						if !valid {
							return nil, true
						}
						return &hir.Expr{Node: l.node(n), Kind: hir.DirectCall, Owner: ref.owner, Name: ref.method, Type: m.Result, Args: args}, true
					}
				}
			}
		}
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

func (l *lowerer) isDeferredTestCallback(n *ast.Node) bool {
	if !l.isTestCallback(n) {
		return false
	}
	callee := n.Parent.Expression()
	if callee.Kind == ast.KindPropertyAccessExpression {
		callee = callee.Expression()
	}
	if callee.Kind != ast.KindIdentifier {
		return false
	}
	switch callee.Text() {
	case "it", "before", "after", "beforeEach", "afterEach":
		return true
	}
	return false
}
