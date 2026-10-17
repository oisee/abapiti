package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
)

// A readonly JS parameter may observe a scalar array through tagged elements.
// Keep its storage and identity: copying would hide writes through other aliases.
// Mutable parameters retain the existing refusal because they could store a
// non-string value into the native string array.
func (l *lowerer) readonlyArrayArgument(x *hir.Expr, target hir.Type, param *ast.Symbol) *hir.Expr {
	if x.Type.Kind != hir.Array || x.Type.Args[0].Kind != hir.String || param == nil || param.ValueDeclaration == nil || !ast.IsInJSFile(param.ValueDeclaration) {
		return x
	}
	if target.Kind == hir.Optional {
		target = target.Args[0]
	}
	if target.Kind != hir.Array || target.Args[0].Kind != hir.Dynamic {
		return x
	}
	// The checker identifies the global array even when ambient Node typings
	// augment it. A declaration-provenance check would reject those typings.
	typ := l.ck.GetNonNullableType(l.ck.GetTypeOfSymbol(param))
	if typ.Symbol() == nil || typ.Symbol().Name != "ReadonlyArray" || !l.ck.IsArrayType(typ) {
		return x
	}
	return l.rtOp("dynamic.arrayView", l.coerce(x, hir.T(hir.Dynamic)), target)
}
