package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

func (l *lowerer) arrayFrom(n *ast.Node) *hir.Expr {
	args := n.Arguments()
	if len(args) > 2 {
		l.diagf(n, "unsupported-call", "Array.from thisArg requires a callback this ABI")
		return nil
	}
	x := l.expr(args[0])
	if x == nil {
		return nil
	}
	if x.Type.Kind == hir.Dynamic {
		// Unannotated JS helpers erase their string input to any. Keep the
		// string iterator's code-point semantics behind the existing checked
		// string projection; other dynamic iterable ABIs remain refused.
		l.diagf(n, "note-arrayfrom-string", "dynamic Array.from source uses checked string iteration")
		x = l.rtOp("dynamic.asString", x, hir.T(hir.String))
	}
	dense := x.Type.Kind == hir.String
	if dense {
		x = l.rtOp("string.codePoints", x, hir.T(hir.Array, hir.T(hir.String)))
		x.Node = l.node(n)
	} else if x.Type.Kind == hir.OrderedSet {
		if len(args) == 2 {
			l.diagf(n, "unsupported-call", "Array.from Set mapFn requires a live set iterator")
			return nil
		}
		x = l.rtOp("set.values", x, hir.T(hir.Array, x.Type.Args[0]))
		if len(args) == 1 {
			return x
		}
	} else if l.generatorABIFor(x.Type) != nil {
		if len(args) == 2 {
			l.diagf(n, "unsupported-call", "Array.from generator mapFn requires interleaved iterator calls")
			return nil
		}
		return l.drainGenerator(x)
	}
	if x.Type.Kind == hir.Array && len(args) == 1 {
		if !l.unprunedFeatures || dense || x.Kind == hir.RuntimeOp && x.Op == "set.values" {
			return l.rtOp("array.slice0", x, x.Type)
		}
		return l.rtOp("array.spread0", x, x.Type)
	}
	array := x.Type.Kind == hir.Array
	if !array {
		t := l.ck.GetTypeAtLocation(args[0])
		if t == nil || l.ck.GetPropertyOfType(t, "length") == nil {
			l.diagf(n, "unsupported-call", "Array.from requires a proven array-like length or supported iterable")
			return nil
		}
	}
	var elem hir.Type
	if array {
		elem = x.Type.Args[0]
		if !dense && !elem.IsRef() && elem.Kind != hir.Optional {
			elem = hir.T(hir.Optional, elem)
		}
	} else {
		elem = hir.T(hir.Dynamic)
	}
	var body *hir.Expr
	var params []string
	if len(args) == 2 {
		cb := args[1]
		if cb.Kind != ast.KindArrowFunction {
			l.diagf(n, "unsupported-call", "Array.from mapFn requires an arrow callback")
			return nil
		}
		if len(cb.Parameters()) > 2 {
			l.diagf(n, "unsupported-call", "Array.from supplies only value and index")
			return nil
		}
		var ok bool
		body, params, ok = l.callbackBody(n, cb, elem)
		if !ok || body == nil {
			return nil
		}
	}
	x = l.tempInit(n, x.Type, x)
	var length *hir.Expr
	if array {
		length = l.rtOp("array.length", x, hir.T(hir.Number))
	} else {
		x = l.tempInit(n, hir.T(hir.Dynamic), l.convertConditionalBranch(args[0], x, hir.T(hir.Dynamic), false))
		length = l.rtOp("arrayLike.length", x, hir.T(hir.Number))
		length.Node = l.node(n)
		length = l.tempInit(n, length.Type, length)
	}
	resultElem := elem
	if body != nil {
		resultElem = body.Type
	}
	result := l.tempVar(n, hir.T(hir.Array, resultElem))
	i := l.tempInit(n, hir.T(hir.Number), hir.L(hir.T(hir.Number), 0))
	var value *hir.Expr
	if array {
		value = l.rtOp("array.get", x, hir.T(hir.Optional, x.Type.Args[0]), l.indexValue(i))
		value = l.coerce(value, elem)
	} else {
		value = l.rtOp("dynamic.get", x, hir.T(hir.Dynamic), l.rtOp("number.toString", i, hir.T(hir.String)))
		value = l.convertConditionalBranch(n, value, elem, false)
	}
	var statements []*hir.Stmt
	if body != nil {
		statements = append(statements, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[0], Type: elem, X: value})
		if len(params) > 1 {
			statements = append(statements, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[1], Type: i.Type, X: i})
		}
		value = body
	}
	statements = append(statements, &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", result, hir.T(hir.I32), value)},
		&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: i, Y: &hir.Expr{Kind: hir.Binary, Type: i.Type, Op: "+", X: i, Y: hir.L(i.Type, 1)}})
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: &hir.Expr{Kind: hir.Binary, Type: hir.T(hir.Bool), Op: "<", X: i, Y: length}, Body: hir.B(statements...)})
	return result
}
