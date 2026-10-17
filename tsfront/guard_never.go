package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// neverGuardNarrowing handles a binding the checker narrows to never, or to
// an intersection that does not lower, inside the true branch of a
// type-predicate guard on it (frontend.mjs cond(): an earlier
// `isExpr(n, Compare)` return dropped ExpressionNode from n's union, so
// `isExpr(n, CondSub)` leaves (StatementNode | TokenNode) & ExpressionNode,
// which lowers to the object root).
// The guard returned true at run time, so the value is the predicate's type:
// the binding is viewed through a checked cast to it. Only unassigned
// parameters and const bindings qualify.
func (l *lowerer) neverGuardNarrowing(n *ast.Node, x *hir.Expr, t *checker.Type) (*hir.Expr, bool) {
	if !l.unprunedFeatures || n.Kind != ast.KindIdentifier || t == nil || t.Flags()&(checker.TypeFlagsNever|checker.TypeFlagsIntersection|checker.TypeFlagsUnion) == 0 {
		return nil, false
	}
	sym := l.resolve(n)
	if sym == nil || sym.ValueDeclaration == nil || !l.unassignedBinding(sym) {
		return nil, false
	}
	for child, p := n, n.Parent; p != nil; child, p = p, p.Parent {
		if ast.IsFunctionLike(p) {
			break
		}
		var guard *ast.Node
		switch p.Kind {
		case ast.KindIfStatement:
			if p.AsIfStatement().ThenStatement == child {
				guard = p.AsIfStatement().Expression
			}
		case ast.KindConditionalExpression:
			if p.AsConditionalExpression().WhenTrue == child {
				guard = p.AsConditionalExpression().Condition
			}
		case ast.KindBinaryExpression:
			if b := p.AsBinaryExpression(); b.OperatorToken.Kind == ast.KindAmpersandAmpersandToken && b.Right == child {
				guard = b.Left
			}
		}
		if guard == nil {
			continue
		}
		predicate := l.guardPredicateType(guard, sym)
		if predicate == nil {
			continue
		}
		before := len(l.diags)
		typ := l.mapCheckerType(n, predicate)
		if hasBlocking(l.diags[before:]) || typ.Kind != hir.ClassRef && typ.Kind != hir.InterfaceRef {
			l.diags = l.diags[:before]
			return nil, false
		}
		l.diags = l.diags[:before]
		return l.narrowed(n, x, typ), true
	}
	return nil, false
}

// guardPredicateType is the asserted type of a true `pred(sym, ...)` or
// `sym instanceof C` guard, possibly one conjunct of `&&`.
func (l *lowerer) guardPredicateType(guard *ast.Node, sym *ast.Symbol) *checker.Type {
	guard = ast.SkipParentheses(guard)
	if guard.Kind == ast.KindBinaryExpression && guard.AsBinaryExpression().OperatorToken.Kind == ast.KindInstanceOfKeyword {
		// `x instanceof C`: the instance type of the class C names.
		b := guard.AsBinaryExpression()
		left := ast.SkipParentheses(b.Left)
		if left.Kind != ast.KindIdentifier || l.resolve(left) != sym {
			return nil
		}
		class := l.resolve(b.Right)
		if class != nil && class.Flags&ast.SymbolFlagsAlias != 0 {
			class, _ = l.ck.ResolveAlias(class)
		}
		if class == nil || class.Flags&ast.SymbolFlagsClass == 0 {
			return nil
		}
		return l.ck.GetDeclaredTypeOfSymbol(class)
	}
	if guard.Kind == ast.KindBinaryExpression {
		b := guard.AsBinaryExpression()
		if b.OperatorToken.Kind != ast.KindAmpersandAmpersandToken {
			return nil
		}
		if t := l.guardPredicateType(b.Right, sym); t != nil {
			return t
		}
		return l.guardPredicateType(b.Left, sym)
	}
	if guard.Kind == ast.KindBinaryExpression && guard.AsBinaryExpression().OperatorToken.Kind == ast.KindInstanceOfKeyword && l.unprunedFeatures {
		// `x instanceof C` held: x is an instance of C's construct result.
		b := guard.AsBinaryExpression()
		if left := ast.SkipParentheses(b.Left); left.Kind != ast.KindIdentifier || l.resolve(left) != sym {
			return nil
		}
		ctor := l.ck.GetTypeAtLocation(b.Right)
		if ctor == nil {
			return nil
		}
		sigs := l.ck.GetSignaturesOfType(ctor, checker.SignatureKindConstruct)
		if len(sigs) != 1 {
			return nil
		}
		return l.ck.GetReturnTypeOfSignature(sigs[0])
	}
	if guard.Kind != ast.KindCallExpression {
		return nil
	}
	sig := l.ck.GetResolvedSignature(guard)
	if sig == nil {
		return nil
	}
	predicate := l.ck.GetTypePredicateOfSignature(sig)
	if predicate == nil || predicate.Kind() != checker.TypePredicateKindIdentifier || predicate.Type() == nil {
		return nil
	}
	args := guard.Arguments()
	i := int(predicate.ParameterIndex())
	if i < 0 || i >= len(args) {
		return nil
	}
	arg := ast.SkipParentheses(args[i])
	if arg.Kind != ast.KindIdentifier || l.resolve(arg) != sym {
		return nil
	}
	return predicate.Type()
}

// unassignedBinding: a const, or a parameter its function never assigns.
func (l *lowerer) unassignedBinding(sym *ast.Symbol) bool {
	d := sym.ValueDeclaration
	if d.Kind == ast.KindVariableDeclaration {
		return ast.IsVarConst(d)
	}
	if d.Kind != ast.KindParameter || d.Name() == nil || d.Name().Kind != ast.KindIdentifier {
		return false
	}
	fn := d.Parent
	if fn == nil || !ast.IsFunctionLike(fn) || fn.Body() == nil {
		return false
	}
	assigned := false
	var walk func(*ast.Node) bool
	walk = func(x *ast.Node) bool {
		if assigned {
			return true
		}
		if x.Kind == ast.KindIdentifier && ast.IsAssignmentTarget(x) && l.resolve(x) == sym {
			assigned = true
			return true
		}
		x.ForEachChild(walk)
		return assigned
	}
	walk(fn.Body())
	return !assigned
}
