package overrides

import "github.com/oisee/abapiti/hir"

const artifactsOwner = "src/artifacts_objects.ts.ArtifactsObjects"

func artifactsMap() *hir.Expr {
	return &hir.Expr{Kind: hir.StaticGet, Owner: artifactsOwner, Name: "objectMap", Type: hir.T(hir.Dynamic)}
}
func constructorArgs(value *hir.Expr) *hir.Expr {
	t := hir.T(hir.Array, hir.T(hir.Dynamic))
	a := hir.V("constructorArgs", t)
	return &hir.Expr{Kind: hir.Seq, Type: t, Stmt: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: a.Name, Type: t, X: &hir.Expr{Kind: hir.New, Type: t}}, &hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: hir.T(hir.I32), X: a, Args: []*hir.Expr{{Kind: hir.RuntimeOp, Op: "dynamic.of", Type: hir.T(hir.Dynamic), X: value}}}}), Y: a}
}
func artifactsBuildObjectMap() *hir.Method {
	str := hir.T(hir.String)
	cv := hir.T(hir.ClassValue)
	typ := hir.T(hir.OrderedMap, str, cv)
	namespace := &hir.Expr{Kind: hir.StaticGet, Owner: "src/objects/index.ts.module", Name: "ns", Type: typ}
	values := hir.V("inventory", typ)
	key := hir.V("key", str)
	candidate := hir.V("candidate", hir.Type{Kind: hir.InterfaceRef, Name: "src/objects/_iobject.ts.IObject"})
	descriptor := &hir.Expr{Kind: hir.Narrow, Type: cv, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "map.get", Type: hir.T(hir.Optional, cv), X: namespace, Args: []*hir.Expr{key}}}
	return &hir.Method{Name: "buildObjectMap", Static: true, Result: hir.T(hir.Void), Body: hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Name: values.Name, Type: typ, X: &hir.Expr{Kind: hir.New, Type: typ}},
		&hir.Stmt{Kind: hir.Assign, X: artifactsMap(), Y: &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.of", Type: hir.T(hir.Dynamic), X: values}},
		&hir.Stmt{Kind: hir.ForEach, Name: key.Name, Type: str, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "map.keys", Type: hir.T(hir.Array, str), X: namespace}, Body: hir.B(
			&hir.Stmt{Kind: hir.VarDecl, Name: candidate.Name, Type: candidate.Type, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "classvalue.newArgs", Type: candidate.Type, X: descriptor, Args: []*hir.Expr{constructorArgs(hir.L(str, "DUMMY_NAME"))}}},
			&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "map.set", Type: typ, X: values, Args: []*hir.Expr{{Kind: hir.VirtualCall, Name: "getType", Type: str, X: candidate}, descriptor}}},
		)},
	)}
}
func artifactsNewObject() *hir.Method {
	str := hir.T(hir.String)
	result := hir.Type{Kind: hir.InterfaceRef, Name: "src/objects/_iobject.ts.IObject"}
	name := hir.V("name", str)
	typ := hir.V("type", str)
	found := hir.V("found", hir.T(hir.Dynamic))
	var body []*hir.Stmt
	body = append(body, &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.isNullish", Type: hir.T(hir.Bool), X: artifactsMap()}, Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.DirectCall, Owner: artifactsOwner, Name: "buildObjectMap", Type: hir.T(hir.Void)}})})
	// The source stores constructors on an Array. Inherited Array/Object members
	// are not absent keys. Those non-constructor domains must refuse, not produce
	// UnknownObject; actual prototype mutation is outside the pinned source.
	for _, key := range []string{"__proto__", "constructor", "length", "toString", "toLocaleString", "valueOf", "hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable", "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__", "at", "concat", "copyWithin", "entries", "every", "fill", "filter", "find", "findIndex", "findLast", "findLastIndex", "flat", "flatMap", "forEach", "includes", "indexOf", "join", "keys", "lastIndexOf", "map", "pop", "push", "reduce", "reduceRight", "reverse", "shift", "slice", "some", "sort", "splice", "toReversed", "toSorted", "toSpliced", "unshift", "values", "with"} {
		body = append(body, &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Binary, Op: "==", Type: hir.T(hir.Bool), X: typ, Y: hir.L(str, key)}, Body: hir.B(&hir.Stmt{Kind: hir.Trap, Name: "Array prototype constructor lookup is unsupported"})})
	}
	body = append(body, &hir.Stmt{Kind: hir.VarDecl, Name: found.Name, Type: found.Type, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.get", Type: found.Type, X: artifactsMap(), Args: []*hir.Expr{typ}}},
		&hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.isNullish", Type: hir.T(hir.Bool), X: found}, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.Cast, Type: result, X: &hir.Expr{Kind: hir.New, Type: hir.Ref("src/objects/_unknown_object.ts.UnknownObject"), Args: []*hir.Expr{name, typ}}}})},
		&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "classvalue.newArgs", Type: result, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.asClassValue", Type: hir.T(hir.ClassValue), X: found}, Args: []*hir.Expr{constructorArgs(name)}}})
	return &hir.Method{Name: "newObject", Static: true, Result: result, Params: []hir.Param{{Name: "name", Type: str}, {Name: "type", Type: str}}, Body: hir.B(body...)}
}
