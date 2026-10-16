package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"math/big"
	"strings"
)

// A canonical decimal string represents a bigint without a machine-word
// bound. Runtime arithmetic other than negation stays a located refusal.
func constantBigInt(n *ast.Node) (*big.Int, bool) {
	if n == nil {
		return nil, false
	}
	if n.Kind == ast.KindParenthesizedExpression {
		return constantBigInt(n.Expression())
	}
	if n.Kind == ast.KindBigIntLiteral {
		v, ok := new(big.Int).SetString(strings.ReplaceAll(strings.TrimSuffix(n.Text(), "n"), "_", ""), 0)
		return v, ok
	}
	if n.Kind == ast.KindPrefixUnaryExpression && n.AsPrefixUnaryExpression().Operator == ast.KindMinusToken {
		v, ok := constantBigInt(n.AsPrefixUnaryExpression().Operand)
		if ok {
			v.Neg(v)
		}
		return v, ok
	}
	if n.Kind != ast.KindBinaryExpression {
		return nil, false
	}
	b := n.AsBinaryExpression()
	a, ok := constantBigInt(b.Left)
	if !ok {
		return nil, false
	}
	v, ok := constantBigInt(b.Right)
	if !ok {
		return nil, false
	}
	switch b.OperatorToken.Kind {
	case ast.KindPlusToken:
		return a.Add(a, v), true
	case ast.KindMinusToken:
		return a.Sub(a, v), true
	case ast.KindAsteriskToken:
		return a.Mul(a, v), true
	case ast.KindLessThanLessThanToken:
		// Bound compile-time allocation. Larger shifts remain a diagnostic.
		if !v.IsInt64() || v.Int64() < 0 || v.Int64() > 1048576 {
			return nil, false
		}
		return a.Lsh(a, uint(v.Int64())), true
	}
	return nil, false
}

func (l *lowerer) bigintExpr(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures || n == nil {
		return nil, false
	}
	if v, ok := constantBigInt(n); ok {
		return &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.BigInt), Value: v.String()}, true
	}
	if n.Kind == ast.KindBigIntLiteral {
		l.diagf(n, "unsupported-bigint", "bigint literal is outside the checked decimal domain")
		return nil, true
	}
	if n.Kind == ast.KindBinaryExpression {
		b := n.AsBinaryExpression()
		t := l.ck.GetTypeAtLocation(n)
		if t != nil && t.Flags()&checker.TypeFlagsBigIntLike != 0 {
			l.diagf(n, "unsupported-bigint", "runtime bigint arithmetic is not lowered; supported operations are decimal construction, negation, comparison and formatting")
			return nil, true
		}
		switch b.OperatorToken.Kind {
		case ast.KindLessThanToken, ast.KindLessThanEqualsToken, ast.KindGreaterThanToken, ast.KindGreaterThanEqualsToken:
			a, btype := l.ck.GetTypeAtLocation(b.Left), l.ck.GetTypeAtLocation(b.Right)
			if a == nil || btype == nil || a.Flags()&checker.TypeFlagsBigIntLike == 0 && btype.Flags()&checker.TypeFlagsBigIntLike == 0 {
				return nil, false
			}
			x, y := l.expr(b.Left), l.expr(b.Right)
			if x == nil || y == nil {
				return nil, true
			}
			if x.Type.Kind != hir.BigInt || y.Type.Kind != hir.BigInt {
				l.diagf(n, "unsupported-bigint", "bigint comparison requires two bigint operands")
				return nil, true
			}
			op := map[ast.Kind]string{ast.KindLessThanToken: "<", ast.KindLessThanEqualsToken: "<=", ast.KindGreaterThanToken: ">", ast.KindGreaterThanEqualsToken: ">="}[b.OperatorToken.Kind]
			return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Type: hir.T(hir.Bool), Op: op, X: l.rtOp("bigint.compare", x, hir.T(hir.Number), y), Y: hir.L(hir.T(hir.Number), 0)}, true
		}
	}
	return nil, false
}
