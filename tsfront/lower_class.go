package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Class and interface lowering: two passes per file, signatures before
// bodies, so any body can resolve members of any class.

func (l *lowerer) signaturesFile(f *ast.SourceFile) {
	for _, stmt := range f.Statements.Nodes {
		switch stmt.Kind {
		case ast.KindClassDeclaration:
			if c, ok := l.classes[stmt.Symbol()]; ok {
				l.classSignatures(stmt, c)
			}
		case ast.KindInterfaceDeclaration:
			if i, ok := l.ifaces[stmt.Symbol()]; ok {
				l.interfaceSignatures(stmt, i)
			}
		}
	}
}

func (l *lowerer) bodiesFile(f *ast.SourceFile) {
	for _, stmt := range f.Statements.Nodes {
		if stmt.Kind != ast.KindClassDeclaration {
			continue
		}
		c, ok := l.classes[stmt.Symbol()]
		if !ok {
			continue
		}
		l.class = c
		for _, m := range stmt.Members() {
			switch m.Kind {
			case ast.KindMethodDeclaration:
				l.lowerMethodBody(m)
			case ast.KindConstructor:
				l.lowerConstructorBody(m, c)
			}
		}
		l.lowerImplicitInitializers(stmt, c)
	}
}

func (l *lowerer) classSignatures(node *ast.Node, c *hir.Class) {
	c.Abstract = node.ModifierFlags()&ast.ModifierFlagsAbstract != 0
	if heritage := node.ClassLikeData().HeritageClauses; heritage != nil {
		for _, clauseNode := range heritage.Nodes {
			clause := clauseNode.AsHeritageClause()
			if clause.Types == nil {
				continue
			}
			for _, t := range clause.Types.Nodes {
				expr := t.Expression()
				if expr == nil {
					continue
				}
				sym := l.resolve(expr)
				switch {
				case clause.Token == ast.KindExtendsKeyword:
					base := l.classOf(sym)
					if base == nil {
						l.diagf(expr, "unsupported-heritage", "superclass %s is not a lowered class", expr.Text())
						continue
					}
					c.Super = base.Name
				default: // implements
					iface := l.ifaceOf(sym)
					if iface == nil {
						l.diagf(expr, "unsupported-heritage", "interface %s is not a lowered interface", expr.Text())
						continue
					}
					c.Implements = append(c.Implements, iface.Name)
				}
			}
		}
	}
	for _, m := range node.Members() {
		switch m.Kind {
		case ast.KindPropertyDeclaration:
			l.propertySignature(m, c)
		case ast.KindMethodDeclaration:
			l.methodSignature(m, c)
		case ast.KindConstructor:
			l.constructorSignature(m, c)
		default:
			if m.Kind == ast.KindGetAccessor || m.Kind == ast.KindSetAccessor || m.Kind == ast.KindIndexSignature {
				l.diagf(m, "unsupported-member", "%s is not lowered", m.Kind.String())
			}
		}
	}
	if c.Ctor == nil {
		for _, m := range node.Members() {
			if m.Kind == ast.KindPropertyDeclaration && m.Initializer() != nil && m.ModifierFlags()&ast.ModifierFlagsStatic == 0 {
				c.Ctor = &hir.Method{Node: l.node(node), Name: "constructor", Result: hir.T(hir.Void)}
				l.implicitCtors[c] = true
				break
			}
		}
	}

}

// propertySignature lowers one property declaration. Constructor parameter
// properties are handled with the constructor.
func (l *lowerer) propertySignature(m *ast.Node, c *hir.Class) {
	name, ok := l.memberName(m, c)
	if !ok {
		return
	}
	static := m.ModifierFlags()&ast.ModifierFlagsStatic != 0
	var typ hir.Type
	switch {
	case m.Type() != nil:
		typ = l.mapTypeNode(m.Type())
	case m.Initializer() != nil:
		if l.isNewCollection(m.Initializer()) {
			l.diagf(m, "unsupported-member", "field %s with collection initializer needs an annotation", name)
			typ = hir.T(hir.Void)
		} else {
			l.hint = hir.Type{}
			typ = l.mapCheckerType(m, l.ck.GetTypeAtLocation(m.Initializer()))
		}
	default:
		if sym := m.Symbol(); sym != nil {
			typ = l.mapCheckerType(m, l.ck.GetTypeOfSymbol(sym))
		} else {
			l.diagf(m, "unsupported-member", "field %s has no type", name)
			typ = hir.T(hir.Void)
		}
	}
	f := hir.Field{Node: l.node(m), Name: name, Type: typ, Static: static}
	c.Fields = append(c.Fields, f)
	if sym := m.Symbol(); sym != nil {
		l.fields[sym] = f
		if sym.Parent != nil {
			l.fieldsBy[sym.Parent.Name+"."+name] = f
		}
	}
}

func (l *lowerer) methodSignature(m *ast.Node, c *hir.Class) {
	name, ok := l.memberName(m, c)
	if !ok {
		return
	}
	hm := &hir.Method{Node: l.node(m), Name: name}
	hm.Static = m.ModifierFlags()&ast.ModifierFlagsStatic != 0
	hm.Abstract = m.ModifierFlags()&ast.ModifierFlagsAbstract != 0
	hm.Virtual = !hm.Static
	l.signature(m, hm)
	c.Methods = append(c.Methods, hm)
	if sym := m.Symbol(); sym != nil {
		l.methods[sym] = hm
		if sym.Parent != nil {
			l.methodsBy[sym.Parent.Name+"."+name] = hm
		}
	}
}

func (l *lowerer) constructorSignature(m *ast.Node, c *hir.Class) {
	hm := &hir.Method{Node: l.node(m), Name: "constructor"}
	l.signature(m, hm)
	c.Ctor = hm
	// Parameter properties ("constructor(private x: number)") declare fields.
	for _, p := range m.Parameters() {
		if !ast.IsParameterPropertyDeclaration(p, m) {
			continue
		}
		if p.Name() == nil || p.Name().Kind != ast.KindIdentifier {
			l.diagf(p, "unsupported-member", "parameter property with a binding pattern")
			continue
		}
		name := p.Name().Text()
		var typ hir.Type
		if p.Type() != nil {
			typ = l.mapTypeNode(p.Type())
		} else if ps := p.Symbol(); ps != nil {
			typ = l.mapCheckerType(p, l.ck.GetTypeOfSymbol(ps))
		}
		f := hir.Field{Node: l.node(p), Name: name, Type: typ}
		c.Fields = append(c.Fields, f)
		if ps := p.Symbol(); ps != nil {
			l.fields[ps] = f
			if ps.Parent != nil {
				l.fieldsBy[ps.Parent.Name+"."+name] = f
			}
		}
	}
}

// signature fills params and result of hm from the checker's signature.
func (l *lowerer) signature(node *ast.Node, hm *hir.Method) {
	sig := l.ck.GetSignatureFromDeclaration(node)
	checked := map[int]bool{}
	for i, p := range node.Parameters() {
		if p.Initializer() != nil {
			l.diagf(p, "unsupported-param-default", "parameter defaults are not lowered")
		}
		if ast.IsThisParameter(p) {
			continue
		}
		name := ""
		if p.Name() != nil && p.Name().Kind == ast.KindIdentifier {
			name = p.Name().Text()
		} else {
			l.diagf(p, "unsupported-param", "parameter with a binding pattern")
			name = "p" + itoa(i)
		}
		var typ hir.Type
		switch {
		case p.Type() != nil:
			typ = l.mapTypeNode(p.Type())
		case sig != nil:
			ps := p.Symbol()
			if ps == nil {
				l.diagf(p, "unsupported-param", "unresolved parameter %s", name)
				typ = hir.T(hir.Void)
			} else {
				typ = l.mapCheckerType(p, l.ck.GetTypeOfSymbol(ps))
			}
		default:
			typ = hir.T(hir.Void)
		}
		if p.QuestionToken() != nil && typ.Kind != hir.Optional {
			l.diagf(p, "note-optional-param", "optional parameter %s lowered as a required Optional", name)
			typ = hir.T(hir.Optional, typ)
		}
		hm.Params = append(hm.Params, hir.Param{Name: name, Type: typ})
		checked[len(hm.Params)-1] = true
	}
	if node.Type() != nil {
		hm.Result = l.mapTypeNode(node.Type())
	} else if sig != nil {
		hm.Result = l.mapCheckerType(node, l.ck.GetReturnTypeOfSignature(sig))
	} else {
		hm.Result = hir.T(hir.Void)
	}
	if hm.Name == "constructor" && hm.Result.Kind != hir.Void {
		hm.Result = hir.T(hir.Void)
	}
}

func (l *lowerer) interfaceSignatures(node *ast.Node, i *hir.Interface) {
	for _, m := range node.Members() {
		if m.Kind != ast.KindMethodDeclaration && m.Kind != ast.KindMethodSignature {
			l.diagf(m, "unsupported-member", "interface %s has a non-method member", i.Name)
			continue
		}
		name, ok := l.memberName(m, nil)
		if !ok {
			continue
		}
		hm := &hir.Method{Node: l.node(m), Name: name, Virtual: true}
		l.signature(m, hm)
		i.Methods = append(i.Methods, hm)
		if sym := m.Symbol(); sym != nil {
			l.methods[sym] = hm
			if sym.Parent != nil {
				l.methodsBy[sym.Parent.Name+"."+name] = hm
			}
		}
	}
}

// memberName returns the lowered name of a member; computed property names
// (Symbol.for(...)) are reported and skipped.
func (l *lowerer) memberName(m *ast.Node, c *hir.Class) (string, bool) {
	name := m.Name()
	if name == nil {
		return "", false
	}
	switch name.Kind {
	case ast.KindIdentifier, ast.KindPrivateIdentifier:
		return name.Text(), true
	case ast.KindStringLiteral:
		l.diagf(m, "skipped-computed-name", "member with a string name %s is skipped", name.Text())
	default:
		l.diagf(m, "skipped-computed-name", "member with %s name is skipped", name.Kind.String())
	}
	return "", false
}

// lowerMethodBody lowers the body of one method (skipping what the signature
// pass rejected).
func (l *lowerer) lowerMethodBody(m *ast.Node) {
	hm := l.methodOfNode(m)
	if hm == nil {
		return
	}
	l.method = hm
	if m.Body() == nil {
		return
	}
	l.push()
	for _, p := range hm.Params {
		l.declare(p.Name, p.Type)
	}
	hm.Body = l.block(m.Body())
	l.pop()
}

func (l *lowerer) lowerConstructorBody(m *ast.Node, c *hir.Class) {
	hm := c.Ctor
	if hm == nil || m.Body() == nil {
		return
	}
	l.method = hm
	l.push()
	for _, p := range hm.Params {
		l.declare(p.Name, p.Type)
	}
	stmts := []*hir.Stmt{}
	var afterSuper []*ast.Node
	seenSuper := false
	for _, s := range m.Body().AsBlock().Statements.Nodes {
		if !seenSuper && s.Kind == ast.KindExpressionStatement && l.isSuperCall(s.Expression()) {
			if touchesThis(s) {
				l.diagf(s, "unsupported-before-super", "super arguments touch this before initialization")
			} else {
				stmts = append(stmts, l.superConstructorCall(s)...)
			}
			seenSuper = true
			continue
		}
		if c.Super != "" && !seenSuper {
			if touchesThis(s) {
				l.diagf(s, "unsupported-before-super", "statement before super touches this")
				continue
			}
			stmts = append(stmts, l.stmts(s)...)
		} else {
			afterSuper = append(afterSuper, s)
		}
	}
	if c.Super != "" && !seenSuper {
		l.diagf(m, "unsupported-super", "derived constructor requires a top-level super call")
	}
	// Parameter properties and field initializers run in declaration order,
	// after super() and before the constructor body.
	prologue := []*hir.Stmt{}
	for _, p := range m.Parameters() {
		if !ast.IsParameterPropertyDeclaration(p, m) || p.Name() == nil || p.Name().Kind != ast.KindIdentifier {
			continue
		}
		name := p.Name().Text()
		prologue = append(prologue, &hir.Stmt{Kind: hir.Assign, Node: l.node(p),
			X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(p), Name: name, Type: l.declaredFieldType(c, name),
				X: l.this(c)},
			Y: hir.V(name, l.declaredFieldType(c, name))})
	}
	prologue = append(prologue, l.fieldInitializers(m.Parent, c, false)...)
	stmts = append(stmts, prologue...)
	for _, s := range afterSuper {
		stmts = append(stmts, l.stmts(s)...)
	}
	hm.Body = hir.B(stmts...)
	l.pop()
}

// fieldInitializers preserves declaration order, with static and instance
// initialization in their respective ABAP lifecycle methods.
func (l *lowerer) fieldInitializers(node *ast.Node, c *hir.Class, static bool) []*hir.Stmt {
	var out []*hir.Stmt
	for _, mem := range node.Members() {
		if mem.Kind != ast.KindPropertyDeclaration || mem.Initializer() == nil || (mem.ModifierFlags()&ast.ModifierFlagsStatic != 0) != static {
			continue
		}
		name, ok := l.memberName(mem, c)
		if !ok {
			continue
		}
		t := l.declaredFieldType(c, name)
		var target *hir.Expr
		if static {
			target = &hir.Expr{Kind: hir.StaticGet, Node: l.node(mem), Owner: c.Name, Name: name, Type: t}
		} else {
			target = &hir.Expr{Kind: hir.FieldGet, Node: l.node(mem), Name: name, Type: t, X: l.this(c)}
		}
		l.hint = t
		x := l.expr(mem.Initializer())
		l.hint = hir.Type{}
		if x != nil {
			out = append(out, &hir.Stmt{Kind: hir.Assign, Node: l.node(mem), X: target, Y: x})
		}
	}
	return out
}

func (l *lowerer) lowerImplicitInitializers(node *ast.Node, c *hir.Class) {
	if l.implicitCtors[c] {
		l.method = c.Ctor
		l.push()
		var body []*hir.Stmt
		for _, p := range c.Ctor.Params {
			l.declare(p.Name, p.Type)
		}
		if c.Super != "" {
			if base := l.baseConstructor(); base != nil {
				var args []*hir.Expr
				for _, p := range base.Params {
					args = append(args, hir.V(p.Name, p.Type))
				}
				body = append(body, &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(node), X: &hir.Expr{Kind: hir.SuperCall, Node: l.node(node), Name: "constructor", Type: hir.T(hir.Void), Args: args}})
			}
		}
		body = append(body, l.fieldInitializers(node, c, false)...)
		c.Ctor.Body = hir.B(body...)
		l.pop()
	}
	init := &hir.Method{Node: l.node(node), Name: "class_constructor", Static: true, Result: hir.T(hir.Void)}
	l.method = init
	l.push()
	body := l.fieldInitializers(node, c, true)
	l.pop()
	if len(body) > 0 {
		init.Body = hir.B(body...)
		c.Methods = append(c.Methods, init)
	}
}

func touchesThis(n *ast.Node) bool {
	if n.Kind == ast.KindThisKeyword {
		return true
	}
	return n.ForEachChild(touchesThis)
}

func (l *lowerer) declaredFieldType(c *hir.Class, name string) hir.Type {
	for _, f := range c.Fields {
		if f.Name == name {
			return f.Type
		}
	}
	return hir.T(hir.Void)
}

func (l *lowerer) this(c *hir.Class) *hir.Expr {
	return &hir.Expr{Kind: hir.This, Node: hir.Node{ID: l.nextID(), Source: l.method.Source}, Type: hir.Ref(c.Name)}
}

func (l *lowerer) methodOfNode(m *ast.Node) *hir.Method {
	if sym := m.Symbol(); sym != nil {
		return l.methodOf(sym)
	}
	return nil
}

func (l *lowerer) isSuperCall(n *ast.Node) bool {
	return n != nil && n.Kind == ast.KindCallExpression && n.Expression() != nil && n.Expression().Kind == ast.KindSuperKeyword
}

// superConstructorCall lowers super(...) inside a constructor.
func (l *lowerer) superConstructorCall(s *ast.Node) []*hir.Stmt {
	call := s.Expression()
	base := l.baseConstructor()
	if base == nil {
		if len(call.Arguments()) > 0 {
			l.diagf(call, "unsupported-super", "super arguments without a lowered base constructor")
		}
		return nil
	}
	args, ok := l.callArgs(call, call.Arguments(), base.Params)
	if !ok {
		return nil
	}
	return []*hir.Stmt{{Kind: hir.ExprStmt, Node: l.node(call),
		X: &hir.Expr{Kind: hir.SuperCall, Node: l.node(call), Name: "constructor", Type: hir.T(hir.Void), Args: args}}}
}

// baseConstructor resolves the nearest lowered constructor of the superclass
// chain of the class being lowered.
func (l *lowerer) baseConstructor() *hir.Method {
	for name := l.class.Super; name != ""; {
		var found *hir.Class
		for _, c := range l.out.Classes {
			if c.Name == name {
				found = c
				break
			}
		}
		if found == nil {
			return nil
		}
		if found.Ctor != nil {
			return found.Ctor
		}
		name = found.Super
	}
	return nil
}
