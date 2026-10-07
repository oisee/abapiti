package tstransforms

import (
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/jsnum"
	"github.com/oisee/abapiti/internal/tsgo/printer"
)

func constantExpression(value any, factory *printer.NodeFactory) *ast.Expression {
	switch value := value.(type) {
	case string:
		return factory.NewStringLiteral(value, ast.TokenFlagsNone)
	case jsnum.Number:
		if value.IsInf() {
			if value > 0 {
				return factory.NewIdentifier("Infinity")
			}
			return factory.NewPrefixUnaryExpression(ast.KindMinusToken, factory.NewIdentifier("Infinity"))
		}
		if value.IsNaN() {
			return factory.NewIdentifier("NaN")
		}
		if value < 0 {
			return factory.NewPrefixUnaryExpression(ast.KindMinusToken, constantExpression(-value, factory))
		}
		return factory.NewNumericLiteral(value.String(), ast.TokenFlagsNone)
	}
	return nil
}
