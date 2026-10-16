package tsfront

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

type containerSpecialization struct {
	fn     *ast.Node
	args   []*checker.Type
	method *hir.Method
	owner  *hir.Class
}

// Container storage cannot use boundary erasure: each direct instantiation
// retains its native storage and reference identity. Only unconstrained,
// synchronous module helpers with Map or record containers enter this path.
func (l *lowerer) containerGeneric(fn *ast.Node) bool {
	if !l.unprunedFeatures || fn == nil || fn.Body() == nil || fn.ModifierFlags()&ast.ModifierFlagsAsync != 0 {
		return false
	}
	sig := l.ck.GetSignatureFromDeclaration(fn)
	if sig == nil || len(sig.TypeParameters()) == 0 || touchesThis(fn.Body()) {
		return false
	}
	container := false
	for _, t := range sig.TypeParameters() {
		if l.ck.GetConstraintOfTypeParameter(t) != nil {
			return false
		}
		for _, p := range sig.Parameters() {
			x := l.ck.GetTypeOfSymbol(p)
			if l.mentionsType(x, t, 0) && !l.boundaryType(x, t, 0) && l.hasGenericContainer(x, t, 0) {
				container = true
			}
		}
	}
	return container
}

func (l *lowerer) hasGenericContainer(x, t *checker.Type, depth int) bool {
	if x == nil || depth > 5 || x.Flags()&checker.TypeFlagsObject == 0 {
		return false
	}
	if x.Symbol() != nil && (x.Symbol().Name == "Map" || x.Symbol().Name == "ReadonlyMap") && l.librarySymbol(x.Symbol()) && l.mentionsType(x, t, 0) {
		return true
	}
	if x.Symbol() == nil || x.Symbol().Flags&ast.SymbolFlagsTypeLiteral != 0 {
		for _, p := range l.ck.GetPropertiesOfType(x) {
			if l.mentionsType(l.ck.GetTypeOfSymbol(p), t, 0) {
				return true
			}
		}
	}
	if x.ObjectFlags()&checker.ObjectFlagsReference != 0 {
		for _, a := range l.ck.GetTypeArguments(x) {
			if l.hasGenericContainer(a, t, depth+1) {
				return true
			}
		}
	}
	for _, s := range l.ck.GetSignaturesOfType(x, checker.SignatureKindCall) {
		if l.hasGenericContainer(l.ck.GetReturnTypeOfSignature(s), t, depth+1) {
			return true
		}
	}
	return false
}

func (l *lowerer) specializedType(t *checker.Type) *checker.Type {
	if t == nil || len(l.containerParams) == 0 {
		return t
	}
	return l.ck.SubstituteTypes(t, l.containerParams, l.containerArgs)
}

func (l *lowerer) containerGenericCall(n *ast.Node) (*hir.Expr, bool) {
	callee := gogenCallee(n.Expression())
	if callee == nil || callee.Kind != ast.KindIdentifier {
		return nil, false
	}
	sym := l.resolve(callee)
	if sym == nil {
		return nil, false
	}
	fn := l.containerFunctions[sym.ValueDeclaration]
	if fn == nil {
		return nil, false
	}
	args := l.ck.ResolvedTypeArguments(l.ck.GetResolvedSignature(n))
	if len(args) == 0 || len(args) != len(l.ck.GetSignatureFromDeclaration(fn).TypeParameters()) {
		l.diagf(n, "unsupported-generic", "container helper requires resolved type arguments")
		return nil, true
	}
	var key strings.Builder
	for i, t := range args {
		args[i] = l.specializedType(t)
		// Checker names include nominal owners; HIR names additionally distinguish
		// structural layouts that TypeToString abbreviates to an alias.
		typ := l.mapCheckerType(n, args[i])
		if typ.Kind == hir.Void {
			return nil, true
		}
		fmt.Fprintf(&key, "%s;%s;", l.ck.TypeToString(args[i]), typ.String())
	}
	f := ast.GetSourceFileOfNode(fn)
	name := sym.Name + "_specialized_" + fmt.Sprintf("%x", sha256.Sum256([]byte(key.String())))[:16]
	owner := l.moduleClassOf(f)
	method := l.methodsBy[owner.Name+"."+name]
	if method == nil {
		count := 0
		for _, s := range l.containerQueue {
			if s.fn == fn {
				count++
			}
		}
		if count >= 64 {
			l.diagf(n, "unsupported-generic", "container helper exceeds 64 concrete instantiations (possible polymorphic recursion)")
			return nil, true
		}
		method = &hir.Method{Node: l.node(n), Name: name, Static: true, Internal: true, Result: hir.T(hir.Void)}
		l.methodsBy[owner.Name+"."+name] = method
		owner.Methods = append(owner.Methods, method)
		l.containerQueue = append(l.containerQueue, containerSpecialization{fn, args, method, owner})
		savedFile, savedCK, savedClass, savedMethod := l.file, l.ck, l.class, l.method
		savedParams, savedArgs := l.containerParams, l.containerArgs
		ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck, l.class, l.method = f, ck, owner, method
		method.Node = l.node(fn)
		l.containerParams, l.containerArgs = ck.GetSignatureFromDeclaration(fn).TypeParameters(), args
		l.signature(fn, method)
		// A JS expression body may admit absence even when its JSDoc result
		// omits it (Map.get after set is the motivating case). Preserve that
		// absence in the concrete ABI; callers apply their usual checked view.
		if fn.Body().Kind != ast.KindBlock && method.Result.IsRef() && l.checkerMayBeUndefined(fn.Body()) {
			method.Result = hir.T(hir.Optional, method.Result)
		}
		l.containerParams, l.containerArgs = savedParams, savedArgs
		l.file, l.ck, l.class, l.method = savedFile, savedCK, savedClass, savedMethod
		done()
	}
	ref := funcRef{owner: owner.Name, method: name}
	// Reading a const callee precedes argument effects, including preludes
	// extracted by callArgsStatic. Use the ordinary module-call ordering.
	if check := l.moduleCallableCall(n, callee, hir.L(hir.T(hir.Bool), true)); check.Kind == hir.Seq {
		l.pendStmt(check.Stmt)
	}
	xs, ok := l.callArgsStatic(n, n.Arguments(), ref)
	if !ok {
		return nil, true
	}
	call := &hir.Expr{Kind: hir.DirectCall, Node: l.node(n), Owner: owner.Name, Name: name, Type: method.Result, Args: xs}
	return call, true
}

func (l *lowerer) lowerContainerSpecializations() {
	// Processing may discover another concrete instantiation. Registration before
	// bodies also makes recursive calls reuse the same method.
	for i := 0; i < len(l.containerQueue); i++ {
		s := l.containerQueue[i]
		f := ast.GetSourceFileOfNode(s.fn)
		ck, done := l.prog.prog.GetTypeCheckerForFile(context.Background(), f)
		l.file, l.ck = f, ck
		l.containerParams, l.containerArgs = ck.GetSignatureFromDeclaration(s.fn).TypeParameters(), s.args
		l.functionBodyToMethod(s.fn, s.owner, s.method.Name)
		l.containerParams, l.containerArgs = nil, nil
		done()
	}
}
