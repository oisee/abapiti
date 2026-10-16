package tsfront

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Namespace records. gogen passes require-bound module namespaces as values
// in helper records (`{Statements, Expressions, typeOf}`, `{..., abaplint}`)
// whose declared slot is `typeof import("...")` or `typeof ns.Member`. The
// checker maps that namespace object type to a structural record (one field
// per value export). The value is that record, built from the exports:
// classes are their class values, nested namespaces their own records.
// Node's namespace object is one frozen object per module, so the record is
// built once per module and record type (identity is preserved) at its first
// read. Any other export kind is refused at the reference.
func (l *lowerer) requireNamespaceOf(n *ast.Node) *ast.Symbol {
	sym := l.ck.GetSymbolAtLocation(n)
	if n.Parent != nil && n.Parent.Kind == ast.KindShorthandPropertyAssignment {
		sym = l.ck.GetShorthandAssignmentValueSymbol(n.Parent)
	}
	target := l.requireTarget(sym)
	if target == nil || target.Flags&ast.SymbolFlagsValueModule == 0 {
		return nil
	}
	return target
}

func (l *lowerer) namespaceRecord(n *ast.Node) (*hir.Expr, bool) {
	if !l.unprunedFeatures {
		return nil, false
	}
	mod := l.requireNamespaceOf(n)
	if mod == nil {
		return nil, false
	}
	// Only a slot declared with the namespace's own type receives the
	// record; any other use (unknown, any, a structural supertype) keeps
	// the refusal, since such a reader could enumerate the object.
	if !l.namespaceSlot(n, mod) {
		return nil, false
	}
	return l.namespaceRecordOf(n, mod, 0), true
}

// namespaceSlot: n flows into a slot typed as the namespace mod itself, or
// is the receiver of a read of a nested namespace that does (`core.Kinds`).
func (l *lowerer) namespaceSlot(n *ast.Node, mod *ast.Symbol) bool {
	if context := l.ck.GetContextualType(n, 0); context != nil && (context == l.ck.GetTypeOfSymbol(mod) || context.Symbol() == mod) {
		return true
	}
	p := n.Parent
	if p == nil || p.Kind != ast.KindPropertyAccessExpression || p.Expression() != n {
		return false
	}
	member := l.namespaceExport(mod, p.Name().Text())
	return member != nil && member.Flags&ast.SymbolFlagsValueModule != 0 && l.namespaceSlot(p, member)
}

func (l *lowerer) namespaceRecordOf(n *ast.Node, mod *ast.Symbol, depth int) *hir.Expr {
	return l.namespaceRecordNamed(n, n.Text(), mod, depth)
}

func (l *lowerer) namespaceRecordNamed(n *ast.Node, name string, mod *ast.Symbol, depth int) *hir.Expr {
	file := l.fileOfSymbol(mod)
	if file == nil || depth > 16 {
		l.diagf(n, "unsupported-namespace-record", "namespace %s has no lowered module", name)
		return nil
	}
	before := len(l.diags)
	typ := l.mapCheckerType(n, l.ck.GetTypeOfSymbol(mod))
	if hasBlocking(l.diags[before:]) {
		return nil
	}
	var shape *hir.Class
	if typ.Kind == hir.ClassRef {
		for _, c := range l.out.Classes {
			if c.Name == typ.Name {
				shape = c
				break
			}
		}
	}
	if shape == nil || shape.Ctor == nil || len(shape.Ctor.Params) != len(shape.Fields) {
		l.diagf(n, "unsupported-namespace-record", "namespace %s: its type %s is not a lowered record", name, typ)
		return nil
	}
	args := make([]*hir.Expr, 0, len(shape.Ctor.Params))
	var refused []string
	for _, p := range shape.Ctor.Params {
		sym := l.namespaceExport(mod, p.Name)
		var value *hir.Expr
		switch {
		case sym == nil:
		case sym.Flags&ast.SymbolFlagsClass != 0 && l.classOf(sym) != nil:
			value = &hir.Expr{Kind: hir.ClassOf, Node: l.node(n), Owner: l.classOf(sym).Name, Type: hir.T(hir.ClassValue)}
		case sym.Flags&ast.SymbolFlagsEnum != 0:
			if f := l.fileOfSymbol(sym); f != nil && (l.enumOf(sym) != nil || l.numericEnumOf(sym) != nil) {
				// The enum's module carries its namespace map; it may not
				// have been visited yet.
				l.moduleClassOf(f)
			}
			value, _ = l.enumNamespaceOf(n, sym)
		case sym.Flags&ast.SymbolFlagsFunction != 0 && sym.ValueDeclaration != nil && sym.ValueDeclaration.Name() != nil:
			// The function value of a module function, resolved from its
			// declaration name (no local function may shadow it here).
			if fn := sym.ValueDeclaration.Name(); l.localFns[fn.Text()] == nil && l.pendingFns[fn.Text()] == nil {
				value = l.functionValue(fn)
			}
		case sym.Flags&ast.SymbolFlagsBlockScopedVariable != 0 && immutableExport(sym):
			value = l.namespaceConst(n, sym)
		case sym.Flags&ast.SymbolFlagsValueModule != 0:
			value = l.namespaceRecordNamed(n, name+"."+p.Name, sym, depth+1)
			if value == nil {
				return nil
			}
		}
		if value == nil {
			kind := "unresolved"
			if sym != nil {
				kind = fmt.Sprintf("flags %#x", uint64(sym.Flags))
			}
			refused = append(refused, p.Name+" ("+kind+")")
			continue
		}
		converted := l.coerce(value, p.Type)
		if !l.acceptsType(p.Type, converted.Type) {
			l.diagf(n, "unsupported-namespace-record", "namespace %s: export %s does not fit its record slot %s", name, p.Name, p.Type)
			return nil
		}
		args = append(args, converted)
	}
	if len(refused) > 0 {
		l.diagf(n, "unsupported-namespace-record", "namespace %s: exports %s are not classes, functions, enums, const bindings or namespaces", name, strings.Join(refused, ", "))
		return nil
	}
	owner := l.moduleClassOf(file)
	field := fmt.Sprintf("nsrec_%x", sha256.Sum256([]byte(typ.Name)))[:18]
	slot := hir.T(hir.Optional, typ)
	declared := false
	for _, f := range owner.Fields {
		declared = declared || (f.Static && f.Name == field)
	}
	if !declared {
		owner.Fields = append(owner.Fields, hir.Field{Name: field, Static: true, Type: slot})
	}
	read := func() *hir.Expr {
		return &hir.Expr{Kind: hir.StaticGet, Node: l.node(n), Owner: owner.Name, Name: field, Type: slot}
	}
	build := &hir.Expr{Kind: hir.New, Node: l.node(n), Type: typ, Args: args}
	l.pendStmt(&hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: read()},
		Body: hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: read(), Y: l.coerce(build, slot)})})
	return &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: typ, X: read()}
}

// namespaceConst reads a const module binding exported by the namespace.
func (l *lowerer) namespaceConst(n *ast.Node, sym *ast.Symbol) *hir.Expr {
	owner, field, ok := l.modvarOf(sym)
	if !ok {
		return nil
	}
	for _, mod := range l.out.Classes {
		if mod.Name != owner {
			continue
		}
		for _, f := range mod.Fields {
			if f.Name == field {
				return l.testModuleRead(n, mod, field, f.Type)
			}
		}
	}
	return nil
}

// namespaceExport finds the module's export named name (an alias resolved
// to its target).
func (l *lowerer) namespaceExport(mod *ast.Symbol, name string) *ast.Symbol {
	for _, sym := range l.ck.GetExportsOfModule(mod) {
		if sym.Name != name {
			continue
		}
		if sym.Flags&ast.SymbolFlagsAlias != 0 {
			if target, ok := l.ck.ResolveAlias(sym); ok {
				return target
			}
		}
		return sym
	}
	return nil
}
