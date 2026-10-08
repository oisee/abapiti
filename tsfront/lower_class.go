package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Class and interface lowering: two passes per file, signatures before
// bodies, so any body can resolve members of any class.

func (l *lowerer) signaturesFile(f *ast.SourceFile) {
	for _, stmt := range f.Statements.Nodes {
		switch stmt.Kind {
		case ast.KindClassDeclaration:
			if c, ok := l.classes[stmt.Symbol()]; ok {
				l.classSignatures(stmt, c)
				l.synthesizeDerivedCtor(c)
			}
		case ast.KindInterfaceDeclaration:
			if l.isDataInterface(stmt) {
				c := l.classOf(stmt.Symbol())
				if c != nil {
					l.dataInterfaceClass(stmt, &hir.Interface{Node: c.Node, Name: c.Name})
				}
				continue
			}
			i, ok := l.ifaces[stmt.Symbol()]
			if !ok {
				continue
			}
			if l.isDataInterface(stmt) {
				l.dataInterfaceClass(stmt, i)
				continue
			}
			if len(i.Methods) == 0 {
				l.interfaceSignatures(stmt, i)
			}
		}
	}
}

// isDataInterface reports whether the interface has a property member: those
// lower to synthesized shape classes (constructed like object literals).
func (l *lowerer) isDataInterface(node *ast.Node) bool {
	for _, m := range node.Members() {
		if m.Kind == ast.KindPropertySignature || m.Kind == ast.KindPropertyDeclaration {
			return true
		}
	}
	return false
}

// dataInterfaceClass converts a registered interface with data members into
// a synthesized class with a constructing ctor.
func (l *lowerer) dataInterfaceClass(node *ast.Node, i *hir.Interface) {
	sym := node.Symbol()
	// Remove the interface from the registries.
	delete(l.ifaces, sym)
	delete(l.ifacesByName, l.file.FileName()+" "+sym.Name)
	for k, x := range l.out.Interfaces {
		if x == i {
			l.out.Interfaces = append(l.out.Interfaces[:k], l.out.Interfaces[k+1:]...)
			break
		}
	}
	c := l.classOf(sym)
	if c == nil {
		c = &hir.Class{Node: i.Node, Name: i.Name}
		l.out.Classes = append(l.out.Classes, c)
	}
	if c.Ctor != nil {
		return
	}
	ctor := &hir.Method{Node: i.Node, Name: "constructor", Result: hir.T(hir.Void)}
	c.Ctor = ctor
	var list []*hir.Stmt
	if heritage := node.AsInterfaceDeclaration().HeritageClauses; heritage != nil {
		for _, clause := range heritage.Nodes {
			for _, baseNode := range clause.AsHeritageClause().Types.Nodes {
				sym := l.resolve(baseNode.Expression())
				if sym == nil {
					continue
				}
				base := l.classOf(sym)
				if base != nil && base.Ctor == nil && len(sym.Declarations) > 0 && sym.Declarations[0].Kind == ast.KindInterfaceDeclaration {
					decl := sym.Declarations[0]
					saved := l.file
					l.file = ast.GetSourceFileOfNode(decl)
					l.dataInterfaceClass(decl, &hir.Interface{Node: base.Node, Name: base.Name})
					l.file = saved
				}
				if base == nil {
					base = l.synthFromAlias(baseNode, sym)
				}
				if base == nil {
					l.diagf(baseNode, "unsupported-type", "data-interface base is not a shape")
					continue
				}
				for _, field := range base.Fields {
					field.Node = l.node(node)
					c.Fields = append(c.Fields, field)
					ctor.Params = append(ctor.Params, hir.Param{Name: field.Name, Type: field.Type})
					list = append(list, &hir.Stmt{Kind: hir.Assign, Node: l.node(node), X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(node), Name: field.Name, Type: field.Type, X: &hir.Expr{Kind: hir.This, Node: l.node(node), Type: hir.Ref(c.Name)}}, Y: hir.V(field.Name, field.Type)})
				}
			}
		}
	}
	for _, m := range node.Members() {
		if m.Kind != ast.KindPropertySignature && m.Kind != ast.KindPropertyDeclaration {
			l.diagf(m, "skipped-interface-member", "data interface %s: member %s is skipped", i.Name, m.Kind.String())
			continue
		}
		if m.Name() == nil || (m.Name().Kind != ast.KindIdentifier && m.Name().Kind != ast.KindStringLiteral && m.Name().Kind != ast.KindPrivateIdentifier) {
			l.diagf(m, "skipped-computed-name", "data interface %s has a computed property name", i.Name)
			continue
		}
		name := m.Name().Text()
		var ft hir.Type
		before := len(l.diags)
		if e, ok := l.overrides[node]; ok && e.Types[m.Name().Text()].Kind != "" {
			ft = e.Types[m.Name().Text()]
			l.diagf(m, "note-override", "%s: %s", e.ID, e.Rationale)
		} else if m.Type() != nil {
			ft = l.mapTypeNode(m.Type())
		} else if ms := m.Symbol(); ms != nil {
			ft = l.mapCheckerType(m, l.ck.GetTypeOfSymbol(ms))
		}
		if hasBlocking(l.diags[before:]) || ft.Kind == hir.Void {
			l.diags = l.diags[:before]
			l.diagf(m, "skipped-interface-member", "data interface %s: member %s has unlowered types", i.Name, name)
			continue
		}
		if m.QuestionToken() != nil && ft.Kind != hir.Optional {
			ft = hir.T(hir.Optional, ft)
		}
		c.Fields = append(c.Fields, hir.Field{Node: l.node(m), Name: name, Type: ft})
		ctor.Params = append(ctor.Params, hir.Param{Name: name, Type: ft})
		list = append(list, &hir.Stmt{Kind: hir.Assign, Node: l.node(m),
			X: &hir.Expr{Kind: hir.FieldGet, Node: l.node(m), Name: name, Type: ft,
				X: &hir.Expr{Kind: hir.This, Node: l.node(m), Type: hir.Ref(c.Name)}},
			Y: hir.V(name, ft)})
		if ms := m.Symbol(); ms != nil {
			l.fields[ms] = hir.Field{Node: l.node(m), Name: name, Type: ft}
		}
	}
	ctor.Body = hir.B(list...)
	c.Ctor = ctor
	l.classesByName[l.file.FileName()+" "+sym.Name] = c
	l.diagf(node, "note-data-interface", "interface %s with data members lowered to a shape class", i.Name)
}

// synthesizeDerivedCtor gives a derived class without its own constructor a
// constructor forwarding to the base (TS synthesizes (...args) => super(...args)).
func (l *lowerer) synthesizeDerivedCtor(c *hir.Class) {
	if c.Ctor != nil || c.Super == "" {
		return
	}
	base := l.baseConstructorOf(c.Super)
	if base == nil {
		return
	}
	params := []hir.Param{}
	args := []*hir.Expr{}
	for _, p := range base.Params {
		params = append(params, hir.Param{Name: p.Name, Type: p.Type})
		args = append(args, hir.V(p.Name, p.Type))
	}
	c.Ctor = &hir.Method{Node: c.Node, Name: "constructor", Params: params, Result: hir.T(hir.Void),
		Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: c.Node,
			X: &hir.Expr{Kind: hir.SuperCall, Node: c.Node, Name: "constructor", Type: hir.T(hir.Void), Args: args}})}
}

// baseConstructorOf resolves the nearest lowered constructor of a class
// chain by name.
func (l *lowerer) baseConstructorOf(name string) *hir.Method {
	for name != "" {
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
			case ast.KindPropertyDeclaration:
				// Initializers are handled below and in constructors.
			default:
				l.diagf(m, "unsupported-member", "%s is not lowered", m.Kind.String())
			}
		}
		l.lowerImplicitInitializers(stmt, c)
	}
}

func (l *lowerer) classSignatures(node *ast.Node, c *hir.Class) {
	l.class = c
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
						if sym != nil && sym.Name == "Error" && l.classOf(sym) == nil {
							c.Super = l.builtinError()
							continue
						}
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
			l.diagf(m, "unsupported-member", "%s is not lowered", m.Kind.String())
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
	if m.QuestionToken() != nil && typ.Kind != hir.Optional {
		typ = hir.T(hir.Optional, typ)
	}
	f := hir.Field{Node: l.node(m), Name: name, Type: typ, Static: static, Private: m.ModifierFlags()&ast.ModifierFlagsPrivate != 0, Readonly: m.ModifierFlags()&ast.ModifierFlagsReadonly != 0}
	c.Fields = append(c.Fields, f)
	if sym := m.Symbol(); sym != nil {
		l.fields[sym] = f
		if sym.Parent != nil {
			if pf := l.fileOfSymbol(sym.Parent); pf != nil {
				l.fieldsBy[pf.FileName()+" "+sym.Parent.Name+"."+name] = f
			}
		}
	}
}

func (l *lowerer) methodSignature(m *ast.Node, c *hir.Class) {
	name, ok := l.memberName(m, c)
	if !ok {
		return
	}
	hm := &hir.Method{Node: l.node(m), Name: name, Internal: m.ModifierFlags()&ast.ModifierFlagsPrivate != 0}
	hm.Static = m.ModifierFlags()&ast.ModifierFlagsStatic != 0
	hm.Abstract = m.ModifierFlags()&ast.ModifierFlagsAbstract != 0
	hm.Virtual = !hm.Static
	if e, ok := l.overrides[m]; ok && e.Method != nil {
		hm = e.Method()
		hm.Node = l.node(m)
		l.diagf(m, "note-override", "%s: %s", e.ID, e.Rationale)
	} else {
		l.signature(m, hm)
	}
	c.Methods = append(c.Methods, hm)
	if sym := m.Symbol(); sym != nil {
		l.methods[sym] = hm
		if sym.Parent != nil {
			if pf := l.fileOfSymbol(sym.Parent); pf != nil {
				l.methodsBy[pf.FileName()+" "+sym.Parent.Name+"."+name] = hm
			}
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
		f := hir.Field{Node: l.node(p), Name: name, Type: typ, Private: p.ModifierFlags()&ast.ModifierFlagsPrivate != 0, Readonly: p.ModifierFlags()&ast.ModifierFlagsReadonly != 0}
		c.Fields = append(c.Fields, f)
		if ps := p.Symbol(); ps != nil {
			l.fields[ps] = f
			if ps.Parent != nil {
				if pf := l.fileOfSymbol(ps.Parent); pf != nil {
					l.fieldsBy[pf.FileName()+" "+ps.Parent.Name+"."+name] = f
				}
			}
		}
	}
}

// signature fills params and result of hm from the checker's signature.
func (l *lowerer) signature(node *ast.Node, hm *hir.Method) {
	sig := l.ck.GetSignatureFromDeclaration(node)
	checked := map[int]bool{}
	for i, p := range node.Parameters() {
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
		q := false
		if qt := p.QuestionToken(); qt != nil {
			q = true
		}
		if q && typ.Kind != hir.Optional {
			l.diagf(p, "note-optional-param", "optional parameter %s lowered as a required Optional", name)
			typ = hir.T(hir.Optional, typ)
		}
		if d3 := p.AsParameterDeclaration(); d3 != nil && d3.DotDotDotToken != nil {
			if typ.Kind != hir.Array {
				l.diagf(p, "unsupported-param", "variadic parameter %s without an array type", name)
				typ = hir.T(hir.Array, typ)
			}
			l.diagf(p, "note-variadic", "variadic parameter %s lowered as a trailing array", name)
			hm.Params = append(hm.Params, hir.Param{Name: name, Type: typ, Variadic: true})
			checked[len(hm.Params)-1] = true
			continue
		}
		if p.Initializer() != nil && typ.Kind != hir.Optional {
			// A default value makes the parameter optional; the default is
			// assigned in the body prologue (applyDefaults).
			typ = hir.T(hir.Optional, typ)
			l.diagf(p, "note-default-param", "parameter %s with a default lowered as Optional", name)
		}
		hm.Params = append(hm.Params, hir.Param{Name: name, Type: typ})
		checked[len(hm.Params)-1] = true
	}
	if e, ok := l.overrides[node]; ok && e.Result != nil {
		hm.Result = e.Result()
		l.diagf(node, "note-override", "%s: %s", e.ID, e.Rationale)
	} else if node.Type() != nil {
		hm.Result = l.mapTypeNode(node.Type())
	} else if sig != nil {
		result := l.ck.GetReturnTypeOfSignature(sig)
		inferred := false
		if result != nil && l.ck.IsArrayType(result) {
			if index := l.ck.GetIndexInfoOfType(result, l.ck.GetNumberType()); index != nil && index.ValueType().Flags()&checker.TypeFlagsNever != 0 && l.class != nil {
				for _, name := range l.class.Implements {
					for _, iface := range l.out.Interfaces {
						if iface.Name == name {
							for _, m := range iface.Methods {
								if m.Name == hm.Name {
									hm.Result = m.Result
									inferred = true
								}
							}
						}
					}
				}
			}
		}
		if !inferred {
			hm.Result = l.mapCheckerType(node, result)
		}
	} else {
		hm.Result = hir.T(hir.Void)
	}
	if hm.Name == "constructor" && hm.Result.Kind != hir.Void {
		hm.Result = hir.T(hir.Void)
	}
}

func (l *lowerer) interfaceSignatures(node *ast.Node, i *hir.Interface) {
	if e, ok := l.overrides[node]; ok && e.Interface != nil {
		i.Methods = e.Interface().Methods
		l.diagf(node, "note-override", "%s: %s", e.ID, e.Rationale)
		return
	}
	for _, m := range node.Members() {
		if m.Kind != ast.KindMethodDeclaration && m.Kind != ast.KindMethodSignature {
			name := m.Kind.String()
			if n := m.Name(); n != nil && (n.Kind == ast.KindIdentifier || n.Kind == ast.KindPrivateIdentifier || n.Kind == ast.KindStringLiteral) {
				name = m.Name().Text()
			}
			l.diagf(m, "skipped-interface-member", "interface %s: non-method member %s is skipped", i.Name, name)
			continue
		}
		name, ok := l.memberName(m, nil)
		if !ok {
			continue
		}
		hm := &hir.Method{Node: l.node(m), Name: name, Virtual: true}
		before := len(l.diags)
		l.signature(m, hm)
		if l.methodFailed(hm) || hasBlocking(l.diags[before:]) {
			// Members whose types do not map stay out of the lowered
			// interface; nothing calls them in the lowered set.
			l.diags = append(l.diags[:before], LowerDiagnostic{Category: "skipped-interface-member",
				Loc: l.locOf(m), Message: "interface " + i.Name + ": member " + name + " has unlowered types"})
			continue
		}
		i.Methods = append(i.Methods, hm)
		if sym := m.Symbol(); sym != nil {
			l.methods[sym] = hm
			if sym.Parent != nil {
				if pf := l.fileOfSymbol(sym.Parent); pf != nil {
					l.methodsBy[pf.FileName()+" "+sym.Parent.Name+"."+name] = hm
				}
			}
		}
	}
}

// hasBlocking reports whether any diagnostic is not a policy note.
func hasBlocking(ds []LowerDiagnostic) bool {
	for _, d := range ds {
		if !strings.HasPrefix(d.Category, "note-") {
			return true
		}
	}
	return false
}

// memberName returns the lowered name of a member; computed property names
// (Symbol.for(...)) are reported and skipped.
func (l *lowerer) memberName(m *ast.Node, c *hir.Class) (string, bool) {
	if e, ok := l.overrides[m]; ok && e.Method != nil {
		return e.Method().Name, true
	}
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
	if l.trapUnexecuted(m, hm) {
		return
	}
	if e, ok := l.overrides[m]; ok && e.Method != nil {
		return
	}
	if m.Body() == nil {
		return
	}
	l.push()
	for _, p := range hm.Params {
		l.declare(p.Name, p.Type)
	}
	defaults := l.applyDefaults(m, hm)
	restore := l.withLocalFns(m.Body())
	body := l.block(m.Body())
	restore()
	if len(defaults) > 0 {
		body = hir.B(append(defaults, body)...)
	}
	hm.Body = body
	l.pop()
}

func (l *lowerer) lowerConstructorBody(m *ast.Node, c *hir.Class) {
	hm := c.Ctor
	if hm == nil || m.Body() == nil {
		return
	}
	l.method = hm
	if l.trapUnexecuted(m, hm) {
		return
	}
	l.push()
	for _, p := range hm.Params {
		l.declare(p.Name, p.Type)
	}
	stmts := l.applyDefaults(m, hm)
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
		if static && !l.pureInitializer(mem.Initializer(), mem, map[*ast.Node]bool{}) {
			l.diagf(mem.Initializer(), "unsupported-static-init", "static initializer is not provably pure and order-independent")
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
		if len(call.Arguments()) == 0 {
			// The base's implicit zero-argument constructor: nothing to run.
			return nil
		}
		l.diagf(call, "unsupported-super", "super constructor call without a lowered base constructor")
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
