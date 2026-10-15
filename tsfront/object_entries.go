package tsfront

import (
	"github.com/oisee/abapiti/hir"
	"github.com/oisee/abapiti/internal/tsgo/ast"
	"github.com/oisee/abapiti/internal/tsgo/checker"
	"github.com/oisee/abapiti/tsfront/overrides"
)

// Object.entries is a snapshot of enumerable own record properties. Ordered
// maps also represent JS Map, so require the checker's string index signature
// rather than treating every OrderedMap as a record. Accessors and prototypes
// are outside this existing record model.
func (l *lowerer) objectEntries(n *ast.Node) *hir.Expr {
	args := n.Arguments()
	if len(args) != 1 {
		l.diagf(n, "unsupported-call", "Object.entries requires one string-indexed record")
		return nil
	}
	typ := l.ck.GetTypeAtLocation(args[0])
	if typ != nil && typ.IsUnion() {
		for _, part := range typ.AsUnionOrIntersectionType().Types() {
			if part.Flags()&(checker.TypeFlagsUndefined|checker.TypeFlagsNull) == 0 {
				typ = part
				break
			}
		}
	}
	if typ == nil {
		return nil
	}
	infos := l.ck.GetIndexInfosOfType(typ)
	if l.unprunedFeatures && len(infos) == 0 && typ.Flags()&(checker.TypeFlagsObject|checker.TypeFlagsAny) != 0 {
		recv := l.expr(args[0])
		if recv != nil && (recv.Type.Kind == hir.ClassRef || recv.Type.Kind == hir.Dynamic) {
			return l.viewEntries(n, recv)
		}
		l.diagf(n, "unsupported-call", "Object.entries receiver lacks structural own-property storage")
		return nil
	}
	if len(infos) != 1 || !infos[0].KeyType().IsStringLike() {
		l.diagf(n, "unsupported-call", "Object.entries requires a string-indexed record; nominal objects and JS Map need own/enumerable property metadata")
		return nil
	}
	recv := l.expr(args[0])
	if recv == nil {
		return nil
	}
	recv = l.presentValue(args[0], recv, hir.Type{})
	if l.unprunedFeatures && recv.Type.Kind == hir.Dynamic {
		return l.viewEntries(n, recv)
	}
	if recv.Type.Kind != hir.OrderedMap || recv.Type.Args[0].Kind != hir.String {
		l.diagf(n, "unsupported-call", "Object.entries receiver is not a lowered string-indexed record")
		return nil
	}
	if recv.Type.Args[1].Kind == hir.Optional {
		l.diagf(n, "unsupported-call", "Object.entries optional record values need a presence-preserving nested optional ABI")
		return nil
	}
	recv = l.tempInit(n, recv.Type, recv)
	keys := l.recordPropertyKeys(n, recv)
	str, i := hir.T(hir.String), hir.T(hir.I32)
	valueType := recv.Type.Args[1]
	pair := l.tupleShape(n, []hir.Type{str, valueType})
	result := l.tempVar(n, hir.T(hir.Array, pair))
	l.serial++
	key := hir.V("entryKey"+itoa(l.serial), str)
	value := &hir.Expr{Kind: hir.Narrow, Node: l.node(n), Type: valueType, X: l.rtOp("map.get", recv, hir.T(hir.Optional, valueType), key)}
	row := &hir.Expr{Kind: hir.New, Node: l.node(n), Type: pair, Args: []*hir.Expr{key, value}}
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: key.Name, Type: str, X: keys, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", result, i, row)})})
	return result
}

// ECMAScript puts canonical array-index keys (0..2^32-2) first, numerically,
// then other string keys in insertion order. Parsing at most ten ASCII digits
// avoids Number overflow and rounding. Digit-only, equal-length keys can use
// the existing exact registry-key comparator's admitted decimal domain.
func (l *lowerer) recordPropertyKeys(n *ast.Node, recv *hir.Expr) *hir.Expr {
	str, i, b := hir.T(hir.String), hir.T(hir.I32), hir.T(hir.Bool)
	keysType := hir.T(hir.Array, str)
	numeric := l.tempVar(n, keysType)
	other := l.tempVar(n, keysType)
	l.serial++
	key := hir.V("propertyKey"+itoa(l.serial), str)
	binary := func(op string, x, y *hir.Expr, typ hir.Type) *hir.Expr {
		return &hir.Expr{Kind: hir.Binary, Node: l.node(n), Op: op, Type: typ, X: x, Y: y}
	}
	length := l.tempInit(n, i, hir.L(i, 0))
	index := l.tempInit(n, i, hir.L(i, 0))
	isIndex := l.tempInit(n, b, hir.L(b, false))
	assign := func(x, y *hir.Expr) *hir.Stmt { return &hir.Stmt{Kind: hir.Assign, Node: l.node(n), X: x, Y: y} }
	lenOK := binary("&&", binary(">", length, hir.L(i, 0), b), binary("<=", length, hir.L(i, 10), b), b)
	noLeadingZero := binary("||", binary("==", length, hir.L(i, 1), b), binary("!=", l.rtOp("string.charAt", key, str, hir.L(i, 0)), hir.L(str, "0"), b), b)
	code := l.tempInit(n, i, hir.L(i, 0))
	digit := binary("&&", binary(">=", code, hir.L(i, 48), b), binary("<=", code, hir.L(i, 57), b), b)
	atLimit := l.rtOp("string.compareRegistryKey", key, i, hir.L(str, "4294967294"))
	loopBody := hir.B(
		assign(length, l.rtOp("string.length", key, i)), assign(index, hir.L(i, 0)), assign(isIndex, binary("&&", lenOK, noLeadingZero, b)),
		&hir.Stmt{Kind: hir.While, Node: l.node(n), X: binary("&&", isIndex, binary("<", index, length, b), b), Body: hir.B(assign(code, l.rtOp("string.charCodeAt", key, i, index)), assign(isIndex, digit), assign(index, binary("+", index, hir.L(i, 1), i)))},
		&hir.Stmt{Kind: hir.If, Node: l.node(n), X: binary("&&", isIndex, binary("==", length, hir.L(i, 10), b), b), Body: hir.B(assign(isIndex, binary("<=", atLimit, hir.L(i, 0), b)))},
		&hir.Stmt{Kind: hir.If, Node: l.node(n), X: isIndex, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", numeric, i, key)}), Else: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", other, i, key)})},
	)
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: key.Name, Type: str, X: l.rtOp("map.keys", recv, keysType), Body: loopBody})
	sorted := overrides.StableProjectedSort(numeric, func(left, right *hir.Expr) *hir.Expr {
		a, c := l.rtOp("string.length", left, i), l.rtOp("string.length", right, i)
		return &hir.Expr{Kind: hir.Conditional, Node: l.node(n), Type: i, X: binary("!=", a, c, b), Y: binary("-", a, c, i), Z: l.rtOp("string.compareRegistryKey", left, i, right)}
	})
	return l.tempInit(n, keysType, l.rtOp("array.concat", sorted, keysType, other))
}

// View entries read each value from the original storage after snapshotting keys.
func (l *lowerer) viewEntries(n *ast.Node, recv *hir.Expr) *hir.Expr {
	recv = l.tempInit(n, hir.T(hir.Dynamic), l.coerce(recv, hir.T(hir.Dynamic)))
	str := hir.T(hir.String)
	pair := l.tupleShape(n, []hir.Type{str, hir.T(hir.Dynamic)})
	result := l.tempVar(n, hir.T(hir.Array, pair))
	l.serial++
	key := hir.V("viewKey"+itoa(l.serial), str)
	row := &hir.Expr{Kind: hir.New, Node: l.node(n), Type: pair, Args: []*hir.Expr{key, l.rtOp("dynamic.get", recv, hir.T(hir.Dynamic), key)}}
	l.pendStmt(&hir.Stmt{Kind: hir.ForEach, Node: l.node(n), Name: key.Name, Type: str, X: l.rtOp("dynamic.keys", recv, hir.T(hir.Array, str)), Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, Node: l.node(n), X: l.rtOp("array.push", result, hir.T(hir.I32), row)})})
	return result
}
