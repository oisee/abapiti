package tsfront

import (
	"github.com/oisee/abapiti/internal/tsgo/ast"

	"github.com/oisee/abapiti/hir"
)

// numberConversion lowers the global Number(value) call: ECMAScript ToNumber
// for the typed primitive arguments. Strings use StringToNumber (whole-string
// grammar, radix prefixes, Infinity, NaN otherwise); undefined is NaN. Other
// argument types refuse at the call. Boxed values convert at run time (Go);
// ABAP refuses the boxed conversion at its location. Full TS lowering only: the result may be
// NaN or infinite.
func (l *lowerer) numberConversion(n *ast.Node, args []*ast.Node) *hir.Expr {
	number := hir.T(hir.Number)
	if len(args) == 0 {
		return hir.L(number, "0")
	}
	if len(args) != 1 || args[0].Kind == ast.KindSpreadElement {
		l.diagf(n, "unsupported-call", "Number accepts one non-spread argument")
		return nil
	}
	x := l.expr(args[0])
	if x == nil {
		return nil
	}
	return l.toNumber(n, x)
}

func (l *lowerer) toNumber(n *ast.Node, x *hir.Expr) *hir.Expr {
	number := hir.T(hir.Number)
	switch x.Type.Kind {
	case hir.Number:
		return x
	case hir.I32, hir.I64:
		return l.coerce(x, number)
	case hir.String:
		return l.rtOp("string.toNumber", x, number)
	case hir.Dynamic:
		conv := l.rtOp("dynamic.toNumber", x, number)
		conv.Node = l.node(n)
		return conv
	case hir.Bool:
		return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: number, X: x, Y: hir.L(number, "1"), Z: hir.L(number, "0")}
	case hir.Optional:
		switch x.Type.Args[0].Kind {
		case hir.Number, hir.I32, hir.I64, hir.String, hir.Bool:
			v := l.tempInit(n, x.Type, x)
			present := l.toNumber(n, &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: x.Type.Args[0], X: v})
			return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: number,
				X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: v},
				Y: l.rtOp("string.parseFloatNumber", hir.L(hir.T(hir.String), "NaN"), number),
				Z: present}
		}
	}
	l.diagf(n, "unsupported-call", "Number(value) on %s is not lowered", x.Type.Kind)
	return nil
}
