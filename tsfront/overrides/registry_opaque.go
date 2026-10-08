package overrides

import "github.com/oisee/abapiti/hir"

// Source-pinned opaque external signatures; field/method use remains blocking.
func registryOpaqueSignatures() []Entry {
	return []Entry{
		{ID: "abaplint-opaque-src-lsp--edit-ts-LSPEdit", Key: Key{"src/lsp/_edit.ts", "LSPEdit", "KindClassDeclaration"}, SHA256: "f5fe9a8f025162f0bbe261a1bc7cd3f3994caabb85c2b3049b96dae7b35ffd7b", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"LServer.WorkspaceEdit": hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-lsp-code-actions-ts-CodeActions", Key: Key{"src/lsp/code_actions.ts", "CodeActions", "KindClassDeclaration"}, SHA256: "2e51dcd5c16090c279c806f7e909bde55641c25dd0e03784949675d74022992b", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"LServer.CodeAction": hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-lsp-language-server-ts-LanguageServer", Key: Key{"src/lsp/language_server.ts", "LanguageServer", "KindClassDeclaration"}, SHA256: "1a63823219eacd123ec383f23ff4170634c9248ad610d6fdacb007bd34f38d7a", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"LServer.CodeAction":     hir.Ref(hir.RootObject),
			"LServer.DocumentSymbol": hir.Ref(hir.RootObject),
			"LServer.WorkspaceEdit":  hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-lsp-rename-ts-Rename", Key: Key{"src/lsp/rename.ts", "Rename", "KindClassDeclaration"}, SHA256: "b9d21d4b6e55366d8603238e8327abaeccc8a5063ac62467deb7fc1cf65c09f2", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"LServer.WorkspaceEdit": hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-lsp-symbols-ts-Symbols", Key: Key{"src/lsp/symbols.ts", "Symbols", "KindClassDeclaration"}, SHA256: "f38101b683586e7ca14740b98326204cbfb793518190521335b2f196b5ce890e", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"LServer.DocumentSymbol": hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-objects-rename-rename-data-element-ts-RenameDataElement", Key: Key{"src/objects/rename/rename_data_element.ts", "RenameDataElement", "KindClassDeclaration"}, SHA256: "bb504a366212447ea16b931d06eb0c4e03176aa5125d3eba628e5c12dfdd7967", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"TextDocumentEdit": hir.Ref(hir.RootObject),
			"WorkspaceEdit":    hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-objects-rename-rename-domain-ts-RenameDomain", Key: Key{"src/objects/rename/rename_domain.ts", "RenameDomain", "KindClassDeclaration"}, SHA256: "0f12367a740f8827319af167890bd180c82fa437e7190c2f0aa008ac22828237", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"TextDocumentEdit": hir.Ref(hir.RootObject),
			"WorkspaceEdit":    hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-objects-rename-rename-global-class-ts-RenameGlobalClass", Key: Key{"src/objects/rename/rename_global_class.ts", "RenameGlobalClass", "KindClassDeclaration"}, SHA256: "ac8747d200260869609be2f7c16990c0697de5a8d31b7d9fa6e0a0a2037990a1", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"TextDocumentEdit": hir.Ref(hir.RootObject),
			"WorkspaceEdit":    hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-objects-rename-rename-global-interface-ts-RenameGlobalInterface", Key: Key{"src/objects/rename/rename_global_interface.ts", "RenameGlobalInterface", "KindClassDeclaration"}, SHA256: "9d59db8fdd4ff47f745a46bfcb1a4be5d3305ef6ce146edcd55776b606fc6aec", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"TextDocumentEdit": hir.Ref(hir.RootObject),
			"WorkspaceEdit":    hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-objects-rename-rename-icf-service-ts-RenameICFService", Key: Key{"src/objects/rename/rename_icf_service.ts", "RenameICFService", "KindClassDeclaration"}, SHA256: "1636844fa8145b5e3cd7feb5146cf7e91e1d48ca2d19b8fdbb09e40159997f91", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"TextDocumentEdit": hir.Ref(hir.RootObject),
			"WorkspaceEdit":    hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-objects-rename-rename-message-class-ts-RenameMessageClass", Key: Key{"src/objects/rename/rename_message_class.ts", "RenameMessageClass", "KindClassDeclaration"}, SHA256: "a647f41d669b245bf10721021ef30f4e003881fc0ad382a1eb072f3702d328ca", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"TextDocumentEdit": hir.Ref(hir.RootObject),
			"WorkspaceEdit":    hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-objects-rename-rename-program-ts-RenameProgram", Key: Key{"src/objects/rename/rename_program.ts", "RenameProgram", "KindClassDeclaration"}, SHA256: "a05b69ef887bcadfb7de9d032be8e7bdf6c0671e3a7b13c579e1220d4de8fe52", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"TextDocumentEdit": hir.Ref(hir.RootObject),
			"WorkspaceEdit":    hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-objects-rename-rename-table-ts-RenameTable", Key: Key{"src/objects/rename/rename_table.ts", "RenameTable", "KindClassDeclaration"}, SHA256: "8e46a83b69b8d27388119ae98faf4a3221523997fcf26f280950c43404e5d77d", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"TextDocumentEdit": hir.Ref(hir.RootObject),
			"WorkspaceEdit":    hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-objects-rename-rename-table-type-ts-RenameTableType", Key: Key{"src/objects/rename/rename_table_type.ts", "RenameTableType", "KindClassDeclaration"}, SHA256: "b6d82eb220ab5868cef95f4a7f4620d66c290eea87a21129766db8af1804bd24", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"TextDocumentEdit": hir.Ref(hir.RootObject),
			"WorkspaceEdit":    hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-objects-rename-renamer-ts-Renamer", Key: Key{"src/objects/rename/renamer.ts", "Renamer", "KindClassDeclaration"}, SHA256: "8374920374d788896cbae5e5cea96fa220a973f85fc69529dbc130b8fc994511", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"TextDocumentEdit": hir.Ref(hir.RootObject),
			"WorkspaceEdit":    hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-opaque-src-objects-rename-renamer-helper-ts-RenamerHelper", Key: Key{"src/objects/rename/renamer_helper.ts", "RenamerHelper", "KindClassDeclaration"}, SHA256: "81fa38246130e135879e1f87499f140d407e0b6453479d9f664ecafce5ffd0bc", Rationale: "external LSP result retained as opaque reference; excluded bodies keep coverage traps and any field use remains blocking; generalise later", Patterns: &Patterns{Annotations: map[string]hir.Type{
			"TextDocumentEdit": hir.Ref(hir.RootObject),
		}}},
		{ID: "abaplint-debug-trap-ABAPObject", Key: Key{"src/objects/_abap_object.ts", "ABAPObject", "KindMethodDeclaration"}, SHA256: "203e3d6d63879b5cea1b485ee17afa14b5dd09ed0427ff532794de246f80758d", Rationale: "symbol debug inspector outside the executed workloads retains a located coverage trap; generalise later", Method: func() *hir.Method {
			return &hir.Method{Name: "debugDescription", Virtual: true, Result: hir.T(hir.String), Body: hir.B(&hir.Stmt{Kind: hir.Trap, Name: "src/objects/_abap_object.ts:20"})}
		}},
		{ID: "abaplint-debug-trap-SpaghettiScopeNode", Key: Key{"src/abap/5_syntax/spaghetti_scope.ts", "SpaghettiScopeNode", "KindMethodDeclaration"}, SHA256: "ce0f33646b833fe6f265c8cb97d04f41cf4f2c98f9ab5fda786df8630e32a81e", Rationale: "symbol debug inspector outside the executed workloads retains a located coverage trap; generalise later", Method: func() *hir.Method {
			return &hir.Method{Name: "debugDescription", Virtual: true, Result: hir.T(hir.String), Body: hir.B(&hir.Stmt{Kind: hir.Trap, Name: "src/abap/5_syntax/spaghetti_scope.ts:37"})}
		}},
		{ID: "abaplint-debug-trap-TypedIdentifier", Key: Key{"src/abap/types/_typed_identifier.ts", "TypedIdentifier", "KindMethodDeclaration"}, SHA256: "36b84b6865b9bab96bddcee198c00676873517b2cfc63f043a286063dfa68337", Rationale: "symbol debug inspector outside the executed workloads retains a located coverage trap; generalise later", Method: func() *hir.Method {
			return &hir.Method{Name: "debugDescription", Virtual: true, Result: hir.T(hir.String), Body: hir.B(&hir.Stmt{Kind: hir.Trap, Name: "src/abap/types/_typed_identifier.ts:35"})}
		}},
	}
}
