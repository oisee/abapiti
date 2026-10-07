package tsfront

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"github.com/oisee/abapiti/internal/tsgo/scanner"
)

// LowerDiagnostic is one finding of the lowering. Categories starting with
// "note-" record policy decisions (constructs that were mapped, not dropped);
// every other category marks a construct the lowering does not support, and
// the member that contains it is left out of the program (never lowered
// wrongly and silently).
type LowerDiagnostic struct {
	Category string `json:"category"`
	Loc      string `json:"loc"` // file:line:col
	Message  string `json:"message"`
}

func (d LowerDiagnostic) String() string { return d.Category + " " + d.Loc + ": " + d.Message }

// Lower lowers the named files of the program into the object HIR. The files
// must be part of the program (paths as they appear in SourceFiles); every
// declaration they reference at run time must be in the list, because the
// lowering only resolves to declarations it has lowered. Types come from the
// checker; the lowering never infers a type from syntax alone. The second
// result reports unsupported constructs and policy notes. Files should be
// passed in a deterministic order; declaration order in the HIR follows it.
func (p *Program) Lower(files []string) (*hir.Program, []LowerDiagnostic, error) {
	l := &lowerer{
		prog:          p,
		implicitCtors: map[*hir.Class]bool{},
		out:           &hir.Program{},
		classes:       map[*ast.Symbol]*hir.Class{},
		ifaces:        map[*ast.Symbol]*hir.Interface{},
		methods:       map[*ast.Symbol]*hir.Method{},
		fields:        map[*ast.Symbol]hir.Field{},
		synths:        map[*ast.Symbol]*hir.Class{},
		modules:       map[*ast.SourceFile]*hir.Class{},
		modvars:       map[*ast.Symbol]string{},
		classesByName: map[string]*hir.Class{},
		ifacesByName:  map[string]*hir.Interface{},
		methodsBy:     map[string]*hir.Method{},
		fieldsBy:      map[string]hir.Field{},
		modvarsByName: map[string]modvarRef{},
		synthsByName:  map[string]*hir.Class{},
	}
	for _, name := range files {
		f, ok := p.File(name)
		if !ok {
			return nil, nil, fmt.Errorf("file %s is not part of the program", name)
		}
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		l.registerFile(f)
		done()
	}
	// Module values first: they only contain literals, so they cannot
	// reference class members.
	for _, name := range files {
		f, _ := p.File(name)
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		l.lowerModule(f)
		done()
	}
	// Signatures before bodies, so bodies resolve every member.
	for _, name := range files {
		f, _ := p.File(name)
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		l.signaturesFile(f)
		done()
	}
	// An implicit derived constructor forwards the inherited parameter list.
	var inherit func(*hir.Class)
	visited := map[*hir.Class]bool{}
	inherit = func(c *hir.Class) {
		if visited[c] {
			return
		}
		visited[c] = true
		if base := l.classesByQualifiedName(c.Super); base != nil {
			inherit(base)
			if l.implicitCtors[c] {
				if ctor := l.constructorOf(base); ctor != nil {
					c.Ctor.Params = append([]hir.Param{}, ctor.Params...)
				}
			}
		}
	}
	for _, c := range l.out.Classes {
		inherit(c)
	}
	for _, name := range files {
		f, _ := p.File(name)
		ck, done := p.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		l.bodiesFile(f)
		done()
	}
	return l.out, l.diags, nil
}

// lowerer carries the state of one lowering run; file and ck are the per-file
// state of the pass being run.
type lowerer struct {
	prog          *Program
	ck            *checker.Checker
	file          *ast.SourceFile
	out           *hir.Program
	diags         []LowerDiagnostic
	classes       map[*ast.Symbol]*hir.Class
	ifaces        map[*ast.Symbol]*hir.Interface
	methods       map[*ast.Symbol]*hir.Method
	fields        map[*ast.Symbol]hir.Field
	synths        map[*ast.Symbol]*hir.Class // object-literal alias symbol -> class
	modules       map[*ast.SourceFile]*hir.Class
	modvars       map[*ast.Symbol]string // module variable symbol -> field name
	scope         []map[string]hir.Type
	class         *hir.Class  // class whose member is being lowered
	method        *hir.Method // method being lowered
	implicitCtors map[*hir.Class]bool
	serial        int
	hint          hir.Type // contextual type for undefined literals

	// The per-file checkers hand out distinct symbol pointers for the same
	// cross-file declaration, so every registry also has a by-name view keyed
	// by the TypeScript simple names (owner name plus member name), which are
	// stable. A name registered twice for different declarations is rejected
	// rather than resolved by order.
	classesByName map[string]*hir.Class
	ifacesByName  map[string]*hir.Interface
	methodsBy     map[string]*hir.Method
	fieldsBy      map[string]hir.Field
	modvarsByName map[string]modvarRef // current file + var name -> module field
	synthsByName  map[string]*hir.Class
}

// classOf resolves a class symbol through both registries.
func (l *lowerer) classOf(sym *ast.Symbol) *hir.Class {
	if sym == nil {
		return nil
	}
	if c, ok := l.classes[sym]; ok {
		return c
	}
	if sym.Parent == nil {
		if c, ok := l.classesByName[sym.Name]; ok {
			return c
		}
	}
	return l.classesByName[sym.Name]
}

// ifaceOf resolves an interface symbol through both registries.
func (l *lowerer) ifaceOf(sym *ast.Symbol) *hir.Interface {
	if i, ok := l.ifaces[sym]; ok {
		return i
	}
	return l.ifacesByName[sym.Name]
}

// methodOf resolves a member symbol through both registries.
func (l *lowerer) methodOf(sym *ast.Symbol) *hir.Method {
	if m, ok := l.methods[sym]; ok {
		return m
	}
	if sym.Parent != nil {
		return l.methodsBy[sym.Parent.Name+"."+sym.Name]
	}
	return nil
}

// fieldOf resolves a field symbol through both registries.
func (l *lowerer) fieldOf(sym *ast.Symbol) (hir.Field, bool) {
	if f, ok := l.fields[sym]; ok {
		return f, true
	}
	if sym.Parent != nil {
		if f, ok := l.fieldsBy[sym.Parent.Name+"."+sym.Name]; ok {
			return f, true
		}
	}
	return hir.Field{}, false
}

// modvarRef names the module class and its static field for one value.
type modvarRef struct {
	owner, field string
}

// modvarOf resolves a module-level value symbol to its module class and
// static field name. Module values are visible only inside their own file,
// so the current file disambiguates equal names in different files (each
// lexer file has its own EOF).
func (l *lowerer) modvarOf(sym *ast.Symbol) (string, string, bool) {
	if name, ok := l.modvars[sym]; ok {
		if mod := l.modules[l.fileOfSymbol(sym)]; mod != nil {
			return mod.Name, name, true
		}
	}
	if r, ok := l.modvarsByName[l.file.FileName()+" "+sym.Name]; ok {
		return r.owner, r.field, true
	}
	return "", "", false
}

// synthOf resolves an object-shape alias symbol through both registries.
func (l *lowerer) synthOf(sym *ast.Symbol) *hir.Class {
	if c, ok := l.synths[sym]; ok {
		return c
	}
	return l.synthsByName[sym.Name]
}

func (l *lowerer) diagf(n *ast.Node, category, format string, args ...any) {
	l.diags = append(l.diags, LowerDiagnostic{Category: category, Loc: l.locOf(n), Message: fmt.Sprintf(format, args...)})
}

// node returns the identity (ID and source location) for a new HIR node.
func (l *lowerer) node(n *ast.Node) hir.Node {
	return hir.Node{ID: l.nextID(), Source: l.locOf(n)}
}

func (l *lowerer) nextID() int {
	l.serial++
	return l.serial
}

func (l *lowerer) locOf(n *ast.Node) string {
	if n == nil {
		if l.file == nil {
			return "<unknown>"
		}
		return locString(l.file, 0)
	}
	return locString(l.file, scanner.GetTokenPosOfNode(n, l.file, false /*includeJSDoc*/))
}

// qualifiedName builds the stable identity of a declaration: the file path
// relative to the tsconfig directory plus the declaration name.
func (l *lowerer) qualifiedName(f *ast.SourceFile, name string) string {
	rel, err := filepath.Rel(l.prog.configDir, f.FileName())
	if err != nil {
		rel = f.FileName()
	}
	return rel + "." + name
}

// resolve returns the symbol at node with import aliases resolved.
func (l *lowerer) resolve(n *ast.Node) *ast.Symbol {
	sym := l.ck.GetSymbolAtLocation(n)
	if sym == nil {
		return nil
	}
	if sym.Flags&ast.SymbolFlagsAlias != 0 {
		if target, ok := l.ck.ResolveAlias(sym); ok {
			return target
		}
	}
	return sym
}

func (l *lowerer) registerFile(f *ast.SourceFile) {
	for _, stmt := range f.Statements.Nodes {
		switch stmt.Kind {
		case ast.KindClassDeclaration:
			if name := stmt.Name(); name != nil && stmt.Symbol() != nil {
				c := &hir.Class{Node: l.node(stmt), Name: l.qualifiedName(f, name.Text())}
				if old, ok := l.classesByName[name.Text()]; ok && old != c {
					l.diagf(stmt, "unsupported-decl", "duplicate class name %s", name.Text())
					continue
				}
				l.classes[stmt.Symbol()] = c
				l.classesByName[name.Text()] = c
				l.out.Classes = append(l.out.Classes, c)
			}
		case ast.KindInterfaceDeclaration:
			if name := stmt.Name(); name != nil && stmt.Symbol() != nil {
				i := &hir.Interface{Node: l.node(stmt), Name: l.qualifiedName(f, name.Text())}
				if old, ok := l.ifacesByName[name.Text()]; ok && old != i {
					l.diagf(stmt, "unsupported-decl", "duplicate interface name %s", name.Text())
					continue
				}
				l.ifaces[stmt.Symbol()] = i
				l.ifacesByName[name.Text()] = i
				l.out.Interfaces = append(l.out.Interfaces, i)
			}
		}
	}
}

func (l *lowerer) lowerModule(f *ast.SourceFile) {
	var decls []*ast.Node
	for _, stmt := range f.Statements.Nodes {
		if stmt.Kind != ast.KindVariableStatement {
			switch stmt.Kind {
			case ast.KindClassDeclaration, ast.KindInterfaceDeclaration, ast.KindTypeAliasDeclaration, ast.KindImportDeclaration, ast.KindExportDeclaration, ast.KindEmptyStatement:
			default:
				l.diagf(stmt, "unsupported-top-level", "top-level %s is not lowered", stmt.Kind.String())
			}
			continue
		}
		list := stmt.AsVariableStatement().DeclarationList
		if list == nil || list.Kind != ast.KindVariableDeclarationList {
			continue
		}
		if list.Flags&ast.NodeFlagsConst == 0 {
			l.diagf(stmt, "unsupported-top-level", "only const initializers are supported at module scope")
		}
		for _, d := range list.AsVariableDeclarationList().Declarations.Nodes {
			if d.Name() != nil && d.Name().Kind == ast.KindIdentifier && d.Symbol() != nil {
				decls = append(decls, d)
			} else {
				l.diagf(d, "unsupported-top-level", "module bindings require a simple identifier")
			}
		}
	}
	if len(decls) == 0 {
		return
	}
	qname := l.qualifiedName(f, "module")
	mod := &hir.Class{Node: hir.Node{ID: l.nextID(), Source: qname}, Name: qname}
	init := &hir.Method{Node: hir.Node{ID: l.nextID(), Source: qname}, Name: "class_constructor", Static: true, Result: hir.T(hir.Void)}
	l.class = mod
	l.method = init
	l.modules[f] = mod // before the initializers, which reference earlier fields
	l.out.Classes = append(l.out.Classes, mod)
	body := []*hir.Stmt{}
	for _, d := range decls {
		body = append(body, l.moduleVar(d, mod)...)
	}
	init.Body = hir.B(body...)
	mod.Methods = append(mod.Methods, init)
}

// moduleVar lowers one module-level const/let into a static field plus the
// initializing statement of the class constructor. The field type comes from
// the annotation if present, otherwise from the lowered initializer (the
// checker's type of `new Set<number>` carries no readable type arguments).
func (l *lowerer) moduleVar(d *ast.Node, mod *hir.Class) []*hir.Stmt {
	name := d.Name().Text()
	init := d.Initializer()
	if init == nil {
		l.diagf(d, "unsupported-top-level", "module const requires an initializer")
		return nil
	}
	var typ hir.Type
	var stmts []*hir.Stmt
	switch {
	case d.Type() != nil:
		typ = l.mapTypeNode(d.Type())
		stmts = []*hir.Stmt{l.assignStatic(mod, d, name, l.expr(init), typ)}
	case l.isNewCollection(init):
		var expr *hir.Expr
		stmts, expr = l.collectionInit(init)
		if expr == nil {
			return nil
		}
		typ = expr.Type
		stmts = append(stmts, l.assignStatic(mod, d, name, expr, typ))
	default:
		expr := l.expr(init)
		if expr == nil {
			return nil
		}
		typ = expr.Type
		stmts = []*hir.Stmt{l.assignStatic(mod, d, name, expr, typ)}
	}
	mod.Fields = append(mod.Fields, hir.Field{Node: l.node(d), Name: name, Type: typ, Static: true})
	l.modvars[d.Symbol()] = name
	l.modvarsByName[l.file.FileName()+" "+name] = modvarRef{owner: mod.Name, field: name}
	return stmts
}

func (l *lowerer) assignStatic(mod *hir.Class, at *ast.Node, name string, x *hir.Expr, t hir.Type) *hir.Stmt {
	return &hir.Stmt{Kind: hir.Assign, Node: l.node(at),
		X: &hir.Expr{Kind: hir.StaticGet, Node: l.node(at), Owner: mod.Name, Name: name, Type: t}, Y: x}
}

// isNewCollection reports whether the expression is `new Set<T>([...])`,
// `new Map<K,V>([...])` or `new Array<T>([...])`, which lower to a
// construction sequence in statement context.
func (l *lowerer) isNewCollection(n *ast.Node) bool {
	if n == nil || n.Kind != ast.KindNewExpression || n.Expression() == nil {
		return false
	}
	t := n.Expression()
	if t.Kind != ast.KindIdentifier {
		return false
	}
	sym := l.resolve(t)
	if sym == nil {
		return false
	}
	// Only the library collections, never a lowered class of the same name.
	if l.classes[sym] != nil {
		return false
	}
	return sym.Name == "Set" || sym.Name == "Map" || sym.Name == "Array"
}

// collectionInit lowers `new Set<T>([...])` in a statement context into a
// fresh collection plus one insert per element.
func (l *lowerer) collectionInit(n *ast.Node) ([]*hir.Stmt, *hir.Expr) {
	name := n.Expression().Text()
	args := n.Arguments()
	var elemType hir.Type
	var ok bool
	switch name {
	case "Set":
		elemType, ok = l.typeArgAt(n, 0)
	case "Array":
		elemType, ok = l.typeArgAt(n, 0)
	default:
		l.diagf(n, "unsupported-new", "new %s is not lowered", name)
		return nil, nil
	}
	if !ok {
		return nil, nil
	}
	if len(args) != 1 || args[0].Kind != ast.KindArrayLiteralExpression {
		l.diagf(n, "unsupported-new", "new %s needs one array literal argument", name)
		return nil, nil
	}
	kind := hir.OrderedSet
	if name == "Array" {
		kind = hir.Array
	}
	typ := hir.T(kind, elemType)
	l.serial++
	tmp := "c" + itoa(l.serial)
	l.declare(tmp, typ)
	stmts := []*hir.Stmt{{Kind: hir.VarDecl, Node: l.node(n), Name: tmp, Type: typ,
		X: &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ}}}
	for _, el := range args[0].AsArrayLiteralExpression().Elements.Nodes {
		v := l.expr(el)
		if v == nil {
			return nil, nil
		}
		op := "set.add"
		if name == "Array" {
			op = "array.push"
		}
		recv := hir.V(tmp, typ)
		var res hir.Type = typ
		if name == "Array" {
			res = hir.T(hir.I32)
		}
		stmts = append(stmts, &hir.Stmt{Kind: hir.ExprStmt, Node: l.node(el), X: l.rtOp(op, recv, res, v)})
	}
	return stmts, hir.V(tmp, typ)
}

// typeArgAt maps the i-th syntactic type argument of a new expression.
func (l *lowerer) typeArgAt(n *ast.Node, i int) (hir.Type, bool) {
	targs := n.TypeArguments()
	if len(targs) <= i {
		l.diagf(n, "unsupported-new", "missing explicit type argument")
		return hir.Type{}, false
	}
	return l.mapTypeNode(targs[i]), true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func (l *lowerer) classesByQualifiedName(name string) *hir.Class {
	for _, c := range l.out.Classes {
		if c.Name == name {
			return c
		}
	}
	return nil
}
