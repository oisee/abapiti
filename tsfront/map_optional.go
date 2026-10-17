package tsfront

import "github.com/oisee/abapiti/hir"

// mapRead flattens lookup absence with value absence, as JS Map.get and record
// reads do. Membership stays in map.has; no nested optional reaches HIR.
func (l *lowerer) mapRead(recv, key *hir.Expr) *hir.Expr {
	if l.unprunedFeatures && recv.Type.Args[0].Kind == hir.Dynamic {
		key = l.coerce(key, recv.Type.Args[0])
	}
	result := recv.Type.Args[1]
	if result.Kind != hir.Optional {
		result = hir.T(hir.Optional, result)
	}
	return l.rtOp("map.get", recv, result, key)
}
