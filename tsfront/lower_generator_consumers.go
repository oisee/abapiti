package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

func (l *lowerer) generatorLoop(x *hir.Expr, name string, body *hir.Stmt) *hir.Stmt {
	a := l.generatorABIFor(x.Type)
	l.serial++
	id := itoa(l.serial)
	iter := hir.V("generatorIter"+id, a.iterator)
	step := hir.V("generatorStep"+id, a.result)
	body = l.closeGeneratorConsumer(body, iter, 0)
	return hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: iter.Name, Type: iter.Type, X: &hir.Expr{Kind: hir.VirtualCall, X: x, Name: "iterate", Type: a.iterator}},
		&hir.Stmt{Kind: hir.While, X: hir.L(hir.T(hir.Bool), true), Body: hir.B(
			&hir.Stmt{Kind: hir.VarDecl, Name: step.Name, Type: step.Type, X: &hir.Expr{Kind: hir.VirtualCall, X: iter, Name: "next", Type: a.result}},
			&hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.FieldGet, X: step, Name: "done", Type: hir.T(hir.Bool)}, Body: hir.B(&hir.Stmt{Kind: hir.Break})},
			&hir.Stmt{Kind: hir.VarDecl, Name: name, Type: a.element, X: &hir.Expr{Kind: hir.Narrow, Type: a.element, X: &hir.Expr{Kind: hir.FieldGet, X: step, Name: "value", Type: hir.T(hir.Optional, a.element)}}}, body)})
}
func (l *lowerer) drainGenerator(x *hir.Expr) *hir.Expr {
	a := l.generatorABIFor(x.Type)
	l.serial++
	id := itoa(l.serial)
	out := hir.V("generatorArray"+id, hir.T(hir.Array, a.element))
	name := "generatorValue" + id
	push := &hir.Stmt{Kind: hir.ExprStmt, X: l.rtOp("array.push", out, hir.T(hir.Number), hir.V(name, a.element))}
	return &hir.Expr{Kind: hir.Seq, Type: out.Type, Y: out, Stmt: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: out.Name, Type: out.Type, X: &hir.Expr{Kind: hir.New, Type: out.Type}}, l.generatorLoop(x, name, push))}
}

func (l *lowerer) generatorClose(iter *hir.Expr) *hir.Stmt {
	return &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.VirtualCall, X: iter, Name: "close", Type: hir.T(hir.Void)}}
}

// IteratorClose runs on an owned break, return or consumer exception. Continue
// and breaks of inner loops keep the outer iterator alive. Generator bodies
// themselves use the machine's exception/close path rather than nested finally.
func (l *lowerer) closeGeneratorConsumer(s *hir.Stmt, iter *hir.Expr, depth int) *hir.Stmt {
	if s == nil {
		return nil
	}
	if s.Kind == hir.Try || s.Kind == hir.Finally {
		if generatorConsumerExits(s) {
			l.diags = append(l.diags, LowerDiagnostic{Category: "unsupported-generator", Loc: s.Source, Message: "generator consumer exception regions with return/break/continue need structured iterator closing"})
			return s
		}
		// Protect the entire region: a caught body exception must not close the
		// iterator. Only an exception escaping the catch/finally closes it.
		l.serial++
		done := hir.V("generatorRegionDone"+itoa(l.serial), hir.T(hir.Bool))
		return hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: done.Name, Type: done.Type, X: hir.L(done.Type, false)}, &hir.Stmt{Kind: hir.Finally, Body: hir.B(s, &hir.Stmt{Kind: hir.Assign, X: done, Y: hir.L(done.Type, true)}), Else: hir.B(&hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: done.Type, X: done}, Body: l.generatorClose(iter)})})
	}
	y := *s
	if s.Kind == hir.Break && depth == 0 {
		return hir.B(l.generatorClose(iter), s)
	}
	if s.Kind == hir.Return || s.Kind == hir.Throw {
		if s.X == nil {
			return hir.B(l.generatorClose(iter), s)
		}
		l.serial++
		name := "generatorExit" + itoa(l.serial)
		value := hir.V(name, s.X.Type)
		y.X = value
		return hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: name, Type: value.Type, X: l.protectGeneratorConsumerExpr(s.X, iter)}, l.generatorClose(iter), &y)
	}
	if s.Kind == hir.ExprStmt && s.X != nil && s.X.Type.Kind == hir.Void && l.generatorYield == nil {
		l.serial++
		done := hir.V("generatorEvalDone"+itoa(l.serial), hir.T(hir.Bool))
		return hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: done.Name, Type: done.Type, X: hir.L(done.Type, false)}, &hir.Stmt{Kind: hir.Finally, Body: hir.B(s, &hir.Stmt{Kind: hir.Assign, X: done, Y: hir.L(done.Type, true)}), Else: hir.B(&hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: done.Type, X: done}, Body: l.generatorClose(iter)})})
	}
	if s.X != nil && s.Kind != hir.Assign {
		y.X = l.protectGeneratorConsumerExpr(s.X, iter)
	}
	if s.Kind == hir.Assign {
		// Lvalue receiver/index evaluation can throw as well. Protect its operands
		// while retaining the writable field/index node itself.
		target := *s.X
		target.X = l.protectGeneratorConsumerExpr(target.X, iter)
		target.Y = l.protectGeneratorConsumerExpr(target.Y, iter)
		y.X = &target
		y.Y = l.protectGeneratorConsumerExpr(s.Y, iter)
	}
	nextDepth := depth
	if s.Kind == hir.ForEach || s.Kind == hir.While {
		nextDepth++
	}
	y.Body = l.closeGeneratorConsumer(s.Body, iter, nextDepth)
	y.Else = l.closeGeneratorConsumer(s.Else, iter, depth)
	y.List = nil
	for _, b := range s.List {
		y.List = append(y.List, l.closeGeneratorConsumer(b, iter, depth))
	}
	return &y
}
func (l *lowerer) protectGeneratorConsumerExpr(x, iter *hir.Expr) *hir.Expr {
	if x == nil || x.Type.Kind == hir.Void || l.generatorYield != nil {
		return x
	}
	// Plain reads/literals cannot throw, and keeping them plain avoids expensive
	// exception regions in ordinary membership checks.
	if x.Kind == hir.Lit || x.Kind == hir.Local || x.Kind == hir.This {
		return x
	}
	l.serial++
	id := itoa(l.serial)
	done := hir.V("generatorEvalDone"+id, hir.T(hir.Bool))
	value := hir.V("generatorEvalValue"+id, x.Type)
	pre := []*hir.Stmt{{Kind: hir.VarDecl, Name: value.Name, Type: value.Type}}
	body := []*hir.Stmt{{Kind: hir.Assign, X: value, Y: x}}
	body = append(body, &hir.Stmt{Kind: hir.Assign, X: done, Y: hir.L(done.Type, true)})
	pre = append(pre, &hir.Stmt{Kind: hir.VarDecl, Name: done.Name, Type: done.Type, X: hir.L(done.Type, false)}, &hir.Stmt{Kind: hir.Finally, Body: hir.B(body...), Else: hir.B(&hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: done.Type, X: done}, Body: l.generatorClose(iter)})})

	return &hir.Expr{Kind: hir.Seq, Type: x.Type, Y: value, Stmt: hir.B(pre...)}
}

// Keep the existing for-in lowering intact. Inside a suspended generator,
// retain its receiver and skip keys deleted before the iterator visits them.
func (l *lowerer) generatorForIn(n *ast.Node) *hir.Stmt {
	expression := n.AsForInOrOfStatement().Expression
	// The fallback [] contributes no keys. Indexed record reads preserve
	// absence in HIR, so the existing optional-record loop handles both arms.
	old := len(l.replacements)
	if expression.Kind == ast.KindBinaryExpression {
		b := expression.AsBinaryExpression()
		if b.OperatorToken.Kind == ast.KindBarBarToken && b.Right.Kind == ast.KindArrayLiteralExpression && len(b.Right.AsArrayLiteralExpression().Elements.Nodes) == 0 {
			value := l.expr(b.Left)
			if value != nil && (value.Type.Kind == hir.OrderedMap || (value.Type.Kind == hir.Optional && value.Type.Args[0].Kind == hir.OrderedMap)) {
				l.replacements = append(l.replacements, [2]any{expression, value})
			}
		}
	}
	s := l.forInStatement(n)
	l.replacements = l.replacements[:old]
	if l.generatorKeyLoops == nil {
		l.generatorKeyLoops = map[*hir.Stmt]bool{}
	}
	var mark func(*hir.Stmt)
	mark = func(x *hir.Stmt) {
		if x == nil {
			return
		}
		if x.Kind == hir.ForEach {
			l.generatorKeyLoops[x] = true
			return
		}
		mark(x.Body)
		mark(x.Else)
		for _, b := range x.List {
			mark(b)
		}
	}
	mark(s)
	return s
}

func checkerHasGenerator(t *checker.Type) bool {
	if t == nil {
		return false
	}
	if t.IsUnion() {
		for _, part := range t.AsUnionOrIntersectionType().Types() {
			if checkerHasGenerator(part) {
				return true
			}
		}
		return false
	}
	return t.Symbol() != nil && t.Symbol().Name == "Generator"
}

// An absent optional generator and [] both enumerate nothing. Preserve lazy
// iteration in the present arm rather than draining it just to unify the ABI.
func (l *lowerer) optionalGeneratorForOf(n, decl *ast.Node) (*hir.Stmt, bool) {
	f := n.AsForInOrOfStatement()
	if f.Expression.Kind != ast.KindBinaryExpression || decl.Name().Kind != ast.KindIdentifier {
		return nil, false
	}
	b := f.Expression.AsBinaryExpression()
	if b.OperatorToken.Kind != ast.KindBarBarToken || b.Right.Kind != ast.KindArrayLiteralExpression || len(b.Right.AsArrayLiteralExpression().Elements.Nodes) != 0 || !checkerHasGenerator(l.ck.GetTypeAtLocation(b.Left)) {
		return nil, false
	}
	x := l.expr(b.Left)
	if x == nil {
		return nil, true
	}
	base := x.Type
	optional := base.Kind == hir.Optional
	if optional {
		base = base.Args[0]
	}
	a := l.generatorABIFor(base)
	if a == nil {
		return nil, false
	} // pinned eager ABI
	l.push()
	name := decl.Name().Text()
	l.declare(name, a.element)
	body := l.scopeBlock(f.Statement)
	l.pop()
	if !optional {
		return l.generatorLoop(x, name, body), true
	}
	l.serial++
	value := hir.V("optionalGenerator"+itoa(l.serial), x.Type)
	present := &hir.Expr{Kind: hir.Narrow, Type: base, X: value}
	return hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: value.Name, Type: value.Type, X: x}, &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: value}}, Body: l.generatorLoop(present, name, body)}), true
}

// Iterable parameters accept arrays through a lazy iterator adapter. It reads
// each element and the current length on next(), preserving mutation between
// consumer steps; a completed array iterator stays exhausted after append.
func (l *lowerer) generatorArrayAdapter(x *hir.Expr, dst hir.Type) *hir.Expr {
	a := l.generatorABIFor(dst)
	if a.arrayClass.Kind == "" {
		a.arrayClass = hir.Ref("generator.array." + typeSig(a.element)[:16])
		c := &hir.Class{Name: a.arrayClass.Name, Implements: []string{a.iterator.Name}}
		self := &hir.Expr{Kind: hir.This, Type: a.arrayClass}
		arrayType := hir.T(hir.Array, a.element)
		field := func(name string, t hir.Type) *hir.Expr {
			return &hir.Expr{Kind: hir.FieldGet, X: self, Name: name, Type: t}
		}
		array := field("array", arrayType)
		index := field("index", hir.T(hir.Number))
		closed := field("closed", hir.T(hir.Bool))
		c.Fields = []hir.Field{{Name: "array", Type: arrayType}, {Name: "index", Type: index.Type}, {Name: "closed", Type: closed.Type}}
		c.Ctor = &hir.Method{Name: "constructor", Params: []hir.Param{{Name: "array", Type: arrayType}}, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: array, Y: hir.V("array", arrayType)})}
		end := &hir.Expr{Kind: hir.New, Type: a.result, Args: []*hir.Expr{hir.L(hir.T(hir.Bool), true), hir.L(hir.T(hir.Optional, a.element), nil)}}
		exhausted := &hir.Expr{Kind: hir.Binary, Op: ">=", Type: hir.T(hir.Bool), X: index, Y: l.rtOp("array.length", array, hir.T(hir.Number))}
		value := hir.V("arrayIteratorValue", hir.T(hir.Optional, a.element))
		next := &hir.Method{Name: "next", Virtual: true, Result: a.result, Body: hir.B(
			&hir.Stmt{Kind: hir.If, X: closed, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: end})},
			&hir.Stmt{Kind: hir.If, X: exhausted, Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: closed, Y: hir.L(closed.Type, true)}, &hir.Stmt{Kind: hir.Return, X: end})},
			&hir.Stmt{Kind: hir.VarDecl, Name: value.Name, Type: value.Type, X: l.rtOp("array.get", array, value.Type, index)},
			&hir.Stmt{Kind: hir.Assign, X: index, Y: &hir.Expr{Kind: hir.Binary, Op: "+", Type: index.Type, X: index, Y: hir.L(index.Type, float64(1))}},
			&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Type: a.result, Args: []*hir.Expr{hir.L(hir.T(hir.Bool), false), value}}})}
		close := &hir.Method{Name: "close", Virtual: true, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: closed, Y: hir.L(closed.Type, true)})}
		iterate := &hir.Method{Name: "iterate", Virtual: true, Result: a.iterator, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.Cast, Type: a.iterator, X: &hir.Expr{Kind: hir.New, Type: a.arrayClass, Args: []*hir.Expr{array}}}})}
		c.Methods = []*hir.Method{next, close, iterate}
		l.out.Classes = append(l.out.Classes, c)
	}
	return &hir.Expr{Kind: hir.Cast, Type: dst, X: &hir.Expr{Kind: hir.New, Type: a.arrayClass, Args: []*hir.Expr{x}}}
}

func generatorConsumerExits(s *hir.Stmt) bool {
	if s == nil {
		return false
	}
	if s.Kind == hir.Return || s.Kind == hir.Break || s.Kind == hir.Continue {
		return true
	}
	if generatorConsumerExits(s.Body) || generatorConsumerExits(s.Else) {
		return true
	}
	for _, b := range s.List {
		if generatorConsumerExits(b) {
			return true
		}
	}
	return false
}
