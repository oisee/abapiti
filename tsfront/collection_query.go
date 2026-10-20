package tsfront

import "github.com/oisee/abapiti/hir"

func (l *lowerer) collectionIdentityQuery(recv, query *hir.Expr, get bool) *hir.Expr {
	if !l.unprunedFeatures || query == nil {
		return nil
	}
	element := recv.Type.Args[0]
	if element.Equal(hir.Ref(hir.RootObject)) {
		return nil
	}
	if element.Equal(query.Type) || !element.IsRef() || !query.Type.IsRef() {
		return nil
	}
	op, result := "collection.has", hir.T(hir.Bool)
	if get {
		op = "collection.get"
		result = hir.T(hir.Optional, recv.Type.Args[1])
	}
	return l.rtOp(op, recv, result, l.coerce(query, hir.T(hir.Dynamic)))
}
