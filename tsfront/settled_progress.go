package tsfront

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// This is a fact about this compilation's sources, never an interface contract
// or an allowlist. Tests loaded into the Program participate in the same scan.
// A source implementation outside Lower's files cannot close the world either.
func (l *lowerer) closedProgressCall(n *ast.Node, seen map[*ast.Node]bool) bool {
	if n == nil || n.Kind != ast.KindCallExpression {
		return false
	}
	callee := n.Expression()
	if callee == nil || callee.Kind != ast.KindPropertyAccessExpression || callee.Name().Text() != "tick" {
		return false
	}
	sym := l.resolve(callee)
	if sym == nil {
		return false
	}
	for _, d := range sym.Declarations {
		if d.Kind != ast.KindMethodSignature || d.Parent == nil || d.Parent.Kind != ast.KindInterfaceDeclaration || d.Parent.Name().Text() != "IProgress" {
			continue
		}
		iface := d.Parent
		if reason, ok := l.settlementWorld[iface]; ok {
			l.settlementFailure = reason
			return reason == ""
		}
		// Recursive interface calls are not evidence of termination/settlement.
		if seen[iface] {
			l.settlementFailure = "recursive IProgress.tick provider"
			return false
		}
		seen[iface] = true
		reason, implementations, suspends := l.progressSettlementWorld(iface, seen)
		delete(seen, iface)
		l.settlementWorld[iface] = reason
		l.settlementSuspends[iface] = suspends
		l.settlementFailure = reason
		if reason == "" {
			l.diagf(n, "note-settled-progress", "closed-world IProgress.tick: %d implementations certified (%s)", len(implementations), strings.Join(implementations, ", "))
		}
		return reason == ""
	}
	return false
}

func (l *lowerer) progressSettlementWorld(iface *ast.Node, seen map[*ast.Node]bool) (string, []string, bool) {
	target := l.ck.GetDeclaredTypeOfSymbol(iface.Symbol())
	if target == nil {
		return "IProgress has no checked interface identity", nil, false
	}
	matches := func(t *checker.Type) bool {
		if t == nil || t.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown|checker.TypeFlagsNever) != 0 {
			return false
		}
		t = l.ck.GetNonNullableType(t)
		return l.ck.GetPropertyOfType(t, "tick") != nil && l.ck.IsTypeAssignableTo(t, target)
	}
	// Inspect data fields recursively. Tests can wrap IProgress in their own
	// input types; neither a particular property name nor any erasure closes
	// those sources. Cache only complete root queries, so recursive shapes do
	// not cache a partial negative result.
	carriers := map[*checker.Type]bool{}
	var contains func(*checker.Type, map[*checker.Type]bool) bool
	contains = func(t *checker.Type, seen map[*checker.Type]bool) bool {
		if t == nil {
			return false
		}
		t = l.ck.GetNonNullableType(t)
		if result, ok := carriers[t]; ok {
			return result
		}
		if matches(t) {
			return true
		}
		if seen[t] || t.Flags()&checker.TypeFlagsStructuredType == 0 {
			return false
		}
		seen[t] = true
		for _, prop := range l.ck.GetPropertiesOfType(t) {
			if contains(l.ck.GetTypeOfSymbol(prop), seen) {
				return true
			}
		}
		return false
	}
	carriesProgress := func(t *checker.Type) bool {
		if t == nil {
			return false
		}
		t = l.ck.GetNonNullableType(t)
		if result, ok := carriers[t]; ok {
			return result
		}
		result := contains(t, map[*checker.Type]bool{})
		carriers[t] = result
		return result
	}
	unproven := func(source, expected *checker.Type) bool {
		type pair struct{ source, expected *checker.Type }
		seen := map[pair]bool{}
		var check func(*checker.Type, *checker.Type) bool
		check = func(source, expected *checker.Type) bool {
			// Erasing a provider or wrapper to any hides subsequent mutation.
			if carriesProgress(source) && expected != nil && expected.Flags()&checker.TypeFlagsAny != 0 {
				return true
			}
			if !carriesProgress(expected) || absentProgressType(source) {
				return false
			}
			if source == nil || source.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
				return true
			}
			if matches(expected) {
				return !matches(source)
			}
			source, expected = l.ck.GetNonNullableType(source), l.ck.GetNonNullableType(expected)
			key := pair{source, expected}
			if seen[key] {
				return false
			}
			seen[key] = true
			for _, prop := range l.ck.GetPropertiesOfType(expected) {
				targetType := l.ck.GetTypeOfSymbol(prop)
				if !carriesProgress(targetType) {
					continue
				}
				actual := l.ck.GetTypeOfPropertyOfType(source, prop.Name)
				if actual == nil && prop.Flags&ast.SymbolFlagsOptional != 0 {
					continue
				}
				if check(actual, targetType) {
					return true
				}
			}
			return false
		}
		return check(source, expected)
	}
	label := func(n *ast.Node) string {
		name := "object literal"
		if n.Name() != nil {
			name = n.Name().Text()
		} else if n.Kind == ast.KindObjectLiteralExpression && n.Parent != nil && n.Parent.Name() != nil {
			name = n.Parent.Name().Text() + " (object literal)"
		}
		f := ast.GetSourceFileOfNode(n)
		path, _ := filepath.Rel(l.prog.configDir, f.FileName())
		return fmt.Sprintf("%s at %s", name, filepath.ToSlash(path))
	}
	var implementations []string
	suspends := false
	reason := ""
	for _, file := range l.prog.SourceFiles() {
		f, _ := l.prog.File(file)
		if f == nil || f.IsDeclarationFile {
			continue
		}
		var walk func(*ast.Node)
		walk = func(n *ast.Node) {
			if n == nil || reason != "" {
				return
			}
			switch n.Kind {
			case ast.KindClassDeclaration, ast.KindClassExpression, ast.KindObjectLiteralExpression:
				t := l.ck.GetTypeAtLocation(n)
				if n.Kind != ast.KindObjectLiteralExpression && n.Symbol() != nil {
					t = l.ck.GetDeclaredTypeOfSymbol(n.Symbol())
				}
				declares := false
				var heritage *ast.NodeList
				if n.Kind == ast.KindClassDeclaration {
					heritage = n.AsClassDeclaration().HeritageClauses
				}
				if n.Kind == ast.KindClassExpression {
					heritage = n.AsClassExpression().HeritageClauses
				}
				if heritage != nil {
					for _, clause := range heritage.Nodes {
						for _, base := range clause.AsHeritageClause().Types.Nodes {
							if sym := l.resolve(base.Expression()); sym != nil && matches(l.ck.GetDeclaredTypeOfSymbol(sym)) {
								declares = true
							}
						}
					}
				}
				candidates := []*checker.Type{t}
				if n.Kind != ast.KindObjectLiteralExpression && n.Symbol() != nil {
					candidates = append(candidates, l.ck.GetTypeOfSymbol(n.Symbol()))
				}
				for index, candidate := range candidates {
					if !matches(candidate) && !(index == 0 && declares) {
						continue
					}
					t = candidate
					who := label(n)
					if index == 1 {
						who += " (static provider)"
					}
					if !l.settlementFiles[f] {
						reason = who + " is outside the translated closure"
						return
					}
					tick := l.ck.GetPropertyOfType(t, "tick")
					var body *ast.Node
					if tick != nil {
						for _, d := range tick.Declarations {
							body = l.progressTickBody(d, map[*ast.Node]bool{})
							if body != nil {
								break
							}
						}
					}
					if body == nil || !l.settlementFiles[ast.GetSourceFileOfNode(body)] || !l.certifiedProgressBody(body, seen) {
						reason = who + " tick is not provably already-settled"
						return
					}
					implementations = append(implementations, who)
					suspends = suspends || l.settledBodyHasAwait(body, map[*ast.Node]bool{})
				}
			case ast.KindAsExpression, ast.KindTypeAssertionExpression:
				if unproven(l.ck.GetTypeAtLocation(n.Expression()), l.ck.GetTypeAtLocation(n)) {
					reason = label(n) + " casts an unproven IProgress implementer"
					return
				}
			case ast.KindBinaryExpression:
				b := n.AsBinaryExpression()
				// Mutation invalidates a body's dispatch proof, even for an otherwise
				// settled replacement: the current model pins source method bodies.
				if ast.IsAssignmentOperator(b.OperatorToken.Kind) {
					left := b.Left
					if ((left.Kind == ast.KindPropertyAccessExpression && left.Name().Text() == "tick") || left.Kind == ast.KindElementAccessExpression) && matches(l.ck.GetTypeAtLocation(left.Expression())) {
						reason = label(left.Expression()) + " mutates tick"
						return
					}
					if unproven(l.ck.GetTypeAtLocation(b.Right), l.ck.GetTypeAtLocation(left)) {
						reason = label(b.Right) + " assigns an unproven IProgress implementer"
						return
					}
				}
			case ast.KindVariableDeclaration, ast.KindPropertyDeclaration:
				if n.Initializer() == nil && carriesProgress(l.ck.GetTypeAtLocation(n)) && n.ModifierFlags()&ast.ModifierFlagsAmbient != 0 {
					reason = label(n) + " declares an external IProgress provider"
					return
				}
				if n.Initializer() != nil && unproven(l.ck.GetTypeAtLocation(n.Initializer()), l.ck.GetTypeAtLocation(n)) {
					reason = label(n) + " initializes an unproven IProgress implementer"
					return
				}
			case ast.KindIdentifier:
				if carriesProgress(l.ck.GetTypeAtLocation(n)) {
					sym := l.resolve(n)
					if sym != nil && sym.ValueDeclaration != nil {
						d := sym.ValueDeclaration
						source := ast.GetSourceFileOfNode(d)
						if source.IsDeclarationFile || (d.Kind == ast.KindVariableDeclaration && d.Initializer() == nil && ast.GetCombinedModifierFlags(d)&ast.ModifierFlagsAmbient != 0) {
							reason = label(n) + " names an external IProgress provider"
							return
						}
					}
				}
			case ast.KindReturnStatement:
				value := n.AsReturnStatement().Expression
				if value != nil && unproven(l.ck.GetTypeAtLocation(value), l.ck.GetContextualType(value, checker.ContextFlagsNone)) {
					reason = label(value) + " returns an unproven IProgress implementer"
					return
				}
			case ast.KindPropertyAssignment:
				if unproven(l.ck.GetTypeAtLocation(n.Initializer()), l.ck.GetContextualTypeForObjectLiteralElement(n, checker.ContextFlagsNone)) {
					reason = label(n) + " supplies an unproven IProgress implementer"
					return
				}
			case ast.KindCallExpression, ast.KindNewExpression:
				for i, arg := range n.Arguments() {
					if unproven(l.ck.GetTypeAtLocation(arg), l.ck.GetContextualTypeForArgumentAtIndex(n, i)) {
						reason = label(arg) + " passes an unproven IProgress implementer"
						return
					}
				}
				if carriesProgress(l.ck.GetTypeAtLocation(n)) {
					if n.Kind == ast.KindNewExpression {
						t := l.ck.GetTypeAtLocation(n)
						sym := t.Symbol()
						if sym == nil || len(sym.Declarations) == 0 || !l.settlementFiles[ast.GetSourceFileOfNode(sym.Declarations[0])] {
							reason = label(n) + " constructs an external IProgress implementer"
							return
						}
					}
					sig := l.ck.GetResolvedSignature(n)
					if sig == nil || sig.Declaration() == nil || sig.Declaration().Body() == nil {
						// Constructors of translated classes are covered by the class scan.
						if n.Kind != ast.KindNewExpression {
							reason = label(n) + " obtains IProgress from an external provider"
							return
						}
					}
				}
			}
			n.ForEachChild(func(c *ast.Node) bool { walk(c); return reason != "" })
		}
		walk(f.AsNode())
		if reason != "" {
			break
		}
	}
	return reason, implementations, suspends
}

// A synchronous tick must actually return a promise on every accepted path.
// Async ticks create a fulfillment even when their bodies fall through.
func (l *lowerer) certifiedProgressBody(d *ast.Node, seen map[*ast.Node]bool) bool {
	if d == nil || d.Body() == nil {
		return false
	}
	if d.ModifierFlags()&ast.ModifierFlagsAsync == 0 {
		promise := func(n *ast.Node) bool {
			if n == nil {
				return false
			}
			t := l.ck.GetTypeAtLocation(n)
			return t != nil && t.Symbol() != nil && t.Symbol().Name == "Promise" && l.librarySymbol(t.Symbol()) && l.certifiedSettled(n, seen)
		}
		body := d.Body()
		if body.Kind != ast.KindBlock {
			return promise(body)
		}
		stmts := body.AsBlock().Statements.Nodes
		if len(stmts) == 0 || (stmts[len(stmts)-1].Kind != ast.KindReturnStatement && stmts[len(stmts)-1].Kind != ast.KindThrowStatement) {
			return false
		}
		safe := true
		var returns func(*ast.Node)
		returns = func(n *ast.Node) {
			if n == nil || !safe {
				return
			}
			if n.Kind == ast.KindArrowFunction || n.Kind == ast.KindFunctionExpression || n.Kind == ast.KindFunctionDeclaration {
				return
			}
			if n.Kind == ast.KindReturnStatement {
				safe = promise(n.AsReturnStatement().Expression)
				return
			}
			n.ForEachChild(func(c *ast.Node) bool { returns(c); return !safe })
		}
		returns(body)
		if !safe {
			return false
		}
	}
	return l.certifiedSettledBody(d, seen)
}

func absentProgressType(t *checker.Type) bool {
	if t == nil {
		return false
	}
	// undefined is the optional skip path. Null is not the IProgress contract.
	return t.Flags()&(checker.TypeFlagsUndefined|checker.TypeFlagsVoid|checker.TypeFlagsNever) != 0
}

// Follow immutable aliases of a callback without treating its Promise
// annotation as evidence. Mutable callback variables stay unproven.
func (l *lowerer) progressTickBody(n *ast.Node, seen map[*ast.Node]bool) *ast.Node {
	if n == nil || seen[n] {
		return nil
	}
	seen[n] = true
	if n.Body() != nil {
		return n
	}
	if n.Kind == ast.KindPropertyAssignment || n.Kind == ast.KindPropertyDeclaration {
		return l.progressTickBody(n.Initializer(), seen)
	}
	if n.Kind == ast.KindParenthesizedExpression {
		return l.progressTickBody(n.Expression(), seen)
	}
	if n.Kind == ast.KindShorthandPropertyAssignment {
		sym := l.ck.GetShorthandAssignmentValueSymbol(n)
		if sym != nil && sym.ValueDeclaration != nil {
			d := sym.ValueDeclaration
			if d.Kind == ast.KindVariableDeclaration && d.Parent != nil && d.Parent.Flags&ast.NodeFlagsConst != 0 {
				return l.progressTickBody(d.Initializer(), seen)
			}
		}
		return nil
	}
	if n.Kind == ast.KindIdentifier {
		sym := l.resolve(n)
		if sym == nil || sym.ValueDeclaration == nil {
			return nil
		}
		d := sym.ValueDeclaration
		if d.Kind == ast.KindVariableDeclaration && d.Parent != nil && d.Parent.Flags&ast.NodeFlagsConst != 0 {
			return l.progressTickBody(d.Initializer(), seen)
		}
	}
	return nil
}

func (l *lowerer) progressCallHasAwait(n *ast.Node, seen map[*ast.Node]bool) bool {
	if !l.closedProgressCall(n, seen) {
		return false
	}
	sym := l.resolve(n.Expression())
	for _, d := range sym.Declarations {
		if d.Kind == ast.KindMethodSignature && d.Parent != nil && l.settlementSuspends[d.Parent] {
			return true
		}
	}
	return false
}
