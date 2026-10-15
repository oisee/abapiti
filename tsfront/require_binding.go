package tsfront

import (
	"context"

	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Static require bindings. JSDoc-typed .mjs (OSG's gogen) loads a package
// through Node's createRequire so that it shares the copy another package
// resolves:
//
//	import {createRequire} from "node:module";
//	const require = createRequire(import.meta.url);
//	const other = createRequire(require.resolve("pkg/package.json"));
//	/** @type {typeof import("lib")} */
//	const lib = other("lib");
//	/** @type {typeof import("lib/x")} */
//	const {X} = require("lib/x");
//	const {A, B} = lib;
//
// A requirer call whose declaration is typed `typeof import("<literal>")`
// with the same literal is the static import of that module: the bound
// names resolve (l.resolve) to the module symbol or its resolved exports,
// exactly as `import * as lib` / `import {X}` would. The requirers and the
// bindings are plumbing: their statements are not lowered (statementNodes).
// Any other use of createRequire or a requirer keeps today's lowering and
// its blocking diagnostics; a requirer call that cannot be proven static
// gets an unsupported-require diagnostic instead of a guess.

// requireBindings holds the facts of all lowered files.
type requireBindings struct {
	targets map[*ast.Symbol]*ast.Symbol // binding variable -> module or export
	skip    map[*ast.Node]bool          // top-level statements that are only plumbing
}

// requireTarget maps a static require binding to its module or export symbol.
func (l *lowerer) requireTarget(sym *ast.Symbol) *ast.Symbol {
	if l.require == nil || sym == nil {
		return nil
	}
	return l.require.targets[sym]
}

// dropRequirePlumbing removes plumbing statements from a statement list.
func (l *lowerer) dropRequirePlumbing(nodes []*ast.Node) []*ast.Node {
	if l.require == nil || len(l.require.skip) == 0 {
		return nodes
	}
	var out []*ast.Node
	for _, n := range nodes {
		if !l.require.skip[n] {
			out = append(out, n)
		}
	}
	return out
}

// scanRequireBindings records the static require bindings of files. It runs
// before any statement list is read.
func (l *lowerer) scanRequireBindings(files []string) {
	l.require = &requireBindings{targets: map[*ast.Symbol]*ast.Symbol{}, skip: map[*ast.Node]bool{}}
	saved, savedCk := l.file, l.ck
	defer func() { l.file, l.ck = saved, savedCk }()
	for _, name := range files {
		f, ok := l.prog.File(name)
		if !ok || !requireCandidate(f) {
			continue
		}
		ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		l.scanRequireFile(f)
		done()
	}
}

// requireCandidate: only files importing createRequire can bind statically.
func requireCandidate(f *ast.SourceFile) bool {
	for _, stmt := range f.Statements.Nodes {
		if stmt.Kind == ast.KindImportDeclaration && isNodeModuleSpecifier(stmt.AsImportDeclaration().ModuleSpecifier) {
			return true
		}
	}
	return false
}

func isNodeModuleSpecifier(spec *ast.Node) bool {
	return spec != nil && spec.Kind == ast.KindStringLiteral && (spec.Text() == "node:module" || spec.Text() == "module")
}

func (l *lowerer) scanRequireFile(f *ast.SourceFile) {
	creators := map[*ast.Symbol]bool{}  // createRequire import bindings
	requirers := map[*ast.Symbol]bool{} // createRequire results
	for _, stmt := range f.Statements.Nodes {
		if stmt.Kind != ast.KindImportDeclaration || !isNodeModuleSpecifier(stmt.AsImportDeclaration().ModuleSpecifier) {
			continue
		}
		clause := stmt.AsImportDeclaration().ImportClause
		if clause == nil || clause.AsImportClause().NamedBindings == nil || clause.AsImportClause().NamedBindings.Kind != ast.KindNamedImports {
			continue
		}
		for _, el := range clause.AsImportClause().NamedBindings.AsNamedImports().Elements.Nodes {
			imported := el.Name()
			if prop := el.AsImportSpecifier().PropertyName; prop != nil {
				imported = prop
			}
			if imported != nil && imported.Text() == "createRequire" && el.Symbol() != nil {
				creators[el.Symbol()] = true
			}
		}
	}
	if len(creators) == 0 {
		return
	}
	symbolIn := func(n *ast.Node, set map[*ast.Symbol]bool) bool {
		if n == nil || n.Kind != ast.KindIdentifier {
			return false
		}
		sym := l.ck.GetSymbolAtLocation(n)
		return sym != nil && set[sym]
	}
	// createRequire(import.meta.url) or createRequire(r.resolve("<literal>")).
	isRequirer := func(init *ast.Node) bool {
		if init == nil || init.Kind != ast.KindCallExpression || !symbolIn(init.Expression(), creators) {
			return false
		}
		args := init.Arguments()
		if len(args) != 1 {
			return false
		}
		arg := args[0]
		if arg.Kind == ast.KindPropertyAccessExpression && arg.Name().Text() == "url" && ast.IsImportMeta(arg.Expression()) {
			return true
		}
		if arg.Kind != ast.KindCallExpression || arg.Expression().Kind != ast.KindPropertyAccessExpression {
			return false
		}
		callee := arg.Expression()
		inner := arg.Arguments()
		return callee.Name().Text() == "resolve" && symbolIn(callee.Expression(), requirers) && len(inner) == 1 && inner[0].Kind == ast.KindStringLiteral
	}
	for _, stmt := range f.Statements.Nodes {
		if stmt.Kind != ast.KindVariableStatement {
			continue
		}
		decls := stmt.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
		plumbing := 0
		for _, d := range decls {
			init := d.Initializer()
			switch {
			case d.Name().Kind == ast.KindIdentifier && isRequirer(init):
				requirers[d.Symbol()] = true
				plumbing++
			case init != nil && init.Kind == ast.KindCallExpression && symbolIn(init.Expression(), requirers):
				if mod := l.requiredModule(d, init); mod != nil && l.bindRequired(d, mod) {
					plumbing++
				}
			case init != nil && init.Kind == ast.KindIdentifier && d.Name().Kind == ast.KindObjectBindingPattern:
				if mod := l.requireTarget(l.ck.GetSymbolAtLocation(init)); mod != nil && mod.Flags&ast.SymbolFlagsValueModule != 0 && l.bindRequired(d, mod) {
					plumbing++
				}
			}
		}
		if plumbing == len(decls) {
			l.require.skip[stmt] = true
		} else if plumbing > 0 {
			l.diagf(stmt, "unsupported-require", "require bindings must not share a declaration statement with other values")
		}
	}
}

// requiredModule proves `req("<literal>")` static: the declaration carries
// `typeof import("<same literal>")`; the module is the program's resolution
// of that import type.
func (l *lowerer) requiredModule(d, call *ast.Node) *ast.Symbol {
	args := call.Arguments()
	if len(args) != 1 || args[0].Kind != ast.KindStringLiteral {
		l.diagf(call, "unsupported-require", "dynamic require: the argument must be one string literal")
		return nil
	}
	spec := args[0].Text()
	typ := d.Type()
	if typ == nil || !ast.IsLiteralImportTypeNode(typ) || !typ.AsImportTypeNode().IsTypeOf || typ.AsImportTypeNode().Qualifier != nil || typ.AsImportTypeNode().TypeArguments != nil {
		l.diagf(call, "unsupported-require", "dynamic require(%q): the declaration needs a JSDoc @type {typeof import(%q)}", spec, spec)
		return nil
	}
	literal := typ.AsImportTypeNode().Argument.AsLiteralTypeNode().Literal
	if literal.Text() != spec {
		l.diagf(call, "unsupported-require", "dynamic require(%q) is annotated typeof import(%q): the literals differ", spec, literal.Text())
		return nil
	}
	mod := l.ck.GetSymbolAtLocation(literal)
	if mod == nil || mod.Flags&ast.SymbolFlagsValueModule == 0 {
		l.diagf(call, "unsupported-require", "require(%q): the module does not resolve in the program", spec)
		return nil
	}
	return mod
}

// bindRequired maps the declared names of d to the module (identifier) or to
// its resolved exports (object destructuring of plain names).
func (l *lowerer) bindRequired(d *ast.Node, mod *ast.Symbol) bool {
	name := d.Name()
	if name.Kind == ast.KindIdentifier {
		l.require.targets[d.Symbol()] = mod
		return true
	}
	if name.Kind != ast.KindObjectBindingPattern {
		l.diagf(d, "unsupported-require", "require binding must be a name or an object pattern")
		return false
	}
	exports := map[string]*ast.Symbol{}
	for _, sym := range l.ck.GetExportsOfModule(mod) {
		exports[sym.Name] = sym
	}
	bound := map[*ast.Symbol]*ast.Symbol{}
	for _, el := range name.AsBindingPattern().Elements.Nodes {
		be := el.AsBindingElement()
		if be.DotDotDotToken != nil || el.Initializer() != nil || el.Name() == nil || el.Name().Kind != ast.KindIdentifier {
			l.diagf(el, "unsupported-require", "require destructuring supports plain names only")
			return false
		}
		key := el.Name()
		if be.PropertyName != nil {
			key = be.PropertyName
		}
		if key.Kind != ast.KindIdentifier {
			l.diagf(el, "unsupported-require", "require destructuring supports plain names only")
			return false
		}
		target := exports[key.Text()]
		if target == nil {
			l.diagf(el, "unsupported-require", "module has no export %s", key.Text())
			return false
		}
		if target.Flags&ast.SymbolFlagsAlias != 0 {
			if resolved, ok := l.ck.ResolveAlias(target); ok {
				target = resolved
			}
		}
		if !immutableExport(target) {
			// Node copies the current value; a static binding would read live.
			l.diagf(el, "unsupported-require", "require destructuring of %s: only class, function, enum, namespace and const exports (unchangeable after initialization) may be destructured", key.Text())
			return false
		}
		bound[el.Symbol()] = target
	}
	for k, v := range bound {
		l.require.targets[k] = v
	}
	return true
}

// immutableExport: destructuring copies the export's value at that moment,
// which equals a static binding only when the export cannot be reassigned
// after module initialization.
func immutableExport(sym *ast.Symbol) bool {
	if sym.Flags&(ast.SymbolFlagsClass|ast.SymbolFlagsFunction|ast.SymbolFlagsEnum|ast.SymbolFlagsValueModule) != 0 {
		return true
	}
	d := sym.ValueDeclaration
	return sym.Flags&ast.SymbolFlagsBlockScopedVariable != 0 && d != nil && d.Kind == ast.KindVariableDeclaration && ast.IsVarConst(d)
}
