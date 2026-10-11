# Train 7 verification

Integration base: `8776b82`; measured compiler revision: `06cd4cc` (the only change from the integration base is the Grace golden). Main baseline: `9480bfa`, 10,452 passes. Upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`, original source with locked dependencies, TypeScript 6.0.3. All 625 test files were measured in 17 disjoint shards, using one worker and Go build parallelism at most four.

**10,614 / 11,060 pass, FAIL 0: +162 versus main.** Traps decrease 548 → 386. Not-run stays 92 (64 unregistered, 28 skipped/focused); async-pending is zero. Node has 11,060 active passes and 32 pending cases, 11,092 total. Whole-suite and shard title multisets reconcile, with no excluded roots, unmatched executions, infrastructure failures, or wiring failures. Pending Node cases explain why pass + trap + not-run exceeds the active denominator.

The branches were measured on older bases, so their isolated gains are not additive. Main already has the combined syntax and root pass counts. The measured gains versus main are abap +31, expressions +2, statements +20, LSP +37, objects +10, pretty-printer +11, and rules +51.

[Machine-readable measurement](abaplint-tests-latest.json). The runner removed disposable builds, closures, and raw records after successful reconciliation.

| Shard | Pass | FAIL | Trap | Not-run | Pass delta vs main |
| --- | ---: | ---: | ---: | ---: | ---: |
| abap | 199 | 0 | 17 | 4 | +31 |
| abap/expressions | 44 | 0 | 6 | 57 | +2 |
| abap/lexer | 200 | 0 | 0 | 0 | +0 |
| abap/nodes | 2 | 0 | 0 | 0 | +0 |
| abap/statements | 4,962 | 0 | 0 | 0 | +20 |
| abap/structures | 138 | 0 | 0 | 2 | +0 |
| abap/syntax | 1,353 | 0 | 10 | 10 | +0 |
| abap/types | 23 | 0 | 6 | 0 | +0 |
| root | 71 | 0 | 23 | 0 | +0 |
| cds | 355 | 0 | 3 | 1 | +0 |
| ddl | 32 | 0 | 0 | 0 | +0 |
| lsp | 134 | 0 | 54 | 5 | +37 |
| objects | 115 | 0 | 29 | 1 | +10 |
| objects/rename | 0 | 0 | 31 | 0 | +0 |
| pretty_printer | 21 | 0 | 10 | 0 | +11 |
| rules | 2,957 | 0 | 190 | 12 | +51 |
| utils | 8 | 0 | 7 | 0 | +0 |
| **TOTAL** | **10,614** | **0** | **386** | **92** | **+162** |

Verification:

- `go build ./...` and `go vet ./...`: pass with `GOFLAGS=-p=4`. Vet was rerun after the concurrent lint canary was removed.
- `go test -short -p 4 ./...`: pass.
- `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`: pass, executed explicitly.
- Grace lexer oracle, reference equality, termination and idempotence: pass. Exported the combined facts once and copied the report into the golden; the follow-up test passed without re-exporting.
- `TestGraceFullRegistryClosure`: pass with `ABAPITI_GRACE_FULL=1`; 1,927 classes and 64 interfaces are unchanged, so no expectation update.
- `lint.sh gate origin/main`: 0 new issues, canary 5/5, checksum-verified golangci-lint 2.13.2.
- Default generated output versus an isolated `origin/main` compiler: all 6,312 files byte-identical. Both compilers wrote to the same absolute scratch output path, with the main output moved aside before integration generation. File-level differences: **none**.
- Integration Go, the release check kit, and fresh original Node: byte-identical clean **0 issues** and seeded **5 issues**. The Node harness ran directly because the optional timing helper requires an unavailable `/usr/bin/time`.
- `abaplint --full`: 37 blockers (27 root, 10 cascade), 87 HIR errors in 24 files. The decrease from main’s 42 blockers is the LSP branch’s five removed guards/receiver/cascade diagnostics, as documented in [the branch evidence](fit-lsp-mix.md). This unpruned metric is separate from the passing reachability-pruned Grace closure.

The only merge repair is the Grace lexer catalogue golden. `lsp.isTextDocumentEdit` and `lsp.isRenameFile` add two facts each to `op_aliases`, `op_allocates`, `op_may_raise`, and `op_writes` (122 → 124), plus three reads (184 → 187): two receivers and the rename guard’s condition argument. These are catalogue entries, including false/no-effect values, rather than extra allocations or writes in lexer execution. Method and expression facts are unchanged.

Top 15 trap groups, normalized by the existing report tool (exact messages and examples are retained in the JSON):

1. **195** — `src/rules/function_module_recommendations.ts: value outside supported numeric, ordering or conversion domain`
2. **23** — `unsupported-expr src/objects/rename/renamer.ts: in requires a record or a literal property on a lowered shape (got classref<builtin.object>)`
3. **19** — `src/objects/domain.ts: value outside supported numeric, ordering or conversion domain`
4. **18** — `src/rules_runner.ts: value outside supported numeric, ordering or conversion domain`
5. **12** — `unsupported-callback src/lsp/references.ts: callback KindPropertyAccessExpression is not an arrow function`
6. **9** — `unsupported-call src/rules/identical_conditions.ts: find on primitive arrays needs an absence-preserving callback ABI`
7. **7** — `unsupported-assignment test/pretty_printer/pretty_printer.ts: assignment to property keyword_case is not lowered`
8. **7** — `unsupported-type test/utils/name_validator.ts: object literal property patternKind is not in shape src/rules/_naming_rule_config.ts.NamingRuleConfig; unsupported-expr test/utils/name_validator.ts: identifier ruleViolated is not a lowered local or module value`
9. **5** — `module binding test/abap/types/form_definition.ts.module.reg was not initialized`
10. **5** — `src/lsp/rename.ts: HIR verification: call argument type mismatch: classref<src/lsp/_interfaces.ts.ITextDocumentPositionParams> <- classref<src/lsp/_interfaces.ts.IRenameParams> in virtual .prepareRename`
11. **5** — `unsupported-type test/abap/expressions/sql_set_op.ts: checker type ([string, { text: string; }, never[]] | [string, {}, ([string, { text: string; }, never[]] | [string, {}, ([string, { text: string; }, never[]] | [string, {}, [string, {}, [string, {}, [...][]][]][]])[]])[]] | [...])[] is not a lowered declaration; unsupported-type test/abap/expressions/sql_set_op.ts: checker type ([string, { text: string; }, never[]] | [string, {}, ([string, { text: string; }, never[]] | [string, {}, ([string, { text: string; }, never[]] | [string, {}, [string, {}, [string, {}, [...][]][]][]])[]])[]] | [...])[] is not a lowered declaration; unsupported-array test/abap/expressions/sql_set_op.ts: empty array literal needs an inferable contextual element type`
12. **4** — `runtime/chai.ts: value outside supported numeric, ordering or conversion domain`
13. **4** — `src/lsp/implementation.ts: HIR verification: call argument type mismatch: classref<node_modules/vscode-languageserver-types/lib/umd/main.d.ts.Location> <- optional<classref<node_modules/vscode-languageserver-types/lib/umd/main.d.ts.Location>> in runtime .`
14. **4** — `test/objects/table.ts: HIR verification: assignment type mismatch`
15. **3** — `src/abap/5_syntax/spaghetti_scope.ts: HIR verification: call argument type mismatch: classref<src/abap/types/_typed_identifier.ts.TypedIdentifier> <- optional<classref<src/abap/types/_typed_identifier.ts.TypedIdentifier>> in new .`

No runtime trap fixes were attempted. The golden repair and progress measurement are separate commits with the requested trailers. Nothing was pushed.
