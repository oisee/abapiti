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
	return l.arrayCallbackLoop(n, operation, receiver, args, false)
}

// Snapshot the length, not the elements: HasProperty and Get are performed at
// each visit, so removals/replacements are observed and pushes are not visited.
func (l *lowerer) arrayCallbackLoop(n *ast.Node, operation string, receiver *hir.Expr, args []*ast.Node, checkPresence bool) *hir.Expr {
	if len(args) != 1 {
		l.diagf(n, "unsupported-callback", "dense %s needs one callback", operation)
		return nil
	}
	recv := l.tempInit(n, receiver.Type, receiver)
	elem := recv.Type.Args[0]
	limit := l.tempInit(n, hir.T(hir.Number), l.rtOp("array.length", recv, hir.T(hir.I32)))
	index := l.tempInit(n, hir.T(hir.Number), hir.L(hir.T(hir.Number), 0))
	optional := hir.T(hir.Optional, elem)
	if elem.Kind == hir.Optional {
		optional = elem
	}
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
	value := l.rtOp("array.get", recv, optional, index)
	if elem.Kind != hir.Optional {
		value = &hir.Expr{Kind: hir.Narrow, Type: elem, X: value}
	}
	body = append(body, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[0], Type: elem, X: value})
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
	if checkPresence {
		body = []*hir.Stmt{{Kind: hir.If, Node: l.node(n), X: l.rtOp("array.has", recv, hir.T(hir.Bool), index), Body: hir.B(body...)}}
	}
	body = append(body, &hir.Stmt{Kind: hir.Assign, X: index, Y: &hir.Expr{Kind: hir.Binary, Op: "+", Type: index.Type, X: index, Y: hir.L(index.Type, 1)}})
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: &hir.Expr{Kind: hir.Binary, Op: "<", Type: hir.T(hir.Bool), X: index, Y: limit}, Body: hir.B(body...)})
	if operation == "forEach" {
		return &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.Optional, hir.T(hir.Dynamic))}
	}
	return result
}

func (l *lowerer) denseCallbackStatements(n, callback *ast.Node, elem hir.Type) (*hir.Stmt, []string, bool) {
	if callback.Kind == ast.KindPropertyAccessExpression || callback.Kind == ast.KindCallExpression {
		value, names, ok := l.methodCallbackBody(n, callback, elem)
		if !ok {
			return nil, nil, false
		}
		return hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: value}), names, true
	}
	if callback.Kind != ast.KindArrowFunction && callback.Kind != ast.KindFunctionExpression {
		l.diagf(n, "unsupported-callback", "dense callback must be an inline function")
		return nil, nil, false
	}
	params := callback.Parameters()
	if len(params) > 3 {
		l.diagf(n, "unsupported-callback", "callback needs 0-3 simple parameters")
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
	if len(names) == 0 {
		l.serial++
		name := "unused" + itoa(l.serial)
		l.declare(name, elem)
		names = append(names, name)
	}
	block := callback.Body()
	if block == nil {
		return nil, nil, false
	}
	if block.Kind == ast.KindBlock {
		if l.containsReturn(block) {
			l.serial++
			lf := l.liftLocalFn("foreach_"+itoa(l.serial), callback)
			if lf == nil {
				return nil, nil, false
			}
			var args []*hir.Expr
			for _, capture := range lf.captures {
				args = append(args, hir.V(capture.name, capture.typ))
			}
			for i := range params {
				args = append(args, hir.V(names[i], lf.method.Params[len(lf.captures)+i].Type))
			}
			call := &hir.Expr{Kind: hir.VirtualCall, Node: l.node(callback), X: l.this(l.class), Name: lf.method.Name, Type: lf.method.Result, Args: args}
			if lf.method.Static {
				call.Kind, call.Owner, call.X = hir.DirectCall, lf.owner.Name, nil
			}
			return hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: call}), names, true
		}
		return l.block(block), names, true
	}
	value := l.expr(block)
	if value == nil {
		return nil, nil, false
	}
	return hir.B(append(l.pend, &hir.Stmt{Kind: hir.ExprStmt, X: value})...), names, true
}

// A non-arrow callback has its own undefined this (without thisArg). Do not
// accidentally inherit the surrounding method's receiver. Nested arrows share
// that callback's this, while nested ordinary functions have another binding.
func callbackUsesThis(n *ast.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind == ast.KindThisKeyword {
		return true
	}
	if n.Kind == ast.KindFunctionExpression || n.Kind == ast.KindFunctionDeclaration {
		return false
	}
	found := false
	n.ForEachChild(func(child *ast.Node) bool { found = callbackUsesThis(child); return found })
	return found
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
