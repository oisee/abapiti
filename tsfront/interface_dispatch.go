package tsfront

import (
	"sort"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// interfaceMethodDispatch lowers recv.name(args) where recv's interface (or
// union view) does not declare name but lowered classes behind it do, as in
// JavaScript's dynamic property lookup on the actual object: a runtime
// class test selects the class's method. No matching class throws a
// TypeError (name is not a function); an optional call recv.name?.(args)
// yields undefined instead. Every candidate must agree on parameter and
// result types; the arguments are evaluated once, before the test.
func (l *lowerer) interfaceMethodDispatch(n *ast.Node, name string, recv *hir.Expr) (*hir.Expr, bool) {
	if !l.unprunedFeatures || recv.Type.Kind != hir.InterfaceRef || n.Kind != ast.KindCallExpression {
		return nil, false
	}
	type candidate struct {
		class  string
		method *hir.Method
	}
	var candidates []candidate
	for _, c := range l.out.Classes {
		if c.Structural || l.shapeLike[c.Name] || !(l.acceptsType(recv.Type, hir.Ref(c.Name)) || l.structurallyImplements(c, recv.Type.Name)) {
			continue
		}
		for owner := c; owner != nil; owner = l.classByName(owner.Super) {
			var found *hir.Method
			for _, m := range owner.Methods {
				if m.Name == name && !m.Static {
					found = m
				}
			}
			if found != nil {
				candidates = append(candidates, candidate{c.Name, found})
				break
			}
		}
	}
	if len(candidates) == 0 {
		return nil, false
	}
	// Most derived classes first, so a subclass override wins its test.
	sort.SliceStable(candidates, func(i, j int) bool {
		return l.classDepth(candidates[i].class) > l.classDepth(candidates[j].class)
	})
	first := candidates[0].method
	sameParams := true
	for _, c := range candidates[1:] {
		if len(c.method.Params) != len(first.Params) || !c.method.Result.Equal(first.Result) {
			l.diagf(n, "unsupported-call", "method %s differs in signature between the classes behind %s", name, recv.Type.Name)
			return nil, true
		}
		for i := range first.Params {
			sameParams = sameParams && c.method.Params[i].Type.Equal(first.Params[i].Type)
		}
	}
	if !sameParams {
		// Differing parameter types: each class converts the arguments
		// itself, which is exact only for side-effect-free argument forms.
		for _, a := range n.Arguments() {
			if !pureArgument(a) {
				l.diagf(n, "unsupported-call", "method %s differs in parameter types between the classes behind %s", name, recv.Type.Name)
				return nil, true
			}
		}
	}
	optionalCall := n.AsCallExpression().QuestionDotToken != nil
	r := l.tempInit(n, recv.Type, recv)
	argsFor := map[string][]*hir.Expr{}
	if sameParams {
		args, ok := l.callArgsMethod(n, n.Arguments(), first, 0)
		if !ok {
			return nil, true
		}
		for i, a := range args {
			args[i] = l.tempInit(n, a.Type, a)
		}
		for _, c := range candidates {
			argsFor[c.class] = args
		}
	} else {
		for _, c := range candidates {
			args, ok := l.callArgsMethod(n, n.Arguments(), c.method, 0)
			if !ok {
				return nil, true
			}
			argsFor[c.class] = args
		}
	}
	result := first.Result
	var out *hir.Expr
	if result.Kind != hir.Void {
		slot := result
		if slot.Kind != hir.Optional {
			slot = hir.T(hir.Optional, slot)
		}
		out = l.tempInit(n, slot, &hir.Expr{Kind: hir.Lit, Node: l.node(n), Type: slot})
	}
	var fallback *hir.Stmt
	if !optionalCall {
		errorType := hir.Ref(l.builtinTypeError())
		fallback = &hir.Stmt{Kind: hir.Throw, Node: l.node(n), X: &hir.Expr{Kind: hir.New, Node: l.node(n), Type: errorType,
			Args: []*hir.Expr{l.coerce(hir.L(hir.T(hir.String), name+" is not a function"), hir.T(hir.Optional, hir.T(hir.String)))}}}
	}
	chain := fallback
	for i := len(candidates) - 1; i >= 0; i-- {
		c := candidates[i]
		view := &hir.Expr{Kind: hir.Cast, Node: l.node(n), Type: hir.Ref(c.class), X: r}
		call := &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), Type: c.method.Result, Name: name, X: view, Args: argsFor[c.class]}
		body := &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: call}
		if out != nil {
			body = &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: out, Y: l.coerce(call, out.Type)}
		}
		test := &hir.Expr{Kind: hir.InstanceOf, Node: l.node(n), Type: hir.T(hir.Bool), X: r, Owner: c.class}
		stmt := &hir.Stmt{Kind: hir.If, Node: l.node(n), X: test, Body: hir.B(body)}
		if chain != nil {
			stmt.Else = hir.B(chain)
		}
		chain = stmt
	}
	l.pendStmt(chain)
	l.diagf(n, "note-interface-dispatch", "%s is not declared by %s; dispatched to %d implementing classes", name, recv.Type.Name, len(candidates))
	if out == nil {
		return &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: hir.T(hir.Bool), Stmt: hir.B(), Y: hir.L(hir.T(hir.Bool), false)}, true
	}
	if optionalCall || result.Kind == hir.Optional {
		return out, true
	}
	return &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: result, X: out}, true
}

func (l *lowerer) classDepth(name string) int {
	depth := 0
	for c := l.classByName(name); c != nil; c = l.classByName(c.Super) {
		depth++
	}
	return depth
}

// pureArgument: literals, undefined/null and plain identifiers evaluate
// without effects, so lowering them once per dispatch branch is exact.
func pureArgument(a *ast.Node) bool {
	switch a.Kind {
	case ast.KindStringLiteral, ast.KindNumericLiteral, ast.KindNullKeyword, ast.KindTrueKeyword, ast.KindFalseKeyword, ast.KindIdentifier, ast.KindNoSubstitutionTemplateLiteral:
		return true
	}
	return false
}

// structurallyImplements: TypeScript interfaces are structural; a class whose
// chain declares every method of the interface can stand behind it even
// before an assignment recorded the implicit implementation.
func (l *lowerer) structurallyImplements(c *hir.Class, iface string) bool {
	var in *hir.Interface
	for _, candidate := range l.out.Interfaces {
		if candidate.Name == iface {
			in = candidate
		}
	}
	if in == nil || len(in.Methods) == 0 {
		return false
	}
	for _, m := range in.Methods {
		found := false
		for owner := c; owner != nil && !found; owner = l.classByName(owner.Super) {
			for _, cm := range owner.Methods {
				found = found || cm.Name == m.Name && !cm.Static
			}
		}
		if !found {
			return false
		}
	}
	return true
}
