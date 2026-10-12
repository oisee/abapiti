# Train 10 verification

Measured integration: `b15a4ba`. Main: `e2ba6ac` (train 8, 10,934 passes). Original upstream pin: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`, TypeScript 6.0.3 with locked root and core packages. All 625 test files ran in 17 disjoint shards, with two workers and Go build/test parallelism at most four.

**11,022 / 11,060 pass, FAIL 0: +88 versus main.** Traps decrease 123 → 35; not-run remains 35 (7 unregistered, 28 skipped/focused). Async-pending is zero. Node has 11,060 active passes and 32 pending cases, 11,092 total. The 38 active cases not passing are 35 traps and 3 unregistered; four additional unregistered cases and all 28 skipped cases are Node-pending, outside the denominator. Whole-suite and shard title multisets reconcile, with no excluded roots, unmatched executions, infrastructure failures or wiring failures.

[Machine-readable measurement](abaplint-tests-latest.json). The runner removed disposable shard builds and raw records after reconciliation.

The genuine train-9 failure **“LSP, actual rename, global class rename global class, add namespace” passes** in this combined run. LSP has 180 passes, 0 FAIL and 8 traps; its live backing-record lookup repair remains effective.

| Shard | Pass | FAIL | Trap | Not-run | Pass delta vs main |
| --- | ---: | ---: | ---: | ---: | ---: |
| abap | 216 | 0 | 0 | 4 | +0 |
| abap/expressions | 107 | 0 | 0 | 0 | +0 |
| abap/lexer | 200 | 0 | 0 | 0 | +0 |
| abap/nodes | 2 | 0 | 0 | 0 | +0 |
| abap/statements | 4,962 | 0 | 0 | 0 | +0 |
| abap/structures | 138 | 0 | 0 | 2 | +0 |
| abap/syntax | 1,362 | 0 | 1 | 10 | +8 |
| abap/types | 28 | 0 | 1 | 0 | +0 |
| cds | 355 | 0 | 3 | 1 | +0 |
| ddl | 32 | 0 | 0 | 0 | +0 |
| lsp | 180 | 0 | 8 | 5 | +2 |
| objects | 134 | 0 | 10 | 1 | +10 |
| objects/rename | 29 | 0 | 2 | 0 | +29 |
| pretty_printer | 30 | 0 | 1 | 0 | +9 |
| root | 91 | 0 | 3 | 0 | +3 |
| rules | 3,141 | 0 | 6 | 12 | +20 |
| utils | 15 | 0 | 0 | 0 | +7 |
| **TOTAL** | **11,022** | **0** | **35** | **35** | **+88** |

Verification:

- `go build -p 4 ./...` and `go vet -p 4 ./...`: pass.
- `ABAPITI_GRACECHECK=1 GRACE_FACTS_OUT=<scratch>/lexer.facts go test -p 4 -count=1 -run '^TestLexerFactsReport$' -timeout 40m ./tsfront`: pass. One combined export is byte-identical to the existing golden; no regeneration change is needed.
- `ABAPITI_GRACE_FULL=1 go test -p 4 -count=1 -run '^TestGraceFullRegistryClosure$' -v -timeout 60m ./tsfront`: pass, including the reference and incremental recomputation oracles, monotonicity, determinism, termination, budgets and idempotence. Lowering has 0 blocking diagnostics and 0 verification errors. Expected 1,927 classes and 64 interfaces remain correct, so no count update.
- `go test -short -p 4 -timeout 40m ./...`: pass, with the pinned upstream supplied through `ABAPITI_TESTS_UPSTREAM`. Focused merged-feature Node oracles also pass with `TSFRONT_ABAPLINT` pointing to the original core packages.
- Explicit `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`: pass, executed without skipping.
- `./.github/ci/lint.sh gate origin/main`: 0 new issues; canary 5/5; checksum-verified golangci-lint 2.13.2.
- Default `--target all,go` output from isolated main and integration compilers, generated at the same absolute scratch path: 6,321 files per tree. File-level differences are **`go/runtime.go`** and **`go/zabaplint-go`**. The runtime diff contains K's JSON indentation, parseFloat, padStart and external XML error helpers, plus L's JSON5 parser and Unicode tables. These additions come from feature commits `51f96c0` and `6410553`, not merge fallout. The shared emitted runtime includes these helpers even though the default operation guard does not use the new operations; the resulting Go executable also differs. All remaining files, including ABAP classes and zip, are byte-identical. The expected empty diff was therefore not observed; no feature behavior was removed to manufacture equality.
- The checksum-verified v0.1.1 zabapgit kit, emitted Go checker and fresh original pinned Node agree byte for byte for clean **0 issues** and seeded **5 issues**. The existing Node worker ran directly because the optional timing wrapper requires an unavailable `/usr/bin/time`.

No merge repair was necessary. Relative to main, the golden's `op_aliases`, `op_allocates`, `op_may_raise` and `op_writes` counts increase **125 → 133**, `op_reads` **188 → 201**, while `op_conservative` stays **21**. Seven operations are from K (`lsp.isCreateFile`, `lsp.isDeleteFile`, `chai.members`, `json.indent`, `string.parseFloat`, `string.padStart`, `xml.parseWithErrors`); L adds `json5.parse`. Each operation has one metadata fact for aliasing, allocation, raising and writing, including explicit “None”/false facts. All eight read the receiver and five also read arguments, explaining the 13 additional read facts. Lexer method and expression facts are unchanged; union fields, config shapes and the namespace repair add no operation records.

Top 15 trap groups, using the existing report tool's normalization; exact examples and all groups are retained in the JSON:

1. **3** — `src/abap/5_syntax/structures/class_data.ts: value outside supported numeric, ordering or conversion domain`
2. **3** — `src/lsp/rename.ts: HIR verification: assignment type mismatch`
3. **2** — `runtime/chai.ts: chai object enumerable keys are unsupported`
4. **2** — `src/objects/_abap_object.ts: src/objects/_abap_object.ts`
5. **2** — `src/objects/_dynpros.ts: value outside supported numeric, ordering or conversion domain`
6. **2** — `src/objects/message_class.ts: value outside supported numeric, ordering or conversion domain`
7. **2** — `src/objects/rename/renamer_helper.ts: value outside supported numeric, ordering or conversion domain`
8. **2** — `src/rules/space_before_dot.ts: value outside supported numeric, ordering or conversion domain`
9. **2** — `test/objects/domain.ts: value outside supported numeric, ordering or conversion domain`
10. **1** — `src/lsp/help.ts: value outside supported numeric, ordering or conversion domain`
11. **1** — `src/lsp/semantic.ts: HIR verification: invalid narrowing classvalue to classref<shape.c3b70776d029edfc69fd75ab61b91f27fe8deaade7fdd314df00aa7ee5f66866>`
12. **1** — `src/objects/domain.ts: input is outside the supported strict JSON subset`
13. **1** — `src/objects/number_range.ts: src/objects/number_range.ts: parseFloat requires a finite result`
14. **1** — `src/objects/proxy_object.ts: HIR verification: assignment type mismatch`
15. **1** — `src/rules/line_only_punc.ts: value outside supported numeric, ordering or conversion domain`

Only progress documentation and measurement evidence were updated. No code, golden or expectation changed. Nothing was pushed.
