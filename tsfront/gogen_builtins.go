package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Resolve the actual global: spelling alone must never capture a local or an
// imported value. Node's process is ambient rather than a default-lib symbol.
func (l *lowerer) globalBuiltin(n *ast.Node, name string) bool {
	if n == nil || n.Kind != ast.KindIdentifier || n.Text() != name {
		return false
	}
	if _, ok := l.lookup(name); ok {
		return false
	}
	s := l.resolve(n)
	if s == nil {
		return false
	}
	if name != "process" {
		return l.librarySymbol(s) && l.classOf(s) == nil
	}
	for _, d := range s.Declarations {
		if d.Kind == ast.KindVariableDeclaration && d.Initializer() == nil && ast.GetCombinedModifierFlags(d)&ast.ModifierFlagsAmbient != 0 {
			return true
		}
	}
	return false
}

func (l *lowerer) numericBuiltin(n *ast.Node) (*hir.Expr, bool) {
	c := n.Expression()
	if c == nil || c.Kind != ast.KindPropertyAccessExpression {
		return nil, false
	}
	p := c.AsPropertyAccessExpression()
	name := c.Name().Text()
	op := ""
	if l.globalBuiltin(p.Expression, "Number") {
		switch name {
		case "isInteger", "isSafeInteger", "isFinite", "isNaN":
			op = "number." + name
		}
	} else if l.globalBuiltin(p.Expression, "Math") && name == "abs" {
		op = "number.abs"
	}
	if op == "" {
		return nil, false
	}
	args := n.Arguments()
	if len(args) != 1 {
		l.diagf(n, "unsupported-call", "%s requires exactly one argument in this lowering", op)
		return nil, true
	}
	x := l.expr(args[0])
	if x == nil {
		return nil, true
	}
	if op == "number.abs" {
		if !numberType(x.Type) {
			l.diagf(n, "unsupported-call", "Math.abs requires a numeric operand")
			return nil, true
		}
		out := l.rtOp(op, l.coerce(x, hir.T(hir.Number)), hir.T(hir.Number))
		out.Node = l.node(n)
		return out, true
	}
	// Number's predicates do not perform ToNumber (unlike global isNaN).
	if op == "number.isNaN" {
		op = "number.strictIsNaN"
	}
	x = l.convertConditionalBranch(args[0], x, hir.T(hir.Dynamic), false)
	out := l.rtOp(op, x, hir.T(hir.Bool))
	out.Node = l.node(n)
	return out, true
}

func (l *lowerer) processRead(n *ast.Node) (*hir.Expr, bool) {
	var receiver, key *ast.Node
	var literal string
	switch n.Kind {
	case ast.KindPropertyAccessExpression:
		receiver = n.Expression()
		literal = n.Name().Text()
	case ast.KindElementAccessExpression:
		receiver = n.AsElementAccessExpression().Expression
		key = n.AsElementAccessExpression().ArgumentExpression
	default:
		return nil, false
	}
	if receiver == nil || receiver.Kind != ast.KindPropertyAccessExpression || receiver.Name().Text() != "env" || !l.globalBuiltin(receiver.Expression(), "process") {
		return nil, false
	}
	x := hir.L(hir.T(hir.String), literal)
	if key != nil {
		x = l.expr(key)
	}
	if x == nil {
		return nil, true
	}
	if x.Type.Kind != hir.String {
		l.diagf(n, "unsupported-process", "process.env key must be a string")
		return nil, true
	}
	x.Node = l.node(n)
	return l.rtOp("host.getenv", x, hir.T(hir.Optional, hir.T(hir.String))), true
}
