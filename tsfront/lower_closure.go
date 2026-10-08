package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Escaping thunks: a parameterless arrow function with a void result that
// is stored (the syntax checker's deferred FORM parameter checks) lowers to
// a closure object. The object captures the enclosing locals it reads, by
// value at creation (they must not be assigned afterwards, so the value is
// the one JavaScript's reference capture would read), and the enclosing
// `this`. Invocation `d()` is the thunk interface's call method.

const thunkInterfaceName = "closure.thunk"

// thunkType returns (creating once) the interface every thunk implements.
func (l *lowerer) thunkType() hir.Type {
	for _, i := range l.out.Interfaces {
		if i.Name == thunkInterfaceName {
			return hir.Type{Kind: hir.InterfaceRef, Name: i.Name}
		}
	}
	i := &hir.Interface{Node: hir.Node{ID: l.nextID(), Source: thunkInterfaceName}, Name: thunkInterfaceName,
		Methods: []*hir.Method{{Node: hir.Node{ID: l.nextID(), Source: thunkInterfaceName}, Name: "call", Virtual: true, Result: hir.T(hir.Void)}}}
	l.out.Interfaces = append(l.out.Interfaces, i)
	if l.ifaceDone == nil {
		l.ifaceDone = map[*hir.Interface]bool{}
	}
	l.ifaceDone[i] = true
	return hir.Type{Kind: hir.InterfaceRef, Name: i.Name}
}

// isThunkType reports a checker type with exactly one call signature that
// takes no parameters and returns void.
func (l *lowerer) isThunkType(t *checker.Type) bool {
	if t == nil {
		return false
	}
	sigs := l.ck.GetSignaturesOfType(t, checker.SignatureKindCall)
	if len(sigs) != 1 || len(sigs[0].Parameters()) != 0 {
		return false
	}
	ret := l.ck.GetReturnTypeOfSignature(sigs[0])
	return ret != nil && ret.Flags()&checker.TypeFlagsVoid != 0
}

// closureValue lowers an arrow function used as a value.
func (l *lowerer) closureValue(n *ast.Node) *hir.Expr {
	if len(n.Parameters()) != 0 {
		l.diagf(n, "unsupported-closure", "only parameterless thunks are lowered as closures")
		return nil
	}
	if t := l.ck.GetTypeAtLocation(n); !l.isThunkType(t) {
		l.diagf(n, "unsupported-closure", "only void thunks are lowered as closures")
		return nil
	}
	body := n.Body()
	if body == nil {
		return nil
	}
	captures := l.freeLocals(n)
	for _, c := range captures {
		if l.assignedAfter(n, c) {
			l.diagf(n, "unsupported-closure", "captured %s is assigned after the closure is created", c)
			return nil
		}
	}
	usesThis := touchesThis(body)
	if usesThis && (l.method == nil || l.method.Static || l.class == nil) {
		l.diagf(n, "unsupported-closure", "closure uses this outside an instance method")
		return nil
	}
	l.serial++
	name := "closure." + itoa(l.serial)
	c := &hir.Class{Node: l.node(n), Name: name, Implements: []string{l.thunkType().Name}}
	ctor := &hir.Method{Node: l.node(n), Name: "constructor", Result: hir.T(hir.Void)}
	var init []*hir.Stmt
	var args []*hir.Expr
	field := func(fname string, t hir.Type, value *hir.Expr) {
		c.Fields = append(c.Fields, hir.Field{Node: l.node(n), Name: fname, Type: t})
		ctor.Params = append(ctor.Params, hir.Param{Name: fname, Type: t})
		init = append(init, &hir.Stmt{Kind: hir.Assign, Node: l.node(n),
			X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: fname, Type: t, X: &hir.Expr{Kind: hir.This, Node: l.node(n), Type: hir.Ref(name)}},
			Y: hir.V(fname, t)})
		args = append(args, value)
	}
	for _, cap := range captures {
		t, ok := l.lookup(cap)
		if !ok || t.Kind == hir.Void {
			l.diagf(n, "unsupported-closure", "captured %s has no lowered type", cap)
			return nil
		}
		field(cap, t, hir.V(cap, t))
	}
	outerType := hir.Type{}
	if usesThis {
		outerType = hir.Ref(l.class.Name)
		field("outer", outerType, l.this(l.class))
	}
	ctor.Body = hir.B(init...)
	c.Ctor = ctor
	call := &hir.Method{Node: l.node(n), Name: "call", Virtual: true, Result: hir.T(hir.Void)}
	c.Methods = append(c.Methods, call)
	l.out.Classes = append(l.out.Classes, c)

	// Lower the body in a scope that sees only the captures (reads of the
	// fields) and the captured this.
	savedScope, savedMethod, savedPend, savedThis := l.scope, l.method, l.pend, l.thisOverride
	l.scope = nil
	l.push()
	self := &hir.Expr{Kind: hir.This, Node: l.node(n), Type: hir.Ref(name)}
	var prologue []*hir.Stmt
	for _, cap := range captures {
		t, _ := savedLookup(savedScope, cap)
		l.declare(cap, t)
		prologue = append(prologue, &hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: cap, Type: t,
			X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: cap, Type: t, X: self}})
	}
	if usesThis {
		l.thisOverride = &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: "outer", Type: outerType, X: self}
	} else {
		l.thisOverride = nil
	}
	l.method = call
	l.pend = nil
	restore := l.withLocalFns(body)
	var inner *hir.Stmt
	if body.Kind == ast.KindBlock {
		inner = l.block(body)
	} else {
		x := l.expr(body)
		pre := l.pend
		l.pend = nil
		if x != nil {
			inner = hir.B(append(pre, &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(body), X: x})...)
		}
	}
	restore()
	l.pop()
	l.scope, l.method, l.pend, l.thisOverride = savedScope, savedMethod, savedPend, savedThis
	if inner == nil {
		return nil
	}
	call.Body = hir.B(append(prologue, inner)...)
	l.diagf(n, "note-closure", "thunk lowered as closure %s capturing %d locals", name, len(captures))
	return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(name), Args: args}
}

func savedLookup(scope []map[string]hir.Type, name string) (hir.Type, bool) {
	for i := len(scope) - 1; i >= 0; i-- {
		if t, ok := scope[i][name]; ok {
			return t, true
		}
	}
	return hir.Type{}, false
}

// assignedAfter reports an assignment or update of the local `name` in the
// enclosing function body positioned after the closure expression.
func (l *lowerer) assignedAfter(fn *ast.Node, name string) bool {
	var body *ast.Node
	for p := fn.Parent; p != nil; p = p.Parent {
		switch p.Kind {
		case ast.KindMethodDeclaration, ast.KindFunctionDeclaration, ast.KindConstructor, ast.KindArrowFunction, ast.KindFunctionExpression, ast.KindGetAccessor, ast.KindSetAccessor:
			body = p.Body()
		}
		if body != nil {
			break
		}
	}
	if body == nil {
		return false
	}
	found := false
	var walk func(*ast.Node)
	walk = func(x *ast.Node) {
		if x == nil || found {
			return
		}
		switch x.Kind {
		case ast.KindBinaryExpression:
			b := x.AsBinaryExpression()
			switch b.OperatorToken.Kind {
			case ast.KindEqualsToken, ast.KindPlusEqualsToken, ast.KindMinusEqualsToken, ast.KindAsteriskEqualsToken, ast.KindSlashEqualsToken, ast.KindQuestionQuestionEqualsToken, ast.KindBarBarEqualsToken, ast.KindAmpersandAmpersandEqualsToken:
				if b.Left.Kind == ast.KindIdentifier && b.Left.Text() == name && x.Pos() > fn.Pos() {
					found = true
					return
				}
			}
		case ast.KindPostfixUnaryExpression, ast.KindPrefixUnaryExpression:
			var operand *ast.Node
			var op ast.Kind
			if x.Kind == ast.KindPostfixUnaryExpression {
				op = x.AsPostfixUnaryExpression().Operator
				operand = x.AsPostfixUnaryExpression().Operand
			} else {
				op = x.AsPrefixUnaryExpression().Operator
				operand = x.AsPrefixUnaryExpression().Operand
			}
			if (op == ast.KindPlusPlusToken || op == ast.KindMinusMinusToken) && operand.Kind == ast.KindIdentifier && operand.Text() == name && x.Pos() > fn.Pos() {
				found = true
				return
			}
		}
		x.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(body)
	return found
}

// thunkCall lowers `d()` for a local holding a thunk.
func (l *lowerer) thunkCall(n *ast.Node, callee *ast.Node) (*hir.Expr, bool) {
	if callee.Kind != ast.KindIdentifier || len(n.Arguments()) != 0 {
		return nil, false
	}
	t, ok := l.lookup(callee.Text())
	if !ok || t.Kind != hir.InterfaceRef || t.Name != thunkInterfaceName {
		return nil, false
	}
	return &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), Name: "call", Type: hir.T(hir.Void), X: hir.V(callee.Text(), t)}, true
}
