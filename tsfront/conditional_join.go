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
	// A fresh unparameterized Map has no existing storage to adapt. Give
	// it the other branch's exact ABI before considering invariant joins.
	if l.unprunedFeatures && y.Type.Kind == hir.OrderedMap && z.Type.Kind == hir.OrderedMap {
		if l.freshConditionalMap(c.WhenTrue, y) {
			y = l.freshConditionalMapValue(c.WhenTrue, z.Type)
		}
		if l.freshConditionalMap(c.WhenFalse, z) {
			z = l.freshConditionalMapValue(c.WhenFalse, y.Type)
		}
	}
	// An already exact join needs no conversion; the enclosing assignment
	// performs any widening. Preserve the default closure's HIR spelling.
	if y.Type.Kind != "" && y.Type.Kind != hir.Void && y.Type.Equal(z.Type) {
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
	if joined, ok := l.conditionalBranchJoin(y.Type, z.Type); ok && l.conditionalJoinFitsContext(joined, y.Type, z.Type, context) {
		candidates = append(candidates, joined)
	}
	for _, target := range candidates {
		if target.Kind == "" || target.Kind == hir.Void {
			continue
		}
		if !l.conditionalConvertible(c.WhenTrue, y, target) || !l.conditionalConvertible(c.WhenFalse, z, target) {
			continue
		}
		left := l.convertConditionalBranch(c.WhenTrue, y, target, l.conditionalRepeatedRead(c.Condition, c.WhenTrue))
		right := l.convertConditionalBranch(c.WhenFalse, z, target, l.conditionalRepeatedRead(c.Condition, c.WhenFalse))
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
	if !l.acceptsType(a, b) && !l.familyMember(a, b) {
		if !l.acceptsType(b, a) && !l.familyMember(b, a) {
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
	if src.Kind == "" || src.Kind == hir.Void || dst.Kind == "" || dst.Kind == hir.Void {
		return false
	}
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
	if l.mapStorageView(dst, src) {
		return true
	}
	if src.Kind == hir.Array && dst.Kind == hir.Array {
		if emptyConditionalArray(n, x) || l.readonlyStringArrayView(n, src, dst) {
			return true
		}
		// Structural reference elements use coerce's live array view, keeping
		// the array and its elements aliased. Primitive/optional element
		// changes still require a storage proof we do not have here.
		return src.Args[0].IsRef() && dst.Args[0].IsRef() && (l.acceptsType(dst, src) || l.unprunedFeatures && l.dataShapeSubtype(dst.Args[0], src.Args[0]))
	}
	if src.Kind == hir.Dynamic {
		if dst.Kind == hir.UnionRef && l.unprunedFeatures {
			u := l.out.Union(dst.Name)
			return u != nil && u.Complete
		}
		switch dst.Kind {
		case hir.String, hir.Number, hir.Bool, hir.ClassValue, hir.ClassRef, hir.InterfaceRef, hir.Array, hir.OrderedMap, hir.OrderedSet:
			return true // existing checked unboxing, never an unchecked retag
		case hir.UnionRef:
			u := l.out.Union(dst.Name)
			return l.unprunedFeatures && u != nil && u.Complete // checked union.project
		}
	}
	if src.Kind == hir.InterfaceRef && dst.Kind == hir.ClassRef && dst.Name != hir.RootObject {
		return l.interfaceHasClassBase(src.Name, dst) || l.unionPartsAccepted(src.Name, dst)
	}
	if l.unprunedFeatures && src.Kind == hir.UnionRef && dst.Kind == hir.UnionRef && l.familyViewSubtype(dst, src, map[string]bool{}) {
		// A ValueExpr branch in an Expr slot: coerce widens it by the
		// checked live view (familyAdmission), the same object.
		return true
	}
	return l.acceptsType(dst, src) || l.familyMember(dst, src) || l.flowProjection(n, src, dst)
}

// flowProjection reports a declared family value that the checker narrowed
// at this branch (`k === "x" ? I : outer`) into a subset that dst includes.
// union.project keeps the same storage and still checks membership.
func (l *lowerer) flowProjection(n *ast.Node, src, dst hir.Type) bool {
	if !l.unprunedFeatures || src.Kind != hir.UnionRef || dst.Kind != hir.UnionRef || src.Equal(dst) {
		return false
	}
	from, to := l.out.Union(src.Name), l.out.Union(dst.Name)
	if from == nil || to == nil || !hir.UnionIncludes(from, to) {
		return false
	}
	flow := l.conditionalCheckerType(n, l.ck.GetTypeAtLocation(n))
	if flow.Kind == hir.Optional {
		flow = flow.Args[0]
	}
	if flow.Kind != hir.UnionRef {
		return false
	}
	narrowed := l.out.Union(flow.Name)
	return narrowed != nil && hir.UnionIncludes(to, narrowed)
}

// A family join is a widening; the enclosing assignment can widen it further
// but never narrows it to a checker-narrowed family subset.
func (l *lowerer) conditionalJoinFitsContext(joined, a, b, context hir.Type) bool {
	base := func(t hir.Type) hir.Type {
		if t.Kind == hir.Optional {
			return t.Args[0]
		}
		return t
	}
	if !l.familyMember(base(joined), base(a)) && !l.familyMember(base(joined), base(b)) {
		return true
	}
	if context.Kind == "" || context.Kind == hir.Void || context.Kind == hir.Dynamic {
		return true
	}
	if joined.Kind == hir.Optional && context.Kind != hir.Optional {
		return false
	}
	return base(context).Equal(base(joined)) || l.familyMember(base(context), base(joined))
}

// familyMember reports a recursive-family value that coerce admits to dst
// without a copy: a payload of one of dst's alternatives (union.admit) or a
// subset family that dst includes (union.widen).
func (l *lowerer) familyMember(dst, src hir.Type) bool {
	if !l.unprunedFeatures || dst.Kind != hir.UnionRef {
		return false
	}
	u := l.out.Union(dst.Name)
	if u == nil {
		return false
	}
	switch src.Kind {
	case hir.UnionRef:
		other := l.out.Union(src.Name)
		return other != nil && hir.UnionIncludes(u, other)
	case hir.ClassRef:
		for _, a := range u.Alternatives {
			if src.Equal(a.Payload) {
				return true
			}
		}
	}
	return false
}

// Keep conversion temporaries inside the chosen branch, including optional
// boxing. Evaluating an unchosen getter or collection would change JS behavior.
func (l *lowerer) convertConditionalBranch(n *ast.Node, x *hir.Expr, dst hir.Type, repeatedRead bool) *hir.Expr {
	saved := l.pend
	l.pend = nil
	out := l.conditionalConversion(n, x, dst)
	pre := l.pend
	l.pend = saved
	if repeatedRead {
		l.pend = append(l.pend, pre...)
		return out
	}
	if out != nil && len(pre) > 0 {
		out = &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: out.Type, Stmt: hir.B(pre...), Y: out}
	}
	return out
}

func (l *lowerer) conditionalConversion(n *ast.Node, x *hir.Expr, dst hir.Type) *hir.Expr {
	if x.Type.Equal(dst) {
		return x
	}
	if dst.Kind == hir.Array && x.Kind == hir.RuntimeOp && x.Op == "dynamic.null" {
		// RegExp.exec uses a nil array reference for a non-match. A null
		// fallback in that same array join must retain absence, rather than
		// ask the live array view to find storage inside a null value.
		absent := l.conditionalAbsent(x.Node, hir.T(hir.Optional, dst))
		return &hir.Expr{Kind: hir.Narrow, Node: x.Node, Type: dst, X: absent}
	}
	if x.Kind == hir.Lit && x.Value == nil && x.Type.Kind == hir.Optional {
		return l.conditionalAbsent(x.Node, dst)
	}
	// Reuse existing proven nominal/interface conversions before decomposing
	// an optional. Only optional boxing needs our presence-preserving path.
	if l.flowProjection(n, x.Type, dst) {
		projected := l.rtOp("union.project", x, dst)
		projected.Node = l.node(n)
		return projected
	}
	boxingOptional := x.Type.Kind == hir.Optional && (dst.Kind == hir.Dynamic || dst.Kind == hir.Optional && dst.Args[0].Kind == hir.Dynamic)
	if !boxingOptional {
		converted := l.coerce(x, dst)
		if l.acceptsType(dst, converted.Type) {
			return converted
		}
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
	if l.readonlyStringArrayView(n, x.Type, dst) {
		return l.rtOp("dynamic.arrayView", l.coerce(x, hir.T(hir.Dynamic)), dst)
	}
	return l.coerce(x, dst)
}

// readonlyStringArrayView reports a branch the checker types as a readonly
// string array (set?.size ? providers : frozenEmpty) joined in a tagged
// array context. Like a readonly argument (readonlyArrayArgument), it is the
// same storage seen through tagged elements; nothing can write through it.
func (l *lowerer) readonlyStringArrayView(n *ast.Node, src, dst hir.Type) bool {
	if !l.unprunedFeatures || src.Kind != hir.Array || dst.Kind != hir.Array || src.Args[0].Kind != hir.String || dst.Args[0].Kind != hir.Dynamic {
		return false
	}
	t := l.ck.GetNonNullableType(l.ck.GetTypeAtLocation(n))
	return t != nil && t.Symbol() != nil && t.Symbol().Name == "ReadonlyArray" && l.ck.IsArrayType(t)
}

func (l *lowerer) conditionalAbsent(node hir.Node, dst hir.Type) *hir.Expr {
	if dst.Kind == hir.Dynamic {
		return l.coerce(&hir.Expr{Kind: hir.Lit, Node: node, Type: hir.T(hir.Optional, dst)}, dst)
	}
	return &hir.Expr{Kind: hir.Lit, Node: node, Type: dst}
}

// A literal-key data read already evaluated by the condition can keep the
// existing temporary placement. No call, getter or computed key is moved;
// checked unboxing itself remains in the selected branch.
func (l *lowerer) conditionalRepeatedRead(condition, branch *ast.Node) bool {
	if condition.Kind == ast.KindPropertyAccessExpression && branch.Kind == ast.KindPropertyAccessExpression {
		a, b := condition.AsPropertyAccessExpression(), branch.AsPropertyAccessExpression()
		// Dynamic property access in this ABI reads data, never a JS getter.
		typ := l.ck.GetTypeAtLocation(a.Expression)
		return a.QuestionDotToken == nil && b.QuestionDotToken == nil && a.Expression.Kind == ast.KindIdentifier && b.Expression.Kind == ast.KindIdentifier && typ != nil && typ.Flags()&checker.TypeFlagsAny != 0 && a.Name().Text() == b.Name().Text() && l.resolve(a.Expression) != nil && l.resolve(a.Expression) == l.resolve(b.Expression)
	}
	if condition.Kind != ast.KindElementAccessExpression || branch.Kind != ast.KindElementAccessExpression {
		return false
	}
	a, b := condition.AsElementAccessExpression(), branch.AsElementAccessExpression()
	return a.Expression.Kind == ast.KindIdentifier && b.Expression.Kind == ast.KindIdentifier && a.ArgumentExpression.Kind == ast.KindStringLiteral && b.ArgumentExpression.Kind == ast.KindStringLiteral && a.ArgumentExpression.Text() == b.ArgumentExpression.Text() && l.resolve(a.Expression) != nil && l.resolve(a.Expression) == l.resolve(b.Expression)
}

func (l *lowerer) freshConditionalMap(n *ast.Node, x *hir.Expr) bool {
	for n.Kind == ast.KindParenthesizedExpression {
		n = n.Expression()
	}
	return n.Kind == ast.KindNewExpression && l.globalBuiltin(n.Expression(), "Map") && len(n.Arguments()) == 0 && len(n.TypeArguments()) == 0 && x.Type.Kind == hir.OrderedMap
}

func (l *lowerer) freshConditionalMapValue(n *ast.Node, t hir.Type) *hir.Expr {
	for n.Kind == ast.KindParenthesizedExpression {
		n = n.Expression()
	}
	return l.sealJSMap(n, &hir.Expr{Kind: hir.New, Node: l.node(n), Type: t})
}
