package tsfront

import (
	"context"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/jsnum"
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
	numbers := map[string]float64{}
	for _, m := range e.Members.Nodes {
		if m.Name() == nil {
			continue
		}
		value := l.ck.GetConstantValue(m)
		switch v := value.(type) {
		case string:
			members[m.Name().Text()] = v
		case jsnum.Number:
			numeric := float64(v)
			// Canonical nonnegative integer reverse keys sort ahead of member
			// names in JS Object.keys/values. Other numeric domains block.
			if numeric < 0 || numeric >= 4294967295 || math.Trunc(numeric) != numeric {
				l.diagf(m, "unsupported-enum", "numeric enum reverse key is outside the supported integer domain")
				return
			}
			numbers[m.Name().Text()] = numeric
		default:
			l.diagf(m, "unsupported-enum", "enum member must have a constant value")
			return
		}
	}
	key := f.FileName() + " " + n.Name().Text()
	if len(members) > 0 && len(numbers) > 0 {
		l.diagf(n, "unsupported-enum", "heterogeneous enum namespace is not lowered")
		return
	}
	if len(numbers) > 0 {
		l.enumNumbers[key] = numbers
	} else {
		l.enums[key] = members
	}
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
			if l.enumOf(stmt.Symbol()) != nil || l.numericEnumOf(stmt.Symbol()) != nil {
				enums = true
			}
		case ast.KindFunctionDeclaration:
			if stmt.Name() != nil {
				funcs = true
			}
		case ast.KindVariableStatement:
			vars = true
		case ast.KindIfStatement, ast.KindExpressionStatement:
			if !l.moduleInitializer(stmt) {
				l.diagf(stmt, "unsupported-top-level", "top-level %s observes class statics and is not lowered", stmt.Kind.String())
				continue
			}
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
		body = append(body, l.enumNamespacesInit(f, mod)...)
	}
	if vars {
		l.lowerModuleVars(f, mod, &body)
	}
	if funcs {
		l.moduleFunctions(f, mod)
	}
	if vars {
		body = append(body, l.moduleStatements(f)...)
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

// enumNamespacesInit creates each string enum object in declaration order.
func (l *lowerer) enumNamespacesInit(f *ast.SourceFile, mod *hir.Class) []*hir.Stmt {
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
		mapType := hir.T(hir.OrderedMap, hir.T(hir.String), hir.T(hir.String))
		mapField := name + "_namespace"
		mod.Fields = append(mod.Fields, hir.Field{Name: mapField, Type: mapType, Static: true})
		out = append(out, l.assignStatic(mod, f.AsNode(), mapField, &hir.Expr{Kind: hir.New, Type: mapType}, mapType))
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
				out = append(out, &hir.Stmt{Kind: hir.ExprStmt,
					X: l.rtOp("map.set", &hir.Expr{Kind: hir.StaticGet, Owner: mod.Name, Name: mapField, Type: mapType}, mapType,
						hir.L(hir.T(hir.String), m.Name().Text()), hir.L(hir.T(hir.String), v))})
			}
		}
	}
	for _, stmt := range l.statementNodes(f) {
		if stmt.Kind != ast.KindEnumDeclaration {
			continue
		}
		members := l.numericEnumOf(stmt.Symbol())
		if members == nil {
			continue
		}
		field := stmt.Name().Text() + "_namespace"
		typ := hir.T(hir.OrderedMap, hir.T(hir.String), hir.T(hir.Dynamic))
		mod.Fields = append(mod.Fields, hir.Field{Name: field, Type: typ, Static: true})
		out = append(out, l.assignStatic(mod, stmt, field, &hir.Expr{Kind: hir.New, Type: typ}, typ))
		reverse := map[float64]string{}
		for _, m := range stmt.AsEnumDeclaration().Members.Nodes {
			reverse[members[m.Name().Text()]] = m.Name().Text()
		}
		var keys []float64
		for k := range reverse {
			keys = append(keys, k)
		}
		sort.Float64s(keys)
		add := func(key string, value *hir.Expr) {
			out = append(out, &hir.Stmt{Kind: hir.ExprStmt, X: l.rtOp("map.set",
				&hir.Expr{Kind: hir.StaticGet, Owner: mod.Name, Name: field, Type: typ}, typ,
				hir.L(hir.T(hir.String), key), l.rtOp("dynamic.of", value, hir.T(hir.Dynamic)))})
		}
		for _, k := range keys {
			add(strconv.FormatFloat(k, 'f', 0, 64), hir.L(hir.T(hir.String), reverse[k]))
		}
		for _, m := range stmt.AsEnumDeclaration().Members.Nodes {
			add(m.Name().Text(), hir.L(hir.T(hir.Number), members[m.Name().Text()]))
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

func relName(name string) string {
	if i := strings.LastIndex(name, string(filepath.Separator)); i >= 0 {
		return name[i+1:]
	}
	return name
}

// String enums are objects when used as values: computed lookup and
// Object.keys must retain keys and declaration order, including duplicate values.
func (l *lowerer) enumNamespace(n *ast.Node) (*hir.Expr, bool) {
	sym := l.resolve(n)
	if sym == nil || (l.enumOf(sym) == nil && l.numericEnumOf(sym) == nil) {
		return nil, false
	}
	f := l.fileOfSymbol(sym)
	if mod := l.modules[f]; mod != nil {
		valueType := hir.T(hir.String)
		if l.numericEnumOf(sym) != nil {
			valueType = hir.T(hir.Dynamic)
		}
		return &hir.Expr{Kind: hir.StaticGet, Node: l.node(n), Owner: mod.Name,
			Name: sym.Name + "_namespace", Type: hir.T(hir.OrderedMap, hir.T(hir.String), valueType)}, true
	}
	return nil, false
}

func (l *lowerer) numericEnumOf(sym *ast.Symbol) map[string]float64 {
	if sym != nil {
		if f := l.fileOfSymbol(sym); f != nil {
			return l.enumNumbers[f.FileName()+" "+sym.Name]
		}
	}
	return nil
}
