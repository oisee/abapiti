package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Second part of the syntax-closure lowerings: Number constants, the
// builtin Error's toString, string slice, in-place stable sort with an
// inlined comparator, localeCompare on the object-name domain, module-level
// statements, index-signature interfaces and spreads of optional shapes.

// numberStatic lowers Number.MAX_SAFE_INTEGER / MIN_SAFE_INTEGER, exact in
// binary64.
func (l *lowerer) numberStatic(n *ast.Node, p *ast.PropertyAccessExpression) (*hir.Expr, bool) {
	if p.Expression == nil || p.Expression.Kind != ast.KindIdentifier || p.Expression.Text() != "Number" || n.Name() == nil {
		return nil, false
	}
	if _, isLocal := l.lookup("Number"); isLocal {
		return nil, false
	}
	sym := l.resolve(p.Expression)
	if sym == nil || l.classOf(sym) != nil {
		return nil, false
	}
	if f := l.fileOfSymbol(sym); f == nil || !l.prog.prog.IsSourceFileDefaultLibrary(f.Path()) {
		return nil, false
	}
	switch n.Name().Text() {
	case "MAX_SAFE_INTEGER":
		return hir.L(hir.T(hir.Number), float64(9007199254740991)), true
	case "MIN_SAFE_INTEGER":
		return hir.L(hir.T(hir.Number), float64(-9007199254740991)), true
	}
	return nil, false
}

// errorToString lowers e.toString() on the builtin Error: "Error" when the
// message is empty, otherwise "Error: " + message (the name is never
// overridden in the closure).
func (l *lowerer) errorToString(n *ast.Node, recv *hir.Expr) *hir.Expr {
	msg := hir.T(hir.String)
	e := l.tempInit(n, recv.Type, recv)
	message := &hir.Expr{Kind: hir.FieldGet, Node: l.node(n), Name: "message", Type: msg, X: e}
	empty := &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "==", Type: hir.T(hir.Bool), X: message, Y: hir.L(msg, "")}
	l.diagf(n, "note-error-tostring", "Error.toString lowered as name and message")
	return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: msg, X: empty, Y: hir.L(msg, "Error"),
		Z: l.rtOp("string.concat", hir.L(msg, "Error: "), msg, message)}
}

// sortWithComparator lowers arr.sort((a, b) => cmp) as an in-place stable
// insertion sort. The comparator is inlined; it must be pure, since the
// sequence of comparisons differs from V8's TimSort.
func (l *lowerer) sortWithComparator(n *ast.Node, recv *hir.Expr, cb *ast.Node) *hir.Expr {
	elem := recv.Type.Args[0]
	arr := l.tempInit(n, recv.Type, recv)
	num := hir.T(hir.Number)
	body, params, ok := l.callbackBody(n, cb, elem)
	if !ok {
		return nil
	}
	if len(params) != 2 {
		l.diagf(n, "unsupported-call", "sort comparator must take two parameters")
		return nil
	}
	if body.Type.Kind != hir.Number {
		l.diagf(n, "unsupported-call", "sort comparator must return a number")
		return nil
	}
	l.diagf(n, "note-sort-inline", "sort comparator inlined into a stable insertion sort; the comparator is assumed pure")
	l.serial++
	i := hir.V("i"+itoa(l.serial), num)
	l.serial++
	j := hir.V("j"+itoa(l.serial), num)
	l.serial++
	swap := hir.V("w"+itoa(l.serial), elem)
	length := l.rtOp("array.length", arr, hir.T(hir.I32))
	at := func(k *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.IndexGet, Node: l.node(n), Type: elem, X: arr, Y: l.indexValue(k)}
	}
	minus := func(k *hir.Expr) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "-", Type: num, X: k, Y: hir.L(num, 1)}
	}
	cmp := &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: ">", Type: hir.T(hir.Bool), X: body, Y: hir.L(num, 0)}
	inner := hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[0], Type: elem, X: at(minus(j))},
		&hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: params[1], Type: elem, X: at(j)},
		&hir.Stmt{Kind: hir.If, Node: l.node(n), X: cmp, Body: hir.B(
			&hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: swap.Name, Type: elem, X: at(minus(j))},
			&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: at(minus(j)), Y: at(j)},
			&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: at(j), Y: swap},
			&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: j, Y: minus(j)},
		), Else: hir.B(&hir.Stmt{Kind: hir.Break, Node: l.node(n)})},
	)
	outer := hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: j.Name, Type: num, X: i},
		&hir.Stmt{Kind: hir.While, Node: l.node(n), X: &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: ">", Type: hir.T(hir.Bool), X: j, Y: hir.L(num, 0)}, Body: inner},
		&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: i, Y: &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "+", Type: num, X: i, Y: hir.L(num, 1)}},
	)
	l.pendStmt(&hir.Stmt{Kind: hir.VarDecl, Node: l.node(n), Name: i.Name, Type: num, X: hir.L(num, 1)})
	l.pendStmt(&hir.Stmt{Kind: hir.While, Node: l.node(n), X: &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: "<", Type: hir.T(hir.Bool), X: i, Y: length}, Body: outer})
	return arr
}

// moduleStatements lowers module-level if and expression statements, in
// source order, into the module initializer after the module constants.
func (l *lowerer) moduleStatements(f *ast.SourceFile) []*hir.Stmt {
	var out []*hir.Stmt
	l.push()
	defer l.pop()
	for _, stmt := range l.statementNodes(f) {
		switch stmt.Kind {
		case ast.KindIfStatement, ast.KindExpressionStatement:
			if !l.moduleInitializer(stmt) {
				continue
			}
			l.diagf(stmt, "note-module-statement", "module-level statement lowered into the module initializer")
			out = append(out, l.stmts(stmt)...)
		}
	}
	return out
}

// indexOnlyInterface maps an interface declared with a single string index
// signature and no other members to the insertion-ordered record map.
func (l *lowerer) indexOnlyInterface(sym *ast.Symbol) (hir.Type, bool) {
	if sym == nil {
		return hir.Type{}, false
	}
	var decl *ast.Node
	for _, d := range sym.Declarations {
		if d.Kind == ast.KindInterfaceDeclaration {
			decl = d
			break
		}
	}
	if decl == nil || decl.AsInterfaceDeclaration().HeritageClauses != nil {
		return hir.Type{}, false
	}
	members := decl.Members()
	if len(members) != 1 || members[0].Kind != ast.KindIndexSignature {
		return hir.Type{}, false
	}
	is := members[0].AsIndexSignatureDeclaration()
	if len(is.Parameters.Nodes) != 1 || is.Parameters.Nodes[0].Type() == nil || is.Parameters.Nodes[0].Type().Kind != ast.KindStringKeyword || members[0].Type() == nil {
		return hir.Type{}, false
	}
	saved := l.file
	l.file = ast.GetSourceFileOfNode(decl)
	value := l.mapTypeNode(members[0].Type())
	l.file = saved
	if value.Kind == hir.Void {
		return hir.Type{}, false
	}
	return hir.T(hir.OrderedMap, hir.T(hir.String), value), true
}

// dataInterfaceDeclares reports whether the interface node itself declares
// a property with the given name.
func (l *lowerer) dataInterfaceDeclares(node *ast.Node, name string) bool {
	for _, m := range node.Members() {
		if (m.Kind == ast.KindPropertySignature || m.Kind == ast.KindPropertyDeclaration) && m.Name() != nil && (m.Name().Kind == ast.KindIdentifier || m.Name().Kind == ast.KindStringLiteral) && m.Name().Text() == name {
			return true
		}
	}
	return false
}

// optionalView presents a present value as its optional type (a box for
// primitives, the same reference otherwise) for comparisons.
func (l *lowerer) optionalView(x *hir.Expr, opt hir.Type) *hir.Expr {
	if x.Type.Kind == hir.Optional {
		return x
	}
	if !opt.Args[0].IsRef() {
		return l.coerce(x, opt)
	}
	return &hir.Expr{Kind: hir.Conditional, Node: x.Node, Type: opt, X: hir.L(hir.T(hir.Bool), true), Y: x, Z: &hir.Expr{Kind: hir.Lit, Type: opt}}
}

// constantBool recognizes a boolean literal, possibly behind a Seq that only
// evaluates operands for their effects.
func constantBool(x *hir.Expr) (bool, bool) {
	if x == nil {
		return false, false
	}
	if x.Kind == hir.Seq && x.Y != nil {
		x = x.Y
	}
	if x.Kind == hir.Lit && x.Type.Kind == hir.Bool {
		v, ok := x.Value.(bool)
		return v, ok
	}
	return false, false
}

// unrelatedClasses reports whether a value of static type t can never be an
// instance of class c: both are lowered classes and neither is an ancestor
// of the other. Interfaces, the object root and non-class types stay dynamic.
func (l *lowerer) unrelatedClasses(t hir.Type, c string) bool {
	if t.Kind == hir.Optional {
		t = t.Args[0]
	}
	if t.Kind != hir.ClassRef || t.Name == hir.RootObject || t.Name == c {
		return false
	}
	if l.classByName(t.Name) == nil || l.classByName(c) == nil {
		return false
	}
	ancestor := func(from, to string) bool {
		for x := l.classByName(from); x != nil; x = l.classByName(x.Super) {
			if x.Name == to {
				return true
			}
		}
		return false
	}
	return !ancestor(t.Name, c) && !ancestor(c, t.Name)
}

// isNeverArray reports a checker type `never[]` (an empty literal's type).
func (l *lowerer) isNeverArray(t *checker.Type) bool {
	if t == nil || !l.ck.IsArrayType(t) {
		return false
	}
	elem := l.ck.GetElementTypeOfArrayType(t)
	return elem != nil && elem.Flags()&checker.TypeFlagsNever != 0
}

// optionalSpreadArgs fills the constructor arguments of a shape from an
// optional shape value: absent spreads leave every (optional) field absent.
func (l *lowerer) optionalSpreadArgs(p *ast.Node, sv *hir.Expr, ctor *hir.Method, args []*hir.Expr) bool {
	present := l.tempInit(p, sv.Type, sv)
	inner := sv.Type.Args[0]
	var source *hir.Class
	for _, c := range l.out.Classes {
		if c.Name == inner.Name {
			source = c
		}
	}
	if source == nil {
		l.diagf(p, "unsupported-type", "spread needs a shape value")
		return false
	}
	absent := &hir.Expr{Kind: hir.IsUndefined, Node: l.node(p), Type: hir.T(hir.Bool), X: present}
	narrowed := &hir.Expr{Kind: hir.Narrow, Node: l.node(p), Type: inner, X: present}
	for i, cp := range ctor.Params {
		for _, f := range source.Fields {
			if f.Name != cp.Name {
				continue
			}
			if cp.Type.Kind != hir.Optional {
				l.diagf(p, "unsupported-type", "spread of an optional value needs optional target field %s", cp.Name)
				return false
			}
			value := l.coerce(&hir.Expr{Kind: hir.FieldGet, Node: l.node(p), Name: f.Name, Type: f.Type, X: narrowed}, cp.Type)
			args[i] = &hir.Expr{Kind: hir.Conditional, Node: l.node(p), Type: cp.Type, X: absent, Y: &hir.Expr{Kind: hir.Lit, Type: cp.Type}, Z: value}
		}
	}
	l.diagf(p, "note-object-spread", "optional object spread copies the fields of %s when present", inner.Name)
	return true
}
