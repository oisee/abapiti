package overrides

import "github.com/oisee/abapiti/hir"

// Syntax holds the fingerprinted overrides for the abaplint syntax closure
// (abap/5_syntax, abap/types, abap/4_file_information). Each entry names
// the exact TypeScript span it replaces and why the replacement is
// equivalent for the deployment; a changed span fails loudly.
func Syntax() []Entry {
	methodDefinition := hir.Type{Kind: hir.InterfaceRef, Name: "src/abap/types/_method_definition.ts.IMethodDefinition"}
	methodDefinitions := hir.Ref("src/abap/types/method_definitions.ts.MethodDefinitions")
	all := hir.T(hir.OrderedMap, hir.T(hir.String), methodDefinition)
	typeData := hir.T(hir.Optional, hir.Ref("src/abap/types/basic/_abstract_type.ts.AbstractTypeData"))
	singleton := func(id, file, class, sha string) Entry {
		ref := hir.Ref(file + "." + class)
		return Entry{ID: id, Key: Key{file, class + ".get", "KindMethodDeclaration"}, SHA256: sha, Rationale: "JSON.stringify cache key is not lowered; a fresh instance per call carries the same data (identity-only divergence, never compared)", Method: func() *hir.Method {
			return &hir.Method{Name: "get", Static: true, Result: ref, Params: []hir.Param{{Name: "input", Type: typeData}},
				Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Type: ref, Args: []*hir.Expr{hir.V("input", typeData)}}})}
		}}
	}
	return []Entry{
		{ID: "abaplint-method-definitions-iterable", Key: Key{"src/abap/types/_method_definitions.ts", "IMethodDefinitions", "KindInterfaceDeclaration"}, SHA256: "4159bdeebae32af25c5560449c668340f34c79561acec5aa3cf0d8615a2b4c56", Rationale: "getAll's generator is consumed only by for-of loops that do not add methods while iterating; an array snapshot of the live entries is equivalent", Interface: func() *hir.Interface {
			return &hir.Interface{Methods: []*hir.Method{
				{Name: "getAll", Virtual: true, Result: hir.T(hir.Array, methodDefinition)},
				{Name: "getByName", Virtual: true, Params: []hir.Param{{Name: "name", Type: hir.T(hir.Optional, hir.T(hir.String))}}, Result: hir.T(hir.Optional, methodDefinition)},
			}}
		}},
		{ID: "abaplint-method-definitions-get-all", Key: Key{"src/abap/types/method_definitions.ts", "MethodDefinitions.getAll", "KindMethodDeclaration"}, SHA256: "2d0c85819374caded649e0fb1f3f1c7ddad2ace261d2d599bd7bfe4122f0b68e", Rationale: "generator over the record's values replaced by a values snapshot in insertion order (consumers only iterate)", Method: func() *hir.Method {
			this := &hir.Expr{Kind: hir.This, Type: methodDefinitions}
			field := &hir.Expr{Kind: hir.FieldGet, Name: "all", Type: all, X: this}
			values := &hir.Expr{Kind: hir.RuntimeOp, Op: "map.values", Type: hir.T(hir.Array, methodDefinition), X: field}
			return &hir.Method{Name: "getAll", Virtual: true, Result: hir.T(hir.Array, methodDefinition), Body: hir.B(&hir.Stmt{Kind: hir.Return, X: values})}
		}},
		{ID: "abaplint-builtin-char-bounds", Key: Key{"src/abap/5_syntax/_builtin.ts", "BuiltIn.get", "KindMethodDeclaration"}, SHA256: "c5d99e60fe3eca46802e67938e7b6879c2db7f6e00319a9c4d2d62b814ff1cc9", Rationale: "Node Buffer hex decoding of FDFF/0000 as UTF-8 is the constant text U+FFFD U+FFFD / U+0000 U+0000", Expressions: map[string]func() *hir.Expr{
			`Buffer.from("FDFF", "hex").toString()`: func() *hir.Expr { return hir.L(hir.T(hir.String), "��") },
			`Buffer.from("0000", "hex").toString()`: func() *hir.Expr { return hir.L(hir.T(hir.String), "\u0000\u0000") },
		}},
		{ID: "abaplint-builtin-method-table", Key: Key{"src/abap/5_syntax/_builtin.ts", "BuiltIn.methods", "KindPropertyDeclaration"}, SHA256: "5ce13c9e09e5db77c4f0b63ee76c4490625e6847424fec9b777b2e149a9594c9", Rationale: "the table literal only increments BuiltIn.counter (same class, no other reader) and constructs type objects; lazy first-use initialization yields the same table", Assume: "pure-static-initializer"},
		{ID: "abaplint-spaghetti-debug-description", Key: Key{"src/abap/5_syntax/spaghetti_scope.ts", "SpaghettiScopeNode", "KindMethodDeclaration"}, SHA256: "ce0f33646b833fe6f265c8cb97d04f41cf4f2c98f9ab5fda786df8630e32a81e", Rationale: "symbol debug inspector is outside syntax execution; invocation traps", Method: func() *hir.Method { return excludedVirtual("debugDescription") }},
		{ID: "abaplint-typed-identifier-debug-description", Key: Key{"src/abap/types/_typed_identifier.ts", "TypedIdentifier", "KindMethodDeclaration"}, SHA256: "36b84b6865b9bab96bddcee198c00676873517b2cfc63f043a286063dfa68337", Rationale: "symbol debug inspector is outside syntax execution; invocation traps", Method: func() *hir.Method { return excludedVirtual("debugDescription") }},
		singleton("abaplint-any-type-singleton", "src/abap/types/basic/any_type.ts", "AnyType", "fbde355b6efd20f751858d55ff48b1af39f9896904c4bcc6db7ed3ce610d0b8a"),
		singleton("abaplint-integer-type-singleton", "src/abap/types/basic/integer_type.ts", "IntegerType", "27e74fe307427d9701a123831097ff9c09516d92cf58da70b5166eed4020f696"),
		singleton("abaplint-string-type-singleton", "src/abap/types/basic/string_type.ts", "StringType", "21b4fd9421c07253aca18f8169321f18eac32b8c05039a15a9e1ce3c82b01ded"),
		singleton("abaplint-xstring-type-singleton", "src/abap/types/basic/xstring_type.ts", "XStringType", "3bf9332f2b4492c5e92b59c17ed4ca7dabec3db3f7eba2ecce03909a1e7350e2"),
		{ID: "abaplint-void-type-singleton", Key: Key{"src/abap/types/basic/void_type.ts", "VoidType.get", "KindMethodDeclaration"}, SHA256: "08434bfae50257df2244dbddcda15123ae1cac4654270f89ceb514d7c8562d51", Rationale: "JSON.stringify cache key is not lowered; a fresh instance per call carries the same data (identity-only divergence, never compared)", Method: func() *hir.Method {
			ref := hir.Ref("src/abap/types/basic/void_type.ts.VoidType")
			opt := hir.T(hir.Optional, hir.T(hir.String))
			return &hir.Method{Name: "get", Static: true, Result: ref, Params: []hir.Param{{Name: "voided", Type: opt}, {Name: "qualifiedName", Type: opt}},
				Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Type: ref, Args: []*hir.Expr{hir.V("voided", opt), hir.V("qualifiedName", opt)}}})}
		}},
	}
}
