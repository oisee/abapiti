package overrides

import "github.com/oisee/abapiti/hir"

// The workload's consumers finish enumeration before any membership mutation.
// These source-pinned replacements are not a general Generator/Iterable ABI.
func snapshotMap(receiver *hir.Expr, element hir.Type, out *hir.Expr, keyName string) *hir.Stmt {
	key := hir.V(keyName, hir.T(hir.String))
	optional := hir.T(hir.Optional, element)
	value := &hir.Expr{Kind: hir.RuntimeOp, Op: "map.get", Type: optional, X: receiver, Args: []*hir.Expr{key}}
	return &hir.Stmt{Kind: hir.ForEach, Name: keyName, Type: key.Type,
		X:    &hir.Expr{Kind: hir.RuntimeOp, Op: "map.keys", Type: hir.T(hir.Array, key.Type), X: receiver},
		Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: hir.T(hir.I32), X: out, Args: []*hir.Expr{{Kind: hir.Narrow, Type: element, X: value}}}})}
}

func ObjectSnapshotMethod(owner string, element hir.Type) *hir.Method {
	mapType := hir.T(hir.OrderedMap, hir.T(hir.String), element)
	objects := &hir.Expr{Kind: hir.FieldGet, Name: "objects", Type: hir.T(hir.OrderedMap, hir.T(hir.String), mapType), X: &hir.Expr{Kind: hir.This, Type: hir.Ref(owner)}}
	outType := hir.T(hir.Array, element)
	out := hir.V("snapshot", outType)
	name := hir.V("objectName", hir.T(hir.String))
	inner := hir.V("objectTypes", mapType)
	return &hir.Method{Name: "getObjects", Virtual: true, Result: outType, Body: hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Name: out.Name, Type: outType, X: &hir.Expr{Kind: hir.New, Type: outType}},
		&hir.Stmt{Kind: hir.ForEach, Name: name.Name, Type: name.Type, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "map.keys", Type: hir.T(hir.Array, name.Type), X: objects}, Body: hir.B(
			&hir.Stmt{Kind: hir.VarDecl, Name: inner.Name, Type: mapType, X: &hir.Expr{Kind: hir.Narrow, Type: mapType, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "map.get", Type: hir.T(hir.Optional, mapType), X: objects, Args: []*hir.Expr{name}}}},
			snapshotMap(inner, element, out, "objectType"),
		)},
		&hir.Stmt{Kind: hir.Return, X: out},
	)}
}
func ObjectTypeSnapshotMethod(owner string, element hir.Type) *hir.Method {
	mapType := hir.T(hir.OrderedMap, hir.T(hir.String), element)
	objects := &hir.Expr{Kind: hir.FieldGet, Name: "objectsByType", Type: hir.T(hir.OrderedMap, hir.T(hir.String), mapType), X: &hir.Expr{Kind: hir.This, Type: hir.Ref(owner)}}
	optional := hir.T(hir.Optional, mapType)
	selected := hir.V("selected", optional)
	outType := hir.T(hir.Array, element)
	out := hir.V("snapshot", outType)
	return &hir.Method{Name: "getObjectsByType", Virtual: true, Result: outType, Params: []hir.Param{{Name: "type", Type: hir.T(hir.String)}}, Body: hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Name: out.Name, Type: outType, X: &hir.Expr{Kind: hir.New, Type: outType}},
		&hir.Stmt{Kind: hir.VarDecl, Name: selected.Name, Type: optional, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "map.get", Type: optional, X: objects, Args: []*hir.Expr{hir.V("type", hir.T(hir.String))}}},
		&hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: selected}, Body: hir.B(&hir.Stmt{Kind: hir.Return, X: out})},
		snapshotMap(&hir.Expr{Kind: hir.Narrow, Type: mapType, X: selected}, element, out, "objectName"),
		&hir.Stmt{Kind: hir.Return, X: out},
	)}
}
func DefinitionSnapshotMethod(owner string, element hir.Type) *hir.Method {
	all := &hir.Expr{Kind: hir.FieldGet, Name: "all", Type: hir.T(hir.OrderedMap, hir.T(hir.String), element), X: &hir.Expr{Kind: hir.This, Type: hir.Ref(owner)}}
	outType := hir.T(hir.Array, element)
	out := hir.V("snapshot", outType)
	return &hir.Method{Name: "getAll", Virtual: true, Result: outType, Body: hir.B(
		&hir.Stmt{Kind: hir.VarDecl, Name: out.Name, Type: outType, X: &hir.Expr{Kind: hir.New, Type: outType}},
		snapshotMap(all, element, out, "definitionName"),
		&hir.Stmt{Kind: hir.Return, X: out},
	)}
}

func registryIterators() []Entry {
	object := hir.Type{Kind: hir.InterfaceRef, Name: "src/objects/_iobject.ts.IObject"}
	file := hir.Type{Kind: hir.InterfaceRef, Name: "src/files/_ifile.ts.IFile"}
	definition := hir.Type{Kind: hir.InterfaceRef, Name: "src/abap/types/_method_definition.ts.IMethodDefinition"}
	reason := "Addendum 4: immediate enumeration without membership mutation in reached consumers; snapshot of ordered keys, same objects; generalise later"
	entries := []Entry{
		{ID: "abaplint-registry-object-snapshot", Key: Key{"src/registry.ts", "Registry.getObjects", "KindMethodDeclaration"}, SHA256: "acb11334f5c1da1e7e582da919a7abb352e116292417fee9bae3040b0a193d6c", Rationale: reason, Method: func() *hir.Method { return ObjectSnapshotMethod("src/registry.ts.Registry", object) }},
		{ID: "abaplint-registry-type-snapshot", Key: Key{"src/registry.ts", "Registry.getObjectsByType", "KindMethodDeclaration"}, SHA256: "e3e4ad206d0585c7629fd59a40025a36d05c9a9e4727b6920677b99a608bf442", Rationale: reason, Method: func() *hir.Method { return ObjectTypeSnapshotMethod("src/registry.ts.Registry", object) }},
		{ID: "abaplint-registry-files-generator-trap", Key: Key{"src/registry.ts", "Registry.getFiles", "KindMethodDeclaration"}, SHA256: "6dd23921294696b058b2c2882d9d81431c5ee16095c18ac0b6db31a09d3c6d6b", Rationale: "unreached getFiles retains located trap; opaque array-shaped signature only", Method: func() *hir.Method {
			return &hir.Method{Name: "getFiles", Virtual: true, Result: hir.T(hir.Array, file), Body: hir.B(&hir.Stmt{Kind: hir.Trap, Name: "src/registry.ts:136"})}
		}},
		{ID: "abaplint-method-definition-snapshot", Key: Key{"src/abap/types/method_definitions.ts", "MethodDefinitions.getAll", "KindMethodDeclaration"}, SHA256: "2d0c85819374caded649e0fb1f3f1c7ddad2ace261d2d599bd7bfe4122f0b68e", Rationale: "private all map is populated during construction; reached checks only read membership; generalise later", Method: func() *hir.Method {
			return DefinitionSnapshotMethod("src/abap/types/method_definitions.ts.MethodDefinitions", definition)
		}},
		{ID: "abaplint-rules-object-snapshot-parameter", Key: Key{"src/rules_runner.ts", "RulesRunner.objectsToCheck", "KindMethodDeclaration"}, SHA256: "4d4956e192c842b9d32536981497005dabc9074713c5434c55205aba7f4b6df0", Rationale: reason, Patterns: &Patterns{Annotations: map[string]hir.Type{"Iterable<IObject>": hir.T(hir.Array, object)}}},
	}
	for _, spec := range []struct {
		id, file, symbol, sha string
		element               hir.Type
	}{
		{"abaplint-registry-object-snapshot-signature", "src/_iregistry.ts", "IRegistry.getObjects", "a4089f370302ffd9fbc98ae88641acbef1022700d22d9f98ba4a9522572d7432", object},
		{"abaplint-registry-type-snapshot-signature", "src/_iregistry.ts", "IRegistry.getObjectsByType", "7356342567ff9ca4e1511cc5ffc925ca14fc9cc009b01985bac08fb3dce029dc", object},
		{"abaplint-registry-file-trap-signature", "src/_iregistry.ts", "IRegistry.getFiles", "5af33fff3f50a013bd61e25ac02822d24d4bd32bffbc8c266c25d2a82ebb7578", file},
		{"abaplint-method-definition-snapshot-signature", "src/abap/types/_method_definitions.ts", "IMethodDefinitions.getAll", "86d34082919fe853416f0e26dedcd32181595f37e2699593089887a163f3ef2e", definition},
	} {
		entries = append(entries, Entry{ID: spec.id, Key: Key{spec.file, spec.symbol, "KindMethodSignature"}, SHA256: spec.sha, Rationale: reason, Result: func() hir.Type { return hir.T(hir.Array, spec.element) }})
	}
	return entries
}
