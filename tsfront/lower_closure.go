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
// takes no parameters and returns void (or never, an always-throwing thunk).
func (l *lowerer) isThunkType(t *checker.Type) bool {
	if t == nil {
		return false
	}
	sigs := l.ck.GetSignaturesOfType(t, checker.SignatureKindCall)
	if len(sigs) != 1 || len(sigs[0].Parameters()) != 0 {
		return false
	}
	ret := l.ck.GetReturnTypeOfSignature(sigs[0])
	return ret != nil && ret.Flags()&(checker.TypeFlagsVoid|checker.TypeFlagsNever) != 0
}

// closureValue lowers an arrow function used as a value.
func (l *lowerer) closureValue(n *ast.Node) *hir.Expr {
	before := len(l.diags)
	x := l.closureValueInner(n)
	if !l.integerOptions.TrapOnFailure || x != nil {
		return x
	}
	l.serial++
	name := "closure." + itoa(l.serial)
	m := &hir.Method{Node: l.node(n), Name: "call", Virtual: true, Result: hir.T(hir.Void)}
	l.trapBodyDiagnostics(n, m, before)
	if m.Body == nil {
		m.Body = failureBody(m.Node, l.locOf(n)+": unsupported callback")
	}
	l.out.Classes = append(l.out.Classes, &hir.Class{Node: l.node(n), Name: name, Implements: []string{l.thunkType().Name}, Methods: []*hir.Method{m}})
	return &hir.Expr{Node: l.node(n), Kind: hir.New, Type: hir.Ref(name)}
}

func (l *lowerer) closureValueInner(n *ast.Node) *hir.Expr {
	if l.integerOptions.TrapOnFailure && n.ModifierFlags()&ast.ModifierFlagsAsync != 0 {
		l.diagf(n, "unsupported-async", "async callback is not run by the synchronous test host")
		return nil
	}
	if len(n.Parameters()) != 0 {
		if l.integerOptions.TrapOnFailure && l.isTestCallback(n) {
			l.diagf(n, "unsupported-async", "done/parameterized callback is not run by the synchronous test host")
			return nil
		}
		l.diagf(n, "unsupported-closure", "only parameterless thunks are lowered as closures")
		return nil
	}
	if t := l.ck.GetTypeAtLocation(n); !l.isThunkType(t) && !l.isThunkType(l.ck.GetContextualType(n, 0)) {
		l.diagf(n, "unsupported-closure", "only void thunks are lowered as closures")
		return nil
	}
	body := n.Body()
	if body == nil {
		return nil
	}
	captures := l.freeLocals(n)
	for _, c := range captures {
		if l.assignedAfter(n, c.name) {
			l.diagf(n, "unsupported-closure", "captured %s is assigned after the closure is created", c.name)
			return nil
		}
	}
	usesThis := touchesThis(body)
	contextThis := l.integerOptions.TrapOnFailure && n.Kind == ast.KindFunctionExpression && usesThis && l.isTestCallback(n)
	if contextThis {
		usesThis = false
	}
	if usesThis && (l.method == nil || l.method.Static || l.class == nil) {
		l.diagf(n, "unsupported-closure", "closure uses this outside an instance method")
		return nil
	}
	l.serial++
	name := "closure." + itoa(l.serial)
	c := &hir.Class{Node: l.node(n), Name: name, Implements: []string{l.thunkType().Name}}
	// Preserve the legacy serial-based target name, but identify the thunk by
	// its enclosing qualified method and original TS location for certificates.
	owner := l.relFile(l.file)
	if l.class != nil {
		owner = l.class.Name
		if l.class.SiteOwner != "" {
			owner = l.class.SiteOwner
		}
	}
	if l.method != nil {
		if l.method.SiteOwner != "" {
			owner = l.method.SiteOwner
		} else {
			owner += "." + l.method.Name
		}
	}
	c.SiteOwner = owner + ".[closure@" + l.prog.siteSource(l.locOf(n)) + "]"
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
	for _, captured := range captures {
		cap := captured.name
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
	if contextThis {
		outerType = hir.Ref("runtime/mocha.ts.TestContext")
		field("context", outerType, &hir.Expr{Node: l.node(n), Kind: hir.New, Type: outerType})
	}
	ctor.Body = hir.B(init...)
	c.Ctor = ctor
	call := &hir.Method{Node: l.node(n), Name: "call", Virtual: true, Result: hir.T(hir.Void)}
	call.SiteOwner = c.SiteOwner + ".call"
	c.Methods = append(c.Methods, call)
	l.out.Classes = append(l.out.Classes, c)

	// Lower the body in a scope that sees only the captures (reads of the
	// fields) and the captured this.
	savedScope, savedMethod, savedPend, savedThis := l.scope, l.method, l.pend, l.thisOverride
	l.scope = nil
	l.push()
	self := &hir.Expr{Kind: hir.This, Node: l.node(n), Type: hir.Ref(name)}
	var prologue []*hir.Stmt
	for _, captured := range captures {
		cap := captured.name
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
	if contextThis {
		l.thisOverride = &hir.Expr{Node: l.node(n), Kind: hir.FieldGet, Name: "context", Type: outerType, X: self}
	}
	l.method = call
	before := len(l.diags)
	l.pend = nil
	restore := l.withLocalFns(body)
	inner := l.functionBody(body)
	restore()
	l.pop()
	l.scope, l.method, l.pend, l.thisOverride = savedScope, savedMethod, savedPend, savedThis
	if inner == nil {
		return nil
	}
	call.Body = hir.B(append(prologue, inner)...)
	l.trapBodyDiagnostics(n, call, before)
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

// thunkCall lowers a parameterless thunk held in a local or object field.
func (l *lowerer) thunkCall(n *ast.Node, callee *ast.Node) (*hir.Expr, bool) {
	if l.integerOptions.TrapOnFailure && l.relFile(l.file) == "runtime/mocha.ts" && callee.Kind == ast.KindPropertyAccessExpression && callee.Name().Text() == "call" {
		x := l.expr(callee.Expression())
		if x != nil && x.Type.Name == thunkInterfaceName {
			return &hir.Expr{Node: l.node(n), Kind: hir.VirtualCall, Name: "call", Type: hir.T(hir.Void), X: x}, true
		}
	}
	if len(n.Arguments()) != 0 {
		return nil, false
	}
	// Named functions have their own direct-call path; only stored values
	// use the thunk interface. Do not re-evaluate a method's receiver here.
	if callee.Kind == ast.KindIdentifier {
		t, ok := l.lookup(callee.Text())
		if !ok || t.Kind != hir.InterfaceRef || t.Name != thunkInterfaceName {
			return nil, false
		}
	} else if callee.Kind == ast.KindPropertyAccessExpression {
		if sym := l.resolve(callee); sym != nil && sym.Flags&ast.SymbolFlagsMethod != 0 {
			return nil, false
		}
		if !l.isThunkType(l.ck.GetTypeAtLocation(callee)) {
			return nil, false
		}
	} else {
		return nil, false
	}
	x := l.expr(callee)
	if x == nil {
		return nil, true
	}
	if x.Type.Kind != hir.InterfaceRef || x.Type.Name != thunkInterfaceName {
		return nil, false
	}
	return &hir.Expr{Kind: hir.VirtualCall, Node: l.node(n), Name: "call", Type: hir.T(hir.Void), X: x}, true
}
