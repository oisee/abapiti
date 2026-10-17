package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// callableReplace lowers s.replace(/re/flags, (match, ...captures) => text)
// as RegExp.prototype[@@replace] does: lastIndex starts at 0 for a global
// pattern, an empty match advances it by one code unit, each callback result
// is inserted literally (no $ substitutions), and the unmatched tail follows.
// The callback binds the match and captures that always participate; offset,
// input and groups parameters, and u/y/v patterns, are refused here.
// callableReplaceRefusal reports the first reason the literal callback falls
// outside the subset callableReplace lowers, or a nil node when it fits.
func (l *lowerer) callableReplaceRefusal(n *ast.Node, recv *hir.Expr, args []*ast.Node) (*ast.Node, string) {
	refuse := func(at *ast.Node, msg string) (*ast.Node, string) { return at, msg }
	if recv.Type.Kind != hir.String {
		return refuse(n, "needs a string receiver")
	}
	if args[0].Kind != ast.KindRegularExpressionLiteral {
		return refuse(args[0], "requires a literal RegExp pattern")
	}
	text := args[0].Text()
	slash := strings.LastIndex(text, "/")
	source, flags := text[1:slash], text[slash+1:]
	if strings.ContainsAny(flags, "uyvd") {
		return refuse(args[0], "with u, v, y or d flags is not lowered")
	}
	global := strings.Contains(flags, "g")
	if n.Expression().Name().Text() == "replaceAll" && !global {
		return refuse(args[0], "replaceAll requires a global RegExp")
	}
	captures, always := regexCaptures(source)
	cb := args[1]
	if cb.Kind == ast.KindFunctionExpression && callbackUsesThis(cb.Body()) {
		return refuse(cb, "function callback this binding is not lowered")
	}
	params := cb.Parameters()
	if len(params) > 1+captures || len(params) > 3 {
		return refuse(cb, "binds the match and its captures; offset, input and groups parameters are not lowered")
	}
	for i, p := range params {
		d := p.AsParameterDeclaration()
		if p.Initializer() != nil || d.DotDotDotToken != nil || p.Name() == nil || p.Name().Kind != ast.KindIdentifier {
			return refuse(p, "parameters must be plain identifiers")
		}
		t := l.ck.GetTypeAtLocation(p)
		if t == nil || (t.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) == 0 && !l.ck.IsTypeAssignableTo(l.ck.GetStringType(), t)) {
			return refuse(p, "parameters must accept a string")
		}
		if i > 0 && !always {
			return refuse(p, "captures that may not participate (undefined) are not lowered")
		}
	}
	return nil, ""
}

func (l *lowerer) callableReplace(n *ast.Node, recv *hir.Expr, args []*ast.Node) *hir.Expr {
	refuse := func(at *ast.Node, msg string) *hir.Expr {
		l.diagf(at, "unsupported-regex", "callable replace %s", msg)
		return nil
	}
	if at, msg := l.callableReplaceRefusal(n, recv, args); at != nil {
		return refuse(at, msg)
	}
	text := args[0].Text()
	global := strings.Contains(text[strings.LastIndex(text, "/")+1:], "g")
	cb := args[1]
	pat := l.expr(args[0])
	if pat == nil {
		return nil
	}
	if pat.Type.Kind != hir.RegExp {
		return refuse(args[0], "requires a RegExp pattern")
	}
	pat = l.tempInit(args[0], pat.Type, pat)
	str, num, b := hir.T(hir.String), hir.T(hir.Number), hir.T(hir.Bool)
	at := l.node(n)
	if global {
		l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: at, X: l.rtOp("regexp.setLastIndex", pat, hir.T(hir.Void), hir.L(num, 0))})
	}
	out := l.tempInit(n, str, hir.L(str, ""))
	last := l.tempInit(n, num, hir.L(num, 0))
	body, names, ok := l.callbackBody(n, cb, str)
	if !ok || body == nil {
		return nil
	}
	if body.Type.Kind != hir.String {
		body = l.primitiveString(cb, body)
		if body == nil || body.Type.Kind != hir.String {
			return refuse(cb, "callback must return a string or a primitive")
		}
	}
	l.serial++
	id := itoa(l.serial)
	match := hir.V("replaceMatch"+id, hir.T(hir.Optional, hir.T(hir.Array, str)))
	row := hir.V("replaceRow"+id, hir.T(hir.Array, str))
	whole := hir.V("replaceWhole"+id, str)
	start := hir.V("replaceStart"+id, num)
	end := hir.V("replaceEnd"+id, num)
	get := func(k int) *hir.Expr {
		return &hir.Expr{Kind: hir.Narrow, Node: at, Type: str, X: l.rtOp("array.get", row, hir.T(hir.Optional, str), hir.L(num, k))}
	}
	bin := func(op string, x, y *hir.Expr, t hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Node: at, Op: op, Type: t, X: x, Y: y}
	}
	loop := []*hir.Stmt{
		{Kind: hir.VarDecl, Node: at, Name: match.Name, Type: match.Type, X: l.rtOp("regexp.exec", pat, match.Type, recv)},
		{Kind: hir.If, Node: at, X: &hir.Expr{Kind: hir.IsUndefined, Node: at, Type: b, X: match}, Body: hir.B(&hir.Stmt{Kind: hir.Break, Node: at})},
		{Kind: hir.VarDecl, Node: at, Name: row.Name, Type: row.Type, X: &hir.Expr{Kind: hir.Narrow, Node: at, Type: row.Type, X: match}},
		{Kind: hir.VarDecl, Node: at, Name: whole.Name, Type: str, X: get(0)},
		{Kind: hir.VarDecl, Node: at, Name: start.Name, Type: num, X: l.rtOp("array.matchIndex", row, num)},
		{Kind: hir.VarDecl, Node: at, Name: end.Name, Type: num, X: bin("+", start, l.rtOp("string.length", whole, num), num)},
	}
	for i, name := range names {
		value := whole
		if i > 0 {
			value = get(i)
		}
		loop = append(loop, &hir.Stmt{Kind: hir.VarDecl, Node: at, Name: name, Type: str, X: value})
	}
	piece := l.rtOp("string.concat", l.rtOp("string.slice", recv, str, last, start), str, body)
	loop = append(loop,
		&hir.Stmt{Kind: hir.Assign, Node: at, X: out, Y: l.rtOp("string.concat", out, str, piece)},
		&hir.Stmt{Kind: hir.Assign, Node: at, X: last, Y: end})
	if !global {
		loop = append(loop, &hir.Stmt{Kind: hir.Break, Node: at})
	} else {
		advance := bin("+", end, hir.L(num, 1), num)
		loop = append(loop, &hir.Stmt{Kind: hir.If, Node: at, X: bin("==", l.rtOp("string.length", whole, num), hir.L(num, 0), b),
			Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: at, X: l.rtOp("regexp.setLastIndex", pat, hir.T(hir.Void), advance)})})
	}
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: at, X: hir.L(b, true), Body: hir.B(loop...)})
	l.diagf(n, "note-callable-replace", "replace callback inlined into an exec loop")
	return l.rtOp("string.concat", out, str, l.rtOp("string.slice", recv, str, last, l.rtOp("string.length", recv, num)))
}

// regexCaptures counts capturing groups in a JS pattern source and reports
// whether every one participates in each match: no alternation, no group
// followed by an optional quantifier, and no negative lookaround.
func regexCaptures(src string) (int, bool) {
	count, always := 0, true
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case '[':
			for i++; i < len(src) && src[i] != ']'; i++ {
				if src[i] == '\\' {
					i++
				}
			}
		case '|':
			always = false
		case '(':
			rest := src[i+1:]
			switch {
			case strings.HasPrefix(rest, "?!") || strings.HasPrefix(rest, "?<!"):
				always = false
			case strings.HasPrefix(rest, "?<=") || strings.HasPrefix(rest, "?=") || strings.HasPrefix(rest, "?:"):
			case strings.HasPrefix(rest, "?"):
				count++ // named group (?<name>...)
			default:
				count++
			}
		case ')':
			if i+1 < len(src) && strings.ContainsRune("?*{", rune(src[i+1])) {
				always = false
			}
		}
	}
	return count, always || count == 0
}
