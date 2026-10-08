package overrides

import "github.com/oisee/abapiti/hir"

// Syntax holds the fingerprinted overrides for the abaplint syntax closure
// (abap/5_syntax, abap/types, abap/4_file_information). Each entry names
// the exact TypeScript span it replaces and why the replacement is
// equivalent for the deployment; a changed span fails loudly.
func Syntax() []Entry {
	typeData := hir.T(hir.Optional, hir.Ref("src/abap/types/basic/_abstract_type.ts.AbstractTypeData"))
	singleton := func(id, file, class, sha string) Entry {
		ref := hir.Ref(file + "." + class)
		return Entry{ID: id, Key: Key{file, class + ".get", "KindMethodDeclaration"}, SHA256: sha, Rationale: "JSON.stringify cache key is not lowered; a fresh instance per call carries the same data (identity-only divergence, never compared)", Method: func() *hir.Method {
			return &hir.Method{Name: "get", Static: true, Result: ref, Params: []hir.Param{{Name: "input", Type: typeData}},
				Body: hir.B(&hir.Stmt{Kind: hir.Return, X: &hir.Expr{Kind: hir.New, Type: ref, Args: []*hir.Expr{hir.V("input", typeData)}}})}
		}}
	}
	return []Entry{
		{ID: "abaplint-builtin-char-bounds", Key: Key{"src/abap/5_syntax/_builtin.ts", "BuiltIn.get", "KindMethodDeclaration"}, SHA256: "c5d99e60fe3eca46802e67938e7b6879c2db7f6e00319a9c4d2d62b814ff1cc9", Rationale: "Node Buffer hex decoding of FDFF/0000 as UTF-8 is the constant text U+FFFD U+FFFD / U+0000 U+0000", Expressions: map[string]func() *hir.Expr{
			`Buffer.from("FDFF", "hex").toString()`: func() *hir.Expr { return hir.L(hir.T(hir.String), "��") },
			`Buffer.from("0000", "hex").toString()`: func() *hir.Expr { return hir.L(hir.T(hir.String), "\u0000\u0000") },
		}},
		{ID: "abaplint-builtin-method-table", Key: Key{"src/abap/5_syntax/_builtin.ts", "BuiltIn.methods", "KindPropertyDeclaration"}, SHA256: "5ce13c9e09e5db77c4f0b63ee76c4490625e6847424fec9b777b2e149a9594c9", Rationale: "the table literal only increments BuiltIn.counter (same class, no other reader) and constructs type objects; lazy first-use initialization yields the same table", Assume: "pure-static-initializer"},
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
		// Tech debt (fable, registry HIR gate): declared types where the
		// checker infers an untyped collection or an anonymous shape.
		{ID: "abaplint-indent-global-classes", Key: Key{"src/pretty_printer/indent.ts", "Indent", "KindClassDeclaration"}, SHA256: "74e398937b3cc07c2d189c2cc8ea0d8d70def8c8f4caf89fbb3c279658214fca", Rationale: "`new Set()` infers Set<unknown>; the only members added and tested are upper-cased class names (strings)", Types: map[string]hir.Type{"globalClasses": hir.T(hir.OrderedSet, hir.T(hir.String))}},
		{ID: "abaplint-method-source-find-top-children", Key: Key{"src/abap/5_syntax/expressions/method_source.ts", "MethodSource", "KindClassDeclaration"}, SHA256: "6781f3039cf6c46c519d9c795245e8cae0e74f53a4dbfe651136fab35300aa52", Rationale: "findTop's `children: any[]` only receives node.getChildren().slice() (INode[]); an untyped array cannot alias a typed one", Types: map[string]hir.Type{"findTop.children": hir.T(hir.Array, hir.Type{Kind: hir.InterfaceRef, Name: "src/abap/nodes/_inode.ts.INode"})}},
		{ID: "abaplint-constants-values-bag", Key: Key{"src/abap/5_syntax/structures/constants.ts", "Constants.runSyntax", "KindMethodDeclaration"}, SHA256: "291b46b7e3bf1f24988dcbd5f181ba2edd25bf9ef33b8fd32a0f56db2f369e85", Rationale: "`values: any = {}` is a property bag of constant values whose entries are strings or nested bags (BEGIN OF); the declared {[index: string]: string} understates it and consumers take it as any", Patterns: &Patterns{Annotations: map[string]hir.Type{"any": hir.T(hir.OrderedMap, hir.T(hir.String), hir.T(hir.Dynamic)), "{[index: string]: string}": hir.T(hir.OrderedMap, hir.T(hir.String), hir.T(hir.Dynamic))}}},
		{ID: "abaplint-abap-parser-is-release", Key: Key{"src/abap/abap_parser.ts", "ABAPParser.isRelease", "KindMethodDeclaration"}, SHA256: "203de4498e2e57a9131b587ef71572290209d78dbd720ab88b4f60b408ce9cbb", Rationale: "`typeof (o as ABAPRelease).ordinal === \"number\"` discriminates the data classes ABAPRelease and IABAPParserOptions (no ordinal); the lowered constituents are nominal, so the test is the class membership", Patterns: &Patterns{Expressions: map[string]func() *hir.Expr{"typeof (o as ABAPRelease).ordinal === \"number\"": func() *hir.Expr {
			return &hir.Expr{Kind: hir.InstanceOf, Type: hir.T(hir.Bool), Owner: "src/version.ts.ABAPRelease", X: hir.V("o", hir.Type{Kind: hir.InterfaceRef, Name: "union.f3e4e2a38607cc25b56b465ae4ed5aa2a598ccbf33b48e7c32a197934e381726"})}
		}}}},
		{ID: "abaplint-ddic-lookup-ddls-result", Key: Key{"src/ddic.ts", "DDIC.lookupDDLS", "KindMethodDeclaration"}, SHA256: "482a897a928b5ac4eec9cd583f30a918286e0430e4f6292e0ed04104d1e31207", Rationale: "the inferred anonymous {type, object} result is structurally ILookupResult, which every caller returns; HIR shapes are nominal", Result: func() hir.Type { return hir.T(hir.Optional, hir.Ref("src/ddic.ts.ILookupResult")) }},
	}
}
