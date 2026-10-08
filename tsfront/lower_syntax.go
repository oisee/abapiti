package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Lowerings added for the abaplint syntax closure (abap/5_syntax, abap/types,
// abap/4_file_information): more inlined array callbacks, the shift/unshift
// and set operations, parseInt/isNaN, Array.from/isArray, postfix updates on
// fields, string indexing, for-in over optional records, optional chains on
// primitives and `continue` inside for loops with an update expression. Every
// lowering keeps the JavaScript result or reports a blocking diagnostic.

// syntaxHofCall inlines find, findIndex, every, forEach and flatMap like the
// phase-2 map/filter/some loops (ADR-0006).
func (l *lowerer) syntaxHofCall(n *ast.Node, name string, recv *hir.Expr, args []*ast.Node) (*hir.Expr, bool) {
	switch name {
	case "find", "findIndex", "every", "forEach", "flatMap":
	default:
		return nil, false
	}
	elem := recv.Type.Args[0]
	if len(args) != 1 {
		l.diagf(n, "unsupported-call", "%s needs one callback", name)
		return nil, true
	}
	if name == "forEach" {
		return l.forEachLoop(n, recv, elem, args[0]), true
	}
	body, params, ok := l.callbackBody(n, args[0], elem)
	if !ok {
		return nil, true
	}
	l.diagf(n, "note-callback-inline", "%s callback inlined into a loop", name)
	if body.Type.Kind == hir.Void {
		return nil, true
	}
	// The element index: the callback's second parameter, or a private
	// counter for findIndex.
	index := ""
	if len(params) >= 2 {
		index = params[1]
	} else if name == "findIndex" {
		l.serial++
		index = "i" + itoa(l.serial)
	}
	var idxInit *hir.Stmt
	if index != "" {
		l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: index, Type: hir.T(hir.Number), X: hir.L(hir.T(hir.Number), -1)})
		idxInit = &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: hir.V(index, hir.T(hir.Number)),
			Y: &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "+", Type: hir.T(hir.Number), X: hir.V(index, hir.T(hir.Number)), Y: hir.L(hir.T(hir.Number), 1)}}
	}
	if len(params) >= 3 {
		l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[2], Type: recv.Type, X: recv})
	}
	loopBody := func(inner ...*hir.Stmt) *hir.Stmt {
		if idxInit != nil {
			return hir.B(append([]*hir.Stmt{idxInit}, inner...)...)
		}
		return hir.B(inner...)
	}
	loop := func(body *hir.Stmt) {
		l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: params[0], Type: elem, X: recv, Body: body})
	}
	switch name {
	case "find":
		optT := hir.T(hir.Optional, elem)
		if elem.Kind == hir.Optional {
			optT = elem
		}
		out := l.tempInit(n, optT, &hir.Expr{Kind: hir.Lit, Type: optT})
		loop(loopBody(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: l.toBool(n, body),
			Body: hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: out, Y: l.coerce(hir.V(params[0], elem), optT)},
				&hir.Stmt{Kind: hir.Break, Node: l.node(n)})}))
		return out, true
	case "findIndex":
		out := l.tempInit(n, hir.T(hir.Number), hir.L(hir.T(hir.Number), -1))
		loop(loopBody(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: l.toBool(n, body),
			Body: hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: out, Y: hir.V(index, hir.T(hir.Number))},
				&hir.Stmt{Kind: hir.Break, Node: l.node(n)})}))
		return out, true
	case "every":
		out := l.tempInit(n, hir.T(hir.Bool), hir.L(hir.T(hir.Bool), true))
		loop(loopBody(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Node: l.node(n), Op: "!", Type: hir.T(hir.Bool), X: l.toBool(n, body)},
			Body: hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: out, Y: hir.L(hir.T(hir.Bool), false)},
				&hir.Stmt{Kind: hir.Break, Node: l.node(n)})}))
		return out, true
	case "flatMap":
		if body.Type.Kind != hir.Array {
			l.diagf(n, "unsupported-call", "flatMap callback must return an array")
			return nil, true
		}
		out := l.tempVar(n, body.Type)
		l.serial++
		row := hir.V("r"+itoa(l.serial), body.Type.Args[0])
		loop(loopBody(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: row.Name, Type: row.Type, X: body,
			Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", out, hir.T(hir.I32), row)})}))
		return out, true
	}
	return nil, false
}

// forEachLoop lowers arr.forEach(cb) with a statement-bodied callback: the
// block runs once per element. A `return` inside the block (continue in the
// original) is rejected. The result is the (void) false literal in an
// expression statement.
func (l *lowerer) forEachLoop(n *ast.Node, recv *hir.Expr, elem hir.Type, cb *ast.Node) *hir.Expr {
	if cb.Kind != ast.KindArrowFunction && cb.Kind != ast.KindFunctionExpression {
		l.diagf(n, "unsupported-callback", "forEach callback %s is not an arrow function", cb.Kind.String())
		return nil
	}
	params := cb.Parameters()
	if len(params) == 0 || params[0].Name() == nil || params[0].Name().Kind != ast.KindIdentifier {
		l.diagf(n, "unsupported-callback", "forEach callback needs a named element parameter")
		return nil
	}
	if len(params) > 2 {
		l.diagf(n, "unsupported-callback", "forEach callback with an array parameter is not lowered")
		return nil
	}
	if cb.Body() == nil {
		return nil
	}
	if cb.Body().Kind == ast.KindBlock && l.containsReturn(cb.Body()) {
		l.diagf(n, "unsupported-callback", "forEach callback with return is not lowered")
		return nil
	}
	elemName := params[0].Name().Text()
	l.push()
	l.declare(elemName, elem)
	var prelude []*hir.Stmt
	index := ""
	if len(params) == 2 && params[1].Name() != nil && params[1].Name().Kind == ast.KindIdentifier {
		index = params[1].Name().Text()
		l.declare(index, hir.T(hir.Number))
		l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: index, Type: hir.T(hir.Number), X: hir.L(hir.T(hir.Number), -1)})
		prelude = append(prelude, &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: hir.V(index, hir.T(hir.Number)),
			Y: &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "+", Type: hir.T(hir.Number), X: hir.V(index, hir.T(hir.Number)), Y: hir.L(hir.T(hir.Number), 1)}})
	}
	var body []*hir.Stmt
	if cb.Body().Kind == ast.KindBlock {
		body = l.stmtList(cb.Body().AsBlock().Statements.Nodes)
	} else {
		saved := l.pend
		l.pend = nil
		x := l.expr(cb.Body())
		body = append(body, l.pend...)
		l.pend = saved
		if x == nil {
			l.pop()
			return nil
		}
		body = append(body, &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(cb), X: x})
	}
	l.pop()
	l.diagf(n, "note-callback-inline", "forEach callback inlined into a loop")
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: elemName, Type: elem, X: recv, Body: hir.B(append(prelude, body...)...)})
	return &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: hir.T(hir.Bool), Value: false}
}

// containsLoopControl reports a break or continue that would leave the
// node (nested loops and functions keep their own).
func (l *lowerer) containsLoopControl(n *ast.Node) bool {
	targets := map[*ast.Node]bool{}
	continueTargets(n, targets)
	breakTargets(n, targets)
	return len(targets) > 0
}

func (l *lowerer) containsReturn(n *ast.Node) bool {
	found := false
	var walk func(*ast.Node)
	walk = func(x *ast.Node) {
		if x == nil || found {
			return
		}
		if x.Kind == ast.KindReturnStatement {
			found = true
			return
		}
		if x.Kind == ast.KindFunctionExpression || x.Kind == ast.KindArrowFunction || x.Kind == ast.KindFunctionDeclaration {
			return
		}
		x.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(n)
	return found
}

// syntaxLibraryCall adds array shift/unshift, set delete/values.
func (l *lowerer) syntaxLibraryCall(n *ast.Node, name string, recv *hir.Expr) (*hir.Expr, bool) {
	args := n.Arguments()
	switch recv.Type.Kind {
	case hir.Array:
		switch name {
		case "shift":
			if len(args) != 0 {
				return nil, false
			}
			return l.rtOp("array.shift", recv, hir.T(hir.Optional, recv.Type.Args[0])), true
		case "sort":
			if len(args) != 1 || (args[0].Kind != ast.KindArrowFunction && args[0].Kind != ast.KindFunctionExpression) {
				l.diagf(n, "unsupported-call", "sort is lowered only with an inline comparator")
				return nil, true
			}
			return l.sortWithComparator(n, recv, args[0]), true
		case "unshift":
			if len(args) != 1 {
				l.diagf(n, "unsupported-call", "unshift with %d arguments is not lowered", len(args))
				return nil, true
			}
			old := l.hint
			l.hint = recv.Type.Args[0]
			value := l.expr(args[0])
			l.hint = old
			if value == nil {
				return nil, true
			}
			return l.rtOp("array.unshift", recv, hir.T(hir.I32), l.coerce(value, recv.Type.Args[0])), true
		}
	case hir.OrderedSet:
		switch name {
		case "delete":
			if len(args) != 1 {
				return nil, false
			}
			key := l.expr(args[0])
			if key == nil {
				return nil, true
			}
			return l.rtOp("set.delete", recv, hir.T(hir.Bool), key), true
		case "values":
			if len(args) != 0 {
				return nil, false
			}
			return l.rtOp("set.values", recv, hir.T(hir.Array, recv.Type.Args[0])), true
		}
	case hir.OrderedMap:
		if name == "values" && len(args) == 0 {
			return l.rtOp("map.values", recv, hir.T(hir.Array, recv.Type.Args[1])), true
		}
	case hir.String:
		switch name {
		case "slice":
			if len(args) == 0 || len(args) > 2 {
				return nil, false
			}
			recv = l.tempInit(n, recv.Type, recv)
			a := l.expr(args[0])
			if a == nil {
				return nil, true
			}
			var b *hir.Expr
			if len(args) == 2 {
				b = l.expr(args[1])
				if b == nil {
					return nil, true
				}
			} else {
				b = l.rtOp("string.length", recv, hir.T(hir.I32))
			}
			return l.rtOp("string.slice", recv, hir.T(hir.String), a, b), true
		case "localeCompare":
			if len(args) != 1 {
				l.diagf(n, "unsupported-call", "localeCompare with locales or options is not lowered")
				return nil, true
			}
			other := l.expr(args[0])
			if other == nil {
				return nil, true
			}
			l.diagf(n, "note-locale-compare", "localeCompare lowered on the object-name alphabet [_/0-9A-Z]; other code units raise")
			return l.rtOp("string.localeCompareNames", recv, hir.T(hir.I32), other), true
		}
	case hir.ClassRef:
		if recv.Type.Name == "builtin.Error" && name == "toString" && len(args) == 0 {
			return l.errorToString(n, recv), true
		}
	}
	return nil, false
}

// globalCall lowers calls of the global functions parseInt and isNaN.
func (l *lowerer) globalCall(n *ast.Node, callee *ast.Node) (*hir.Expr, bool) {
	if callee.Kind != ast.KindIdentifier {
		return nil, false
	}
	if _, isLocal := l.lookup(callee.Text()); isLocal {
		return nil, false
	}
	sym := l.resolve(callee)
	if sym == nil {
		return nil, false
	}
	if f := l.fileOfSymbol(sym); f == nil || !l.prog.prog.IsSourceFileDefaultLibrary(f.Path()) {
		return nil, false
	}
	args := n.Arguments()
	switch callee.Text() {
	case "parseInt":
		if len(args) != 2 || args[1].Kind != ast.KindNumericLiteral || args[1].Text() != "10" {
			l.diagf(n, "unsupported-call", "parseInt is lowered only with an explicit radix 10")
			return nil, true
		}
		s := l.expr(args[0])
		if s == nil {
			return nil, true
		}
		if s.Type.Kind == hir.Dynamic {
			// parseInt converts its argument with String() first.
			s = l.rtOp("dynamic.toString", s, hir.T(hir.String))
		}
		if s.Type.Kind != hir.String {
			l.diagf(n, "unsupported-call", "parseInt needs a string argument")
			return nil, true
		}
		// NaN has no binary64 representation in ABAP: the absent result
		// traps at this location instead of flowing on as a number.
		optT := hir.T(hir.Optional, hir.T(hir.Number))
		parsed := l.tempInit(n, optT, l.rtOp("string.parseInt10", s, optT))
		l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: parsed},
			Body: hir.B(&hir.Stmt{Kind: hir.Trap, Node: l.node(n), Name: l.locOf(n)})})
		l.diagf(n, "note-parse-int", "parseInt(s, 10) lowered; a NaN result traps with this location")
		return &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: hir.T(hir.Number), X: parsed}, true
	case "isNaN":
		if len(args) != 1 {
			return nil, false
		}
		x := l.expr(args[0])
		if x == nil {
			return nil, true
		}
		if x.Type.Kind != hir.Number {
			l.diagf(n, "unsupported-call", "isNaN on %s is not lowered", x.Type.Kind)
			return nil, true
		}
		// A HIR Number is never NaN: every NaN-producing operation traps.
		// The operand is still evaluated for its effects.
		l.diagf(n, "note-is-nan", "isNaN on a HIR number is the constant false")
		return &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: hir.T(hir.Bool), Stmt: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: x}), Y: hir.L(hir.T(hir.Bool), false)}, true
	}
	return nil, false
}

// isGlobalArrayExpr matches the `Array` global used as a static receiver.
func (l *lowerer) isGlobalArrayExpr(e *ast.Node) bool {
	if e == nil || e.Kind != ast.KindIdentifier || e.Text() != "Array" {
		return false
	}
	if _, isLocal := l.lookup("Array"); isLocal {
		return false
	}
	sym := l.resolve(e)
	if sym == nil || l.classOf(sym) != nil {
		return false
	}
	f := l.fileOfSymbol(sym)
	return f != nil && l.prog.prog.IsSourceFileDefaultLibrary(f.Path())
}

// arrayStatic lowers Array.from(x) and Array.isArray(x).
func (l *lowerer) arrayStatic(n *ast.Node, name string) (*hir.Expr, bool) {
	args := n.Arguments()
	if len(args) != 1 {
		return nil, false
	}
	switch name {
	case "from":
		x := l.expr(args[0])
		if x == nil {
			return nil, true
		}
		switch x.Type.Kind {
		case hir.Array:
			return l.rtOp("array.slice0", x, x.Type), true
		case hir.OrderedSet:
			return l.rtOp("set.values", x, hir.T(hir.Array, x.Type.Args[0])), true
		}
		l.diagf(n, "unsupported-call", "Array.from on %s is not lowered", x.Type.Kind)
		return nil, true
	case "isArray":
		x := l.expr(args[0])
		if x == nil {
			return nil, true
		}
		switch x.Type.Kind {
		case hir.Array:
			l.diagf(n, "note-is-array", "Array.isArray on an array is the constant true")
			return &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: hir.T(hir.Bool), Stmt: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: x}), Y: hir.L(hir.T(hir.Bool), true)}, true
		case hir.Dynamic, hir.Optional:
			// When every object constituent of the checker's union is an
			// array, the box's "object" tag decides.
			if t := l.ck.GetTypeAtLocation(args[0]); t != nil && l.objectPartsAreArrays(t) {
				d := x
				if x.Type.Kind == hir.Optional {
					if x.Type.Args[0].Kind != hir.Dynamic {
						break
					}
					present := l.tempInit(n, x.Type, x)
					defined := &hir.Expr{Kind: hir.Unary, Node: l.node(n), Type: hir.T(hir.Bool), Op: "!", X: &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: present}}
					d = &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: hir.T(hir.Dynamic), X: present}
					tag := &hir.Expr{Kind: hir.Binary, Node: l.node(n), Type: hir.T(hir.Bool), Op: "==", X: l.rtOp("dynamic.typeof", d, hir.T(hir.String)), Y: hir.L(hir.T(hir.String), "object")}
					l.diagf(n, "note-is-array", "Array.isArray on a tagged value decided by its object tag")
					return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Type: hir.T(hir.Bool), Op: "&&", X: defined, Y: tag}, true
				}
				l.diagf(n, "note-is-array", "Array.isArray on a tagged value decided by its object tag")
				return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Type: hir.T(hir.Bool), Op: "==", X: l.rtOp("dynamic.typeof", d, hir.T(hir.String)), Y: hir.L(hir.T(hir.String), "object")}, true
			}
		}
		if x.Type.Kind == hir.Dynamic {
			l.diagf(n, "note-is-array", "Array.isArray on a dynamic value reads the box's array tag")
			return l.rtOp("dynamic.isArray", x, hir.T(hir.Bool)), true
		}
		if x.Type.Kind == hir.Optional && x.Type.Args[0].Kind == hir.Dynamic {
			present := l.tempInit(n, x.Type, x)
			defined := &hir.Expr{Kind: hir.Unary, Node: l.node(n), Type: hir.T(hir.Bool), Op: "!", X: &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: present}}
			d := &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: hir.T(hir.Dynamic), X: present}
			l.diagf(n, "note-is-array", "Array.isArray on a dynamic value reads the box's array tag")
			return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Type: hir.T(hir.Bool), Op: "&&", X: defined, Y: l.rtOp("dynamic.isArray", d, hir.T(hir.Bool))}, true
		}
		l.diagf(n, "unsupported-call", "Array.isArray on %s is not lowered", x.Type.Kind)
		return nil, true
	}
	return nil, false
}

// postfixUpdate lowers x++ / x-- on a field or static operand: the old value
// is the result, the store happens in the prelude.
func (l *lowerer) postfixUpdate(n *ast.Node, x *hir.Expr, op string) *hir.Expr {
	if x.Kind != hir.FieldGet && x.Kind != hir.StaticGet {
		return nil
	}
	if x.Type.Kind != hir.Number && x.Type.Kind != hir.I32 {
		return nil
	}
	if x.Kind == hir.FieldGet {
		// Evaluate the receiver once.
		x.X = l.tempInit(n, x.X.Type, x.X)
	}
	old := l.tempInit(n, x.Type, x)
	l.pendStmt(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: x, Y: &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: op, Type: x.Type, X: old, Y: hir.L(x.Type, 1)}})
	return old
}

// setFromArray lowers new Set(arrayExpr): a fresh set with one add per
// element, in order.
func (l *lowerer) setFromArray(n *ast.Node, source *hir.Expr) *hir.Expr {
	typ := hir.T(hir.OrderedSet, source.Type.Args[0])
	out := l.tempVar(n, typ)
	l.serial++
	row := hir.V("s"+itoa(l.serial), source.Type.Args[0])
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: row.Name, Type: row.Type, X: source,
		Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("set.add", out, typ, row)})})
	l.diagf(n, "note-set-from-array", "new Set(array) lowered to an ordered set built in element order")
	return out
}

// setFromOptionalArray lowers new Set(x) for x: T[] | undefined: an empty
// set when absent.
func (l *lowerer) setFromOptionalArray(n *ast.Node, source *hir.Expr) *hir.Expr {
	inner := source.Type.Args[0]
	typ := hir.T(hir.OrderedSet, inner.Args[0])
	present := l.tempInit(n, source.Type, source)
	out := l.tempVar(n, typ)
	l.serial++
	row := hir.V("s"+itoa(l.serial), inner.Args[0])
	loop := &hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: row.Name, Type: row.Type, X: &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: inner, X: present},
		Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("set.add", out, typ, row)})}
	absent := &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: present}
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Node: l.node(n), Op: "!", Type: hir.T(hir.Bool), X: absent}, Body: hir.B(loop)})
	l.diagf(n, "note-set-from-array", "new Set(array | undefined) lowered to an ordered set, empty when absent")
	return out
}

// continueTargets collects the `continue` statements that belong to the loop
// body n (not to a nested loop or function).
func continueTargets(n *ast.Node, into map[*ast.Node]bool) {
	var walk func(*ast.Node)
	walk = func(x *ast.Node) {
		if x == nil {
			return
		}
		switch x.Kind {
		case ast.KindContinueStatement:
			into[x] = true
			return
		case ast.KindForStatement, ast.KindForInStatement, ast.KindForOfStatement, ast.KindWhileStatement, ast.KindDoStatement,
			ast.KindFunctionExpression, ast.KindArrowFunction, ast.KindFunctionDeclaration:
			return
		}
		x.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(n)
}

// forBodyWithContinue wraps the body of a for loop that has an update
// expression in a one-shot loop: `continue` becomes a break out of that
// loop, so the update still runs; the body's own `break` is hoisted through
// a flag.
func (l *lowerer) forBodyWithContinue(n *ast.Node, body *ast.Node, update *ast.Node) *hir.Stmt {
	targets := map[*ast.Node]bool{}
	continueTargets(body, targets)
	for t := range targets {
		if t.AsContinueStatement().Label != nil {
			l.diagf(t, "unsupported-statement", "labeled continue is not lowered")
			return nil
		}
	}
	breaks := map[*ast.Node]bool{}
	breakTargets(body, breaks)
	if l.continueAsBreak == nil {
		l.continueAsBreak = map[*ast.Node]bool{}
	}
	for t := range targets {
		l.continueAsBreak[t] = true
	}
	defer func() {
		for t := range targets {
			delete(l.continueAsBreak, t)
		}
	}()
	// A break inside the one-shot loop must leave the outer loop too.
	flag := l.tempInit(n, hir.T(hir.Bool), hir.L(hir.T(hir.Bool), false))
	if l.breakViaFlag == nil {
		l.breakViaFlag = map[*ast.Node]*hir.Expr{}
	}
	for t := range breaks {
		l.breakViaFlag[t] = flag
	}
	defer func() {
		for t := range breaks {
			delete(l.breakViaFlag, t)
		}
	}()
	inner := []*hir.Stmt{l.scopeBlock(body), {Kind: hir.Break, Node: l.node(n)}}
	oneShot := &hir.Stmt{Kind: hir.While, Node: l.node(n), X: hir.L(hir.T(hir.Bool), true), Body: hir.B(inner...)}
	out := []*hir.Stmt{oneShot}
	if len(breaks) > 0 {
		out = append(out, &hir.Stmt{Kind: hir.If, Node: l.node(n), X: flag, Body: hir.B(&hir.Stmt{Kind: hir.Break, Node: l.node(n)})})
	}
	out = append(out, l.expressionStatement(update))
	l.diagf(n, "note-for-continue", "continue inside a for loop with an update lowered through a one-shot inner loop")
	return hir.B(out...)
}

func breakTargets(n *ast.Node, into map[*ast.Node]bool) {
	var walk func(*ast.Node)
	walk = func(x *ast.Node) {
		if x == nil {
			return
		}
		switch x.Kind {
		case ast.KindBreakStatement:
			into[x] = true
			return
		case ast.KindForStatement, ast.KindForInStatement, ast.KindForOfStatement, ast.KindWhileStatement, ast.KindDoStatement,
			ast.KindSwitchStatement, ast.KindFunctionExpression, ast.KindArrowFunction, ast.KindFunctionDeclaration:
			return
		}
		x.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(n)
}

// trivialConstructorChain reports whether constructing the class behind sym
// has no effect beyond its own fields: every constructor in the chain only
// calls super with pure or parameter arguments and assigns parameters or
// pure values to own fields; instance field initializers are pure.
func (l *lowerer) trivialConstructorChain(sym *ast.Symbol, visited map[*ast.Symbol]bool) bool {
	if sym == nil || visited[sym] {
		return false
	}
	visited[sym] = true
	if l.classOf(sym) == nil {
		return false
	}
	var decl *ast.Node
	for _, d := range sym.Declarations {
		if d.Kind == ast.KindClassDeclaration {
			decl = d
			break
		}
	}
	if decl == nil {
		return false
	}
	params := map[string]bool{}
	for _, m := range decl.Members() {
		switch m.Kind {
		case ast.KindPropertyDeclaration:
			if m.ModifierFlags()&ast.ModifierFlagsStatic != 0 || m.Initializer() == nil {
				continue
			}
			if !l.pureInitializer(m.Initializer(), m, map[*ast.Node]bool{}) {
				return false
			}
		case ast.KindConstructor:
			for _, p := range m.Parameters() {
				if p.Name() != nil && p.Name().Kind == ast.KindIdentifier {
					params[p.Name().Text()] = true
				}
				if p.Initializer() != nil && !l.pureInitializer(p.Initializer(), nil, map[*ast.Node]bool{}) {
					return false
				}
			}
			if m.Body() == nil {
				continue
			}
			for _, s := range m.Body().AsBlock().Statements.Nodes {
				if !l.trivialConstructorStatement(s, params) {
					return false
				}
			}
		case ast.KindMethodDeclaration, ast.KindGetAccessor, ast.KindSetAccessor, ast.KindIndexSignature:
		default:
			return false
		}
	}
	if heritage := decl.ClassLikeData().HeritageClauses; heritage != nil {
		for _, clauseNode := range heritage.Nodes {
			clause := clauseNode.AsHeritageClause()
			if clause.Token != ast.KindExtendsKeyword || clause.Types == nil {
				continue
			}
			for _, t := range clause.Types.Nodes {
				if t.Expression() == nil || !l.trivialConstructorChain(l.resolve(t.Expression()), visited) {
					return false
				}
			}
		}
	}
	return true
}

func (l *lowerer) trivialConstructorStatement(s *ast.Node, params map[string]bool) bool {
	if s.Kind != ast.KindExpressionStatement {
		return false
	}
	x := s.Expression()
	pureOrParam := func(a *ast.Node) bool {
		if a.Kind == ast.KindIdentifier && params[a.Text()] {
			return true
		}
		if a.Kind == ast.KindObjectLiteralExpression {
			for _, p := range a.AsObjectLiteralExpression().Properties.Nodes {
				if p.Kind != ast.KindPropertyAssignment {
					return false
				}
				v := p.Initializer()
				if !(v.Kind == ast.KindIdentifier && params[v.Text()]) && !l.pureInitializer(v, nil, map[*ast.Node]bool{}) {
					return false
				}
			}
			return true
		}
		return l.pureInitializer(a, nil, map[*ast.Node]bool{})
	}
	switch x.Kind {
	case ast.KindCallExpression:
		if x.Expression() == nil || x.Expression().Kind != ast.KindSuperKeyword {
			return false
		}
		for _, a := range x.Arguments() {
			if !pureOrParam(a) {
				return false
			}
		}
		return true
	case ast.KindBinaryExpression:
		b := x.AsBinaryExpression()
		if b.OperatorToken.Kind != ast.KindEqualsToken || b.Left.Kind != ast.KindPropertyAccessExpression || b.Left.Expression().Kind != ast.KindThisKeyword {
			return false
		}
		return pureOrParam(b.Right)
	}
	return false
}

// libraryCollectionName names the default-library collection constructed by
// a new expression ("Set", "Map", "Array") or returns "".
func (l *lowerer) libraryCollectionName(n *ast.Node) string {
	if n == nil || n.Kind != ast.KindNewExpression || n.Expression() == nil || n.Expression().Kind != ast.KindIdentifier {
		return ""
	}
	sym := l.resolve(n.Expression())
	if sym == nil || l.classOf(sym) != nil {
		return ""
	}
	// A script-global class named Set merges with the library symbol: every
	// declaration must come from the default library and no lowered class of
	// the current file may carry the name.
	if len(sym.Declarations) == 0 || l.classesByName[l.file.FileName()+" "+sym.Name] != nil {
		return ""
	}
	for _, d := range sym.Declarations {
		f := ast.GetSourceFileOfNode(d)
		if f == nil || !l.prog.prog.IsSourceFileDefaultLibrary(f.Path()) {
			return ""
		}
	}
	switch sym.Name {
	case "Set", "Map", "Array":
		return sym.Name
	}
	return ""
}

// pureSyntaxInitializer extends the static-initializer whitelist: the
// `undefined` literal, empty library collections, object literals of pure
// property values and construction of classes with trivial constructor
// chains (lazy ABAP initialization cannot observe a different order).
func (l *lowerer) pureSyntaxInitializer(n, owner *ast.Node, visiting map[*ast.Node]bool) bool {
	pure := func(x *ast.Node) bool { return l.pureInitializer(x, owner, visiting) }
	switch n.Kind {
	case ast.KindIdentifier:
		if n.Text() == "undefined" {
			if _, isLocal := l.lookup("undefined"); !isLocal {
				return true
			}
		}
	case ast.KindObjectLiteralExpression:
		for _, p := range n.AsObjectLiteralExpression().Properties.Nodes {
			if p.Kind != ast.KindPropertyAssignment || p.Name() == nil || p.Name().Kind == ast.KindComputedPropertyName || !pure(p.Initializer()) {
				return false
			}
		}
		return true
	case ast.KindNewExpression:
		if name := l.libraryCollectionName(n); name != "" {
			if len(n.Arguments()) == 0 {
				return name == "Set" || name == "Map"
			}
			return false
		}
		for _, arg := range n.Arguments() {
			if !pure(arg) {
				return false
			}
		}
		if n.Expression() == nil {
			return false
		}
		return l.trivialConstructorChain(l.resolve(n.Expression()), map[*ast.Symbol]bool{})
	}
	return false
}
