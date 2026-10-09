package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
)

func (l *lowerer) pinnedDenseCallback(n *ast.Node) bool {
	text := l.file.Text()[scanner.GetTokenPosOfNode(n, l.file, false):n.End()]
	for parent := n; parent != nil; parent = parent.Parent {
		if e, ok := l.overrides[parent]; ok && e.Patterns != nil && e.Patterns.DenseCallbacks[text] {
			l.diagf(n, "note-override", "%s: %s", e.ID, e.Rationale)
			return true
		}
	}
	return false
}

// This is a lowering mechanism for fingerprinted adapters, not a general
// sparse-array feature. Each permitted call carries its own density/membership
// proof under an enclosing source hash. Element assignment keeps ref identity.
func (l *lowerer) denseCallbackLoop(n *ast.Node, operation string, receiver *hir.Expr, args []*ast.Node) *hir.Expr {
	if len(args) != 1 {
		l.diagf(n, "unsupported-callback", "dense %s needs one callback", operation)
		return nil
	}
	recv := l.tempInit(n, receiver.Type, receiver)
	elem := recv.Type.Args[0]
	limit := l.tempInit(n, hir.T(hir.Number), l.rtOp("array.length", recv, hir.T(hir.I32)))
	index := l.tempInit(n, hir.T(hir.Number), hir.L(hir.T(hir.Number), 0))
	optional := hir.T(hir.Optional, elem)
	var result *hir.Expr
	switch operation {
	case "find":
		result = l.tempInit(n, optional, &hir.Expr{Kind: hir.Lit, Type: optional})
	case "every":
		result = l.tempInit(n, hir.T(hir.Bool), hir.L(hir.T(hir.Bool), true))
	}
	var expression *hir.Expr
	var statements *hir.Stmt
	var params []string
	if operation == "forEach" {
		var ok bool
		statements, params, ok = l.denseCallbackStatements(n, args[0], elem)
		if !ok {
			return nil
		}
	} else {
		var ok bool
		expression, params, ok = l.callbackBody(n, args[0], elem)
		if !ok {
			return nil
		}
	}
	var body []*hir.Stmt
	body = append(body, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[0], Type: elem, X: &hir.Expr{Kind: hir.Narrow, Type: elem, X: l.rtOp("array.get", recv, optional, index)}})
	if len(params) >= 2 {
		body = append(body, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[1], Type: hir.T(hir.Number), X: index})
	}
	if len(params) >= 3 {
		body = append(body, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[2], Type: recv.Type, X: recv})
	}
	switch operation {
	case "forEach":
		body = append(body, statements)
	case "find":
		body = append(body, &hir.Stmt{Kind: hir.If, X: l.toBool(n, expression), Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: result, Y: l.coerce(hir.V(params[0], elem), optional)}, &hir.Stmt{Kind: hir.Break})})
	case "every":
		body = append(body, &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: l.toBool(n, expression)}, Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: result, Y: hir.L(hir.T(hir.Bool), false)}, &hir.Stmt{Kind: hir.Break})})
	}
	body = append(body, &hir.Stmt{Kind: hir.Assign, X: index, Y: &hir.Expr{Kind: hir.Binary, Op: "+", Type: index.Type, X: index, Y: hir.L(index.Type, 1)}})
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: &hir.Expr{Kind: hir.Binary, Op: "<", Type: hir.T(hir.Bool), X: index, Y: limit}, Body: hir.B(body...)})
	if operation == "forEach" {
		return &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.Optional, hir.T(hir.Dynamic))}
	}
	return result
}

func (l *lowerer) denseCallbackStatements(n, callback *ast.Node, elem hir.Type) (*hir.Stmt, []string, bool) {
	if callback.Kind != ast.KindArrowFunction && callback.Kind != ast.KindFunctionExpression {
		l.diagf(n, "unsupported-callback", "dense callback must be an inline function")
		return nil, nil, false
	}
	params := callback.Parameters()
	if len(params) < 1 || len(params) > 3 {
		l.diagf(n, "unsupported-callback", "dense callback needs 1-3 simple parameters")
		return nil, nil, false
	}
	saved := l.pend
	l.pend = nil
	l.push()
	defer func() { l.pop(); l.pend = saved }()
	var names []string
	for index, param := range params {
		if param.Name() == nil || param.Name().Kind != ast.KindIdentifier || param.Initializer() != nil {
			l.diagf(param, "unsupported-callback", "dense callback parameter must be a plain identifier")
			return nil, nil, false
		}
		typ := elem
		if index == 1 {
			typ = hir.T(hir.Number)
		}
		if index == 2 {
			typ = hir.T(hir.Array, elem)
		}
		names = append(names, param.Name().Text())
		l.declare(param.Name().Text(), typ)
	}
	block := callback.Body()
	if block == nil {
		return nil, nil, false
	}
	if block.Kind == ast.KindBlock {
		returns := false
		var walk func(*ast.Node)
		walk = func(node *ast.Node) {
			if node.Kind == ast.KindReturnStatement {
				returns = true
			}
			node.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
		}
		walk(block)
		if returns {
			l.diagf(callback, "unsupported-callback", "dense forEach block return needs its own control-flow ABI")
			return nil, nil, false
		}
		return l.block(block), names, true
	}
	value := l.expr(block)
	if value == nil {
		return nil, nil, false
	}
	return hir.B(append(l.pend, &hir.Stmt{Kind: hir.ExprStmt, X: value})...), names, true
}

// pinnedCheckedCast reports whether an enclosing fingerprinted override
// certifies this exact assertion as a proven downcast.
func (l *lowerer) pinnedCheckedCast(n *ast.Node) bool {
	text := l.file.Text()[scanner.GetTokenPosOfNode(n, l.file, false):n.End()]
	for parent := n; parent != nil; parent = parent.Parent {
		if e, ok := l.overrides[parent]; ok && e.Patterns != nil && e.Patterns.CheckedCasts[text] {
			l.diagf(n, "note-override", "%s: %s", e.ID, e.Rationale)
			return true
		}
	}
	return false
}
