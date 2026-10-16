package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Default Array.sort compares ToString results by UTF-16 units, never locale
// order or the backend's native string representation. Only primitive values
// lowered tuples, and checked flat primitive arrays have a certified ToString.
func (l *lowerer) defaultSortKey(at *ast.Node, x *hir.Expr, tupleElement bool) *hir.Expr {
	str := hir.T(hir.String)
	if tupleElement && x.Type.Kind == hir.Optional {
		key := l.defaultSortKey(at, &hir.Expr{Kind: hir.Narrow, Node: l.node(at), Type: x.Type.Args[0], X: x}, true)
		if key == nil {
			return nil
		}
		return &hir.Expr{Kind: hir.Conditional, Node: l.node(at), Type: str, X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: x}, Y: hir.L(str, ""), Z: key}
	}
	switch x.Type.Kind {
	case hir.Dynamic:
		key := l.dynamicSortKey(at, x)
		if tupleElement {
			return &hir.Expr{Kind: hir.Conditional, Node: l.node(at), Type: str, X: l.rtOp("dynamic.isNullish", x, hir.T(hir.Bool)), Y: hir.L(str, ""), Z: key}
		}
		return key
	case hir.String, hir.Number, hir.I32, hir.I64, hir.Bool:
		return l.primitiveString(at, x)
	case hir.Array:
		if x.Type.Args[0].Kind == hir.String {
			return l.rtOp("array.join", x, str, hir.L(str, ","))
		}
	case hir.ClassRef:
		if strings.HasPrefix(x.Type.Name, "tuple.") {
			c := l.classByName(x.Type.Name)
			key := hir.L(str, "")
			for i, f := range c.Fields {
				part := l.defaultSortKey(at, &hir.Expr{Kind: hir.FieldGet, Node: l.node(at), Type: f.Type, Name: f.Name, X: x}, true)
				if part == nil {
					return nil
				}
				if i > 0 {
					key = l.rtOp("string.concat", key, str, hir.L(str, ","))
				}
				key = l.rtOp("string.concat", key, str, part)
			}
			return key
		}
	}
	l.diagf(at, "unsupported-call", "default sort requires primitive elements or tuples of primitives; %s has no certified sort string conversion", x.Type.String())
	return nil
}

func (l *lowerer) defaultSortCompare(at *ast.Node, left, right *hir.Expr) *hir.Expr {
	a, z := l.defaultSortKey(at, left, false), l.defaultSortKey(at, right, false)
	if a == nil || z == nil {
		return nil
	}
	saved := l.pend
	l.pend = nil
	defer func() { l.pend = saved }()
	str, i, num, b := hir.T(hir.String), hir.T(hir.I32), hir.T(hir.Number), hir.T(hir.Bool)
	a = l.tempInit(at, str, a)
	z = l.tempInit(at, str, z)
	al := l.tempInit(at, i, l.rtOp("string.length", a, i))
	zl := l.tempInit(at, i, l.rtOp("string.length", z, i))
	index := l.tempInit(at, i, hir.L(i, 0))
	result := l.tempInit(at, i, hir.L(i, 0))
	binary := func(op string, x, y *hir.Expr, t hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Node: l.node(at), Type: t, Op: op, X: x, Y: y}
	}
	assign := func(x, y *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Assign, Node: l.node(at), X: x, Y: y} }
	cond := binary("&&", binary("<", index, al, b), binary("<", index, zl, b), b)
	loop := hir.B(assign(result, binary("-", l.rtOp("string.charCodeAt", a, i, index), l.rtOp("string.charCodeAt", z, i, index), i)),
		&hir.Stmt{Kind: hir.If, Node: l.node(at), X: binary("!=", result, hir.L(i, 0), b), Body: hir.B(&hir.Stmt{Kind: hir.Break, Node: l.node(at)})},
		assign(index, binary("+", index, hir.L(i, 1), i)))
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(at), X: cond, Body: loop})
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(at), X: binary("==", result, hir.L(i, 0), b), Body: hir.B(assign(result, binary("-", al, zl, i)))})
	return &hir.Expr{Kind: hir.Seq, Node: l.node(at), Type: num, Stmt: hir.B(l.pend...), Y: l.coerce(result, num)}
}

// Boxed map-entry rows retain actual array storage. Convert only primitives
// or flat arrays of primitives; prototype hooks and nested objects refuse at
// the located runtime boundary instead of inventing a host string ordering.
func (l *lowerer) dynamicSortKey(at *ast.Node, x *hir.Expr) *hir.Expr {
	saved := l.pend
	l.pend = nil
	defer func() { l.pend = saved }()
	str, dyn, b := hir.T(hir.String), hir.T(hir.Dynamic), hir.T(hir.Bool)
	x = l.tempInit(at, dyn, x)
	result := l.tempInit(at, str, hir.L(str, ""))
	assign := func(value *hir.Expr) *hir.Stmt {
		return &hir.Stmt{Kind: hir.Assign, Node: l.node(at), X: result, Y: value}
	}
	fail := func() *hir.Stmt {
		return failureBody(l.node(at), l.locOf(at)+": default sort supports primitives and flat primitive arrays only")
	}
	primitive := func(v *hir.Expr) *hir.Stmt {
		typ := l.rtOp("dynamic.typeofValue", v, str)
		eq := func(s string) *hir.Expr {
			return &hir.Expr{Kind: hir.Binary, Node: l.node(at), Type: b, Op: "==", X: typ, Y: hir.L(str, s)}
		}
		or := func(a, z *hir.Expr) *hir.Expr {
			return &hir.Expr{Kind: hir.Binary, Node: l.node(at), Type: b, Op: "||", X: a, Y: z}
		}
		valid := or(or(eq("string"), eq("number")), or(eq("boolean"), l.rtOp("dynamic.isNullish", v, b)))
		return hir.B(&hir.Stmt{Kind: hir.If, Node: l.node(at), X: &hir.Expr{Kind: hir.Unary, Type: b, Op: "!", X: valid}, Body: fail()}, assign(l.rtOp("dynamic.toString", v, str)))
	}
	l.serial++
	item := hir.V("sortItem"+itoa(l.serial), dyn)
	joined := l.tempInit(at, str, hir.L(str, ""))
	first := l.tempInit(at, b, hir.L(b, true))
	concat := func(a, z *hir.Expr) *hir.Expr { return l.rtOp("string.concat", a, str, z) }
	rows := l.rtOp("dynamic.arrayView", x, hir.T(hir.Array, dyn))
	rows.Node = l.node(at)
	loop := &hir.Stmt{Kind: hir.ForEach, Node: l.node(at), Name: item.Name, Type: dyn, X: rows, Body: hir.B(
		&hir.Stmt{Kind: hir.If, Node: l.node(at), X: &hir.Expr{Kind: hir.Unary, Type: b, Op: "!", X: first}, Body: hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(at), X: joined, Y: concat(joined, hir.L(str, ","))})},
		&hir.Stmt{Kind: hir.Assign, Node: l.node(at), X: first, Y: hir.L(b, false)},
		&hir.Stmt{Kind: hir.If, Node: l.node(at), X: l.rtOp("dynamic.isNullish", item, b), Body: hir.B(assign(hir.L(str, ""))), Else: primitive(item)},
		&hir.Stmt{Kind: hir.Assign, Node: l.node(at), X: joined, Y: concat(joined, result)})}
	array := hir.B(loop, assign(joined))
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(at), X: &hir.Expr{Kind: hir.IsUndefined, Node: l.node(at), Type: b, X: x}, Body: fail()})
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(at), X: l.rtOp("dynamic.isArray", x, b), Body: array, Else: primitive(x)})
	return &hir.Expr{Kind: hir.Seq, Node: l.node(at), Type: str, Stmt: hir.B(l.pend...), Y: result}
}
