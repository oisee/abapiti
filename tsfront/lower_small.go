package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// updateExpression captures the reference and old value before writing. Both
// forms share this path so receiver/index side effects execute exactly once.
func (l *lowerer) updateExpression(n, operand *ast.Node, operator ast.Kind, prefix bool) *hir.Expr {
	op := "+"
	if operator == ast.KindMinusMinusToken {
		op = "-"
	} else if operator != ast.KindPlusPlusToken {
		l.diagf(n, "unsupported-expr", "update operator %s is not lowered", operator.String())
		return nil
	}
	target := l.assignTarget(operand)
	if target == nil {
		return nil
	}
	return l.updateTarget(n, target, op, prefix)
}

// optionalNumber reports `number | undefined` storage (an optional property).
func optionalNumber(t hir.Type) bool {
	return t.Kind == hir.Optional && len(t.Args) == 1 && t.Args[0].Kind == hir.Number
}

// updateTarget applies ++/-- to an already lowered reference. An optional
// number reads through ToNumeric (undefined is NaN); the postfix result is
// that numeric old value, never undefined, and the stored value is present.
func (l *lowerer) updateTarget(n *ast.Node, target *hir.Expr, op string, prefix bool) *hir.Expr {
	if target.Kind == hir.RuntimeOp && target.Op == "regexp.setLastIndex" {
		receiver := l.tempInit(n, target.X.Type, target.X)
		number := hir.T(hir.Number)
		previous := l.tempInit(n, number, l.coerce(l.rtOp("regexp.lastIndex", receiver, hir.T(hir.I32)), number))
		next := l.tempInit(n, number, &hir.Expr{Kind: hir.Binary, Node: l.node(n), Type: number, Op: op, X: previous, Y: hir.L(number, 1)})
		l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("regexp.setLastIndex", receiver, hir.T(hir.Void), next)})
		if prefix {
			return next
		}
		return previous
	}
	if optionalNumber(target.Type) {
		number := hir.T(hir.Number)
		switch target.Kind {
		case hir.FieldGet:
			target.X = l.tempInit(n, target.X.Type, target.X)
		case hir.Local, hir.StaticGet:
		default:
			l.diagf(n, "unsupported-expr", "update target is not lowered")
			return nil
		}
		old := l.tempInit(n, number, l.toNumber(n, l.tempInit(n, target.Type, target)))
		next := l.tempInit(n, number, &hir.Expr{Kind: hir.Binary, Type: number, Op: op, X: old, Y: hir.L(number, 1)})
		l.pendStmt(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: target, Y: l.coerce(next, target.Type)})
		if prefix {
			return next
		}
		return old
	}
	if target.Type.Kind != hir.Number && target.Type.Kind != hir.I32 && target.Type.Kind != hir.I64 {
		l.diagf(n, "unsupported-expr", "update needs a native numeric operand")
		return nil
	}
	switch target.Kind {
	case hir.FieldGet:
		target.X = l.tempInit(n, target.X.Type, target.X)
	case hir.IndexGet:
		target.X = l.tempInit(n, target.X.Type, target.X)
		target.Y = l.tempInit(n, target.Y.Type, target.Y)
	case hir.Local, hir.StaticGet:
	default:
		l.diagf(n, "unsupported-expr", "update target is not lowered")
		return nil
	}
	previous := l.tempInit(n, target.Type, target)
	next := &hir.Expr{Kind: hir.Binary, Type: target.Type, Op: op, X: previous, Y: hir.L(target.Type, 1)}
	if prefix {
		next = l.tempInit(n, target.Type, next)
	}
	l.pendStmt(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: target, Y: next})
	if prefix {
		return next
	}
	return previous
}

// optionalNumberCompound lowers `o.n op= v` for `n?: number`: the receiver
// and the old value are read once before v is evaluated, undefined reads as
// NaN, and the result is stored present.
func (l *lowerer) optionalNumberCompound(n *ast.Node, target *hir.Expr, op string, rhs *ast.Node) *hir.Stmt {
	number := hir.T(hir.Number)
	switch target.Kind {
	case hir.FieldGet:
		target.X = l.tempInit(n, target.X.Type, target.X)
	case hir.Local, hir.StaticGet:
	default:
		l.diagf(n, "unsupported-assignment", "compound assignment target is not lowered")
		return nil
	}
	old := l.tempInit(n, target.Type, target)
	y := l.expr(rhs)
	if y == nil {
		return nil
	}
	if y.Type.Kind != hir.Number && y.Type.Kind != hir.I32 && y.Type.Kind != hir.I64 {
		l.diagf(n, "unsupported-assignment", "compound assignment to an optional number needs a numeric operand")
		return nil
	}
	y = l.tempInit(n, number, l.coerce(y, number))
	next := &hir.Expr{Kind: hir.Binary, Node: l.node(n), Type: number, Op: op, X: l.toNumber(n, old), Y: y}
	l.pendStmt(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: target, Y: l.coerce(l.tempInit(n, number, next), target.Type)})
	pre := l.pend
	l.pend = nil
	return hir.B(pre...)
}

// flatArray implements depth one with ordinary HIR iteration and pushes: the
// result is fresh, ordered, and shares element references, not the input arrays.
func (l *lowerer) flatArray(n *ast.Node, recv *hir.Expr, args []*ast.Node) *hir.Expr {
	if len(args) > 1 || (len(args) == 1 && (args[0].Kind != ast.KindNumericLiteral || args[0].Text() != "1")) {
		l.diagf(n, "unsupported-call", "flat supports only depth 1")
		return nil
	}
	recv = l.tempInit(n, recv.Type, recv)
	elem := recv.Type.Args[0]
	if elem.Kind == hir.Dynamic || elem.Kind == hir.Optional {
		l.diagf(n, "unsupported-call", "flat on heterogeneous or sparse arrays requires element presence tags")
		return nil
	}
	if elem.Kind != hir.Array {
		return l.rtOp("array.slice0", recv, recv.Type)
	}
	out := l.tempVar(n, elem)
	l.serial++
	row := hir.V("t"+itoa(l.serial), elem)
	l.serial++
	item := hir.V("t"+itoa(l.serial), elem.Args[0])
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: row.Name, Type: row.Type, X: recv, Body: hir.B(
		&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: item.Name, Type: item.Type, X: row, Body: hir.B(
			&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", out, hir.T(hir.I32), item)},
		)},
	)})
	return out
}

// inOperator tests property presence, not the property's truthiness. Typed
// shapes have a fixed property set; index-signature records use map.has.
func (l *lowerer) inOperator(n *ast.Node, b *ast.BinaryExpression) *hir.Expr {
	key := l.expr(b.Left)
	if key == nil {
		return nil
	}
	key = l.tempInit(n, key.Type, key)
	obj := l.expr(b.Right)
	if obj == nil {
		return nil
	}
	obj = l.tempInit(n, obj.Type, obj)
	if l.unprunedFeatures && obj.Type.Kind == hir.Dynamic {
		obj = &hir.Expr{Kind: hir.Cast, Node: l.node(n), Type: hir.Ref(hir.RootObject), X: obj}
	}
	if x := l.gogenStructuralPresence(n, obj, key); x != nil {
		return x
	}
	if obj.Type.Kind == hir.OrderedMap {
		// A TS index-signature record is represented by an ordered map;
		// an actual JS Map's entries are NOT its object properties.
		t := l.ck.GetTypeAtLocation(b.Right)
		if t != nil && t.Symbol() != nil && t.Symbol().Name == "Map" {
			l.diagf(n, "unsupported-expr", "in on a JavaScript Map tests object properties, not entries")
			return nil
		}
		return l.rtOp("map.has", obj, hir.T(hir.Bool), key)
	}
	if l.unprunedFeatures && key.Type.Kind == hir.String && (obj.Type.Kind == hir.UnionRef || l.familyPayloadOptional(obj.Type, b.Left)) {
		// A recursive family value (or a narrowed payload whose optional
		// field may be absent) keeps its original own keys: erase it to the
		// object root so presence comes from those keys, not the layout.
		if obj.Type.Kind == hir.UnionRef {
			// A family can be backed by a tagged record as well as a native
			// structural class. The property ABI accepts both directly.
			obj = &hir.Expr{Kind: hir.Cast, Node: l.node(n), Type: hir.Ref(hir.RootObject), X: l.coerce(obj, hir.T(hir.Dynamic))}
		} else {
			obj = &hir.Expr{Kind: hir.Cast, Node: obj.Node, Type: hir.Ref(hir.RootObject), X: obj}
		}
	}
	if l.unprunedFeatures && obj.Type.Kind == hir.ClassRef && obj.Type.Name == hir.RootObject && key.Type.Kind == hir.String {
		// The erased object/union keeps its concrete storage. The existing
		// property operation checks original presence, not declared fields.
		return l.rtOp("object.has", obj, hir.T(hir.Bool), key)
	}
	if obj.Type.Kind == hir.ClassRef {
		c := l.classesByQualifiedName(obj.Type.Name)
		if c != nil && b.Left.Kind == ast.KindStringLiteral {
			name := b.Left.Text()
			for ; c != nil; c = l.classesByQualifiedName(c.Super) {
				for _, f := range c.Fields {
					if f.Name == name && !f.Static {
						if f.Type.Kind == hir.Optional {
							l.diagf(n, "unsupported-expr", "in on an optional shape field requires property-presence metadata")
							return nil
						}
						return hir.L(hir.T(hir.Bool), true)
					}
				}
				for _, m := range c.Methods {
					if m.Name == name && !m.Static {
						return hir.L(hir.T(hir.Bool), true)
					}
				}
			}
			return hir.L(hir.T(hir.Bool), false)
		}
	}
	l.diagf(n, "unsupported-expr", "in requires a record or a literal property on a lowered shape (got %s)", obj.Type.String())
	return nil
}

// familyPayloadOptional reports a literal key naming an optional field of a
// recursive family payload layout (family.payload.* or a family shape).
func (l *lowerer) familyPayloadOptional(t hir.Type, key *ast.Node) bool {
	if t.Kind != hir.ClassRef || key.Kind != ast.KindStringLiteral || l.familyChecks[t.Name] == nil {
		return false
	}
	c := l.classByName(t.Name)
	if c == nil || !c.Structural || c.Super != "" {
		return false
	}
	for _, f := range c.Fields {
		if f.Name == key.Text() && !f.Static {
			return f.Type.Kind == hir.Optional
		}
	}
	return false
}
