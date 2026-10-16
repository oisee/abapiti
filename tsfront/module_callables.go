package tsfront

import (
	"context"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// Immutable module callables need no general JS function-object ABI. Direct
// calls can use a static method, provided the source binding cannot be replaced
// and the callee does not observe a receiver. Other function-value uses still
// refuse at their use sites. Keep the pinned deployment unchanged.
func (l *lowerer) registerModuleCallables(files []string) {
	if !l.unprunedFeatures {
		return
	}
	l.moduleCallables = map[string]*ast.Node{}
	// Whole-program binding-write screen. Declaration nodes retain identity
	// across the checker pool; unrelated shadowed names must not block aliases.
	written := map[*ast.Node]bool{}
	var mark func(*ast.Node)
	mark = func(x *ast.Node) {
		if x == nil {
			return
		}
		switch x.Kind {
		case ast.KindParenthesizedExpression, ast.KindAsExpression, ast.KindTypeAssertionExpression, ast.KindNonNullExpression, ast.KindSpreadElement:
			mark(x.Expression())
			return
		case ast.KindArrayLiteralExpression:
			for _, el := range x.AsArrayLiteralExpression().Elements.Nodes {
				mark(el)
			}
			return
		case ast.KindObjectLiteralExpression:
			for _, p := range x.AsObjectLiteralExpression().Properties.Nodes {
				if p.Kind == ast.KindShorthandPropertyAssignment {
					if s := l.ck.GetShorthandAssignmentValueSymbol(p); s != nil {
						written[s.ValueDeclaration] = true
					}
				} else if p.Kind == ast.KindSpreadAssignment {
					mark(p.Expression())
				} else {
					mark(p.Initializer())
				}
			}
			return
		case ast.KindBinaryExpression:
			if x.AsBinaryExpression().OperatorToken.Kind == ast.KindEqualsToken {
				mark(x.AsBinaryExpression().Left)
				return
			}
		}
		if sym := l.resolve(x); sym != nil {
			written[sym.ValueDeclaration] = true
		}
	}
	var writes func(*ast.Node)
	writes = func(n *ast.Node) {
		if n == nil {
			return
		}
		var x *ast.Node
		if n.Kind == ast.KindBinaryExpression && ast.IsAssignmentOperator(n.AsBinaryExpression().OperatorToken.Kind) {
			x = n.AsBinaryExpression().Left
		}
		if n.Kind == ast.KindPrefixUnaryExpression {
			p := n.AsPrefixUnaryExpression()
			if p.Operator == ast.KindPlusPlusToken || p.Operator == ast.KindMinusMinusToken {
				x = p.Operand
			}
		}
		if n.Kind == ast.KindPostfixUnaryExpression {
			x = n.AsPostfixUnaryExpression().Operand
		}
		if n.Kind == ast.KindForOfStatement {
			mark(n.AsForInOrOfStatement().Initializer)
		}
		if n.Kind == ast.KindForInStatement {
			mark(n.AsForInOrOfStatement().Initializer)
		}
		mark(x)
		n.ForEachChild(func(c *ast.Node) bool { writes(c); return false })
	}
	for _, f := range l.prog.prog.SourceFiles() {
		if !f.IsDeclarationFile {
			ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
			l.file, l.ck = f, ck
			writes(f.AsNode())
			done()
		}
	}
	var target func(*ast.Node, map[*ast.Node]bool) (funcRef, bool)
	target = func(d *ast.Node, seen map[*ast.Node]bool) (funcRef, bool) {
		if d == nil || seen[d] || d.Name() == nil || d.Name().Kind != ast.KindIdentifier || written[d] {
			return funcRef{}, false
		}
		seen[d] = true
		defer delete(seen, d)
		f := ast.GetSourceFileOfNode(d)
		if d.Kind == ast.KindFunctionDeclaration {
			if touchesThis(d.Body()) {
				return funcRef{}, false
			}
			r, ok := l.funcsBy[f.FileName()+" "+d.Name().Text()]
			return r, ok
		}
		if d.Kind != ast.KindVariableDeclaration || !ast.IsVarConst(d) || d.Parent == nil || d.Parent.Parent == nil || d.Parent.Parent.Parent == nil || d.Parent.Parent.Parent.Kind != ast.KindSourceFile {
			return funcRef{}, false
		}
		init := d.Initializer()
		if init == nil {
			return funcRef{}, false
		}
		if init.Kind == ast.KindArrowFunction {
			if touchesThis(init.Body()) {
				return funcRef{}, false
			}
			key := f.FileName() + " " + d.Name().Text()
			if r, ok := l.funcsBy[key]; ok {
				return r, true
			}
			mod := l.moduleClassOf(f)
			m := &hir.Method{Node: l.node(init), Name: d.Name().Text(), Static: true, Internal: true, Result: hir.T(hir.Void)}
			savedFile, savedCK, savedClass, savedMethod := l.file, l.ck, l.class, l.method
			ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
			l.file, l.ck, l.class, l.method = f, ck, mod, m
			if l.containerGeneric(init) {
				l.containerFunctions[d] = init
			} else {
				l.signature(init, m)
			}
			l.file, l.ck, l.class, l.method = savedFile, savedCK, savedClass, savedMethod
			done()
			if l.containerFunctions[d] == nil {
				mod.Methods = append(mod.Methods, m)
				l.methodsBy[mod.Name+"."+m.Name] = m
			}
			r := funcRef{owner: mod.Name, method: m.Name}
			l.funcsBy[key] = r
			return r, true
		}
		if init.Kind != ast.KindIdentifier && init.Kind != ast.KindPropertyAccessExpression {
			return funcRef{}, false
		}
		// Resolve in the initializer's checker, not the importing file's checker.
		ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
		saved := l.ck
		l.ck = ck
		sym := l.resolve(init)
		l.ck = saved
		done()
		if sym == nil {
			return funcRef{}, false
		}
		return target(sym.ValueDeclaration, seen)
	}
	for _, name := range files {
		f, _ := l.prog.File(name)
		ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		for _, stmt := range l.statementNodes(f) {
			if stmt.Kind != ast.KindVariableStatement {
				continue
			}
			for _, d := range stmt.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
				if r, ok := target(d, map[*ast.Node]bool{}); ok {
					key := f.FileName() + " " + d.Name().Text()
					l.funcsBy[key] = r
					l.moduleCallables[key] = d
					mod := l.moduleClassOf(f)
					mod.Fields = append(mod.Fields, hir.Field{Name: testReadyPrefix + d.Name().Text(), Type: hir.T(hir.Bool), Static: true})
				}
			}
		}
		done()
	}
}

func (l *lowerer) hasModuleCallables(f *ast.SourceFile) bool {
	for key := range l.moduleCallables {
		if strings.HasPrefix(key, f.FileName()+" ") {
			return true
		}
	}
	return false
}

func (l *lowerer) moduleCallable(d *ast.Node) bool {
	return d.Name() != nil && d.Name().Kind == ast.KindIdentifier && l.moduleCallables[l.file.FileName()+" "+d.Name().Text()] == d
}

// Const callables have a TDZ, unlike hoisted function declarations. The test
// recovery boundary must not invoke one after its initialization was refused.
func (l *lowerer) moduleCallableCall(n, callee *ast.Node, call *hir.Expr) *hir.Expr {
	sym := l.resolve(callee)
	if sym != nil {
		if f := l.fileOfSymbol(sym); f != nil && l.moduleCallables[f.FileName()+" "+sym.Name] != nil {
			if mod := l.modules[f]; mod != nil {
				// Reuse the binding-ready guard, but not a nonexistent function field.
				ready := &hir.Expr{Kind: hir.StaticGet, Node: l.node(n), Owner: mod.Name, Name: testReadyPrefix + sym.Name, Type: hir.T(hir.Bool)}
				guard := &hir.Stmt{Kind: hir.If, Node: l.node(n), X: &hir.Expr{Kind: hir.Unary, Type: hir.T(hir.Bool), Op: "!", X: ready}, Body: failureBody(l.node(n), "module callable "+mod.Name+"."+sym.Name+" was not initialized")}
				return &hir.Expr{Kind: hir.Seq, Node: l.node(n), Type: call.Type, Stmt: hir.B(guard), Y: call}
			}
		}
	}
	return call
}
