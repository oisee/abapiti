package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// A literal fixes the callback ABI: match, captures, UTF-16 offset, input,
// and (only with named captures) groups. All matches are collected before
// calling user code, as RegExp @@replace requires. Regex matching remains at
// the existing checked runtime boundary.
func (l *lowerer) replaceCallback(n *ast.Node, recv, re, callback *hir.Expr) *hir.Expr {
	args := n.Arguments()
	pattern := args[0]
	if pattern.Kind == ast.KindIdentifier {
		if sym := l.resolve(pattern); sym != nil && sym.ValueDeclaration != nil {
			d := sym.ValueDeclaration
			if d.Kind == ast.KindVariableDeclaration && d.Parent != nil && d.Parent.Kind == ast.KindVariableDeclarationList && d.Parent.Flags&ast.NodeFlagsConst != 0 && d.Initializer() != nil {
				pattern = d.Initializer()
			}
		}
	}
	m := l.callableMethod(callback.Type)
	if re.Type.Kind != hir.RegExp || m == nil {
		l.diagf(args[1], "unsupported-regex", "replace callback requires a RegExp and a lowered callable")
		return nil
	}
	source, flags := "", ""
	if pattern.Kind == ast.KindRegularExpressionLiteral {
		text := pattern.Text()
		slash := strings.LastIndex(text, "/")
		source, flags = text[1:slash], text[slash+1:]
	} else if pattern.Kind == ast.KindNewExpression && len(m.Params) <= 1 && (len(m.Params) == 0 || !m.Params[0].Variadic) {
		// A callback consuming only the whole match does not depend on the
		// number of captures. Constructor flags still fix cursor semantics.
		arguments := pattern.Arguments()
		if len(arguments) == 2 && arguments[1].Kind == ast.KindStringLiteral {
			flags = arguments[1].Text()
		} else if len(arguments) != 1 {
			l.diagf(pattern, "unsupported-regex", "replace callback requires known RegExp flags")
			return nil
		}
	} else {
		l.diagf(pattern, "unsupported-regex", "replace callback capture arguments require a literal RegExp")
		return nil
	}
	global := strings.Contains(flags, "g")
	if n.Expression().Name().Text() == "replaceAll" && !global {
		l.diagf(pattern, "unsupported-regex", "replaceAll requires a global RegExp")
		return nil
	}
	captures, named := 0, false
	inClass, escape := false, false
	for i := 0; i < len(source); i++ {
		c := source[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if c == '[' {
			inClass = true
			continue
		}
		if c == ']' {
			inClass = false
			continue
		}
		if c != '(' || inClass {
			continue
		}
		if i+1 == len(source) || source[i+1] != '?' {
			captures++
			continue
		}
		if i+3 < len(source) && source[i+2] == '<' && source[i+3] != '=' && source[i+3] != '!' {
			captures++
			named = true
		}
	}
	str, num, boolean := hir.T(hir.String), hir.T(hir.Number), hir.T(hir.Bool)
	rt := func(op string, x *hir.Expr, t hir.Type, args ...*hir.Expr) *hir.Expr {
		v := l.rtOp(op, x, t, args...)
		v.Node = l.node(n)
		return v
	}
	matchType := hir.T(hir.Array, str)
	callback = l.tempInit(args[1], callback.Type, callback)
	matches := l.tempInit(n, hir.T(hir.Array, matchType), &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.T(hir.Array, matchType)})
	run := func(x *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: x} }
	assign := func(x, y *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: x, Y: y} }
	binary := func(op string, x, y *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Type: num, Op: op, X: x, Y: y}
	}
	if global {
		l.pendStmt(run(rt("regexp.setLastIndex", re, hir.T(hir.Void), hir.L(num, 0))))
	}
	l.serial++
	maybe := hir.V("replaceMatch"+itoa(l.serial), hir.T(hir.Optional, matchType))
	op := "regexp.exec"
	if global && strings.ContainsAny(flags, "uv") {
		op = "regexp.matchAllNext"
	}
	advance := rt(op, re, maybe.Type, recv)
	advance.Node = l.node(n)
	collect := []*hir.Stmt{
		{Kind: hir.VarDecl, Node: l.node(n), Name: maybe.Name, Type: maybe.Type, X: advance},
		{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.IsUndefined, Type: boolean, X: maybe}, Body: hir.B(&hir.Stmt{Kind: hir.Break, Node: l.node(n)})},
		run(rt("array.push", matches, hir.T(hir.I32), &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: matchType, X: maybe})),
	}
	if !global {
		collect = append(collect, &hir.Stmt{Kind: hir.Break, Node: l.node(n)})
	} else if !strings.ContainsAny(flags, "uv") {
		// RegExp @@replace advances an empty non-Unicode match by one
		// UTF-16 unit. Collect matches before invoking any callbacks.
		row := &hir.Expr{Kind: hir.Narrow, Type: matchType, X: maybe}
		whole := &hir.Expr{Kind: hir.Narrow, Type: str, X: rt("array.get", row, hir.T(hir.Optional, str), hir.L(num, 0))}
		empty := &hir.Expr{Kind: hir.Binary, Op: "==", Type: boolean, X: rt("string.length", whole, num), Y: hir.L(num, 0)}
		next := binary("+", rt("array.matchIndex", row, num), hir.L(num, 1))
		collect = append(collect, &hir.Stmt{Kind: hir.If, Node: l.node(n), X: empty, Body: hir.B(run(rt("regexp.setLastIndex", re, hir.T(hir.Void), next)))})
	}
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: hir.L(boolean, true), Body: hir.B(collect...)})
	result := l.tempInit(n, str, hir.L(str, ""))
	last := l.tempInit(n, num, hir.L(num, 0))
	l.serial++
	row := hir.V("replaceRow"+itoa(l.serial), matchType)
	index := rt("array.matchIndex", row, num)
	full := &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: str, X: rt("array.get", row, hir.T(hir.Optional, str), hir.L(num, 0))}
	actual := []*hir.Expr{full}
	for i := 1; i <= captures; i++ {
		actual = append(actual, rt("array.get", row, hir.T(hir.Optional, str), hir.L(num, i)))
	}
	actual = append(actual, index, recv)
	if named {
		actual = append(actual, rt("array.matchGroups", row, hir.T(hir.Dynamic)))
	}
	// Keep argument and conversion temporaries within the callback loop.
	saved := l.pend
	l.pend = nil
	var callArgs []*hir.Expr
	for i, p := range m.Params {
		if p.Variadic {
			if p.Type.Kind != hir.Array {
				l.pend = saved
				l.diagf(args[1], "unsupported-regex", "replace callback rest parameter requires array storage")
				return nil
			}
			rest := l.tempInit(n, p.Type, &hir.Expr{Kind: hir.New, Node: l.node(n), Type: p.Type})
			for j := i; j < len(actual); j++ {
				l.pendStmt(run(rt("array.push", rest, hir.T(hir.I32), l.coerce(actual[j], p.Type.Args[0]))))
			}
			callArgs = append(callArgs, rest)
			break
		}
		value := &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.Optional, hir.T(hir.Dynamic))}
		if i < len(actual) {
			value = actual[i]
		}
		callArgs = append(callArgs, l.coerce(value, p.Type))
	}
	call := &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), X: callback, Name: m.Name, Type: m.Result, Args: callArgs}
	replacement := l.primitiveString(args[1], call)
	if replacement == nil {
		l.pend = saved
		return nil
	}
	replacement = l.tempInit(n, str, replacement)
	concat := func(x, y *hir.Expr) *hir.Expr { return rt("string.concat", x, str, y) }
	prefix := rt("string.slice", recv, str, last, index)
	body := append(l.pend, assign(result, concat(concat(result, prefix), replacement)), assign(last, binary("+", index, rt("string.length", full, num))))
	l.pend = saved
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: row.Name, Type: row.Type, X: matches, Body: hir.B(body...)})
	return concat(result, rt("string.slice", recv, str, last, rt("string.length", recv, num)))
}
