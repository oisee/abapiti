package tsfront

import (
	"context"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/core"
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
//	/** @type {typeof import("lib")} */
//	const direct = createRequire(import.meta.url)("lib");
//
// A requirer call whose declaration is typed `typeof import("<spec>")`,
// where the program resolves the require literal (CommonJS mode) and the
// import type to the same source file (".../x" and ".../x.js" may both name
// x.ts), is the static import of that module: the bound
// names resolve (l.resolve) to the module symbol or its resolved exports,
// exactly as `import * as lib` / `import {X}` would. The requirers and the
// bindings are plumbing: their statements are not lowered (statementNodes).
// Any other use of createRequire or a requirer keeps today's lowering and
// its blocking diagnostics; a requirer call that cannot be proven static
// gets an unsupported-require diagnostic instead of a guess.
//
// Destructuring copies values in Node, so it may only name exports that
// cannot change after module initialization (classes, functions, enums,
// namespaces, const variables); a plain binding reads members live.
//
// Contract: the binding is resolved at build time through the program's TS
// resolution of `typeof import("<literal>")` (tsconfig paths included). It
// does not model Node's runtime resolution relative to the requirer's base
// (import.meta.url or the require.resolve result), nor load errors thrown by
// createRequire, require.resolve or the require call. The build's closure
// must contain exactly the module Node loads; for gogen this is checked by
// byte comparison of the generated outputs.

// requireBindings holds the facts of all lowered files.
type requireBindings struct {
	targets   map[*ast.Symbol]*ast.Symbol // binding variable -> module or export
	skip      map[*ast.Node]bool          // top-level statements that are only plumbing
	requirers map[*ast.Symbol]bool        // createRequire results, of every file
	inline    map[*ast.Node]*ast.Symbol   // typed inline require expression -> module
}

// inlineRequire returns the module of a typed inline require expression,
// `/** @type {typeof import("<spec>")} */ (req("<spec>"))`, or nil.
func (l *lowerer) inlineRequire(n *ast.Node) *ast.Symbol {
	if l.require == nil || len(l.require.inline) == 0 {
		return nil
	}
	for n != nil && n.Kind == ast.KindParenthesizedExpression {
		if mod := l.require.inline[n]; mod != nil {
			return mod
		}
		n = n.Expression()
	}
	if n == nil {
		return nil
	}
	return l.require.inline[n]
}

// requireTarget maps a static require binding to its module or export symbol.
// An import or re-export of a binding (named, renamed, namespace member,
// chains) is an alias that resolves to the binding itself.
func (l *lowerer) requireTarget(sym *ast.Symbol) *ast.Symbol {
	if l.require == nil || sym == nil || len(l.require.targets) == 0 {
		return nil
	}
	if target := l.require.targets[sym]; target != nil {
		return target
	}
	if sym.Flags&ast.SymbolFlagsAlias != 0 && l.ck != nil {
		if resolved, ok := l.ck.ResolveAlias(sym); ok {
			return l.require.targets[resolved]
		}
	}
	return nil
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
	l.require = &requireBindings{targets: map[*ast.Symbol]*ast.Symbol{}, skip: map[*ast.Node]bool{},
		requirers: map[*ast.Symbol]bool{}, inline: map[*ast.Node]*ast.Symbol{}}
	saved, savedCk := l.file, l.ck
	defer func() { l.file, l.ck = saved, savedCk }()
	// Files creating requirers first; then files that only import one (a
	// requirer exported by another module, as gogen's frontend-base.mjs).
	for _, creating := range []bool{true, false} {
		for _, name := range files {
			f, ok := l.prog.File(name)
			if !ok || requireCandidate(f) != creating {
				continue
			}
			ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
			l.file, l.ck = f, ck
			if creating || l.importsRequirer(f) {
				l.scanRequireFile(f)
			}
			done()
		}
	}
}

// importsRequirer: f imports a binding that resolves to a requirer.
func (l *lowerer) importsRequirer(f *ast.SourceFile) bool {
	if len(l.require.requirers) == 0 {
		return false
	}
	for _, stmt := range f.Statements.Nodes {
		if stmt.Kind != ast.KindImportDeclaration {
			continue
		}
		clause := stmt.AsImportDeclaration().ImportClause
		if clause == nil || clause.AsImportClause().NamedBindings == nil || clause.AsImportClause().NamedBindings.Kind != ast.KindNamedImports {
			continue
		}
		for _, el := range clause.AsImportClause().NamedBindings.AsNamedImports().Elements.Nodes {
			if l.requirerSymbol(el.Symbol()) {
				return true
			}
		}
	}
	return false
}

// requirerSymbol: sym is a requirer or an import/export alias of one.
func (l *lowerer) requirerSymbol(sym *ast.Symbol) bool {
	if sym == nil {
		return false
	}
	if l.require.requirers[sym] {
		return true
	}
	if sym.Flags&ast.SymbolFlagsAlias != 0 {
		if resolved, ok := l.ck.ResolveAlias(sym); ok {
			return l.require.requirers[resolved]
		}
	}
	return false
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
	creators := map[*ast.Symbol]bool{} // createRequire import bindings
	requirers := l.require.requirers   // createRequire results
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
	symbolIn := func(n *ast.Node, set map[*ast.Symbol]bool) bool {
		if n == nil || n.Kind != ast.KindIdentifier {
			return false
		}
		sym := l.ck.GetSymbolAtLocation(n)
		if sym != nil && sym.Flags&ast.SymbolFlagsAlias != 0 && !set[sym] {
			if resolved, ok := l.ck.ResolveAlias(sym); ok {
				sym = resolved
			}
		}
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
			case init != nil && init.Kind == ast.KindCallExpression && (symbolIn(init.Expression(), requirers) || isRequirer(init.Expression()) && d.Type() != nil):
				// req("<spec>") or the direct createRequire(base)("<spec>").
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
	// A function may require lazily (an import cycle broken at call time):
	// `const {f} = require("./m.mjs")` in a body binds like the top-level
	// form. Destructuring is limited to unchangeable exports, so the values
	// Node copies at the call are the module's own.
	var nested func(*ast.Node, bool)
	nested = func(n *ast.Node, top bool) {
		if n == nil {
			return
		}
		if !top && n.Kind == ast.KindAsExpression && ast.IsJSDocTypeAssertion(n.Parent) {
			// A typed inline require: (req("<spec>")).member, the
			// annotation proving the module as for a declaration.
			if call := n.Expression(); call != nil && call.Kind == ast.KindCallExpression && (symbolIn(call.Expression(), requirers) || isRequirer(call.Expression())) {
				if mod := l.requiredModuleTyped(n.Type(), call); mod != nil {
					l.require.inline[n.Parent] = mod
				}
				return
			}
		}
		if !top && n.Kind == ast.KindVariableStatement {
			decls := n.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
			if len(decls) == 1 {
				d := decls[0]
				init := d.Initializer()
				// An untyped lazy require keeps its ordinary lowering.
				if init != nil && init.Kind == ast.KindCallExpression && d.Type() != nil && (symbolIn(init.Expression(), requirers) || isRequirer(init.Expression())) {
					if mod := l.requiredModule(d, init); mod != nil && l.bindRequired(d, mod) {
						l.require.skip[n] = true
					}
					return
				}
			}
		}
		n.ForEachChild(func(c *ast.Node) bool { nested(c, false); return false })
	}
	for _, stmt := range f.Statements.Nodes {
		if stmt.Kind == ast.KindVariableStatement {
			for _, d := range stmt.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
				if d.Initializer() != nil {
					nested(d.Initializer(), false)
				}
			}
			continue
		}
		nested(stmt, true)
	}
}

// requiredModule proves `req("<literal>")` static: the declaration carries
// `typeof import("<same literal>")`; the module is the program's resolution
// of that import type.
func (l *lowerer) requiredModule(d, call *ast.Node) *ast.Symbol {
	return l.requiredModuleTyped(d.Type(), call)
}

func (l *lowerer) requiredModuleTyped(declared, call *ast.Node) *ast.Symbol {
	args := call.Arguments()
	if len(args) != 1 || args[0].Kind != ast.KindStringLiteral {
		l.diagf(call, "unsupported-require", "dynamic require: the argument must be one string literal")
		return nil
	}
	spec := args[0].Text()
	typ := l.pickedImportType(declared)
	if typ == nil || !ast.IsLiteralImportTypeNode(typ) || !typ.AsImportTypeNode().IsTypeOf || typ.AsImportTypeNode().Qualifier != nil || typ.AsImportTypeNode().TypeArguments != nil {
		l.diagf(call, "unsupported-require", "dynamic require(%q): the declaration needs a JSDoc @type {typeof import(%q)}", spec, spec)
		return nil
	}
	literal := typ.AsImportTypeNode().Argument.AsLiteralTypeNode().Literal
	mod := l.ck.GetSymbolAtLocation(literal)
	if mod == nil || mod.Flags&ast.SymbolFlagsValueModule == 0 {
		l.diagf(call, "unsupported-require", "require(%q): typeof import(%q) does not resolve in the program", spec, literal.Text())
		return nil
	}
	// The literals may differ in spelling (".../x" vs ".../x.js"); the
	// program's own resolution of the require literal (CommonJS mode, from
	// this file) must reach the same source file as the import type.
	modFile := l.fileOfSymbol(mod)
	resolved := l.prog.prog.ResolveModuleName(spec, l.file.FileName(), core.ResolutionModeCommonJS)
	var reqFile *ast.SourceFile
	if resolved.IsResolved() {
		reqFile, _ = l.prog.File(resolved.ResolvedFileName)
	}
	if modFile == nil || reqFile == nil || reqFile != modFile {
		target := "nothing in the program"
		if reqFile != nil {
			target = l.relFile(reqFile)
		}
		l.diagf(call, "unsupported-require", "require(%q) resolves to %s, but it is annotated typeof import(%q): different modules", spec, target, literal.Text())
		return nil
	}
	return mod
}

// pickedImportType unwraps `Pick<typeof import("<spec>"), K>`: destructuring
// such a binding names only members of K, each the module's own export.
func (l *lowerer) pickedImportType(typ *ast.Node) *ast.Node {
	if typ == nil || typ.Kind != ast.KindTypeReference {
		return typ
	}
	ref := typ.AsTypeReferenceNode()
	if ref.TypeName.Kind != ast.KindIdentifier || ref.TypeName.Text() != "Pick" || ref.TypeArguments == nil || len(ref.TypeArguments.Nodes) != 2 {
		return typ
	}
	if sym := l.ck.GetSymbolAtLocation(ref.TypeName); sym == nil || !l.librarySymbol(sym) {
		return typ
	}
	return ref.TypeArguments.Nodes[0]
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

// requireNamespaceValue refuses a require-bound module (or an import of
// one) used as a value: Node's object is the package's exports, which the
// class-value export map does not model. Members and classes stay usable.
func (l *lowerer) requireNamespaceValue(n *ast.Node) bool {
	target := l.requireTarget(l.ck.GetSymbolAtLocation(n))
	if target == nil || target.Flags&ast.SymbolFlagsValueModule == 0 {
		return false
	}
	l.diagf(n, "unsupported-require", "namespace %s bound by require is not a value; use its members", n.Text())
	return true
}

// requiredMember resolves `m.name` where m is a require-bound module (a
// binding, an import of one, or a typed inline require) to the module's
// export symbol, aliases resolved; nil otherwise. The member is read live
// from the module, as Node's exports object would give it.
func (l *lowerer) requiredMember(n *ast.Node) *ast.Symbol {
	if l.require == nil || n == nil || n.Kind != ast.KindPropertyAccessExpression {
		return nil
	}
	recv := n.Expression()
	mod := l.inlineRequire(recv)
	if mod == nil && recv.Kind == ast.KindIdentifier {
		mod = l.requireTarget(l.ck.GetSymbolAtLocation(recv))
	}
	if mod == nil || mod.Flags&ast.SymbolFlagsValueModule == 0 {
		return nil
	}
	name := n.Name().Text()
	for _, sym := range l.ck.GetExportsOfModule(mod) {
		if sym.Name != name {
			continue
		}
		if sym.Flags&ast.SymbolFlagsAlias != 0 {
			if resolved, ok := l.ck.ResolveAlias(sym); ok {
				return resolved
			}
		}
		return sym
	}
	return nil
}

// requiredMemberRead lowers a read of a module variable through a required
// module, like an imported binding's read.
func (l *lowerer) requiredMemberRead(n *ast.Node) (*hir.Expr, bool) {
	sym := l.requiredMember(n)
	if sym == nil {
		return nil, false
	}
	owner, fieldName, ok := l.modvarOf(sym)
	if !ok {
		return nil, false
	}
	for _, mod := range l.out.Classes {
		if mod.Name != owner {
			continue
		}
		for _, f := range mod.Fields {
			if f.Name == fieldName {
				return l.testModuleRead(n, mod, fieldName, f.Type), true
			}
		}
	}
	return nil, false
}

// requiredMemberCall lowers `m.f(args)` for a module function f of a
// required module m as the direct call `f(args)`.
func (l *lowerer) requiredMemberCall(n, callee *ast.Node) (*hir.Expr, bool) {
	sym := l.requiredMember(callee)
	if sym == nil {
		return nil, false
	}
	f := l.fileOfSymbol(sym)
	if f == nil {
		return nil, false
	}
	ref, ok := l.funcsBy[f.FileName()+" "+sym.Name]
	if !ok {
		if _, _, isVar := l.modvarOf(sym); isVar {
			return l.functionValueCall(n, callee), true
		}
		return nil, false
	}
	args, ok := l.callArgsStatic(n, n.Arguments(), ref)
	if !ok {
		return nil, true
	}
	return &hir.Expr{Kind: hir.DirectCall, Node: l.node(n), Owner: ref.owner, Name: ref.method,
		Type: l.callbackResultType(ref), Args: args}, true
}
