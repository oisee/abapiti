package tsfront

import (
	"fmt"
	"path/filepath"

	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

type settledDispatchKey struct {
	receiver *checker.Type
	name     string
	exact    bool
}
type settledDispatchResult struct {
	bodies []*ast.Node
	reason string
}

func (l *lowerer) settledLabel(n *ast.Node) string {
	name := "provider"
	if n.Name() != nil {
		name = n.Name().Text()
	}
	if n.Parent != nil && (n.Parent.Kind == ast.KindClassDeclaration || n.Parent.Kind == ast.KindClassExpression) && n.Parent.Name() != nil {
		name = n.Parent.Name().Text() + "." + name
	}
	f := ast.GetSourceFileOfNode(n)
	path, _ := filepath.Rel(l.prog.configDir, f.FileName())
	return fmt.Sprintf("%s at %s", name, filepath.ToSlash(path))
}

// The dynamic class is pinned only by a fresh constructor and immutable aliases.
// A fluent call preserves it only when every return is `this`; its annotation
// alone would also permit returning an instance of a different subclass.
func (l *lowerer) settledReceiver(n *ast.Node, seen map[*ast.Node]bool) *checker.Type {
	if n == nil || seen[n] {
		return nil
	}
	seen[n] = true
	if n.Kind == ast.KindParenthesizedExpression {
		return l.settledReceiver(n.Expression(), seen)
	}
	if n.Kind == ast.KindNewExpression {
		return l.ck.GetTypeAtLocation(n)
	}
	if n.Kind == ast.KindIdentifier {
		if s := l.resolve(n); s != nil && s.ValueDeclaration != nil {
			d := s.ValueDeclaration
			if d.Kind == ast.KindVariableDeclaration && d.Parent != nil && d.Parent.Flags&ast.NodeFlagsConst != 0 {
				return l.settledReceiver(d.Initializer(), seen)
			}
		}
	}
	if n.Kind == ast.KindCallExpression && n.Expression().Kind == ast.KindPropertyAccessExpression {
		receiver := l.settledReceiver(n.Expression().Expression(), seen)
		if receiver == nil {
			return nil
		}
		sym := l.ck.GetPropertyOfType(receiver, n.Expression().Name().Text())
		if sym == nil {
			return nil
		}
		for _, d := range sym.Declarations {
			if d.Body() == nil {
				continue
			}
			good, returns := true, false
			var walk func(*ast.Node)
			walk = func(x *ast.Node) {
				if x == nil {
					return
				}
				if ast.IsFunctionLike(x) {
					return
				}
				if x.Kind == ast.KindReturnStatement {
					returns = true
					v := x.AsReturnStatement().Expression
					good = good && v != nil && v.Kind == ast.KindThisKeyword
					return
				}
				x.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
			}
			if d.Body().Kind != ast.KindBlock {
				good = d.Body().Kind == ast.KindThisKeyword
				returns = true
			} else {
				walk(d.Body())
			}
			if d.Body().Kind == ast.KindBlock {
				stmts := d.Body().AsBlock().Statements.Nodes
				good = good && len(stmts) > 0 && (stmts[len(stmts)-1].Kind == ast.KindReturnStatement || stmts[len(stmts)-1].Kind == ast.KindThrowStatement)
			}
			if good && returns {
				return receiver
			}
		}
	}
	return nil
}

// Enumerate actual dispatch bodies from this Program, including loaded tests.
// An implementation outside the translated closure or without a source body
// refuses the site. Cache the inventory, never a cross-build type contract.
func (l *lowerer) settledDispatch(n *ast.Node) ([]*ast.Node, string) {
	callee := n.Expression()
	sym := l.resolve(callee)
	if sym == nil {
		return nil, ""
	}
	virtual := false
	for _, d := range sym.Declarations {
		if (d.Kind == ast.KindMethodDeclaration || d.Kind == ast.KindMethodSignature) && d.ModifierFlags()&(ast.ModifierFlagsStatic|ast.ModifierFlagsPrivate) == 0 {
			virtual = true
		}
	}
	if !virtual {
		for _, decl := range sym.Declarations {
			d := decl
			if d.Kind == ast.KindVariableDeclaration && d.Parent != nil && d.Parent.Flags&ast.NodeFlagsConst != 0 {
				d = l.progressTickBody(d.Initializer(), map[*ast.Node]bool{})
			}
			if d != nil && d.Body() != nil {
				if !l.settlementFiles[ast.GetSourceFileOfNode(d)] {
					return nil, l.settledLabel(d) + " is outside the translated closure"
				}
				return []*ast.Node{d}, ""
			}
		}
		return nil, ""
	}
	if callee.Kind != ast.KindPropertyAccessExpression {
		return nil, "virtual call has no checked receiver"
	}
	receiver := callee.Expression()
	typ := l.ck.GetNonNullableType(l.ck.GetTypeAtLocation(receiver))
	exact := l.settledReceiver(receiver, map[*ast.Node]bool{})
	if exact != nil {
		typ = exact
	} else if typ != nil {
		// Polymorphic `this` and constrained parameters dispatch within their
		// checked apparent class/interface, not to the type parameter itself.
		typ = l.ck.GetApparentType(typ)
	}
	if typ == nil || typ.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
		return nil, "virtual receiver is not closed"
	}
	key := settledDispatchKey{typ, callee.Name().Text(), exact != nil}
	if l.settledDispatches == nil {
		l.settledDispatches = map[settledDispatchKey]settledDispatchResult{}
	}
	if r, ok := l.settledDispatches[key]; ok {
		return r.bodies, r.reason
	}
	result := settledDispatchResult{}
	add := func(t *checker.Type, owner *ast.Node, declared bool) {
		if t == nil || !declared && !l.ck.IsTypeAssignableTo(t, typ) {
			return
		}
		prop := l.ck.GetPropertyOfType(t, key.name)
		if prop == nil {
			if declared {
				result.reason = l.settledLabel(owner) + " has no implementation of " + key.name
			}
			return
		}
		var body *ast.Node
		for _, d := range prop.Declarations {
			body = l.progressTickBody(d, map[*ast.Node]bool{})
			if body != nil {
				break
			}
		}
		if body == nil {
			result.reason = l.settledLabel(owner) + "." + key.name + " has no provable source body"
			return
		}
		if !l.settlementFiles[ast.GetSourceFileOfNode(body)] {
			result.reason = l.settledLabel(body) + " is outside the translated closure"
			return
		}
		for _, old := range result.bodies {
			if old == body {
				return
			}
		}
		result.bodies = append(result.bodies, body)
	}
	l.indexSettlementWorld()
	if exact != nil {
		add(exact, n, false)
	} else {
		for _, x := range l.settledShapes {
			if x.Kind == ast.KindObjectLiteralExpression {
				add(l.ck.GetTypeAtLocation(x), x, false)
			} else if x.Symbol() != nil && x.ModifierFlags()&ast.ModifierFlagsAbstract == 0 {
				add(l.ck.GetDeclaredTypeOfSymbol(x.Symbol()), x, l.declaresSettlementReceiver(x, typ))
				add(l.ck.GetTypeOfSymbol(x.Symbol()), x, false)
			}
			if result.reason != "" {
				break
			}
		}
	}
	// Source method bodies are pinned only in the absence of mutation and
	// unsafe provider erasure. This audit also applies to a fresh local receiver.
	resultType := l.ck.GetNonNullableType(l.ck.GetTypeAtLocation(n))
	if result.reason == "" && resultType != nil && l.ck.GetPropertyOfType(resultType, "then") != nil {
		matchCache := map[*checker.Type]bool{}
		for _, x := range l.settledWorldChecks {
			if reason := l.settledWorldFailure(x, typ, key.name, matchCache); reason != "" {
				result.reason = reason
				break
			}
		}
	}
	if len(result.bodies) == 0 && result.reason == "" {
		result.reason = "no closed-world implementation of " + key.name
	}
	l.settledDispatches[key] = result
	return result.bodies, result.reason
}

// Follow synchronous source helpers as well: hiding a timer or pending producer
// behind a void wrapper must not establish settlement. External non-thenable
// runtime calls retain the existing synchronous ABI contract.
func (l *lowerer) settledBodyCall(n *ast.Node, seen map[*ast.Node]bool) bool {
	callee := n.Expression()
	pendingCallee := callee
	aliases := map[*ast.Node]bool{}
	for pendingCallee.Kind == ast.KindIdentifier && !aliases[pendingCallee] {
		aliases[pendingCallee] = true
		s := l.resolve(pendingCallee)
		if s == nil || s.ValueDeclaration == nil {
			break
		}
		d := s.ValueDeclaration
		if d.Kind != ast.KindVariableDeclaration || d.Parent == nil || d.Parent.Flags&ast.NodeFlagsConst == 0 || d.Initializer() == nil {
			break
		}
		pendingCallee = d.Initializer()
	}
	if pendingCallee.Kind == ast.KindIdentifier {
		switch pendingCallee.Text() {
		case "setTimeout", "setInterval", "queueMicrotask":
			l.settlementFailure = pendingCallee.Text() + " is a pending source"
			return false
		}
	}
	for _, arg := range n.Arguments() {
		if body := l.progressTickBody(arg, map[*ast.Node]bool{}); body != nil && !l.proveSettledBody(body, seen, false) {
			return false
		}
	}
	t := l.ck.GetTypeAtLocation(n)
	if t != nil && l.ck.GetPropertyOfType(t, "then") != nil {
		return l.certifiedSettled(n, seen)
	}
	targets, _ := l.settledDispatch(n)
	for _, d := range targets {
		if !l.proveSettledBody(d, seen, false) {
			return false
		}
	}
	return true
}

// Each body is an AND node in a monotone call graph. Nodes start at true (the
// greatest fixpoint); a pending or unprovable leaf turns false and invalidates
// every predecessor, including already-expanded members of the same SCC.
// This proves absence of reachable pending work, not termination: ordinary
// recursion can still diverge synchronously. The graph is private to one Lower.
type settledProofKey struct {
	body      *ast.Node
	returning bool
}
type settledProof struct {
	safe    bool
	reason  string
	parents map[settledProofKey]bool
}

func (l *lowerer) certifiedSettledBody(d *ast.Node, seen map[*ast.Node]bool) bool {
	return l.proveSettledBody(d, seen, true)
}

func (l *lowerer) proveSettledBody(d *ast.Node, seen map[*ast.Node]bool, returning bool) bool {
	if d == nil || d.Body() == nil || !l.settlementFiles[ast.GetSourceFileOfNode(d)] {
		return false
	}
	if l.settledProofs == nil {
		l.settledProofs = map[settledProofKey]*settledProof{}
	}
	key := settledProofKey{d, returning}
	proof, exists := l.settledProofs[key]
	if !exists {
		proof = &settledProof{safe: true, parents: map[settledProofKey]bool{}}
		l.settledProofs[key] = proof
	}
	if l.settledActive != nil {
		proof.parents[*l.settledActive] = true
	}
	if !exists {
		parent := l.settledActive
		l.settledActive = &key
		safe := l.checkSettledBody(d, seen, returning)
		l.settledActive = parent
		if !safe {
			reason := l.settledLabel(d) + " is not provably already-settled"
			if l.settlementFailure != "" {
				reason += ": " + l.settlementFailure
			}
			queue := []settledProofKey{key}
			for len(queue) > 0 {
				k := queue[0]
				queue = queue[1:]
				p := l.settledProofs[k]
				if !p.safe {
					continue
				}
				p.safe = false
				p.reason = reason
				for pred := range p.parents {
					queue = append(queue, pred)
				}
			}
		}
	}
	if !proof.safe {
		l.settlementFailure = proof.reason
	}
	return proof.safe
}

func (l *lowerer) indexSettlementWorld() {
	if l.settledWorldReady {
		return
	}
	l.settledWorldReady = true
	for _, file := range l.prog.SourceFiles() {
		f, _ := l.prog.File(file)
		if f == nil {
			continue
		}
		var walk func(*ast.Node)
		walk = func(n *ast.Node) {
			switch n.Kind {
			case ast.KindClassDeclaration, ast.KindClassExpression, ast.KindObjectLiteralExpression:
				l.settledShapes = append(l.settledShapes, n)
			case ast.KindAsExpression, ast.KindTypeAssertionExpression, ast.KindBinaryExpression, ast.KindVariableDeclaration, ast.KindPropertyDeclaration, ast.KindCallExpression, ast.KindNewExpression, ast.KindIdentifier:
				if !f.IsDeclarationFile {
					l.settledWorldChecks = append(l.settledWorldChecks, n)
				}
			}
			n.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
		}
		walk(f.AsNode())
	}
}

func (l *lowerer) settledWorldFailure(n *ast.Node, target *checker.Type, name string, cache map[*checker.Type]bool) string {
	matches := func(t *checker.Type) bool {
		if t == nil || t.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown|checker.TypeFlagsNever) != 0 {
			return false
		}
		if value, ok := cache[t]; ok {
			return value
		}
		value := l.ck.IsTypeAssignableTo(l.ck.GetNonNullableType(t), target)
		cache[t] = value
		return value
	}
	erased := func(source, expected *checker.Type) bool {
		if source == nil || expected == nil {
			return false
		}
		if expected.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
			return matches(source)
		}
		return source.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 && matches(expected)
	}
	switch n.Kind {
	case ast.KindCallExpression, ast.KindNewExpression:
		if matches(l.ck.GetTypeAtLocation(n)) {
			sig := l.ck.GetResolvedSignature(n)
			if n.Kind == ast.KindCallExpression && (sig == nil || sig.Declaration() == nil || sig.Declaration().Body() == nil || !l.settlementFiles[ast.GetSourceFileOfNode(sig.Declaration())]) {
				return l.settledLabel(n) + " obtains an external " + name + " provider"
			}
		}
	case ast.KindIdentifier:
		s := l.resolve(n)
		if s != nil && s.ValueDeclaration != nil && ast.GetSourceFileOfNode(s.ValueDeclaration).IsDeclarationFile {
			if matches(l.ck.GetTypeAtLocation(n)) {
				return l.settledLabel(n) + " names an external " + name + " provider"
			}
		}
	case ast.KindAsExpression, ast.KindTypeAssertionExpression:
		if erased(l.ck.GetTypeAtLocation(n.Expression()), l.ck.GetTypeAtLocation(n)) {
			return l.settledLabel(n) + " erases or casts an unproven " + name + " provider"
		}
	case ast.KindBinaryExpression:
		b := n.AsBinaryExpression()
		if !ast.IsAssignmentOperator(b.OperatorToken.Kind) {
			return ""
		}
		left := b.Left
		if (left.Kind == ast.KindPropertyAccessExpression && left.Name().Text() == name || left.Kind == ast.KindElementAccessExpression) && matches(l.ck.GetTypeAtLocation(left.Expression())) {
			return l.settledLabel(left) + " mutates " + name
		}
		if erased(l.ck.GetTypeAtLocation(b.Right), l.ck.GetTypeAtLocation(left)) {
			return l.settledLabel(b.Right) + " assigns an unproven " + name + " provider"
		}
	case ast.KindVariableDeclaration, ast.KindPropertyDeclaration:
		if n.Initializer() != nil && erased(l.ck.GetTypeAtLocation(n.Initializer()), l.ck.GetTypeAtLocation(n)) {
			return l.settledLabel(n) + " initializes an unproven " + name + " provider"
		}
		if n.Initializer() == nil && matches(l.ck.GetTypeAtLocation(n)) && ast.GetCombinedModifierFlags(n)&ast.ModifierFlagsAmbient != 0 {
			return l.settledLabel(n) + " declares an external " + name + " provider"
		}
	}
	return ""
}

func (l *lowerer) settledConstruction(n *ast.Node, seen map[*ast.Node]bool) bool {
	if seen[n] {
		return true
	}
	seen[n] = true
	defer delete(seen, n)
	t := l.ck.GetTypeAtLocation(n)
	if t != nil && t.Symbol() != nil && t.Symbol().Name == "Promise" && l.librarySymbol(t.Symbol()) {
		l.settlementFailure = "Promise constructor is a pending source"
		return false
	}
	sig := l.ck.GetResolvedSignature(n)
	if sig != nil && sig.Declaration() != nil && sig.Declaration().Body() != nil {
		if !l.proveSettledBody(sig.Declaration(), seen, false) {
			return false
		}
	}
	// Base field initializers execute too, including fields shadowed by a
	// derived class. Walk checked base classes, not only the final properties.
	visited := map[*checker.Type]bool{}
	var fields func(*checker.Type) bool
	fields = func(t *checker.Type) bool {
		if t == nil || visited[t] {
			return true
		}
		visited[t] = true
		if t.Symbol() != nil {
			for _, d := range t.Symbol().Declarations {
				if d.Kind != ast.KindClassDeclaration && d.Kind != ast.KindClassExpression {
					continue
				}
				for _, member := range d.Members() {
					if member.Kind == ast.KindPropertyDeclaration && member.Initializer() != nil && member.ModifierFlags()&ast.ModifierFlagsStatic == 0 && !l.settledInitializer(member.Initializer(), seen) {
						return false
					}
				}
				declared := l.ck.GetDeclaredTypeOfSymbol(d.Symbol())
				for _, base := range l.ck.GetBaseTypes(declared) {
					if !fields(base) {
						return false
					}
				}
			}
		}
		return true
	}
	if !fields(t) {
		return false
	}
	return true
}

func (l *lowerer) settledInitializer(n *ast.Node, seen map[*ast.Node]bool) bool {
	if n == nil || ast.IsFunctionLike(n) {
		return true
	}
	if n.Kind == ast.KindCallExpression && !l.settledBodyCall(n, seen) {
		return false
	}
	if n.Kind == ast.KindNewExpression && !l.settledConstruction(n, seen) {
		return false
	}
	safe := true
	n.ForEachChild(func(c *ast.Node) bool { safe = l.settledInitializer(c, seen); return !safe })
	return safe
}

// The specialized optional-progress proof remains authoritative, including its
// recursive carrier/erasure audit. A generic dispatch proof must not override a
// refusal from that stronger contract.
func (l *lowerer) isProgressDispatch(n *ast.Node) bool {
	s := l.resolve(n.Expression())
	if s == nil {
		return false
	}
	for _, d := range s.Declarations {
		if d.Kind == ast.KindMethodSignature && d.Name() != nil && d.Name().Text() == "tick" && d.Parent != nil && d.Parent.Kind == ast.KindInterfaceDeclaration && d.Parent.Name().Text() == "IProgress" {
			return true
		}
	}
	return false
}

// Explicit implementations count even when their checked shape is incomplete.
// Otherwise a missing method could disappear from an otherwise safe inventory.
func (l *lowerer) declaresSettlementReceiver(n *ast.Node, target *checker.Type) bool {
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
				if s := l.resolve(base.Expression()); s != nil {
					if t := l.ck.GetDeclaredTypeOfSymbol(s); t != nil && l.ck.IsTypeAssignableTo(t, target) {
						return true
					}
				}
			}
		}
	}
	return false
}
