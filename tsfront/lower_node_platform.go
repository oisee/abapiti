package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// Typed Node platform calls on gogen's self-hosting path. Only the overloads
// gogen uses are lowered; every other form of these functions keeps a located
// refusal rather than an approximation:
//
//	path.basename(p)                          POSIX final component
//	fs.readFileSync(p, "utf8")                whole file as UTF-8 text
//	fs.readdirSync(p)                         entry names, native order
//	fs.readdirSync(p, {withFileTypes: true})  Dirent name/isDirectory()
//
// Filesystem failures throw a catchable Error with Node's message
// (`ENOENT: no such file or directory, open '<p>'`); the code property is
// not modelled.

// nodeBuiltinBinding reports the Node builtin module (without the node:
// prefix) and export an identifier is bound to by an import declaration.
// Namespace imports report namespace=true and no export.
func nodeBuiltinBinding(sym *ast.Symbol) (module, export string, namespace bool) {
	if sym == nil {
		return "", "", false
	}
	for _, decl := range sym.Declarations {
		var clause *ast.Node
		name, ns := "", false
		switch decl.Kind {
		case ast.KindImportSpecifier:
			spec := decl.AsImportSpecifier()
			if spec.IsTypeOnly {
				continue
			}
			name = decl.Name().Text()
			if spec.PropertyName != nil {
				name = spec.PropertyName.Text()
			}
			if decl.Parent != nil {
				clause = decl.Parent.Parent
			}
		case ast.KindNamespaceImport:
			ns = true
			clause = decl.Parent
		default:
			continue
		}
		if clause == nil || clause.Kind != ast.KindImportClause || clause.AsImportClause().IsTypeOnly() || clause.Parent == nil || clause.Parent.Kind != ast.KindImportDeclaration {
			continue
		}
		spec := clause.Parent.AsImportDeclaration().ModuleSpecifier
		if spec == nil || spec.Kind != ast.KindStringLiteral {
			continue
		}
		return strings.TrimPrefix(spec.Text(), "node:"), name, ns
	}
	return "", "", false
}

// nodeBuiltinCallee matches `export` imported by name (renames included) or
// `ns.export` through a namespace import of the Node builtin module. A local
// binding that shadows the import never matches.
func (l *lowerer) nodeBuiltinCallee(callee *ast.Node, module, export string) bool {
	if callee == nil || l.ck == nil {
		return false
	}
	switch callee.Kind {
	case ast.KindIdentifier:
		m, e, ns := nodeBuiltinBinding(l.ck.GetSymbolAtLocation(callee))
		return !ns && m == module && e == export
	case ast.KindPropertyAccessExpression:
		p := callee.AsPropertyAccessExpression()
		if p.QuestionDotToken != nil || callee.Name().Text() != export || p.Expression.Kind != ast.KindIdentifier {
			return false
		}
		m, _, ns := nodeBuiltinBinding(l.ck.GetSymbolAtLocation(p.Expression))
		return ns && m == module
	}
	return false
}

func (l *lowerer) nodePlatformCall(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	callee := n.Expression()
	args := n.Arguments()
	for _, a := range args {
		if a.Kind == ast.KindSpreadElement {
			for _, f := range [][2]string{{"path", "basename"}, {"fs", "readFileSync"}, {"fs", "readdirSync"}} {
				if l.nodeBuiltinCallee(callee, f[0], f[1]) {
					l.diagf(a, "unsupported-node-platform", "%s.%s spread arguments are not lowered", f[0], f[1])
					return nil, true
				}
			}
			return nil, false
		}
	}
	switch {
	case l.nodeBuiltinCallee(callee, "url", "fileURLToPath") || l.nodeBuiltinCallee(callee, "url", "pathToFileURL"):
		// gogen converts only module locations (import.meta.url, the
		// process.argv entry check); a compiled program has neither.
		l.diagf(n, "unsupported-node-platform", "node:url file URL conversion is not lowered: module locations (import.meta.url, process.argv entry) have no compiled equivalent")
		return nil, true
	case l.nodeBuiltinCallee(callee, "path", "basename"):
		if len(args) != 1 {
			l.diagf(n, "unsupported-node-platform", "path.basename with a suffix argument is not lowered")
			return nil, true
		}
		file := l.nodeStringArg(n, args[0], "path.basename")
		if file == nil {
			return nil, true
		}
		return l.rtOp("path.basename", file, hir.T(hir.String)), true
	case l.nodeBuiltinCallee(callee, "fs", "readFileSync"):
		if len(args) != 2 || args[1].Kind != ast.KindStringLiteral || args[1].Text() != "utf8" && args[1].Text() != "utf-8" {
			l.diagf(n, "unsupported-node-platform", "fs.readFileSync is lowered only with the \"utf8\" encoding (string result)")
			return nil, true
		}
		file := l.nodeStringArg(n, args[0], "fs.readFileSync")
		if file == nil {
			return nil, true
		}
		l.builtinError()
		return l.rtOp("fs.readText", file, hir.T(hir.String)), true
	case l.nodeBuiltinCallee(callee, "fs", "readdirSync"):
		op := "fs.readdirNames"
		if len(args) == 2 && withFileTypesOnly(args[1]) {
			op = "fs.readdirEntries"
			l.builtinDirent()
		} else if len(args) != 1 {
			l.diagf(n, "unsupported-node-platform", "fs.readdirSync is lowered only without options or with exactly {withFileTypes: true}")
			return nil, true
		}
		dir := l.nodeStringArg(n, args[0], "fs.readdirSync")
		if dir == nil {
			return nil, true
		}
		l.builtinError()
		x := l.rtOp(op, dir, hir.T(hir.Dynamic))
		x.Node = l.node(n)
		return x, true
	}
	return nil, false
}

// withFileTypesOnly matches the literal options object {withFileTypes: true}.
func withFileTypesOnly(n *ast.Node) bool {
	if n.Kind != ast.KindObjectLiteralExpression {
		return false
	}
	props := n.AsObjectLiteralExpression().Properties.Nodes
	if len(props) != 1 || props[0].Kind != ast.KindPropertyAssignment {
		return false
	}
	p := props[0]
	name := p.Name()
	if name == nil || name.Kind != ast.KindIdentifier && name.Kind != ast.KindStringLiteral || name.Text() != "withFileTypes" {
		return false
	}
	return p.Initializer().Kind == ast.KindTrueKeyword
}

func (l *lowerer) nodeStringArg(call, arg *ast.Node, what string) *hir.Expr {
	// A checked non-string path (URL, Buffer, descriptor) is refused at the
	// call, before its operand is lowered.
	if t := l.ck.GetTypeAtLocation(arg); t != nil && t.Flags()&(checker.TypeFlagsStringLike|checker.TypeFlagsAnyOrUnknown) == 0 {
		l.diagf(call, "unsupported-node-platform", "%s is lowered only for string paths", what)
		return nil
	}
	old := l.hint
	l.hint = hir.Type{}
	x := l.expr(arg)
	l.hint = old
	if x == nil {
		return nil
	}
	if x.Type.Kind == hir.Dynamic {
		// Untyped JavaScript: the checked unboxing raises for non-strings
		// (file descriptors, Buffers and URLs are not lowered).
		x = l.coerce(x, hir.T(hir.String))
	}
	if x.Type.Kind != hir.String {
		l.diagf(call, "unsupported-node-platform", "%s is lowered only for string paths", what)
		return nil
	}
	return x
}

// builtinDirent is the Dirent snapshot of readdirSync(dir, {withFileTypes:
// true}): the name and the directory bit resolved by the directory read.
func (l *lowerer) builtinDirent() string {
	const name = "builtin.Dirent"
	if l.classNamed(name) != nil {
		return name
	}
	node := func() hir.Node { return hir.Node{ID: l.nextID(), Source: name} }
	s, b := hir.T(hir.String), hir.T(hir.Bool)
	this := func() *hir.Expr { return &hir.Expr{Kind: hir.This, Node: node(), Type: hir.Ref(name)} }
	assign := func(field string, t hir.Type) *hir.Stmt {
		return &hir.Stmt{Kind: hir.Assign, Node: node(), X: &hir.Expr{Kind: hir.FieldGet, Node: node(), Name: field, Type: t, X: this()}, Y: hir.V(field, t)}
	}
	l.out.Classes = append(l.out.Classes, &hir.Class{Node: node(), Name: name,
		Fields: []hir.Field{{Node: node(), Name: "name", Type: s}, {Node: node(), Name: "directory", Type: b}},
		Ctor: &hir.Method{Node: node(), Name: "constructor", Params: []hir.Param{{Name: "name", Type: s}, {Name: "directory", Type: b}},
			Result: hir.T(hir.Void), Body: hir.B(assign("name", s), assign("directory", b))}})
	return name
}

// nodeDirentType matches the fs Dirent class of the Node declarations
// (declare module "fs" / "node:fs" in a declaration file).
func (l *lowerer) nodeDirentType(t *checker.Type) bool {
	if t == nil || t.Symbol() == nil || t.Symbol().Name != "Dirent" || len(t.Symbol().Declarations) == 0 {
		return false
	}
	for _, d := range t.Symbol().Declarations {
		f := ast.GetSourceFileOfNode(d)
		if f == nil || !f.IsDeclarationFile {
			return false
		}
		m := d.Parent
		for m != nil && m.Kind != ast.KindModuleDeclaration {
			m = m.Parent
		}
		if m == nil || m.Name() == nil || m.Name().Kind != ast.KindStringLiteral || strings.TrimPrefix(m.Name().Text(), "node:") != "fs" {
			return false
		}
	}
	return true
}

// direntMember lowers Dirent.name and Dirent.isDirectory() on a snapshot.
func (l *lowerer) direntMember(access *ast.Node, call bool, args int) (*hir.Expr, bool) {
	if !l.unprunedFeatures || access == nil || access.Kind != ast.KindPropertyAccessExpression || access.AsPropertyAccessExpression().QuestionDotToken != nil {
		return nil, false
	}
	if !l.nodeDirentType(l.ck.GetTypeAtLocation(access.Expression())) {
		return nil, false
	}
	name := access.Name().Text()
	var field string
	var t hir.Type
	switch {
	case !call && name == "name":
		field, t = "name", hir.T(hir.String)
	case call && name == "isDirectory" && args == 0:
		field, t = "directory", hir.T(hir.Bool)
	default:
		l.diagf(access, "unsupported-node-platform", "Dirent.%s is not lowered (only name and isDirectory())", name)
		return nil, true
	}
	old := l.hint
	l.hint = hir.Type{}
	recv := l.expr(access.Expression())
	l.hint = old
	if recv == nil {
		return nil, true
	}
	want := hir.Ref(l.builtinDirent())
	if !recv.Type.Equal(want) {
		recv = l.coerce(recv, want)
	}
	if recv == nil || !recv.Type.Equal(want) {
		l.diagf(access, "unsupported-node-platform", "Dirent receiver is not a readdirSync entry snapshot")
		return nil, true
	}
	return &hir.Expr{Kind: hir.FieldGet, Node: l.node(access), Name: field, Type: t, X: recv}, true
}
