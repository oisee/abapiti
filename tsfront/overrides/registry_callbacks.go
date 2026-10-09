package overrides

import "github.com/oisee/abapiti/hir"

// Reached arrays are dense; callbacks only read them or replace current entries.
// Stack/result pushes target other arrays. Membership cannot change while iterating.
func registryDenseCallbacks() []Entry {
	return []Entry{
		{ID: "abaplint-dense-callback-TypeUtils.listAllInterfaces", Key: Key{"src/abap/5_syntax/_type_utils.ts", "TypeUtils.listAllInterfaces", "KindMethodDeclaration"}, SHA256: "333459b8eabcdf4380d6478559d29ae57cb49951189f03e96d19cdbd79ce8101", Rationale: "source-pinned dense arrays and stable membership: inline callback only reads receiver or replaces current slot; pushes affect a distinct stack/result; generalise later", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"cdef.getImplementing().forEach(i => stack.push(i.name))":                               true,
			"this.scope.findClassDefinition(s)?.getImplementing().forEach(i => stack.push(i.name))": true,
			"idef?.getImplementing().forEach(i => stack.push(i.name))":                              true,
		}}},
		{ID: "abaplint-dense-callback-Select.buildStructureType", Key: Key{"src/abap/5_syntax/expressions/select.ts", "Select.buildStructureType", "KindMethodDeclaration"}, SHA256: "ea16adcf1ab908a9489d8f2f6fe789f36abb6678a48b4d70d4af017074a09c0e", Rationale: "source-pinned dense arrays and stable membership: inline callback only reads receiver or replaces current slot; pushes affect a distinct stack/result; generalise later", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"fields.every(f => isSimple.test(f.code))": true,
		}}},
		{ID: "abaplint-dense-callback-Select.buildTableType", Key: Key{"src/abap/5_syntax/expressions/select.ts", "Select.buildTableType", "KindMethodDeclaration"}, SHA256: "302e6a0988b10bb15205c9023170d621d32285d061edc7f820a6681ada3d0587", Rationale: "source-pinned dense arrays and stable membership: inline callback only reads receiver or replaces current slot; pushes affect a distinct stack/result; generalise later", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"fields.every(f => isSimple.test(f.code))": true,
		}}},
		{ID: "abaplint-dense-callback-SQLIn.isRangeRow", Key: Key{"src/abap/5_syntax/expressions/sql_in.ts", "SQLIn.isRangeRow", "KindMethodDeclaration"}, SHA256: "bb78a13d6a6cb6d16d9b78183a145534af79816b79107217c7ebafc732d754f4", Rationale: "source-pinned dense arrays and stable membership: inline callback only reads receiver or replaces current slot; pushes affect a distinct stack/result; generalise later", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"RANGE_COMPONENTS.every(c => rowType.getComponentByName(c) !== undefined)": true,
		}}},
		{ID: "abaplint-dense-callback-Loop.runSyntax", Key: Key{"src/abap/5_syntax/statements/loop.ts", "Loop.runSyntax", "KindMethodDeclaration"}, SHA256: "1c9442d9091603c3f3c4fde89977987c1861a1c4c98669e5066f7a9c0763783c", Rationale: "source-pinned dense arrays and stable membership: inline callback only reads receiver or replaces current slot; pushes affect a distinct stack/result; generalise later", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"compares.find(c => c === keyField.toUpperCase() + \" IS INITIAL\")": true,
		}}},
		{ID: "abaplint-dense-callback-ABAPFile.getTokens", Key: Key{"src/abap/abap_file.ts", "ABAPFile.getTokens", "KindMethodDeclaration"}, SHA256: "fdc92007edfeaa19910f8feaa223346f0f7918a8c8d5c231af513ced1a88086c", Rationale: "source-pinned dense arrays and stable membership: inline callback only reads receiver or replaces current slot; pushes affect a distinct stack/result; generalise later", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"this.tokens.forEach((t) => {\n        if (!(t instanceof Pragma)) {\n          tokens.push(t);\n        }\n      })": true,
		}}},
		{ID: "abaplint-dense-callback-CheckSyntax.run", Key: Key{"src/rules/check_syntax.ts", "CheckSyntax.run", "KindMethodDeclaration"}, SHA256: "9ea15ab12a5c560536dfebd0b32a6af8f2a6f827f3b57668732e9d161be751d6", Rationale: "source-pinned dense arrays and stable membership: inline callback only reads receiver or replaces current slot; pushes affect a distinct stack/result; generalise later", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"issues.forEach((value: Issue, index: number) => {\n        const data = value.getData();\n        data.severity = this.conf.severity!;\n        issues[index] = new Issue(data);\n      })": true,
		}}},
		{ID: "abaplint-dense-callback-ImplementMethods.findInterfaceMethods", Key: Key{"src/rules/implement_methods.ts", "ImplementMethods.findInterfaceMethods", "KindMethodDeclaration"}, SHA256: "0ec78c773de189e9727adb08e7ce66b15f807764b56f1aa9661521a15c14b746", Rationale: "source-pinned dense arrays and stable membership: inline callback only reads receiver or replaces current slot; pushes affect a distinct stack/result; generalise later", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"sup.methods.forEach(m => {\n          methods.push({objectName: sup.name, method: m});\n        })": true,
		}}},
		{ID: "abaplint-dense-callback-Catch.runSyntax", Key: Key{"src/abap/5_syntax/statements/catch.ts", "Catch.runSyntax", "KindMethodDeclaration"}, SHA256: "4f8f21e98eeeae98acd365c4d6892e14e5209123787172b31c288c6a3882bf31", Rationale: "classNames is Array.from of a Set<string>: dense and not mutated; the callback only reads the scope, so an absent result means no element matched", Patterns: &Patterns{DenseCallbacks: map[string]bool{
			"classNames.find(name => input.scope.findClassDefinition(name) === undefined)": true,
		}}},
		{ID: "abaplint-select-single-full-key-default-conf", Key: Key{"src/rules/select_single_full_key.ts", "SelectSingleFullKey.getConfig", "KindMethodDeclaration"}, SHA256: "b86b0823577309d8039eb10ba09ba8a187bfd82e8c5930ff869cbeb99080da32", Rationale: "the default literal {allowPseudo: true} equals a fresh SelectSingleFullKeyConf: exclude is read from the registry config, and an undefined severity defaults to Error in Issue.atRange", Patterns: &Patterns{Expressions: map[string]func() *hir.Expr{
			"{\n        allowPseudo: true,\n      }": func() *hir.Expr {
				return &hir.Expr{Kind: hir.New, Type: hir.Ref("src/rules/select_single_full_key.ts.SelectSingleFullKeyConf")}
			},
		}}},
		{ID: "abaplint-artifacts-rules-filter", Key: Key{"src/artifacts_rules.ts", "ArtifactsRules.getRules", "KindMethodDeclaration"}, SHA256: "63be4ae807c921245dc2dbdc12ecf569e25639e29ee180013b910335db5b2b44", Rationale: "Rules exports rule classes and their configuration classes; JavaScript constructs each export and keeps those with getMetadata (the IRule implementations). ABAP cannot hold a configuration object in an IRule variable, so each export is constructed as an object and kept when it implements IRule; same order, same members", Patterns: &Patterns{Statements: map[string]func() *hir.Stmt{
			"for (const key in Rules) {\n      if (typeof list[key] === \"function\") {\n        const rule: IRule = new list[key]();\n// note that configuration is also exported from rules\n        if (rule.getMetadata !== undefined) {\n          ret.push(rule);\n        }\n      }\n    }": artifactsRulesLoop,
		}}},
		{ID: "abaplint-checked-cast-UnknownTypes.traverse", Key: Key{"src/rules/unknown_types.ts", "UnknownTypes.traverse", "KindMethodDeclaration"}, SHA256: "1839f90560822532c74d59d0cb24ca5a9fd014f7f6f6406bc13270e39f45e0f8", Rationale: "the assertion sits under `r.resolved.getType() instanceof UnknownType` in the same condition; TypedIdentifier.getType is a plain field read with no override, so the downcast is proven and ?= never raises", Patterns: &Patterns{CheckedCasts: map[string]bool{
			"r.resolved.getType() as UnknownType": true,
		}}},
	}
}

func artifactsRulesLoop() *hir.Stmt {
	irule := hir.Type{Kind: hir.InterfaceRef, Name: "src/rules/_irule.ts.IRule"}
	rules := hir.T(hir.Array, irule)
	ns := hir.T(hir.OrderedMap, hir.T(hir.String), hir.T(hir.ClassValue))
	nsRef := func() *hir.Expr {
		return &hir.Expr{Kind: hir.StaticGet, Owner: "src/rules/index.ts.module", Name: "ns", Type: ns}
	}
	str := hir.T(hir.String)
	object := hir.Ref(hir.RootObject)
	value := &hir.Expr{Kind: hir.Narrow, Type: hir.T(hir.ClassValue), X: &hir.Expr{Kind: hir.RuntimeOp, Op: "map.get", Type: hir.T(hir.Optional, hir.T(hir.ClassValue)), X: nsRef(), Args: []*hir.Expr{hir.V("key", str)}}}
	built := &hir.Expr{Kind: hir.RuntimeOp, Op: "classvalue.new", Type: object, X: value}
	keep := &hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.InstanceOf, Type: hir.T(hir.Bool), Owner: irule.Name, X: hir.V("candidate", object)},
		Body: hir.B(&hir.Stmt{Kind: hir.ExprStmt, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "array.push", Type: hir.T(hir.I32), X: hir.V("ret", rules), Args: []*hir.Expr{{Kind: hir.Cast, Type: irule, X: hir.V("candidate", object)}}}})}
	return &hir.Stmt{Kind: hir.ForEach, Name: "key", Type: str, X: &hir.Expr{Kind: hir.RuntimeOp, Op: "map.keys", Type: hir.T(hir.Array, str), X: nsRef()},
		Body: hir.B(&hir.Stmt{Kind: hir.VarDecl, Name: "candidate", Type: object, X: built}, keep)}
}
