package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"strings"
)

// A temporary yield marker exists only during this frontend pass. It never
// reaches HIR verification or an emitter. All surviving nodes are ordinary
// classes, calls, assignments, branches and loops.
func (l *lowerer) lowerGeneratorBody(n *ast.Node, hm *hir.Method) {
	a := l.generatorABIFor(hm.Result)
	if a == nil {
		l.diagf(n, "unsupported-generator", "generator result needs the resumable Generator<T, void, undefined> ABI")
		return
	}
	// Capturing closures need shared lexical cells; try/finally needs iterator
	// closing. Reject both before constructing a machine rather than changing
	// timing or dropping cleanup on break/throw.
	bad := false
	var check func(*ast.Node)
	check = func(x *ast.Node) {
		if x == nil {
			return
		}
		switch x.Kind {
		case ast.KindTryStatement, ast.KindArrowFunction, ast.KindFunctionExpression, ast.KindFunctionDeclaration:
			l.diagf(x, "unsupported-generator", "generator suspension with exception handlers or nested functions needs iterator closing/capture support")
			bad = true
			return
		case ast.KindYieldExpression:
			if x.Parent == nil || x.Parent.Kind != ast.KindExpressionStatement {
				l.diagf(x, "unsupported-generator", "yield must be a standalone statement; sent values are not lowered")
				bad = true
			}
		}
		x.ForEachChild(func(c *ast.Node) bool { check(c); return false })
	}
	check(n.Body())
	if bad {
		return
	}
	saved := l.generatorYield
	l.generatorYield = &hir.Expr{Type: a.element}
	pre := l.applyDefaults(n, hm)
	body := l.block(n.Body())
	l.generatorYield = saved
	machine := &generatorMachine{l: l, abi: a, node: n, fields: map[string]hir.Type{}}
	// Defaults execute when the generator is called, as in JavaScript, while
	// body local initializers execute only on the first next().
	for _, p := range hm.Params {
		machine.fields[p.Name] = p.Type
	}
	if !machine.collect(body) {
		return
	}
	l.serial++
	name := "generator.body." + itoa(l.serial)
	c := &hir.Class{Node: l.node(n), Name: name, Implements: []string{a.iterator.Name}}
	machine.class = c
	machine.self = &hir.Expr{Kind: hir.This, Type: hir.Ref(name)}
	c.Fields = append(c.Fields, hir.Field{Name: "state", Type: hir.T(hir.Number)}, hir.Field{Name: "running", Type: hir.T(hir.Bool)})
	// Stable ordering follows declaration traversal, not Go map iteration.
	for _, p := range hm.Params {
		c.Fields = append(c.Fields, hir.Field{Name: "local_" + p.Name, Type: p.Type})
	}
	for _, key := range machine.order {
		c.Fields = append(c.Fields, hir.Field{Name: "local_" + key, Type: machine.fields[key]})
	}
	ctor := &hir.Method{Node: l.node(n), Name: "constructor", Result: hir.T(hir.Void)}
	var init []*hir.Stmt
	var args []*hir.Expr
	for _, p := range hm.Params {
		ctor.Params = append(ctor.Params, p)
		args = append(args, hir.V(p.Name, p.Type))
		init = append(init, &hir.Stmt{Kind: hir.Assign, X: machine.local(p.Name, p.Type), Y: hir.V(p.Name, p.Type)})
	}
	if !hm.Static {
		t := hir.Ref(l.class.Name)
		c.Fields = append(c.Fields, hir.Field{Name: "outer", Type: t})
		ctor.Params = append(ctor.Params, hir.Param{Name: "outer", Type: t})
		args = append(args, l.this(l.class))
		init = append(init, &hir.Stmt{Kind: hir.Assign, X: &hir.Expr{Kind: hir.FieldGet, X: machine.self, Name: "outer", Type: t}, Y: hir.V("outer", t)})
	}
	// State zero is the terminal result. New instances start at their body entry.
	machine.blocks = append(machine.blocks, hir.B(&hir.Stmt{Kind: hir.Return, X: machine.result(true, nil)}))
	entry := machine.compile(body, 0, 0, 0)
	if machine.failed {
		return
	}
	init = append(init, &hir.Stmt{Kind: hir.Assign, X: machine.state(), Y: hir.L(hir.T(hir.Number), float64(entry))})
	ctor.Body = hir.B(init...)
	c.Ctor = ctor
	next := &hir.Method{Node: l.node(n), Name: "next", Virtual: true, Result: a.result}
	running := &hir.Expr{Kind: hir.FieldGet, X: machine.self, Name: "running", Type: hir.T(hir.Bool)}
	finished := hir.V("generatorNextFinished", hir.T(hir.Bool))
	result := hir.V("generatorNextResult", a.result)
	var dispatch []*hir.Stmt
	for id, b := range machine.blocks {
		test := &hir.Expr{Kind: hir.Binary, Op: "&&", Type: hir.T(hir.Bool), X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: finished}, Y: &hir.Expr{Kind: hir.Binary, Op: "==", Type: hir.T(hir.Bool), X: machine.state(), Y: hir.L(hir.T(hir.Number), float64(id))}}
		// Closing before each body block makes an exception leave the generator
		// exhausted. Only a successful transition installs a continuation.
		dispatch = append(dispatch, &hir.Stmt{Kind: hir.If, X: test, Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: machine.state(), Y: hir.L(hir.T(hir.Number), float64(0))}, finishGeneratorBlock(b, result, finished))})
	}
	next.Body = hir.B(
		&hir.Stmt{Kind: hir.If, X: running, Body: hir.B(&hir.Stmt{Kind: hir.Trap, Name: l.trapLocation(n) + ": reentrant generator next() is unsupported"})},
		&hir.Stmt{Kind: hir.VarDecl, Name: finished.Name, Type: finished.Type, X: hir.L(finished.Type, false)},
		&hir.Stmt{Kind: hir.VarDecl, Name: result.Name, Type: result.Type},
		&hir.Stmt{Kind: hir.Assign, X: running, Y: hir.L(running.Type, true)},
		&hir.Stmt{Kind: hir.Finally, Body: hir.B(&hir.Stmt{Kind: hir.While, X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: finished}, Body: hir.B(dispatch...)}), Else: hir.B(&hir.Stmt{Kind: hir.Assign, X: running, Y: hir.L(running.Type, false)})},
		&hir.Stmt{Kind: hir.Return, X: result})
	close := &hir.Method{Node: l.node(n), Name: "close", Virtual: true, Result: hir.T(hir.Void), Body: hir.B(&hir.Stmt{Kind: hir.Assign, X: machine.state(), Y: hir.L(hir.T(hir.Number), float64(0))})}
	// Delegated iterators created by a suspended loop are closed with the outer
	// generator. Parameters that have not yet been iterated are left alone.
	for _, field := range c.Fields {
		if strings.HasPrefix(field.Name, "local_generatorIter") && l.generatorABIFor(field.Type) != nil {
			value := &hir.Expr{Kind: hir.FieldGet, X: machine.self, Name: field.Name, Type: field.Type}
			close.Body.List = append(close.Body.List, &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.ToBoolean, Type: hir.T(hir.Bool), X: value}, Body: l.generatorClose(value)})
		}
	}
	iterate := &hir.Method{Node: l.node(n), Name: "iterate", Virtual: true, Result: a.iterator, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.Cast, Type: a.iterator, X: machine.self}})}
	c.Methods = append(c.Methods, next, close, iterate)
	l.out.Classes = append(l.out.Classes, c)
	hm.Body = hir.B(append(pre, &hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.Cast, Type: a.iterator, X: &hir.Expr{Kind: hir.New, Type: hir.Ref(name), Args: args}}})...)
	l.diagf(n, "note-generator-iterator", "generator lowered to resumable next() object; eager draining is not assumed")
}

type generatorMachine struct {
	l      *lowerer
	abi    *generatorABI
	node   *ast.Node
	class  *hir.Class
	self   *hir.Expr
	fields map[string]hir.Type
	order  []string
	blocks []*hir.Stmt
	failed bool
}

func (g *generatorMachine) collect(s *hir.Stmt) bool {
	if s == nil {
		return true
	}
	if s.Kind == hir.VarDecl || s.Kind == hir.ForEach {
		if _, ok := g.fields[s.Name]; ok {
			g.l.diagf(g.node, "unsupported-generator", "shadowed generator local %s needs lexical storage identities", s.Name)
			return false
		}
		g.fields[s.Name] = s.Type
		g.order = append(g.order, s.Name)
	}
	if !g.collect(s.Body) || !g.collect(s.Else) {
		return false
	}
	for _, b := range s.List {
		if !g.collect(b) {
			return false
		}
	}
	var visit func(*hir.Expr) bool
	visit = func(x *hir.Expr) bool {
		if x == nil {
			return true
		}
		if !g.collect(x.Stmt) {
			return false
		}
		if !visit(x.X) || !visit(x.Y) || !visit(x.Z) {
			return false
		}
		for _, a := range x.Args {
			if !visit(a) {
				return false
			}
		}
		return true
	}
	return visit(s.X) && visit(s.Y)
}
func (g *generatorMachine) state() *hir.Expr {
	return &hir.Expr{Kind: hir.FieldGet, X: g.self, Name: "state", Type: hir.T(hir.Number)}
}
func (g *generatorMachine) local(name string, t hir.Type) *hir.Expr {
	return &hir.Expr{Kind: hir.FieldGet, X: g.self, Name: "local_" + name, Type: t}
}
func (g *generatorMachine) result(done bool, value *hir.Expr) *hir.Expr {
	opt := hir.T(hir.Optional, g.abi.element)
	if value == nil {
		value = hir.L(opt, nil)
	} else {
		value = g.l.coerce(value, opt)
	}
	return &hir.Expr{Kind: hir.New, Type: g.abi.result, Args: []*hir.Expr{hir.L(hir.T(hir.Bool), done), value}}
}
func (g *generatorMachine) expr(x *hir.Expr) *hir.Expr {
	if x == nil {
		return nil
	}
	y := *x
	if x.Kind == hir.Local {
		return g.local(x.Name, x.Type)
	}
	if x.Kind == hir.This {
		return &hir.Expr{Kind: hir.FieldGet, X: g.self, Name: "outer", Type: x.Type}
	}
	y.X = g.expr(x.X)
	y.Y = g.expr(x.Y)
	y.Z = g.expr(x.Z)
	y.Args = nil
	for _, a := range x.Args {
		y.Args = append(y.Args, g.expr(a))
	}
	y.Stmt = g.storageStmt(x.Stmt)
	return &y
}
func (g *generatorMachine) storageStmt(s *hir.Stmt) *hir.Stmt {
	if s == nil {
		return nil
	}
	y := *s
	y.X = g.expr(s.X)
	y.Y = g.expr(s.Y)
	y.Body = g.storageStmt(s.Body)
	y.Else = g.storageStmt(s.Else)
	y.List = nil
	for _, b := range s.List {
		y.List = append(y.List, g.storageStmt(b))
	}
	if s.Kind == hir.VarDecl {
		return &hir.Stmt{Kind: hir.Assign, X: g.local(s.Name, s.Type), Y: y.X}
	}
	return &y
}
func (g *generatorMachine) jump(id int) *hir.Stmt {
	return hir.B(&hir.Stmt{Kind: hir.Assign, X: g.state(), Y: hir.L(hir.T(hir.Number), float64(id))})
}
func (g *generatorMachine) add(s *hir.Stmt) int {
	g.blocks = append(g.blocks, s)
	return len(g.blocks) - 1
}
func (g *generatorMachine) scratch(t hir.Type) *hir.Expr {
	g.l.serial++
	name := "machine" + itoa(g.l.serial)
	g.fields[name] = t
	g.class.Fields = append(g.class.Fields, hir.Field{Name: "local_" + name, Type: t})
	return g.local(name, t)
}
func (g *generatorMachine) compile(s *hir.Stmt, next, brk, cont int) int {
	if s == nil {
		return next
	}
	switch s.Kind {
	case hir.Block:
		for i := len(s.List) - 1; i >= 0; i-- {
			next = g.compile(s.List[i], next, brk, cont)
		}
		return next
	case hir.If:
		yes := g.compile(s.Body, next, brk, cont)
		no := g.compile(s.Else, next, brk, cont)
		return g.add(hir.B(&hir.Stmt{Kind: hir.If, X: g.expr(s.X), Body: g.jump(yes), Else: g.jump(no)}))
	case hir.While:
		id := g.add(nil)
		body := g.compile(s.Body, id, next, id)
		g.blocks[id] = hir.B(&hir.Stmt{Kind: hir.If, X: g.expr(s.X), Body: g.jump(body), Else: g.jump(next)})
		return id
	case hir.ForEach:
		if s.X.Type.Kind != hir.Array {
			g.l.diagf(g.node, "unsupported-generator", "generator loop requires a lowered array or iterator")
			g.failed = true
			return next
		}
		array := g.scratch(s.X.Type)
		keys := g.expr(s.X)
		var mapValue *hir.Expr
		if g.l.generatorKeyLoops[s] && s.X.Kind == hir.RuntimeOp && s.X.Op == "map.keys" {
			mapValue = g.scratch(s.X.X.Type)
			keys = g.l.rtOp("map.keys", mapValue, s.X.Type)
		}
		index := g.scratch(hir.T(hir.Number))
		test := g.add(nil)
		inc := g.add(hir.B(&hir.Stmt{Kind: hir.Assign, X: index, Y: &hir.Expr{Kind: hir.Binary, Op: "+", Type: index.Type, X: index, Y: hir.L(index.Type, float64(1))}}, g.jump(test)))
		body := g.compile(s.Body, inc, next, inc)
		if mapValue != nil {
			body = g.add(hir.B(&hir.Stmt{Kind: hir.If, X: g.l.rtOp("map.has", mapValue, hir.T(hir.Bool), g.local(s.Name, s.Type)), Body: g.jump(body), Else: g.jump(inc)}))
		}
		read := g.l.rtOp("array.get", array, s.Type, index)
		bind := g.add(hir.B(&hir.Stmt{Kind: hir.Assign, X: g.local(s.Name, s.Type), Y: g.l.presentValue(g.node, read, s.Type)}, g.jump(body)))
		g.blocks[test] = hir.B(&hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Binary, Op: "<", Type: hir.T(hir.Bool), X: index, Y: g.l.rtOp("array.length", array, hir.T(hir.Number))}, Body: g.jump(bind), Else: g.jump(next)})
		init := []*hir.Stmt{}
		if mapValue != nil {
			init = append(init, &hir.Stmt{Kind: hir.Assign, X: mapValue, Y: g.expr(s.X.X)})
		}
		init = append(init, &hir.Stmt{Kind: hir.Assign, X: array, Y: keys}, &hir.Stmt{Kind: hir.Assign, X: index, Y: hir.L(index.Type, float64(0))}, g.jump(test))
		return g.add(hir.B(init...))
	case hir.Break:
		return g.add(g.jump(brk))
	case hir.Continue:
		return g.add(g.jump(cont))
	case hir.Return:
		if s.X != nil {
			g.l.diagf(g.node, "unsupported-generator", "generator return values are not lowered")
			g.failed = true
		}
		return g.add(g.jump(0))
	case hir.ExprStmt:
		if s.X != nil && s.X.Kind == hir.RuntimeOp && s.X.Op == generatorYieldMarker {
			// Compute the yielded value before installing the resume state. If its
			// evaluation throws, the machine remains terminal.
			temp := g.scratch(g.abi.result)
			return g.add(hir.B(&hir.Stmt{Kind: hir.Assign, X: temp, Y: g.result(false, g.expr(s.X.X))}, &hir.Stmt{Kind: hir.Assign, X: g.state(), Y: hir.L(hir.T(hir.Number), float64(next))}, &hir.Stmt{Kind: hir.Return, X: temp}))
		}
	case hir.Try, hir.Finally:
		g.l.diagf(g.node, "unsupported-generator", "generator exception regions require iterator closing support")
		g.failed = true
		return next
	}
	lowered := g.storageStmt(s)
	if lowered.Kind == hir.Assign && lowered.Y == nil {
		return next
	}
	return g.add(hir.B(lowered, g.jump(next)))
}

// Finish a state-machine step without an abrupt control transfer so existing
// HIR Finally can reset the running guard on both success and exception.
func finishGeneratorBlock(s *hir.Stmt, result, finished *hir.Expr) *hir.Stmt {
	if s == nil {
		return nil
	}
	if s.Kind == hir.Return {
		return hir.B(&hir.Stmt{Kind: hir.Assign, X: result, Y: s.X}, &hir.Stmt{Kind: hir.Assign, X: finished, Y: hir.L(finished.Type, true)})
	}
	y := *s
	y.Body = finishGeneratorBlock(s.Body, result, finished)
	y.Else = finishGeneratorBlock(s.Else, result, finished)
	y.List = nil
	for _, b := range s.List {
		y.List = append(y.List, finishGeneratorBlock(b, result, finished))
	}
	return &y
}
