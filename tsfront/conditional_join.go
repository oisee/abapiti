package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

func (l *lowerer) conditionalExpression(n *ast.Node) *hir.Expr {
	c := n.AsConditionalExpression()
	hint := l.hint
	defer func() { l.hint = hint }()
	x := l.condition(c.Condition)
	if value, ok := constantBool(x); ok {
		l.diagf(n, "note-dead-branch", "conditional with a constant condition keeps only the live branch")
		branch := c.WhenFalse
		if value {
			branch = c.WhenTrue
		}
		l.hint = hint
		live := l.expr(branch)
		if live != nil && x.Kind == hir.Seq {
			live = &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: live.Type, Stmt: x.Stmt, Y: live}
		}
		return live
	}
	// A construction hint may belong to the enclosing operation's result
	// (Set vs iterable). Only query additional checker contexts when needed;
	// speculative mapping can synthesize unused interfaces and generator ABIs.
	context := hint
	inferred := hir.Type{}
	branchHint := context
	baseContext := context
	if baseContext.Kind == hir.Optional {
		baseContext = baseContext.Args[0]
	}
	checkerType := l.ck.GetTypeAtLocation(n)
	if context.Kind == "" || context.Kind == hir.Void || l.ck.IsArrayType(checkerType) && baseContext.Kind != hir.Array {
		inferred = l.conditionalCheckerType(n, checkerType)
		branchHint = inferred
	}
	l.hint = branchHint
	y := l.expr(c.WhenTrue)
	l.hint = branchHint
	z := l.expr(c.WhenFalse)
	l.hint = hint
	if x == nil || y == nil || z == nil {
		return nil
	}
	// An already exact join needs no conversion; the enclosing assignment
	// performs any widening. Preserve the default closure's HIR spelling.
	if y.Type.Equal(z.Type) {
		return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: y.Type, X: x, Y: y, Z: z}
	}
	candidates := []hir.Type{context}
	if !l.conditionalConvertible(c.WhenTrue, y, context) || !l.conditionalConvertible(c.WhenFalse, z, context) {
		if inferred.Kind == "" {
			inferred = l.conditionalCheckerType(n, checkerType)
		}
		candidates = append(candidates, inferred)
		if context.Kind == "" || context.Kind == hir.Void {
			context = l.conditionalCheckerType(n, l.ck.GetContextualType(n, 0))
			candidates = append(candidates, context)
		}
	}
	if joined, ok := l.conditionalBranchJoin(y.Type, z.Type); ok {
		candidates = append(candidates, joined)
	}
	for _, target := range candidates {
		if target.Kind == "" || target.Kind == hir.Void {
			continue
		}
		if !l.conditionalConvertible(c.WhenTrue, y, target) || !l.conditionalConvertible(c.WhenFalse, z, target) {
			continue
		}
		left := l.convertConditionalBranch(c.WhenTrue, y, target)
		right := l.convertConditionalBranch(c.WhenFalse, z, target)
		if left != nil && right != nil && l.acceptsType(target, left.Type) && l.acceptsType(target, right.Type) {
			return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: target, X: x, Y: left, Z: right}
		}
	}
	l.diagf(n, "unsupported-conditional-join", "no exact conditional join for %s and %s (context %s, inferred %s)", y.Type, z.Type, context, inferred)
	return nil
}

// Mapping a candidate is speculative: an unrepresentable checker union must
// not hide a representable join of the actual lowered branches.
func (l *lowerer) conditionalCheckerType(n *ast.Node, t *checker.Type) hir.Type {
	if t == nil {
		return hir.Type{}
	}
	before := len(l.diags)
	mapped := l.mapCheckerType(n, t)
	blocked := hasBlocking(l.diags[before:])
	l.diags = l.diags[:before]
	if blocked {
		return hir.Type{}
	}
	return mapped
}

func (l *lowerer) conditionalBranchJoin(a, b hir.Type) (hir.Type, bool) {
	optional := a.Kind == hir.Optional || b.Kind == hir.Optional
	if a.Kind == hir.Optional {
		a = a.Args[0]
	}
	if b.Kind == hir.Optional {
		b = b.Args[0]
	}
	joined := a
	if !l.acceptsType(a, b) {
		if !l.acceptsType(b, a) {
			return hir.Type{}, false
		}
		joined = b
	}
	if optional {
		joined = hir.T(hir.Optional, joined)
	}
	return joined, true
}

func emptyConditionalArray(n *ast.Node, x *hir.Expr) bool {
	for n.Kind == ast.KindParenthesizedExpression {
		n = n.Expression()
	}
	return n.Kind == ast.KindArrayLiteralExpression && len(n.AsArrayLiteralExpression().Elements.Nodes) == 0 && x.Kind == hir.New && len(x.Args) == 0 && x.Type.Kind == hir.Array
}

// Joining never retags existing generic storage or narrows away absence.
// Fresh [] and undefined can adopt the join without a copy or evaluation.
func (l *lowerer) conditionalConvertible(n *ast.Node, x *hir.Expr, dst hir.Type) bool {
	src := x.Type
	if src.Equal(dst) {
		return true
	}
	if x.Kind == hir.Lit && x.Value == nil && src.Kind == hir.Optional {
		return dst.Kind == hir.Optional || dst.Kind == hir.Dynamic
	}
	if dst.Kind == hir.Optional {
		if src.Kind == hir.Optional {
			copy := *x
			copy.Type = src.Args[0]
			return l.conditionalConvertible(n, &copy, dst.Args[0])
		}
		return l.conditionalConvertible(n, x, dst.Args[0])
	}
	if src.Kind == hir.Optional {
		if dst.Kind != hir.Dynamic {
			return false
		}
		copy := *x
		copy.Type = src.Args[0]
		return l.conditionalConvertible(n, &copy, dst)
	}
	if dst.Kind == hir.Dynamic {
		return true
	}
	if src.Kind == hir.Array && dst.Kind == hir.Array {
		if emptyConditionalArray(n, x) {
			return true
		}
		// Both backends share reference-element storage. Primitive/optional
		// element changes require a live generic view we cannot prove here.
		return src.Args[0].IsRef() && dst.Args[0].IsRef() && l.acceptsType(dst, src)
	}
	return l.acceptsType(dst, src)
}

// Keep conversion temporaries inside the chosen branch, including optional
// boxing. Evaluating an unchosen getter or collection would change JS behavior.
func (l *lowerer) convertConditionalBranch(n *ast.Node, x *hir.Expr, dst hir.Type) *hir.Expr {
	saved := l.pend
	l.pend = nil
	out := l.conditionalConversion(n, x, dst)
	pre := l.pend
	l.pend = saved
	if out != nil && len(pre) > 0 {
		out = &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: out.Type, Stmt: hir.B(pre...), Y: out}
	}
	return out
}

func (l *lowerer) conditionalConversion(n *ast.Node, x *hir.Expr, dst hir.Type) *hir.Expr {
	if x.Type.Equal(dst) {
		return x
	}
	if x.Kind == hir.Lit && x.Value == nil && x.Type.Kind == hir.Optional {
		return l.conditionalAbsent(x.Node, dst)
	}
	if x.Type.Kind == hir.Optional && (dst.Kind == hir.Optional || dst.Kind == hir.Dynamic) {
		target := dst
		if dst.Kind == hir.Optional {
			target = dst.Args[0]
		}
		if l.acceptsType(dst, x.Type) {
			return l.coerce(x, dst)
		}
		saved := l.tempInit(n, x.Type, x)
		present := &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: x.Type.Args[0], X: saved}
		converted := l.conditionalConversion(n, present, target)
		return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: dst, X: &hir.Expr{Kind: hir.IsUndefined, Node: l.node(n), Type: hir.T(hir.Bool), X: saved}, Y: l.conditionalAbsent(l.node(n), dst), Z: converted}
	}
	if dst.Kind == hir.Optional {
		return l.coerce(l.conditionalConversion(n, x, dst.Args[0]), dst)
	}
	if dst.Kind == hir.Array && emptyConditionalArray(n, x) {
		copy := *x
		copy.Type = dst
		return &copy
	}
	return l.coerce(x, dst)
}

func (l *lowerer) conditionalAbsent(node hir.Node, dst hir.Type) *hir.Expr {
	if dst.Kind == hir.Dynamic {
		return l.coerce(&hir.Expr{Kind: hir.Lit, Node: node, Type: hir.T(hir.Optional, dst)}, dst)
	}
	return &hir.Expr{Kind: hir.Lit, Node: node, Type: dst}
}
