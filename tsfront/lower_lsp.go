package tsfront

import (
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// These are declaration-merged interfaces/namespaces, not methods on a Map
// or a class value. Match the resolved declaration, including import aliases,
// never the spelling of the receiver in user code.
func lspFile(f *ast.SourceFile) bool {
	return f != nil && strings.Contains(strings.ReplaceAll(f.FileName(), "\\", "/"), "/node_modules/vscode-languageserver-types/") && strings.HasSuffix(f.FileName(), ".d.ts")
}

func (l *lowerer) lspSymbol(sym *ast.Symbol) bool {
	return sym != nil && lspFile(l.fileOfSymbol(sym))
}

// Materialize external data interfaces lazily. Register before walking members
// so recursive DocumentSymbol.children has a stable named identity. All field
// types and optionality come from the checked library declarations.
func (l *lowerer) lspShape(sym *ast.Symbol) *hir.Class {
	if !l.lspConstructors || !l.lspSymbol(sym) {
		return nil
	}
	switch sym.Name {
	case "Position", "Range", "TextEdit", "TextDocumentEdit", "Command", "DocumentSymbol", "Location", "OptionalVersionedTextDocumentIdentifier", "TextDocumentIdentifier", "VersionedTextDocumentIdentifier", "RenameFile", "RenameFileOptions", "ResourceOperation":
	default:
		return nil
	}
	if c := l.classOf(sym); c != nil {
		return c
	}
	for _, decl := range sym.Declarations {
		if decl.Kind != ast.KindInterfaceDeclaration {
			continue
		}
		file := ast.GetSourceFileOfNode(decl)
		saved := l.file
		l.file = file
		defer func() { l.file = saved }()
		c := &hir.Class{Node: l.node(decl), Name: l.qualifiedName(file, sym.Name)}
		l.classes[sym] = c
		l.classesByName[file.FileName()+" "+sym.Name] = c
		l.out.Classes = append(l.out.Classes, c)
		l.dataInterfaceClass(decl, &hir.Interface{Node: c.Node, Name: c.Name})
		return c
	}
	return nil
}

func (l *lowerer) lspCall(n, receiver *ast.Node, method string) (*hir.Expr, bool) {
	sym := l.resolve(receiver)
	if !l.lspSymbol(sym) {
		return nil, false
	}
	name := sym.Name
	args := n.Arguments()
	supported := (name == "Position" && method == "create" && len(args) == 2) ||
		(name == "Range" && method == "create" && (len(args) == 2 || len(args) == 4)) ||
		(name == "TextEdit" && method == "replace" && len(args) == 2) ||
		(name == "TextDocumentEdit" && method == "create" && len(args) == 2) ||
		(name == "Command" && method == "create" && len(args) >= 2) ||
		(name == "RenameFile" && method == "create" && len(args) >= 2 && len(args) <= 4)
	if !supported {
		l.diagf(n, "unsupported-lsp-call", "vscode-languageserver-types %s.%s with %d arguments is not lowered", name, method, len(args))
		return nil, true
	}
	c := l.lspShape(sym)
	if c == nil || c.Ctor == nil {
		l.diagf(n, "unsupported-lsp-call", "vscode-languageserver-types %s has no lowered data constructor", name)
		return nil, true
	}
	values := map[string]*hir.Expr{}
	if name == "Range" && len(args) == 4 {
		pos := l.lspShape(l.resolveInterfaceSymbol(sym, "Position"))
		if pos == nil {
			return nil, true
		}
		var numbers []*hir.Expr
		for _, arg := range args {
			saved := l.hint
			l.hint = hir.T(hir.Number)
			x := l.expr(arg)
			l.hint = saved
			if x == nil {
				return nil, true
			}
			numbers = append(numbers, l.tempInit(arg, hir.T(hir.Number), l.coerce(x, hir.T(hir.Number))))
		}
		values["start"] = &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(pos.Name), Args: numbers[:2]}
		values["end"] = &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(pos.Name), Args: numbers[2:]}
	} else {
		fields := map[string][]string{"Position": {"line", "character"}, "Range": {"start", "end"}, "TextEdit": {"range", "newText"}, "TextDocumentEdit": {"textDocument", "edits"}, "Command": {"title", "command"}, "RenameFile": {"oldUri", "newUri", "options", "annotationId"}}[name]
		for i, field := range fields {
			if i >= len(args) {
				break
			}
			var typ hir.Type
			for _, p := range c.Ctor.Params {
				if p.Name == field {
					typ = p.Type
				}
			}
			saved := l.hint
			l.hint = typ
			x := l.expr(args[i])
			l.hint = saved
			if x == nil {
				return nil, true
			}
			values[field] = l.tempInit(args[i], typ, l.coerce(x, typ))
		}
		if name == "Command" && len(args) > 2 {
			typ := hir.T(hir.Array, hir.T(hir.Dynamic))
			var items []*hir.Expr
			for _, arg := range args[2:] {
				if arg.Kind == ast.KindSpreadElement {
					l.diagf(arg, "unsupported-lsp-call", "Command.create spread arguments are not lowered")
					return nil, true
				}
				saved := l.hint
				l.hint = hir.T(hir.Dynamic)
				x := l.expr(arg)
				l.hint = saved
				if x == nil {
					return nil, true
				}
				items = append(items, l.tempInit(arg, hir.T(hir.Dynamic), l.coerce(x, hir.T(hir.Dynamic))))
			}
			array := l.tempInit(n, typ, &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ})
			for _, item := range items {
				l.pendStmt(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", array, hir.T(hir.I32), item)})
			}
			values["arguments"] = array
		}
	}
	if name == "RenameFile" {
		values["kind"] = hir.L(hir.T(hir.String), "rename")
	}
	var ctorArgs []*hir.Expr
	for _, p := range c.Ctor.Params {
		x := values[p.Name]
		if x == nil {
			if p.Type.Kind != hir.Optional {
				l.diagf(n, "unsupported-lsp-call", "%s constructor misses %s", name, p.Name)
				return nil, true
			}
			x = &hir.Expr{Kind: hir.Lit, Type: p.Type}
		}
		ctorArgs = append(ctorArgs, l.coerce(x, p.Type))
	}
	return &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(c.Name), Args: ctorArgs}, true
}

func (l *lowerer) scanLSPConstructors(n *ast.Node) {
	if n == nil || l.unexecuted[n] != "" {
		return
	}
	if n.Kind == ast.KindCallExpression {
		callee := n.Expression()
		if callee != nil && callee.Kind == ast.KindPropertyAccessExpression {
			method := callee.Name().Text()
			if (method == "create" || method == "replace") && l.lspSymbol(l.resolve(callee.Expression())) {
				l.lspConstructors = true
			}
		}
	}
	n.ForEachChild(func(child *ast.Node) bool { l.scanLSPConstructors(child); return false })
}

// Find a sibling interface in the same external declaration file.
func (l *lowerer) resolveInterfaceSymbol(sym *ast.Symbol, name string) *ast.Symbol {
	f := l.fileOfSymbol(sym)
	if f == nil {
		return nil
	}
	for _, stmt := range l.statementNodes(f) {
		if stmt.Kind == ast.KindInterfaceDeclaration && stmt.Name() != nil && stmt.Name().Text() == name {
			return stmt.Symbol()
		}
	}
	return nil
}
