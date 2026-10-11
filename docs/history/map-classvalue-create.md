# LSP namespace constructor lowering

Branch: `feat/map-classvalue-create`; comparison commit: `69c79ee` (`origin/main`).

## Library boundary

The receivers are the declaration-merged interfaces/namespaces exported by
`vscode-languageserver-types`, not JavaScript Maps or TypeScript classes. Match
resolved symbols from that package's declaration files, including renamed named
imports and module namespace imports. Unrelated user classes are unaffected.

Core calls these constructors:

| Function | Construction |
| --- | --- |
| `Position.create(line, character)` | `{line, character}` |
| `Range.create(start, end)` | `{start, end}`, preserving position identity |
| `Range.create(sl, sc, el, ec)` | Two fresh positions and `{start, end}` |
| `TextEdit.replace(range, newText)` | `{range, newText}` |
| `TextDocumentEdit.create(textDocument, edits)` | `{textDocument, edits}` |
| `Command.create(title, command, ...args)` | `{title, command}`, with `arguments` present only for nonempty args |
| `RenameFile.create(oldUri, newUri, options?, annotationId?)` | `kind: "rename"` and the supplied fields |

The last constructor is also used in `renamer_helper.ts:225`. Core does **not**
call `WorkspaceEdit.create`, `DocumentSymbol.create`, or `Location.create`.
`DocumentSymbol` is constructed using typed object literals. Its recursive
`children` type is now a named shape, registered before lowering its members.
Shape fields and optionality come from the checked declaration, rather than a
second handwritten schema. Supporting types include document identifiers,
`Location`, and rename operation/options. Unsupported namespace methods retain
an explicit `unsupported-lsp-call` diagnostic (including the `is` predicates).

Unlike the existing fingerprinted JSON5/XML subset overrides, this boundary is
package-symbol based and emits ordinary HIR `New` and field operations. No
backend-specific runtime operation or call-site override is needed.

Reachability-trapped signatures retain their old structural ABI when there is
no callable LSP constructor. This avoids expanding the external ABI in the
existing default workload. Calls evaluate arguments once, left to right, before
reordering them into declaration-field order.

## Gates

Environment: `GOCACHE=$HOME/.cache/go-build GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false`.

- `go test ./tsfront/... ./hir/... ./cmd/...`: pass.
- `go vet ./tsfront/... ./hir/... ./cmd/...`: pass.
- Tests use the embedded real package declarations, check the TypeScript
  checker and HIR verifier, inspect ABAP named-constructor emission, and compile
  and execute Go emission for every supported function. They also cover
  overloads, named import aliases, recursive shapes, optional/variadic fields,
  evaluation order, preserved reference identity, unsupported methods, and a
  shadowing user `Range` class.
- Default `abaplint -o X --target a4h --quiet`: `diff -r` against the binary built
  from `origin/main` is empty, including the ABAP archive and metadata.

## Full-closure evidence

This checkout has no `--full`. A temporary, **uncommitted and removed**
`ABAPITI_LOCAL_FULL` check set `coverage=nil` immediately before `LowerRegistry`.
Evidence was collected with `--evidence --assume-int=false --quiet`.
The default integer pass panics on an already-malformed runtime expression in
full lowering (`assume_integer.go`, dereferencing `e.X.Type`); this is not an LSP
constructor failure. No integer-pass workaround is committed.

| Measurement | Before | After | Reduction |
| --- | ---: | ---: | ---: |
| Blocking diagnostics, measured binary64 full closure | 644 | 515 | 129 |
| Non-identifier diagnostics (explicit root proxy) | 315 | 191 | 124 |
| Unresolved-identifier diagnostics | 329 | 324 | 5 |
| Blocking diagnostics excluding the 9 unchanged binary64 numeric blockers | 635 | 506 | 129 |

The task's supplied baseline is **635 blocking / 355 root**. Binary64 lowering
adds nine unchanged `unsupported-number` diagnostics about Math.min/max integer
proofs. There is no root/cascade classifier in this branch; the root proxy above
is explicitly defined as excluding messages of the form
`identifier … is not a lowered local or module value`, including unresolved
library/global identifiers. It is therefore **not** the task's 355-root metric.
Do not equate these two classifications.

All 81 declaration-namespace top-level diagnostics, all targeted `create` and
`replace` receiver diagnostics, and all five `DocumentSymbol` type diagnostics
are gone. Remaining library predicates are deliberately unsupported.

### Remaining LSP/rename diagnostics (63 total)

Counts include cascades; none is a supported namespace-constructor diagnostic.

| File under `src/` | Count | Reasons |
| --- | ---: | --- |
| `lsp/_edit.ts` | 8 | WorkspaceEdit object shape (2); missing workspace local (6) |
| `lsp/_lookup.ts` | 2 | Object and JSON globals |
| `lsp/code_actions.ts` | 1 | Object literal shape |
| `lsp/dump_scope.ts` | 4 | JS ToPrimitive for identifier/interface/array values |
| `lsp/help.ts` | 7 | delete on nonrecord, JSON globals (3), text local, object-root constructor, getStr |
| `lsp/references.ts` | 1 | Property-access callback rather than arrow |
| `lsp/rename.ts` | 7 | WorkspaceEdit object shape (2); missing workspace local (5) |
| `lsp/semantic.ts` | 1 | for-in on a class shape rather than string-keyed map |
| `lsp/symbols.ts` | 4 | Object literal shape (2); missing symbol local (2), not DocumentSymbol type resolution |
| `objects/rename/rename_data_element.ts` | 1 | Object literal shape |
| `objects/rename/rename_domain.ts` | 1 | Object literal shape |
| `objects/rename/rename_global_class.ts` | 1 | Object literal shape |
| `objects/rename/rename_global_interface.ts` | 1 | Object literal shape |
| `objects/rename/rename_icf_service.ts` | 8 | Parenthesized callee; string match (2); cleanOldName (3), fileNewName locals; object literal shape |
| `objects/rename/rename_message_class.ts` | 1 | Object literal shape |
| `objects/rename/rename_program.ts` | 1 | Object literal shape |
| `objects/rename/rename_table.ts` | 1 | Object literal shape |
| `objects/rename/rename_table_type.ts` | 1 | Object literal shape |
| `objects/rename/renamer.ts` | 3 | TextDocumentEdit.is, RenameFile.is, in operator |
| `objects/rename/renamer_helper.ts` | 9 | Multi-return callback; nonliteral replace (2); updatedUrl (2), newFilename, icfServices locals; Generator type; void-result local ABI |

Local logs, before/after JSON evidence, and empty default-output diff are under
`.local/mapcv-evidence/` (not committed).
