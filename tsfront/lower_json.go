package tsfront

import (
	"math"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// JSON intrinsics are recognized by their default-library binding, never by
// spelling alone. Revivers/replacers refuse; literal indentation operates on
// the existing compact serialization, without changing its storage domain.
func (l *lowerer) jsonCall(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	c := n.Expression()
	if c == nil || c.Kind != ast.KindPropertyAccessExpression {
		return nil, false
	}
	receiver := c.Expression()
	if receiver.Kind != ast.KindIdentifier || receiver.Text() != "JSON" {
		return nil, false
	}
	file := l.fileOfSymbol(l.resolve(receiver))
	if file == nil || !l.prog.prog.IsSourceFileDefaultLibrary(file.Path()) {
		return nil, false
	}
	name := c.Name().Text()
	if name != "parse" && name != "stringify" {
		return nil, false
	}
	args := n.Arguments()
	valid := len(args) == 1
	if name == "stringify" {
		valid = len(args) >= 1 && len(args) <= 3
	}
	for _, arg := range args {
		valid = valid && arg.Kind != ast.KindSpreadElement
	}
	if !valid {
		l.diagf(n, "unsupported-json", "JSON.%s has unsupported argument arity or spread", name)
		return nil, true
	}
	undefined := func(n *ast.Node) bool {
		sym := l.resolve(n)
		return isUndefinedKeyword(n) && sym != nil && sym.Name == "undefined" && sym.ValueDeclaration == nil
	}
	var gap *hir.Expr
	var replacer, keys *hir.Expr
	if name == "stringify" && len(args) > 1 {
		if args[1].Kind != ast.KindNullKeyword && !undefined(args[1]) {
			if replacer, keys = l.jsonReplacer(args[1]); replacer == nil && keys == nil {
				return nil, true
			}
		}
	}
	if name == "stringify" && len(args) == 3 && !undefined(args[2]) {
		if args[2].Kind != ast.KindStringLiteral && args[2].Kind != ast.KindNumericLiteral {
			l.diagf(n, "unsupported-json", "JSON.stringify space requires a string or numeric literal")
			return nil, true
		}
		x := l.expr(args[2])
		if x == nil {
			return nil, true
		}
		if x.Type.Kind == hir.String {
			gap = x
		} else if count, ok := x.Value.(float64); ok {
			count = math.Min(10, math.Max(0, math.Trunc(count)))
			gap = hir.L(hir.T(hir.String), strings.Repeat(" ", int(count)))
		} else {
			l.diagf(n, "unsupported-json", "JSON.stringify numeric space is not a literal")
			return nil, true
		}
	}
	// A fresh literal passed directly to stringify has no nominal storage
	// identity to preserve. Build it in the existing tagged-record domain so
	// source insertion order and undefined fields survive; never retrofit
	// guessed declaration order onto an aliased/native instance.
	savedHint := l.hint
	if name == "stringify" && args[0].Kind == ast.KindObjectLiteralExpression {
		l.hint = hir.T(hir.Dynamic)
	}
	value := l.expr(args[0])
	l.hint = savedHint
	if value == nil {
		return nil, true
	}
	if name == "parse" {
		if value.Type.Kind != hir.String {
			l.diagf(n, "unsupported-json", "JSON.parse input coercion is unsupported")
			return nil, true
		}
		l.builtinError()
		return l.rtOp("json.parseSubset", value, hir.T(hir.Dynamic)), true
	}
	if replacer != nil || keys != nil {
		absentFn := &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.Optional, hir.T(hir.Dynamic))}
		absentKeys := &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.Optional, hir.T(hir.Array, hir.T(hir.String)))}
		if replacer == nil {
			replacer = absentFn
		}
		if keys == nil {
			keys = absentKeys
		} else {
			keys = l.coerce(keys, absentKeys.Type)
		}
		if gap == nil {
			gap = hir.L(hir.T(hir.String), "")
		}
		l.diagf(n, "note-json-replacer", "JSON.stringify replacer applied per property (holder this is not bound)")
		return l.rtOp("json.stringifyReplacer", value, hir.T(hir.Optional, hir.T(hir.String)), replacer, keys, gap), true
	}
	// The writer preserves tagged-record insertion order, array positions and
	// primitive encoding. Native instances without an own-key-order proof trap.
	compact := l.rtOp("json.stringifySubset", value, hir.T(hir.String))
	if gap != nil {
		return l.rtOp("json.indent", compact, hir.T(hir.String), gap), true
	}
	return compact, true
}

// jsonReplacer lowers JSON.stringify's second argument: a (key, value)
// function closure, or an array of property names. A function reading this
// (the holder) is refused: the writer does not bind it.
func (l *lowerer) jsonReplacer(arg *ast.Node) (fn, keys *hir.Expr) {
	inner := arg
	for inner.Kind == ast.KindParenthesizedExpression {
		inner = inner.Expression()
	}
	if inner.Kind == ast.KindFunctionExpression && touchesThis(inner.Body()) {
		l.diagf(arg, "unsupported-json", "JSON.stringify replacer reads this (the holder)")
		return nil, nil
	}
	if inner.Kind == ast.KindArrayLiteralExpression {
		saved := l.hint
		l.hint = hir.T(hir.Array, hir.T(hir.String))
		x := l.expr(inner)
		l.hint = saved
		if x == nil {
			return nil, nil
		}
		if !x.Type.Equal(hir.T(hir.Array, hir.T(hir.String))) {
			l.diagf(arg, "unsupported-json", "JSON.stringify replacer list must contain strings")
			return nil, nil
		}
		return nil, x
	}
	if inner.Kind != ast.KindArrowFunction && inner.Kind != ast.KindFunctionExpression {
		l.diagf(arg, "unsupported-json", "JSON.stringify replacer must be an inline function or an array literal")
		return nil, nil
	}
	typ := l.functionType(inner, l.ck.GetTypeAtLocation(inner))
	saved := l.hint
	l.hint = typ
	x := l.expr(inner)
	l.hint = saved
	if x == nil {
		return nil, nil
	}
	m := l.callableMethod(typ)
	if m != nil && x.Type.Kind == hir.ClassRef {
		// The closure class is called through the callable interface it implements.
		x = &hir.Expr{Kind: hir.Cast, Node: l.node(arg), Type: typ, X: x}
	}
	if m == nil || len(m.Params) != 2 || !m.Params[0].Type.Equal(hir.T(hir.String)) || !m.Params[1].Type.Equal(hir.T(hir.Dynamic)) || !m.Result.Equal(hir.T(hir.Dynamic)) {
		l.diagf(arg, "unsupported-json", "JSON.stringify replacer must have the (key: string, value: any) => any signature")
		return nil, nil
	}
	return x, nil
}
