package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
)

// typeParamErasable: an unconstrained type parameter of a function whose
// signature uses it only at the call boundary: as a parameter type, the
// result type, or a top-level parameter/result of a function-typed
// parameter or result (run: () => T, returning T). Such values pass through
// untouched, so a tagged (Dynamic) representation converted at each call
// site is exact. Inside containers (T[], Map<K, T>, {v: T}) a tagged view of
// typed storage is not exact; module helpers with supported containers use
// checker specializations instead (generic_containers.go).
func (l *lowerer) typeParamErasable(t *checker.Type) bool {
	if !l.unprunedFeatures || t.Symbol() == nil {
		return false
	}
	if l.erasableTypeParams == nil {
		l.erasableTypeParams = map[*ast.Symbol]bool{}
	}
	if v, ok := l.erasableTypeParams[t.Symbol()]; ok {
		return v
	}
	l.erasableTypeParams[t.Symbol()] = false // recursion guard
	ok := false
	if host := typeParamHost(t.Symbol()); host != nil && host.Symbol() != nil {
		ok = true
		for _, sig := range l.ck.GetSignaturesOfType(l.ck.GetTypeOfSymbol(host.Symbol()), checker.SignatureKindCall) {
			ok = ok && l.boundarySignature(sig, t, 0)
		}
		if len(l.ck.GetSignaturesOfType(l.ck.GetTypeOfSymbol(host.Symbol()), checker.SignatureKindCall)) == 0 {
			ok = false
		}
	}
	l.erasableTypeParams[t.Symbol()] = ok
	return ok
}

// typeParamHost finds the function declaring the type parameter (a type
// parameter list or a JSDoc @template tag).
func typeParamHost(sym *ast.Symbol) *ast.Node {
	for _, d := range sym.Declarations {
		for p := d.Parent; p != nil; p = p.Parent {
			switch p.Kind {
			case ast.KindFunctionDeclaration, ast.KindMethodDeclaration, ast.KindArrowFunction, ast.KindFunctionExpression:
				return p
			case ast.KindJSDoc:
				if host := p.Parent; host != nil {
					switch host.Kind {
					case ast.KindFunctionDeclaration, ast.KindMethodDeclaration:
						return host
					case ast.KindVariableStatement:
						if decls := host.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes; len(decls) == 1 {
							if init := decls[0].Initializer(); init != nil && (init.Kind == ast.KindArrowFunction || init.Kind == ast.KindFunctionExpression) {
								return init
							}
						}
					}
				}
				return nil
			case ast.KindClassDeclaration, ast.KindInterfaceDeclaration, ast.KindTypeAliasDeclaration:
				return nil
			}
		}
	}
	return nil
}

func (l *lowerer) boundarySignature(sig *checker.Signature, t *checker.Type, depth int) bool {
	if depth > 2 {
		return false
	}
	for _, p := range sig.Parameters() {
		if !l.boundaryType(l.ck.GetTypeOfSymbol(p), t, depth) {
			return false
		}
	}
	return l.boundaryType(l.ck.GetReturnTypeOfSignature(sig), t, depth)
}

func (l *lowerer) boundaryType(x, t *checker.Type, depth int) bool {
	if x == nil || x == t || !l.mentionsType(x, t, 0) {
		return true
	}
	if x.IsUnion() {
		for _, part := range x.AsUnionOrIntersectionType().Types() {
			if part != t && part.Flags()&(checker.TypeFlagsUndefined|checker.TypeFlagsNull|checker.TypeFlagsVoid) == 0 && l.mentionsType(part, t, 0) {
				return false
			}
		}
		return false // T | undefined has no exact tagged optional view
	}
	sigs := l.ck.GetSignaturesOfType(x, checker.SignatureKindCall)
	if len(sigs) != 1 || len(l.ck.GetPropertiesOfType(x)) != 0 {
		return false
	}
	return l.boundarySignature(sigs[0], t, depth+1)
}

// mentionsType reports whether x refers to t (type arguments, union parts,
// signatures, properties), to a bounded depth.
func (l *lowerer) mentionsType(x, t *checker.Type, depth int) bool {
	if x == nil || depth > 4 {
		return x != nil
	}
	if x == t {
		return true
	}
	if x.Flags()&(checker.TypeFlagsUnion|checker.TypeFlagsIntersection) != 0 {
		for _, part := range x.AsUnionOrIntersectionType().Types() {
			if l.mentionsType(part, t, depth+1) {
				return true
			}
		}
		return false
	}
	if x.Flags()&checker.TypeFlagsObject == 0 {
		return false
	}
	if x.ObjectFlags()&checker.ObjectFlagsReference != 0 {
		for _, a := range l.ck.GetTypeArguments(x) {
			if l.mentionsType(a, t, depth+1) {
				return true
			}
		}
	}
	for _, sig := range l.ck.GetSignaturesOfType(x, checker.SignatureKindCall) {
		for _, p := range sig.Parameters() {
			if l.mentionsType(l.ck.GetTypeOfSymbol(p), t, depth+1) {
				return true
			}
		}
		if l.mentionsType(l.ck.GetReturnTypeOfSignature(sig), t, depth+1) {
			return true
		}
	}
	if x.Symbol() == nil || x.Symbol().Flags&ast.SymbolFlagsTypeLiteral != 0 {
		for _, p := range l.ck.GetPropertiesOfType(x) {
			if l.mentionsType(l.ck.GetTypeOfSymbol(p), t, depth+1) {
				return true
			}
		}
	}
	return false
}

// trailingOptional returns how many trailing inner parameters an outer
// signature omits, or -1 when it is not a plain prefix: no rest parameters
// on either side when the arity differs, and every omitted one optional.
func trailingOptional(inner, outer *hir.Method) int {
	if len(outer.Params) == len(inner.Params) {
		return 0
	}
	if len(outer.Params) > len(inner.Params) {
		return -1
	}
	for _, p := range outer.Params {
		if p.Variadic {
			return -1
		}
	}
	for i, p := range inner.Params {
		if p.Variadic || i >= len(outer.Params) && p.Type.Kind != hir.Optional {
			return -1
		}
	}
	return len(inner.Params) - len(outer.Params)
}

// closureAdapter converts a function value between closure interfaces that
// differ only where one side erased a type parameter to a tagged value
// ((ctx: Dynamic) => Dynamic used as (ctx: string) => string): a wrapper
// converts each argument into the inner interface's parameter type and the
// inner result into the outer result type. Conversions needing statement
// preludes are not adapted.
func (l *lowerer) closureAdapter(n *ast.Node, x *hir.Expr, dst hir.Type) *hir.Expr {
	if !l.unprunedFeatures || x == nil || x.Type.Equal(dst) {
		return nil
	}
	if x.Type.Kind == hir.ClassRef && l.callableMethod(dst) != nil {
		// An inline closure class: view it as the function interface it
		// implements, then adapt that interface.
		c := l.classByName(x.Type.Name)
		if c == nil || len(c.Implements) != 1 || c.Implements[0] == dst.Name {
			return nil
		}
		x = &hir.Expr{Kind: hir.Cast, Node: x.Node, Type: hir.Type{Kind: hir.InterfaceRef, Name: c.Implements[0]}, X: x}
	}
	inner, outer := l.callableMethod(x.Type), l.callableMethod(dst)
	if inner == nil || outer == nil || dst.Name == thunkInterfaceName {
		return nil
	}
	if adapted, handled := l.variadicClosureAdapter(n, x, dst, inner, outer); handled {
		return adapted
	}
	// A function with trailing optional parameters is also a value of a
	// shorter function type: JavaScript binds the omitted ones to undefined.
	omitted := trailingOptional(inner, outer)
	if omitted < 0 {
		return nil
	}
	tagged := omitted > 0
	for i := range outer.Params {
		tagged = tagged || inner.Params[i].Type.Kind == hir.Dynamic || outer.Params[i].Type.Kind == hir.Dynamic
	}
	tagged = tagged || inner.Result.Kind == hir.Dynamic || outer.Result.Kind == hir.Dynamic
	if !tagged {
		return nil
	}
	l.serial++
	name := "closure.adapt." + itoa(l.serial)
	self := &hir.Expr{Kind: hir.This, Type: hir.Ref(name)}
	field := &hir.Expr{Kind: hir.FieldGet, Name: "inner", Type: x.Type, X: self}
	saved := l.pend
	l.pend = nil
	var args []*hir.Expr
	var params []hir.Param
	for i, p := range outer.Params {
		params = append(params, hir.Param{Name: "arg" + itoa(i), Type: p.Type})
		a := l.coerce(hir.V("arg"+itoa(i), p.Type), inner.Params[i].Type)
		if a == nil || !a.Type.Equal(inner.Params[i].Type) {
			l.pend = saved
			return nil
		}
		args = append(args, a)
	}
	for _, p := range inner.Params[len(outer.Params):] {
		args = append(args, &hir.Expr{Kind: hir.Lit, Type: p.Type})
	}
	call := &hir.Expr{Kind: hir.VirtualCall, Name: inner.Name, Type: inner.Result, X: field, Args: args}
	var body *hir.Stmt
	if outer.Result.Kind == hir.Void {
		body = hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: call})
	} else if inner.Result.Kind == hir.Void && outer.Result.Kind == hir.Dynamic {
		// A function without a return value yields undefined.
		body = hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: call}, &hir.Stmt{Kind: hir.Return, X: l.rtOp("dynamic.of", &hir.Expr{Kind: hir.Lit, Type: hir.T(hir.Optional, hir.T(hir.Dynamic))}, hir.T(hir.Dynamic))})
	} else {
		result := l.coerce(call, outer.Result)
		if result == nil || !result.Type.Equal(outer.Result) {
			l.pend = saved
			return nil
		}
		body = hir.B(&hir.Stmt{Kind: hir.Return, X: result})
	}
	if len(l.pend) > 0 {
		l.pend = saved
		return nil
	}
	l.pend = saved
	c := &hir.Class{Node: l.node(n), Name: name, Implements: []string{dst.Name},
		Fields: []hir.Field{{Name: "inner", Type: x.Type}}}
	c.Ctor = &hir.Method{Name: "constructor", Result: hir.T(hir.Void), Params: []hir.Param{{Name: "inner", Type: x.Type}},
		Body: hir.B(&hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: field, Y: hir.V("inner", x.Type)})}
	body.Node = l.node(n)
	for _, st := range body.List {
		st.Node = l.node(n)
	}
	c.Methods = []*hir.Method{{Name: outer.Name, Params: params, Result: outer.Result, Body: body}}
	l.out.Classes = append(l.out.Classes, c)
	l.diagf(n, "note-closure-adapter", "function value converted between tagged, typed or shorter signatures")
	return &hir.Expr{Kind: hir.Cast, Node: l.node(n), Type: dst, X: &hir.Expr{Kind: hir.New, Node: l.node(n), Type: hir.Ref(name), Args: []*hir.Expr{x}}}
}
