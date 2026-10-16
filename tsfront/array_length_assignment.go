package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Array length writes used by storage analyses only shorten existing storage.
// Reuse splice's live mutation ABI; growth (which creates holes) and invalid
// lengths explicitly refuse before mutation. The assigned value is returned.
func (l *lowerer) arrayLengthAssignment(n, lhs, rhs *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures || lhs.Kind != ast.KindPropertyAccessExpression {
		return nil, false
	}
	p := lhs.AsPropertyAccessExpression()
	if p.Name().Text() != "length" || !l.ck.IsArrayType(l.ck.GetTypeAtLocation(p.Expression)) {
		return nil, false
	}
	keep := l.hint
	l.hint = hir.Type{}
	defer func() { l.hint = keep }()
	recv := l.expr(p.Expression)
	if recv == nil {
		return nil, true
	}
	if recv.Type.Kind != hir.Array {
		l.diagf(lhs, "unsupported-assignment", "array length write requires array storage")
		return nil, true
	}
	recv = l.tempInit(p.Expression, recv.Type, recv)
	value := l.expr(rhs)
	if value == nil {
		return nil, true
	}
	if value.Type.Kind != hir.Number && value.Type.Kind != hir.I32 {
		l.diagf(rhs, "unsupported-assignment", "array length write requires a numeric length")
		return nil, true
	}
	value = l.tempInit(rhs, value.Type, value)
	boolean := hir.T(hir.Bool)
	binary := func(op string, a, b *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Type: boolean, Op: op, X: a, Y: b}
	}
	valid := binary("&&", binary(">=", value, hir.L(value.Type, 0)), binary("<=", value, l.rtOp("array.length", recv, hir.T(hir.I32))))
	if value.Type.Kind == hir.Number {
		valid = binary("&&", valid, binary("==", value, l.rtOp("number.ceil", value, hir.T(hir.Number))))
	}
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Node: l.node(n), Type: boolean, Op: "!", X: valid}, Body: hir.B(&hir.Stmt{Kind: hir.Trap, Node: l.node(n), Name: l.trapLocation(n) + ": array length growth or invalid length is not lowered"})})
	l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.splice1", recv, recv.Type, value)})
	return value, true
}
