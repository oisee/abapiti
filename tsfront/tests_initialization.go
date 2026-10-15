package tsfront

import (
	"sort"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

const testInitializationPrefix = "test.initialize."
const testReadyPrefix = "test.ready."

// Each original top-level statement has an independent host recovery boundary.
// No source is rewritten and no oracle titles or synthetic tests are injected.
func (l *lowerer) testModuleActions(f *ast.SourceFile, mod *hir.Class) {
	for i, stmt := range l.statementNodes(f) {
		switch stmt.Kind {
		case ast.KindModuleDeclaration, ast.KindFunctionDeclaration, ast.KindClassDeclaration, ast.KindInterfaceDeclaration, ast.KindTypeAliasDeclaration, ast.KindJSTypeAliasDeclaration, ast.KindEnumDeclaration, ast.KindImportDeclaration, ast.KindExportDeclaration, ast.KindEmptyStatement:
			continue
		}
		action := &hir.Method{Node: l.node(stmt), Name: testInitializationPrefix + itoa(i), Static: true, Result: hir.T(hir.Void)}
		l.method = action
		before := len(l.diags)
		var body []*hir.Stmt
		if stmt.Kind == ast.KindVariableStatement {
			l.lowerModuleVarStatements([]*ast.Node{stmt}, mod, &body)
			for _, d := range stmt.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
				if d.Name() == nil {
					continue
				}
				for _, f := range mod.Fields {
					if f.Name == testReadyPrefix+d.Name().Text() {
						body = append(body, l.assignStatic(mod, d, f.Name, hir.L(hir.T(hir.Bool), true), f.Type))
					}
				}
			}
		} else if stmt.Kind == ast.KindExpressionStatement || stmt.Kind == ast.KindIfStatement || stmt.Kind == ast.KindForOfStatement || stmt.Kind == ast.KindThrowStatement {
			body = l.stmts(stmt)
		} else {
			l.diagf(stmt, "unsupported-top-level", "top-level %s is not lowered", stmt.Kind.String())
		}
		action.Body = hir.B(body...)
		l.trapBodyDiagnostics(stmt, action, before)
		mod.Methods = append(mod.Methods, action)
	}
}

// Recover a typed registration-row context for heterogeneous literals. The
// source checker widens their union to object; the helper's optional row fields
// supply the actual structural context, exactly as for an inline call argument.
func (l *lowerer) testModuleArrayContext(d *ast.Node) (hir.Type, bool) {
	if !l.integerOptions.TrapOnFailure || !strings.HasPrefix(l.relFile(l.file), "test/") || d.Initializer() == nil || d.Initializer().Kind != ast.KindArrayLiteralExpression {
		return hir.Type{}, false
	}
	var result hir.Type
	var walk func(*ast.Node)
	walk = func(n *ast.Node) {
		if n.Kind == ast.KindCallExpression && n.Expression().Kind == ast.KindIdentifier {
			if sym := l.resolve(n.Expression()); sym != nil {
				if f := l.fileOfSymbol(sym); f != nil {
					if ref, ok := l.funcsBy[f.FileName()+" "+sym.Name]; ok {
						if m := l.methodsBy[ref.owner+"."+ref.method]; m != nil {
							for i, arg := range n.Arguments() {
								if i < len(m.Params) && arg.Kind == ast.KindIdentifier && l.resolve(arg) == d.Symbol() && m.Params[i].Type.Kind == hir.Array {
									result = m.Params[i].Type
								}
							}
						}
					}
				}
			}
		}
		n.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
	}
	if d.Parent != nil && d.Parent.Parent != nil && d.Parent.Parent.Parent != nil && d.Parent.Parent.Parent.Kind == ast.KindSourceFile {
		walk(l.file.AsNode())
	}
	if result.Kind == hir.Array {
		return result, true
	}
	// Fresh row literals can have different optional fields. Mapping their
	// reference union to Object loses even common title/data fields. Recover
	// the checker's normalized structural row, but never a nominal class union
	// or a shape that would drop an exclusive own property.
	array := l.ck.GetTypeAtLocation(d.Initializer())
	info := l.ck.GetIndexInfoOfType(array, l.ck.GetNumberType())
	if info == nil {
		return hir.Type{}, false
	}
	row := info.ValueType()
	props := l.ck.GetPropertiesOfType(row)
	if row.IsUnion() {
		props = l.ck.GetAllPossiblePropertiesOfTypes(row.AsUnionOrIntersectionType().Types())
		sort.Slice(props, func(i, j int) bool { return props[i].Name < props[j].Name })
	}
	if len(props) == 0 {
		return hir.Type{}, false
	}
	for _, el := range d.Initializer().AsArrayLiteralExpression().Elements.Nodes {
		if el.Kind != ast.KindObjectLiteralExpression {
			return hir.Type{}, false
		}
		for _, p := range el.AsObjectLiteralExpression().Properties.Nodes {
			if p.Kind != ast.KindPropertyAssignment || p.Name() == nil || (p.Name().Kind != ast.KindIdentifier && p.Name().Kind != ast.KindStringLiteral) {
				return hir.Type{}, false
			}
			found := false
			for _, prop := range props {
				if prop.Name == p.Name().Text() {
					found = true
				}
			}
			if !found {
				return hir.Type{}, false
			}
		}
	}
	if shape := l.synthFromProperties(d, props); shape != nil {
		return hir.T(hir.Array, hir.Ref(shape.Name)), true
	}
	return hir.Type{}, false
}

// A runtime initializer failure must not expose a fabricated zero/undefined
// value to later suites. The host can continue, but reads of that binding trap.
func (l *lowerer) testModuleRead(n *ast.Node, mod *hir.Class, name string, typ hir.Type) *hir.Expr {
	value := &hir.Expr{Kind: hir.StaticGet, Node: l.node(n), Owner: mod.Name, Name: name, Type: typ}
	if l.integerOptions.TrapOnFailure {
		for _, f := range mod.Fields {
			if f.Name == testReadyPrefix+name {
				ready := &hir.Expr{Kind: hir.StaticGet, Node: l.node(n), Owner: mod.Name, Name: f.Name, Type: f.Type}
				guard := &hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Type: hir.T(hir.Bool), Op: "!", X: ready}, Body: failureBody(l.node(n), "module binding "+mod.Name+"."+name+" was not initialized")}
				return &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: typ, Stmt: hir.B(guard), Y: value}
			}
		}
	}
	return value
}

func (l *lowerer) testShapeView(source, view hir.Type) bool {
	if source.Kind != hir.ClassRef || view.Kind != hir.ClassRef || !strings.HasPrefix(source.Name, "shape.") || !strings.HasPrefix(view.Name, "shape.") {
		return false
	}
	a, b := l.classByName(source.Name), l.classByName(view.Name)
	if a == nil || b == nil {
		return false
	}
	seen := map[string]bool{}
	for _, want := range b.Fields {
		found := false
		for _, have := range a.Fields {
			if have.Name == want.Name && (have.Type.Kind == hir.Dynamic || l.acceptsType(have.Type, want.Type)) {
				found = true
				seen[have.Name] = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, have := range a.Fields {
		if !seen[have.Name] && have.Type.Kind != hir.Optional {
			return false
		}
	}
	return true
}

func isTestInitialization(m *hir.Method) bool {
	return strings.HasPrefix(m.Name, testInitializationPrefix)
}
