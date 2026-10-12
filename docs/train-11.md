# Train 11 verification

Measured integration: `8c40fe1` (train 10 `794334c` plus final ABI and traps M). Main: `e2ba6ac`. Original upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`, with locked dependencies and TypeScript 6.0.3. All 625 files ran in 17 disjoint shards; two workers and Go package parallelism at most four.

**11,032 / 11,060 pass, FAIL 0: +10 versus train 10 (11,022).** Traps 35 → 25; not-run 35 (7 unregistered, 28 skipped/focused); async-pending 0. Node has 11,060 active passes and 32 pending cases, 11,092 total. The active tail is 28 cases: 25 traps and 3 unregistered. The four other unregistered cases and 28 skipped cases are Node-pending, outside the denominator.

Whole-suite and shard title multisets reconcile; no excluded roots, unmatched executions, infrastructure failures or wiring failures. [Measurement JSON](abaplint-tests-latest.json). [Every remaining active test, grouped by exact message](train-11-remaining.md).

| Shard | Pass | FAIL | Trap | Not-run | Pass delta vs train 10 |
| --- | ---: | ---: | ---: | ---: | ---: |
| abap | 216 | 0 | 0 | 4 | +0 |
| abap/expressions | 107 | 0 | 0 | 0 | +0 |
| abap/lexer | 200 | 0 | 0 | 0 | +0 |
| abap/nodes | 2 | 0 | 0 | 0 | +0 |
| abap/statements | 4,962 | 0 | 0 | 0 | +0 |
| abap/structures | 138 | 0 | 0 | 2 | +0 |
| abap/syntax | 1,363 | 0 | 0 | 10 | +1 |
| abap/types | 28 | 0 | 1 | 0 | +0 |
| root | 91 | 0 | 3 | 0 | +0 |
| cds | 355 | 0 | 3 | 1 | +0 |
| ddl | 32 | 0 | 0 | 0 | +0 |
| lsp | 181 | 0 | 7 | 5 | +1 |
| objects | 138 | 0 | 6 | 1 | +4 |
| objects/rename | 31 | 0 | 0 | 0 | +2 |
| pretty_printer | 30 | 0 | 1 | 0 | +0 |
| rules | 3,143 | 0 | 4 | 12 | +2 |
| utils | 15 | 0 | 0 | 0 | +0 |
| **TOTAL** | **11,032** | **0** | **25** | **35** | **+10** |

Verification:

- `go build -p 4 ./...` and `go vet -p 4 ./...`: pass.
- One combined `ABAPITI_GRACECHECK=1 GRACE_FACTS_OUT=<scratch>/lexer.facts go test -p 4 -count=1 -run TestLexerFactsReport ./tsfront` export differed only in catalog counts. The exported golden was committed separately, then validated without another export. `op_aliases`, `op_allocates`, `op_may_raise`, `op_writes`: **133 → 137**; `op_reads`: **201 → 205**; `op_conservative`: **21**, unchanged. Final ABI adds `dynamic.recordString`, `string.parseFloatNumber`, `string.parseIntNumber`, `number.isNaN`. Each has four metadata facts and one receiver-read fact, including explicit None/false facts. Lexer facts and all 61 Grace call sites are unchanged.
- `ABAPITI_GRACE_FULL=1 go test -p 4 -count=1 -run TestGraceFullRegistryClosure -v -timeout 60m ./tsfront`: pass, including reference/incremental recomputation, monotonicity, determinism, termination, budgets and idempotence. Lowering: 0 blocking diagnostics, 0 verification errors. **1,927 classes and 64 interfaces** remain correct; no expected count update.
- `go test -short -p 4 -timeout 40m ./...`: pass with pinned upstream and core Node oracles enabled. An earlier overlap with the intentional lint canary made vet/short checks report its known format error; both checks were rerun clean after lint removed it. No source repair was needed.
- The `hir/abap/ops.go` conflict resolution is exactly the union of its two parents: 24 unsupported operations.
- Explicit `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`: pass without skip.
- `./.github/ci/lint.sh gate origin/main`: **0 new issues, canary 5/5**, checksum-pinned golangci-lint 2.13.2.
- Default `--target all,go` output from main and integration, emitted at the same absolute scratch path: **6,321 files each**. File-level differences: **`go/hir.go`, `go/hir_profile.go`, `go/runtime.go`, `go/zabaplint-go`, `sites.json`**. The two HIR files differ only by 1,927 constructor-descriptor methods and 18 constructor lookup cases from traps M (`b67770b`); after removing those additions they match exactly. `sites.json` differs only in generated location data. The runtime contains the inherited K/L JSON indentation, parseFloat, padStart, XML error handling and JSON5 additions (`51f96c0`, `6410553`). The binary reflects these source changes. Every remaining file, including ABAP classes and the zip, is byte-identical. The expected empty diff was not observed; these are feature changes, not merge fallout, so no feature behavior was removed.
- Checksum-verified v0.1.1 kit (ZIP SHA-256 `2e3311517885ed72e651c97004d6cba064fa5093873c75f1104955891e965d7a`): fresh original pinned Node and emitted Go match each other and kit bytes exactly, clean **0 issues**, seeded **5 issues**. `go-target-check.py` also passes both. The existing Node worker ran directly without the optional timing wrapper.

Top 15 trap groups use the standard report normalization; the complete exact messages and every active test are in the tail document.

1. **3** — `src/lsp/rename.ts: HIR verification: assignment type mismatch`
2. **2** — `src/objects/_dynpros.ts: value outside supported numeric, ordering or conversion domain`
3. **2** — `src/objects/message_class.ts: value outside supported numeric, ordering or conversion domain`
4. **2** — `src/rules/space_before_dot.ts: value outside supported numeric, ordering or conversion domain`
5. **2** — `test/objects/domain.ts: value outside supported numeric, ordering or conversion domain`
6. **2** — `unsupported-expr src/lsp/dump_scope.ts: string coercion of classref<src/abap/types/_typed_identifier.ts.TypedIdentifier> requires JavaScript ToPrimitive; unsupported-expr src/lsp/dump_scope.ts: string coercion of classref<src/abap/types/_typed_identifier.ts.TypedIdentifier> requires JavaScript ToPrimitive`
7. **1** — `src/lsp/semantic.ts: HIR verification: invalid narrowing classvalue to classref<shape.c3b70776d029edfc69fd75ab61b91f27fe8deaade7fdd314df00aa7ee5f66866>`
8. **1** — `src/objects/domain.ts: input is outside the supported strict JSON subset`
9. **1** — `src/objects/proxy_object.ts: value outside supported numeric, ordering or conversion domain`
10. **1** — `src/rules/line_only_punc.ts: value outside supported numeric, ordering or conversion domain`
11. **1** — `test/abap/types/types.ts: value outside supported numeric, ordering or conversion domain`
12. **1** — `test/objects/view.ts: value outside supported numeric, ordering or conversion domain`
13. **1** — `unsupported-call test/cds/cds_parser.ts: method get on dynamic is not lowered; unsupported-expr test/cds/cds_parser.ts: identifier k is not a lowered local or module value; unsupported-call test/cds/cds_parser.ts: method getChildren on dynamic is not lowered`
14. **1** — `unsupported-call test/cds/cds_parser.ts: method getFirstToken on dynamic is not lowered; unsupported-expr test/cds/cds_parser.ts: identifier tok is not a lowered local or module value`
15. **1** — `unsupported-expr src/lsp/help.ts: delete supports ordinary lowered records only`

Only the combined Grace golden, progress documentation and measurement evidence changed. No semantic code or expected closure counts changed. Nothing was pushed.
