package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Recognize the complete generic nested-append body, independently of names.
// Specializing its loops at each direct call preserves native array storage
// and aliases; erasing T[] to a copied tagged array would lose caller writes.
func (l *lowerer) genericAppendDeclaration(fn *ast.Node) bool {
	if !l.unprunedFeatures || fn == nil || fn.Kind != ast.KindFunctionDeclaration || len(fn.TypeParameters()) != 1 || len(fn.Parameters()) != 2 || fn.Body() == nil || fn.AsFunctionDeclaration().AsteriskToken != nil || fn.ModifierFlags()&ast.ModifierFlagsAsync != 0 {
		return false
	}
	tp := fn.TypeParameters()[0]
	if tp.AsTypeParameterDeclaration().Constraint != nil || tp.AsTypeParameterDeclaration().DefaultType != nil {
		return false
	}
	ps := fn.Parameters()
	if ps[0].Name().Kind != ast.KindIdentifier || ps[1].Name().Kind != ast.KindIdentifier || ps[0].AsParameterDeclaration().DotDotDotToken != nil || ps[1].AsParameterDeclaration().DotDotDotToken == nil || ps[0].Initializer() != nil || ps[1].Initializer() != nil || fn.Type() == nil || fn.Type().Kind != ast.KindVoidKeyword {
		return false
	}
	var element func(*ast.Node, int) bool
	element = func(t *ast.Node, depth int) bool {
		if t == nil {
			return false
		}
		for t.Kind == ast.KindParenthesizedType || t.Kind == ast.KindTypeOperator && t.AsTypeOperatorNode().Operator == ast.KindReadonlyKeyword {
			t = t.Type()
		}
		if depth == 0 {
			return t.Kind == ast.KindTypeReference && t.AsTypeReferenceNode().TypeName.Kind == ast.KindIdentifier && t.AsTypeReferenceNode().TypeName.Text() == tp.Name().Text() && len(t.TypeArguments()) == 0
		}
		return t.Kind == ast.KindArrayType && element(t.AsArrayTypeNode().ElementType, depth-1)
	}
	if !element(ps[0].Type(), 1) || !element(ps[1].Type(), 2) {
		return false
	}
	single := func(n *ast.Node) *ast.Node {
		for n != nil && n.Kind == ast.KindBlock {
			if len(n.Statements()) != 1 {
				return nil
			}
			n = n.Statements()[0]
		}
		return n
	}
	loop := func(n *ast.Node, source string) (*ast.Node, string) {
		n = single(n)
		if n == nil || n.Kind != ast.KindForOfStatement {
			return nil, ""
		}
		f := n.AsForInOrOfStatement()
		if f.AwaitModifier != nil || f.Expression.Kind != ast.KindIdentifier || f.Expression.Text() != source || f.Initializer.Kind != ast.KindVariableDeclarationList {
			return nil, ""
		}
		ds := f.Initializer.AsVariableDeclarationList().Declarations.Nodes
		if len(ds) != 1 || ds[0].Name().Kind != ast.KindIdentifier || ds[0].Initializer() != nil {
			return nil, ""
		}
		return f.Statement, ds[0].Name().Text()
	}
	inner, array := loop(fn.Body(), ps[1].Name().Text())
	if inner == nil {
		return false
	}
	body, value := loop(inner, array)
	body = single(body)
	if body == nil || body.Kind != ast.KindExpressionStatement {
		return false
	}
	call := body.Expression()
	if call.Kind != ast.KindCallExpression || len(call.Arguments()) != 1 {
		return false
	}
	callee := call.Expression()
	return callee.Kind == ast.KindPropertyAccessExpression && callee.Name().Text() == "push" && callee.Expression().Kind == ast.KindIdentifier && callee.Expression().Text() == ps[0].Name().Text() && call.Arguments()[0].Kind == ast.KindIdentifier && call.Arguments()[0].Text() == value && array != ps[1].Name().Text() && value != ps[1].Name().Text() && array != value && array != ps[0].Name().Text() && value != ps[0].Name().Text()
}

func (l *lowerer) genericAppendCall(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	sym := l.resolve(gogenCallee(n.Expression()))
	if sym == nil || !l.genericAppendDeclaration(sym.ValueDeclaration) {
		return nil, false
	}
	args := n.Arguments()
	if len(args) == 0 || args[0].Kind == ast.KindSpreadElement {
		l.diagf(n, "unsupported-generic", "generic append requires a target array")
		return nil, true
	}
	target := l.expr(args[0])
	if target == nil {
		return nil, true
	}
	if target.Type.Kind != hir.Array {
		l.diagf(args[0], "unsupported-generic", "generic append requires native array storage")
		return nil, true
	}
	target = l.tempInit(args[0], target.Type, target)
	type argument struct {
		value  *hir.Expr
		spread bool
	}
	var values []argument
	for _, arg := range args[1:] {
		node := arg
		spread := arg.Kind == ast.KindSpreadElement
		if spread {
			node = arg.Expression()
		}
		typ := target.Type
		if spread {
			typ = hir.T(hir.Array, typ)
		}
		old := l.hint
		l.hint = typ
		value := l.expr(node)
		l.hint = old
		if value == nil {
			return nil, true
		}
		if !l.acceptsType(typ, value.Type) {
			l.diagf(arg, "unsupported-generic", "generic append requires matching element storage")
			return nil, true
		}
		values = append(values, argument{l.tempInit(node, typ, l.coerce(value, typ)), spread})
	}
	appendLoop := func(xs *hir.Expr) *hir.Stmt {
		l.serial++
		name := "$append" + itoa(l.serial)
		elem := target.Type.Args[0]
		var prefix, body []*hir.Stmt
		var index *hir.Expr
		if !elem.IsRef() && elem.Kind != hir.Optional {
			l.serial++
			index = hir.V("$appendIndex"+itoa(l.serial), hir.T(hir.Number))
			prefix = append(prefix, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: index.Name, Type: index.Type, X: hir.L(index.Type, 0)})
			body = append(body, &hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Type: hir.T(hir.Bool), Op: "!", X: l.rtOp("array.has", xs, hir.T(hir.Bool), index)}, Body: hir.B(&hir.Stmt{Kind: hir.Trap, Node: l.node(n), Name: l.node(n).Source + ": generic append primitive element storage cannot represent undefined from holes"})})
		}
		body = append(body, &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", target, hir.T(hir.I32), hir.V(name, elem))})
		if index != nil {
			body = append(body, &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: index, Y: &hir.Expr{Kind: hir.Binary, Type: index.Type, Op: "+", X: index, Y: hir.L(index.Type, 1)}})
		}
		loop := &hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: name, Type: elem, X: xs, Body: hir.B(body...)}
		return hir.B(append(prefix, loop)...)
	}
	for _, arg := range values {
		if arg.spread {
			l.serial++
			name := "$appendSources" + itoa(l.serial)
			l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: name, Type: target.Type, X: arg.value, Body: hir.B(appendLoop(hir.V(name, target.Type)))})
		} else {
			l.pendStmt(appendLoop(arg.value))
		}
	}
	l.diagf(n, "note-generic-append", "generic append loops specialized to %s", target.Type)
	return &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.Optional, hir.T(hir.Dynamic))}, true
}
