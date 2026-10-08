package tsfront

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Module-level lowering for phase 2: namespace export maps (a module used as
// a value becomes an OrderedMap<string, ClassValue> in export order — the
// CommonJS build the differential oracle runs keeps insertion order, and the
// statements/index.ts order is parse precedence), string enums, and module
// functions (static methods of the module class).

// scanModuleUse marks namespace modules that are used as values and collects
// string enum declarations. It runs before lowerModule so the module classes
// know which parts to build.
func (l *lowerer) scanModuleUse(f *ast.SourceFile) {
	for _, stmt := range l.statementNodes(f) {
		switch stmt.Kind {
		case ast.KindImportDeclaration:
			d := stmt.AsImportDeclaration()
			if d.ImportClause == nil || d.ImportClause.Kind != ast.KindImportClause {
				continue
			}
			ic := d.ImportClause.AsImportClause()
			if ic.NamedBindings == nil || ic.NamedBindings.Kind != ast.KindNamespaceImport {
				continue
			}
			// A namespace import is used as a value when the alias appears
			// outside the import itself.
			alias := ic.NamedBindings.Name()
			if alias == nil {
				continue
			}
			if l.namespaceUsedAsValue(f, alias.Text()) {
				if mod := l.moduleOfDeclaration(stmt); mod != nil {
					l.nsNeeded[mod.FileName()] = true
				}
			}
		case ast.KindEnumDeclaration:
			l.registerEnum(f, stmt)
		}
	}
}

// namespaceUsedAsValue reports whether the alias name appears in the file
// beyond the import statement (as `alias.member` the checker would keep the
// namespace as an alias; any use at all forces the map because `for (k in
// alias)` and `const x: any = alias` also need it).
func (l *lowerer) namespaceUsedAsValue(f *ast.SourceFile, alias string) bool {
	found := false
	var walk func(*ast.Node, bool)
	walk = func(n *ast.Node, inImport bool) {
		if n == nil || found {
			return
		}
		if e, ok := l.overrides[n]; ok && e.Method != nil {
			return
		}
		if n.Kind == ast.KindIdentifier && !inImport && n.Text() == alias {
			if n.Parent != nil && n.Parent.Kind == ast.KindPropertyAccessExpression && n.Parent.Expression() == n {
				return
			}
			found = true
			return
		}
		skip := inImport || n.Kind == ast.KindImportDeclaration
		n.ForEachChild(func(c *ast.Node) bool { walk(c, skip); return false })
	}
	for _, stmt := range l.statementNodes(f) {
		walk(stmt, stmt.Kind == ast.KindImportDeclaration)
	}
	return found
}

// moduleOfDeclaration resolves the module file an import declaration refers
// to (through its module symbol).
func (l *lowerer) moduleOfDeclaration(d *ast.Node) *ast.SourceFile {
	ck := l.ck
	if ck == nil {
		var done func()
		ck, done = l.prog.prog.GetTypeCheckerForFile(context.Background(), l.file)
		if done != nil {
			defer done()
		}
	}
	mod := ck.GetSymbolAtLocation(d.ModuleSpecifier())
	if mod == nil {
		return nil
	}
	return l.fileOfSymbol(mod)
}

// registerEnum collects a string enum's member values.
func (l *lowerer) registerEnum(f *ast.SourceFile, n *ast.Node) {
	e := n.AsEnumDeclaration()
	if n.Name() == nil || n.Symbol() == nil || e.Members == nil {
		return
	}
	members := map[string]string{}
	for _, m := range e.Members.Nodes {
		if m.Name() == nil {
			continue
		}
		init := m.Initializer()
		if init == nil || init.Kind != ast.KindStringLiteral {
			// Non-string-literal members (numbers, computed) are not lowered.
			return
		}
		members[m.Name().Text()] = init.Text()
	}
	l.enums[f.FileName()+" "+n.Name().Text()] = members
}

// enumOf resolves the member table for an enum symbol.
func (l *lowerer) enumOf(sym *ast.Symbol) map[string]string {
	if sym == nil {
		return nil
	}
	if f := l.fileOfSymbol(sym); f != nil {
		return l.enums[f.FileName()+" "+sym.Name]
	}
	return nil
}

// nsExportOrder lists the exported lowerable class names of a module file in
// source export order: `export * from "..."` recurses, `export {a, b}` and
// local declarations contribute in place.
func (l *lowerer) nsExportOrder(f *ast.SourceFile) []string {
	var out []string
	for _, stmt := range l.statementNodes(f) {
		switch stmt.Kind {
		case ast.KindExportDeclaration:
			d := stmt.AsExportDeclaration()
			if d.ModuleSpecifier == nil {
				continue
			}
			if d.ExportClause == nil {
				// export * from: recurse in target order.
				if mod := l.moduleOfDeclaration(stmt); mod != nil && mod.FileName() != f.FileName() {
					out = append(out, l.nsExportOrder(mod)...)
				}
				continue
			}
			if d.ExportClause != nil && d.ExportClause.Kind == ast.KindNamedExports {
				for _, el := range d.ExportClause.AsNamedExports().Elements.Nodes {
					if el.Name() != nil {
						out = append(out, el.Name().Text())
					}
				}
			}
		case ast.KindClassDeclaration:
			if stmt.Name() != nil && ast.HasSyntacticModifier(stmt, ast.ModifierFlagsExport) {
				out = append(out, stmt.Name().Text())
			}
		case ast.KindVariableStatement:
			if !ast.HasSyntacticModifier(stmt, ast.ModifierFlagsExport) {
				continue
			}
			list := stmt.AsVariableStatement().DeclarationList
			if list == nil || list.Kind != ast.KindVariableDeclarationList {
				continue
			}
			for _, d := range list.AsVariableDeclarationList().Declarations.Nodes {
				if d.Name() != nil && d.Name().Kind == ast.KindIdentifier {
					out = append(out, d.Name().Text())
				}
			}
		}
	}
	return out
}

// moduleClassOf returns (creating once) the module class of a file.
func (l *lowerer) moduleClassOf(f *ast.SourceFile) *hir.Class {
	if m := l.modules[f]; m != nil {
		return m
	}
	qname := l.qualifiedName(f, "module")
	mod := &hir.Class{Node: hir.Node{ID: l.nextID(), Source: qname}, Name: qname}
	l.modules[f] = mod
	l.out.Classes = append(l.out.Classes, mod)
	return mod
}

// lowerModule extends phase 1: namespaces, enums and module functions also
// get a module class.
func (l *lowerer) lowerModule2(f *ast.SourceFile) *hir.Class {
	needNS := l.nsNeeded[f.FileName()]
	var enums, funcs, vars bool
	for _, stmt := range l.statementNodes(f) {
		switch stmt.Kind {
		case ast.KindEnumDeclaration:
			if l.enumOf(stmt.Symbol()) != nil {
				enums = true
			}
		case ast.KindFunctionDeclaration:
			if stmt.Name() != nil {
				funcs = true
			}
		case ast.KindVariableStatement:
			vars = true
		case ast.KindClassDeclaration, ast.KindInterfaceDeclaration, ast.KindTypeAliasDeclaration, ast.KindImportDeclaration, ast.KindExportDeclaration, ast.KindEmptyStatement:
		default:
			l.diagf(stmt, "unsupported-top-level", "top-level %s is not lowered", stmt.Kind.String())
		}
	}
	if !needNS && !enums && !funcs && !vars {
		return nil
	}
	mod := l.moduleClassOf(f)
	init := &hir.Method{Node: hir.Node{ID: l.nextID(), Source: mod.Name}, Name: "class_constructor", Static: true, Result: hir.T(hir.Void)}
	l.class = mod
	l.method = init
	body := []*hir.Stmt{}
	if needNS {
		body = append(body, l.nsMapInit(f, mod)...)
	}
	if enums {
		body = append(body, l.enumArraysInit(f, mod)...)
	}
	if vars {
		l.lowerModuleVars(f, mod, &body)
	}
	if funcs {
		l.moduleFunctions(f, mod)
	}
	if len(body) > 0 || funcs {
		init.Body = hir.B(body...)
		mod.Methods = append(mod.Methods, init)
	}
	return mod
}

// nsMapInit builds the namespace export map: an OrderedMap<string,
// ClassValue> with one entry per exported lowered class, in export order.
func (l *lowerer) nsMapInit(f *ast.SourceFile, mod *hir.Class) []*hir.Stmt {
	typ := hir.T(hir.OrderedMap, hir.T(hir.String), hir.T(hir.ClassValue))
	decl := l.assignStatic(mod, f.AsNode(), "ns", &hir.Expr{Kind: hir.New, Node: hir.Node{ID: l.nextID(), Source: mod.Name}, Type: typ}, typ)
	out := []*hir.Stmt{decl}
	skipped := 0
	for _, name := range l.nsExportOrder(f) {
		var c *hir.Class
		for _, sym := range l.ck.GetExportsOfModule(f.Symbol) {
			if sym.Name != name {
				continue
			}
			if sym.Flags&ast.SymbolFlagsAlias != 0 {
				if target, ok := l.ck.ResolveAlias(sym); ok {
					sym = target
				}
			}
			c = l.classOf(sym)
			break
		}
		if c == nil {
			skipped++
			continue
		}
		out = append(out, &hir.Stmt{Kind: hir.ExprStmt, Node: hir.Node{ID: l.nextID(), Source: mod.Name},
			X: l.rtOp("map.set", &hir.Expr{Kind: hir.StaticGet, Node: hir.Node{ID: l.nextID(), Source: mod.Name}, Owner: mod.Name, Name: "ns", Type: typ}, typ,
				&hir.Expr{Kind: hir.Lit, Node: hir.Node{ID: l.nextID(), Source: mod.Name}, Type: hir.T(hir.String), Value: name},
				&hir.Expr{Kind: hir.ClassOf, Node: hir.Node{ID: l.nextID(), Source: c.Node.Source}, Owner: c.Name, Type: hir.T(hir.ClassValue)})})
	}
	if skipped > 0 {
		l.diagf(f.Statements.Nodes[0], "unsupported-namespace-export", "%s: %d exports cannot be represented by the class-value namespace map", relName(f.FileName()), skipped)
	}
	return out
}

// enumArraysInit creates one static string array per enum holding the member
// values in declaration order, for Object.values(E).
func (l *lowerer) enumArraysInit(f *ast.SourceFile, mod *hir.Class) []*hir.Stmt {
	var out []*hir.Stmt
	names := make([]string, 0)
	for key := range l.enums {
		if strings.HasPrefix(key, f.FileName()+" ") {
			names = append(names, strings.TrimPrefix(key, f.FileName()+" "))
		}
	}
	sort.Strings(names)
	for _, name := range names {
		members := l.enums[f.FileName()+" "+name]
		field := name + "_values"
		typ := hir.T(hir.Array, hir.T(hir.String))
		mod.Fields = append(mod.Fields, hir.Field{Node: hir.Node{ID: l.nextID(), Source: name}, Name: field, Type: typ, Static: true})
		decl := l.assignStatic(mod, f.AsNode(), field, &hir.Expr{Kind: hir.New, Node: hir.Node{ID: l.nextID(), Source: name}, Type: typ}, typ)
		out = append(out, decl)
		// declaration order of the enum members
		for _, stmt := range l.statementNodes(f) {
			if stmt.Kind != ast.KindEnumDeclaration || stmt.Name() == nil || stmt.Name().Text() != name {
				continue
			}
			for _, m := range stmt.AsEnumDeclaration().Members.Nodes {
				if m.Name() == nil {
					continue
				}
				v, ok := members[m.Name().Text()]
				if !ok {
					continue
				}
				out = append(out, &hir.Stmt{Kind: hir.ExprStmt, Node: hir.Node{ID: l.nextID(), Source: name},
					X: l.rtOp("array.push", &hir.Expr{Kind: hir.StaticGet, Node: hir.Node{ID: l.nextID(), Source: name}, Owner: mod.Name, Name: field, Type: typ}, hir.T(hir.I32),
						&hir.Expr{Kind: hir.Lit, Node: hir.Node{ID: l.nextID(), Source: name}, Type: hir.T(hir.String), Value: v})})
			}
		}
	}
	return out
}

// lowerModuleVars lowers the plain module variables (phase-1 moduleVar).
func (l *lowerer) lowerModuleVars(f *ast.SourceFile, mod *hir.Class, body *[]*hir.Stmt) {
	for _, stmt := range l.statementNodes(f) {
		if stmt.Kind == ast.KindVariableStatement && stmt.AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsConst == 0 {
			l.diagf(stmt, "unsupported-top-level", "mutable module variables are not lowered")
			continue
		}
		if stmt.Kind != ast.KindVariableStatement {
			continue
		}
		list := stmt.AsVariableStatement().DeclarationList
		if list == nil || list.Kind != ast.KindVariableDeclarationList {
			continue
		}
		for _, d := range list.AsVariableDeclarationList().Declarations.Nodes {
			if d.Name() != nil && d.Name().Kind == ast.KindIdentifier && d.Symbol() != nil {
				*body = append(*body, l.moduleVar(d, mod)...)
			}
		}
	}
}

// moduleFunctions lowers module-level function declarations to static
// methods of the module class.
func (l *lowerer) moduleFunctions(f *ast.SourceFile, mod *hir.Class) {
	for _, stmt := range l.statementNodes(f) {
		if stmt.Kind != ast.KindFunctionDeclaration || stmt.Name() == nil {
			continue
		}
		name := stmt.Name().Text()
		hm := l.functionToMethod(stmt, mod)
		if hm == nil {
			continue
		}
		l.funcsBy[f.FileName()+" "+name] = funcRef{owner: mod.Name, method: name}
		l.methodsBy[mod.Name+"."+name] = hm
	}
}

// enumValuesField returns the module static holding Object.values(E).
func (l *lowerer) enumValuesField(sym *ast.Symbol) (string, string, bool) {
	f := l.fileOfSymbol(sym)
	if f == nil {
		return "", "", false
	}
	if mod := l.modules[f]; mod != nil {
		name := sym.Name + "_values"
		for _, fd := range mod.Fields {
			if fd.Name == name && fd.Static {
				return mod.Name, name, true
			}
		}
	}
	return "", "", false
}

func relName(name string) string {
	if i := strings.LastIndex(name, string(filepath.Separator)); i >= 0 {
		return name[i+1:]
	}
	return name
}
