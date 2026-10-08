package overrides

import "github.com/oisee/abapiti/hir"

// These introspection methods are outside the statement-parser execution
// closure. Preserve a visible failure if a caller reaches an excluded path;
// never return an incomplete registry as though it were the upstream result.
func Abaplint() *Registry {
	entries := structuresOverrides()
	entries = append(entries,
		Entry{ID: "abaplint-registry-input-progress", Key: Key{"src/_iregistry.ts", "IRunInput", "KindInterfaceDeclaration"}, SHA256: "1bcfd155a49444b0afb43f77e4c2c46b5da664a638035f2df8a77fa996d02748", Rationale: "progress callback remains an opaque reference; async registry parsing is outside this closure", Types: map[string]hir.Type{"progress": hir.Ref(hir.RootObject)}},
		Entry{ID: "abaplint-map-input-constructor", Key: Key{"src/abap/2_statements/combi.ts", "mapInput", "KindFunctionDeclaration"}, SHA256: "ae304084bb6ce64c2176d481cbafb04d12414e535cf7ff0008908666f2c8e8f2", Rationale: "the typeof function branch proves the ts-ignore constructor and name accesses", Expressions: map[string]func() *hir.Expr{
			"s.name": func() *hir.Expr {
				cv := &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.asClassValue", Type: hir.T(hir.ClassValue), X: hir.V("s", hir.T(hir.Dynamic))}
				return &hir.Expr{Kind: hir.RuntimeOp, Op: "classvalue.name", Type: hir.T(hir.String), X: cv}
			},
			"new s()": func() *hir.Expr {
				cv := &hir.Expr{Kind: hir.RuntimeOp, Op: "dynamic.asClassValue", Type: hir.T(hir.ClassValue), X: hir.V("s", hir.T(hir.Dynamic))}
				return &hir.Expr{Kind: hir.RuntimeOp, Op: "classvalue.new", Type: hir.Ref("src/abap/2_statements/combi.ts.Expression"), X: cv}
			},
		}},
		Entry{ID: "abaplint-external-include", Key: Key{"src/abap/2_statements/expand_macros.ts", "ExpandMacros.find", "KindMethodDeclaration"}, SHA256: "eb79a507ac3c0379ebecf6a154626405af2370dac011cc73334dd49c972dcacd", Rationale: "without a registry INCLUDE is retained; registry-backed resolution is outside this closure and traps", Statements: map[string]func() *hir.Stmt{"{\n        const includeName = statement.findDirectExpression(Expressions.IncludeName)?.concatTokens();\n        // todo, this does not take function module includes into account\n        // todo, workaround for cyclic includes?\n        const prog = this.reg?.getObject(\"PROG\", includeName) as Program | undefined;\n        if (prog) {\n          prog.parse(this.release, this.globalMacros, this.reg, this.languageVersion);\n          const includeMainFile = prog.getMainABAPFile();\n          if (includeMainFile) {\n            // slow, this copies everything,\n            this.find([...includeMainFile.getStatements()], includeMainFile, false);\n          }\n        }\n      }": func() *hir.Stmt {
			reg := &hir.Expr{Kind: hir.FieldGet, Name: "reg", Type: hir.T(hir.Optional, hir.Type{Kind: hir.InterfaceRef, Name: "src/_iregistry.ts.IRegistry"}), X: &hir.Expr{Kind: hir.This, Type: hir.Ref("src/abap/2_statements/expand_macros.ts.ExpandMacros")}}
			absent := &hir.Expr{Kind: hir.IsUndefined, Type: hir.T(hir.Bool), X: reg}
			return hir.B(&hir.Stmt{Kind: hir.If, X: &hir.Expr{Kind: hir.Unary, Op: "!", Type: hir.T(hir.Bool), X: absent}, Body: hir.B(&hir.Stmt{Kind: hir.Throw, X: hir.L(hir.T(hir.String), "external INCLUDE resolution excluded")})})
		}}},
		Entry{ID: "abaplint-registry-scope", Key: Key{"src/_iregistry.ts", "IRegistry", "KindInterfaceDeclaration"}, SHA256: "2deebf3cc7d259794d99fc56a63ac2632f492386fbabbde56cad98eb16a013d3", Rationale: "only macro-reference lookup belongs to the no-registry statement-parser execution scope", Interface: func() *hir.Interface {
			return &hir.Interface{Methods: []*hir.Method{{Name: "getMacroReferences", Virtual: true, Result: hir.Type{Kind: hir.InterfaceRef, Name: "src/_imacro_references.ts.IMacroReferences"}}}}
		}},
		Entry{ID: "abaplint-artifacts-keywords", Key: Key{"src/abap/artifacts.ts", "ArtifactsABAP.getKeywords", "KindMethodDeclaration"}, SHA256: "6ae2b6f43dedd59b9e0ce546492592f4890d9b8ba4a0624b534e30a5e94a508c", Rationale: "keyword introspection is outside statement parsing; invocation traps", Method: func() *hir.Method {
			return excluded("getKeywords", hir.T(hir.Array, hir.Ref("src/abap/artifacts.ts.IKeyword")))
		}},
		Entry{ID: "abaplint-artifacts-class-name", Key: Key{"src/abap/artifacts.ts", "className", "KindFunctionDeclaration"}, SHA256: "067e03920167446960ac270359d8639f039f13e5645e428b68b346b06e7476d5", Rationale: "helper is used only by excluded keyword introspection; invocation traps", Method: func() *hir.Method {
			m := excluded("className", hir.T(hir.String))
			m.Params = []hir.Param{{Name: "cla", Type: hir.Ref(hir.RootObject)}}
			return m
		}},
		Entry{ID: "abaplint-token-debug-description", Key: Key{"src/abap/1_lexer/tokens/abstract_token.ts", "AbstractToken", "KindMethodDeclaration"}, SHA256: "f40b0e404bdce56c7f5dfd13fe66c8315fe0b3f8aa851a8a861ea0abd738a537", Rationale: "symbol debug inspector is outside parser execution; invocation traps", Method: func() *hir.Method { m := excludedVirtual("debugDescription"); return m }},

		Entry{ID: "abaplint-star-priority-sentinel", Key: Key{"src/abap/2_statements/combi.ts", "StarPriority.run", "KindMethodDeclaration"}, SHA256: "bebd76433152657b6a09a82617e80258e1510de67e5a27a1db4ce80be2dac2a1", Rationale: "sentinel is only compared with remaining token counts bounded by STATEMENT_MAX_TOKENS=20000", Expressions: map[string]func() *hir.Expr{"Number.MAX_SAFE_INTEGER": func() *hir.Expr { return hir.L(hir.T(hir.Number), 2147483647) }}},
		Entry{ID: "abaplint-regex-to-str", Key: Key{"src/abap/2_statements/combi.ts", "Regex.toStr", "KindMethodDeclaration"}, SHA256: "a6bc40849eb4a68b2198c1ba45ae4371139f115490a6cee147c7870bbfb77387", Rationale: "debug rendering is outside statement parsing; invocation traps", Method: func() *hir.Method { return excludedVirtual("toStr") }},
		Entry{ID: "abaplint-token-railroad", Key: Key{"src/abap/2_statements/combi.ts", "Token.railroad", "KindMethodDeclaration"}, SHA256: "7ee9a8d9813bf2fa2ecff12da2bc1108e98813375e2b7e5ad5f6e476bf044163", Rationale: "railroad rendering is outside statement parsing; invocation traps", Method: func() *hir.Method { return excludedVirtual("railroad") }},
		Entry{ID: "abaplint-artifacts-structures", Key: Key{"src/abap/artifacts.ts", "ArtifactsABAP.getStructures", "KindMethodDeclaration"}, SHA256: "3c60af94d540b613c8649a3ac93b494fc327564984a276000ff83058de4d3456", Rationale: "structure introspection is outside the statement-parser closure; invocation traps", Method: func() *hir.Method { return excluded("getStructures", hir.T(hir.Array, hir.Ref(hir.RootObject))) }},
		Entry{ID: "abaplint-artifacts-expressions", Key: Key{"src/abap/artifacts.ts", "ArtifactsABAP.getExpressions", "KindMethodDeclaration"}, SHA256: "b769df9e85d108dacdc51356e02a9715194c7a650a6ed8e73d0c244545f04006", Rationale: "expression introspection is outside the statement-parser closure; invocation traps", Method: func() *hir.Method { return excluded("getExpressions", hir.T(hir.Array, hir.T(hir.ClassValue))) }},
	)
	r, err := New(entries...)
	if err != nil {
		panic(err)
	}
	return r
}
func excluded(name string, result hir.Type) *hir.Method {
	return &hir.Method{Name: name, Static: true, Result: result, Body: hir.B(&hir.Stmt{Kind: hir.Throw, X: hir.L(hir.T(hir.String), "excluded abaplint introspection: "+name)})}
}

func excludedVirtual(name string) *hir.Method {
	m := excluded(name, hir.T(hir.String))
	m.Static = false
	m.Virtual = true
	return m
}
