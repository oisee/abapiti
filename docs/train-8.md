# Train 8 verification

Integration and measured compiler: `4b6a465`. Main: `ab026e7`, 10,614 passes. Pinned original upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`, TypeScript 6.0.3 with locked root and core dependencies. All 625 test files ran in 17 disjoint shards, with two workers and Go build parallelism at most four.

**10,934 / 11,060 pass, FAIL 0: +320 versus main.** Traps decrease 386 → 123; not-run decreases 92 → 35 (7 unregistered, 28 skipped/focused). Async-pending is zero. Node has 11,060 active passes and 32 pending cases, 11,092 total. Whole-suite and shard title multisets reconcile; there are no excluded roots, unmatched executions, infrastructure failures, or wiring failures. Pending Node cases explain why pass + trap + not-run exceeds the active denominator.

[Machine-readable measurement](abaplint-tests-latest.json). Disposable builds, closures and raw records were removed by the runner after reconciliation.

| Shard | Pass | FAIL | Trap | Not-run | Pass delta vs main |
| --- | ---: | ---: | ---: | ---: | ---: |
| abap | 216 | 0 | 0 | 4 | +17 |
| abap/expressions | 107 | 0 | 0 | 0 | +63 |
| abap/lexer | 200 | 0 | 0 | 0 | +0 |
| abap/nodes | 2 | 0 | 0 | 0 | +0 |
| abap/statements | 4,962 | 0 | 0 | 0 | +0 |
| abap/structures | 138 | 0 | 0 | 2 | +0 |
| abap/syntax | 1,354 | 0 | 9 | 10 | +1 |
| abap/types | 28 | 0 | 1 | 0 | +5 |
| root | 88 | 0 | 6 | 0 | +17 |
| cds | 355 | 0 | 3 | 1 | +0 |
| ddl | 32 | 0 | 0 | 0 | +0 |
| lsp | 178 | 0 | 10 | 5 | +44 |
| objects | 124 | 0 | 20 | 1 | +9 |
| objects/rename | 0 | 0 | 31 | 0 | +0 |
| pretty_printer | 21 | 0 | 10 | 0 | +0 |
| rules | 3,121 | 0 | 26 | 12 | +164 |
| utils | 8 | 0 | 7 | 0 | +0 |
| **TOTAL** | **10,934** | **0** | **123** | **35** | **+320** |

Verification:

- Build and vet pass with `-p 4`. Vet was rerun after the concurrent lint gate removed its intentional canary.
- `go test -short -p 4 ./...` and explicit `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`: pass.
- Grace lexer facts exported once with `ABAPITI_GRACECHECK=1 GRACE_FACTS_OUT=<scratch report>`: test passes and the existing golden is byte-identical.
- `TestGraceFullRegistryClosure` with `ABAPITI_GRACE_FULL=1`: pass, including independent reference equality, monotonicity, determinism, budgets, termination and idempotence. Expected 1,927 classes and 64 interfaces are unchanged; no expectation update.
- `lint.sh gate origin/main`: 0 new issues, canary 5/5, checksum-verified golangci-lint 2.13.2.
- Default `--target all,go` output generated at the same absolute scratch path by isolated main and integration compilers: 6,321 files compared. File-level differences are **`go/runtime.go`** and **`go/zabaplint-go`**. All remaining artifacts, including ABAP classes and zip, are byte-identical. The differences are inherited from traps-I commit `933e68f`: empty-regex source normalization and three reviewed regex catalogue entries, as already recorded in [live rows](live-rows.md). They are branch behavior, not merge fallout.
- Emitted Go, fresh pinned original Node and the v0.1.1 release check kit agree byte for byte: clean **0 issues**, seeded **5 issues**. The Node harness ran directly because the optional timing helper requires an unavailable `/usr/bin/time`.

No merge repair was necessary. Relative to main, `dynamic.arrayView` adds one catalogue fact to each of `op_aliases`, `op_allocates`, `op_conservative`, `op_may_raise`, and `op_writes`, plus one receiver read to `op_reads`: 124 → 125, 20 → 21, and 187 → 188 respectively. These describe the new live array view operation (including explicit no-write metadata), not additional lexer execution effects. Lexer method and expression facts remain unchanged; the merged golden already contains the combined result.

Top 15 trap groups, normalized by the existing report tool; exact messages and examples remain in the JSON:

1. **29** — `src/objects/rename/renamer.ts: HIR verification: unresolved field range`
2. **19** — `src/objects/domain.ts: value outside supported numeric, ordering or conversion domain`
3. **7** — `unsupported-assignment test/pretty_printer/pretty_printer.ts: assignment to property keyword_case is not lowered`
4. **7** — `unsupported-type test/utils/name_validator.ts: object literal property patternKind is not in shape src/rules/_naming_rule_config.ts.NamingRuleConfig; unsupported-expr test/utils/name_validator.ts: identifier ruleViolated is not a lowered local or module value`
5. **3** — `src/abap/5_syntax/structures/class_data.ts: value outside supported numeric, ordering or conversion domain`
6. **3** — `src/lsp/rename.ts: HIR verification: assignment type mismatch`
7. **2** — `src/objects/_abap_object.ts: src/objects/_abap_object.ts`
8. **2** — `src/objects/_abstract_object.ts: input is outside the reviewed abapGit XML subset`
9. **2** — `src/objects/_dynpros.ts: value outside supported numeric, ordering or conversion domain`
10. **2** — `src/objects/message_class.ts: value outside supported numeric, ordering or conversion domain`
11. **2** — `src/objects/rename/renamer_helper.ts: value outside supported numeric, ordering or conversion domain`
12. **2** — `src/objects/view.ts: value outside supported numeric, ordering or conversion domain`
13. **2** — `src/rules/space_before_dot.ts: value outside supported numeric, ordering or conversion domain`
14. **2** — `test/abap/syntax/when_type.ts: HIR verification: unresolved method fn_visit`
15. **2** — `unsupported-assignment test/config.ts: assignment to property applyUnspecifiedRules is not lowered`

Only progress documentation and the measurement were updated. No code or golden changed. Nothing was pushed.
