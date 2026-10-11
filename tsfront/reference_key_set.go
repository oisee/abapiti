package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// referenceKeySet proves that a local, empty Set<unknown> only observes
// reference keys. Every use must be a direct add/has (or size read), and add's
// result must be discarded, so neither the set nor its fluent result escapes.
// HIR's root-object key retains JS reference identity across concrete classes.
// Mixed primitive keys and aliases retain the existing dynamic-key refusal.
func (l *lowerer) referenceKeySet(d *ast.Node) (hir.Type, bool) {
	fail := func() (hir.Type, bool) { return hir.Type{}, false }
	init := d.Initializer()
	if d.Type() != nil || d.Symbol() == nil || d.Parent == nil || d.Parent.Flags&ast.NodeFlagsConst == 0 || init == nil || init.Kind != ast.KindNewExpression || len(init.Arguments()) != 0 {
		return fail()
	}
	name := init.Expression()
	if name == nil || name.Kind != ast.KindIdentifier || name.Text() != "Set" {
		return fail()
	}
	sym := l.resolve(name)
	if sym == nil || !l.librarySymbol(sym) || len(init.TypeArguments()) != 1 || init.TypeArguments()[0].Kind != ast.KindUnknownKeyword {
		return fail()
	}
	body := d.Parent
	for body != nil && body.Kind != ast.KindBlock && body.Kind != ast.KindSourceFile {
		body = body.Parent
	}
	if body == nil || body.Kind == ast.KindSourceFile {
		return fail()
	}
	// Include sibling scopes and every nested closure in the enclosing function.
	for parent := body.Parent; parent != nil; parent = parent.Parent {
		if ast.IsFunctionLike(parent) {
			body = parent.Body()
			break
		}
	}
	valid, used := true, false
	var walk func(*ast.Node)
	walk = func(n *ast.Node) {
		if n == nil || !valid {
			return
		}
		symbol := l.resolve(n)
		if n.Kind == ast.KindIdentifier && n.Parent != nil && n.Parent.Kind == ast.KindShorthandPropertyAssignment {
			symbol = l.ck.GetShorthandAssignmentValueSymbol(n.Parent)
		}
		if n.Kind == ast.KindIdentifier && n != d.Name() && symbol == d.Symbol() {
			p := n.Parent
			if p == nil || p.Kind != ast.KindPropertyAccessExpression || p.Expression() != n {
				valid = false
				return
			}
			member := p.Name().Text()
			if member == "size" {
				return
			}
			call := p.Parent
			if (member != "add" && member != "has") || call == nil || call.Kind != ast.KindCallExpression || call.Expression() != p || len(call.Arguments()) != 1 || (member == "add" && (call.Parent == nil || call.Parent.Kind != ast.KindExpressionStatement)) {
				valid = false
				return
			}
			t := l.ck.GetTypeAtLocation(call.Arguments()[0])
			if t == nil || t.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown|checker.TypeFlagsNull|checker.TypeFlagsUndefined) != 0 {
				valid = false
				return
			}
			before := len(l.diags)
			key := l.mapCheckerType(call.Arguments()[0], t)
			blocked := hasBlocking(l.diags[before:])
			l.diags = l.diags[:before]
			if blocked || !key.IsRef() || key.Kind == hir.Dynamic {
				valid = false
				return
			}
			used = true
		}
		n.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
	}
	walk(body)
	if !valid || !used {
		return fail()
	}
	return hir.T(hir.OrderedSet, hir.Ref(hir.RootObject)), true
}
