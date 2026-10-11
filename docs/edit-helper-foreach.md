# Indexed array iteration in edit helpers

Base: `ce2f789946c6fe17259856e433145c3eeaa7258e` (integration train 2).
Upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

The pinned `src/edit_helper.ts:227` is `for (const e of edit[filename])`.
`IEdit` has a string index signature with array values. The frontend preserves
missing keys through `map.get`, whose HIR result is `optional<array<ITextEdit>>`.
Previously `forOfStatement` sent that optional receiver to HIR `ForEach`, which
requires an array matching the binding type. This trapped the entire
`applyEditSingle` body before its first statement.

`forOfStatement` now narrows optional array receivers to the array reference
before selecting the element type. Both backends already support this narrowing;
no backend/runtime operation, callback lowering, or upstream override changes.
The receiver is evaluated once, and the existing array and its elements are
preserved. Missing arrays remain absent rather than becoming empty arrays.

The regression verifies HIR, ABAP emission, and generated Go execution for
reference and numeric record values, empty arrays, and a receiver with a side
effect. It fails on the base with three matching-array errors and passes with
the fix.

## Whole-closure evidence

Commands (unmodified source-built executables):

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"
export GOMAXPROCS=2 GOFLAGS='-p=2 -buildvcs=false'
"$BASE_ABAPITI" abaplint --full -o "$RUN/full-before"
"$AFTER_ABAPITI" abaplint --full -o "$RUN/full-after"
```

| Measurement | Before | After |
| --- | ---: | ---: |
| Blocking lowering diagnostics | 35 | 35 |
| Root / cascade blockers | 29 / 6 | 29 / 6 |
| HIR errors | 151 | 148 |
| Matching-array HIR errors | 3 | 0 |
| Lowering blockers + HIR errors | 186 | 183 |

Blocking diagnostics are identical by message. The sole HIR message reduction
is three occurrences of `foreach requires matching array`; no new HIR messages.
The generally fixed roots are `src/edit_helper.ts:227`,
`src/ddic_references.ts:87`, and `src/rules/cyclic_oo.ts:122`.
[Compact evidence](edit-helper-foreach.json) records all diagnostic message counts.

## Harness method

Complete sorted file selections and fresh original Node Mocha/chai oracles
are used for RULES, `test/edit_helper.ts`, and the complete syntax shard.
Independent generated Go binaries execute RULES, Edit Helper, and the direct
syntax caller file `test/abap/syntax/syntax.ts` with its imports. The complete
syntax shard is blocked before execution as described below. Edit Helper and
the syntax caller contain the direct calls to `applyEditSingle` outside RULES. LSP's test helper applies LSP edits through its own implementation.
Imports can register tests shared between shards, so shard totals are not summed.
Titles are reconciled as a multiset. Assertion mismatches remain failures;
pre-assertion refusals remain traps. No oracle results replace Go execution.

Harness compiler builds use the train's documented
[measurement-only verifier-index patch](evidence/train-2-verifier-index.patch)
in disposable Go build overlays. Body checks are unchanged, with live
class/interface pointers. Fresh/indexed verification matched in 1,158 checks,
including field/method removals. Superseded unindexed harness attempts were
stopped before execution and contribute no results. Code gates, full-closure
measurement, and default-output comparison use ordinary compiler builds.

```sh
go build -overlay "$BASE_OVERLAY" -o "$BASE_TEST_ABAPITI" ./cmd/abapiti
go build -overlay "$AFTER_OVERLAY" -o "$AFTER_TEST_ABAPITI" ./cmd/abapiti
# files: every .ts in test/rules/ or test/abap/syntax/, sorted;
# for the Edit Helper shard: test/edit_helper.ts;
# for the focused syntax caller: test/abap/syntax/syntax.ts.
# Full syntax compiler attempts panic before producing runtime reports.
node tools/abaplint-tests-closure.mjs "$CLOSURE" "$UPSTREAM" $files
node tools/abaplint-tests-oracle.mjs "$NODE_JSON" "$UPSTREAM" $files
"$TEST_ABAPITI" abaplint-tests "$CLOSURE" -o "$OUT" \
  > "$GO_JSON" 2> "$LOG"
node tools/abaplint-tests-report.mjs "$NODE_JSON" "$GO_JSON" "$REPORT_JSON"
```

## RULES execution

| Outcome | Before | After |
| --- | ---: | ---: |
| PASS | 1,235 | 1,371 |
| FAIL | 0 | 0 |
| TRAP | 1,902 | 1,766 |
| NOT-RUN | 22 | 22 |
| Registered / oracle total | 3,147 / 3,159 | 3,147 / 3,159 |

All 139 targeted occurrences reached the edit-helper trap on the base.
Afterwards 136 PASS and three reach
`src/rules/avoid_use.ts:72:3` (`AvoidUse.runParsed`):
`value outside supported numeric, ordering or conversion domain`.
These are the `DESCRIBE TABLE foo LINES bar`,
`DESCRIBE TABLE foo-bar LINES bar`, and lowercase equivalents' quick fixes.
Their edit-helper trap is gone; executing the edited program exposes a later
conversion refusal in the rule. It remains a TRAP, not an assertion mismatch. The same refusal affects 17
other RULES tests on the base (20 afterwards, including these three).
No previously passing RULES test regressed. Async-pending is zero, and all
results reconcile to the original oracle with no unmatched titles.

## Direct Edit Helper execution

The complete `test/edit_helper.ts` shard has 14 original tests. Baseline:
0 PASS, 0 FAIL, 14 TRAP, 0 NOT-RUN. Afterwards: **14 PASS**, 0 FAIL,
0 TRAP, 0 NOT-RUN. All 14 formerly reached the line-227 trap; all now
pass their original assertions. Registration is 14/14, async-pending and
unmatched titles are zero. No passing test regressed.

## Syntax coverage and remaining compiler blocker

The complete syntax shard's original Node oracle has 1,379 tests: 1,369 PASS,
zero FAIL, and 10 pending. Both base and branch compiler attempts panic before
execution: `Unhandled case in Node.Text: *ast.QualifiedName`, through
`typeRefName` at `tsfront/lower_types.go:1017`. Qualified assertions such as
`as Basic.TableType` in `basic_types.ts` and `basic_variables.ts` expose this
pre-existing type-name handling gap. Both attempts therefore have 0 PASS,
0 FAIL, 0 runtime TRAP, and **1,379 NOT-RUN**; no registrations or executed
results are invented from the oracle.

The unchanged `test/abap/syntax/syntax.ts` file containing the edit application
is measured separately with all of its imports. This is a focused caller
closure, not execution of the complete syntax shard. Type-name rendering and
assertion semantics are outside the optional-array root fix and remain unchanged.

The focused caller closure has 1,153 oracle tests (1,143 PASS, zero FAIL,
10 pending). Both Go executions have **0 PASS, 0 FAIL, 1,104 TRAP,
49 NOT-RUN**, with 1,112 registrations and zero async-pending or unmatched
results. No test reaches the edit-helper array mismatch: 1,065 stop in `run`
at `test/abap/syntax/syntax.ts:34:28` because `JSON` is not lowered; 38 reach
artifact loading, and one reaches a separate test-body verification trap.
NOT-RUN comprises eight registered skips and 41 unregistered occurrences,
with a recorded closure/module-reference mismatch during registration at
`syntax.ts:16428`. These earlier barriers are outside this fix.

## Gates

Passed:

```sh
go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 go test -short -p 2 \
  ./hir/... ./tsfront/... ./internal/... ./cmd/...
```

The short gate includes the default backend-coverage guard and Grace facts
golden. No golden changes. Default builds from the base and branch, each with
`abapiti abaplint -o "$RUN/default" --target all,go` and the same output spelling,
have an empty `diff -r` across all 6,348 files. A separately built frozen
`origin/main` (`23b364090e3b9c699ddb768e5a795fa283ca105b`) also has an empty
default-output `diff -r` against the branch.

Only the compact evidence is retained; all scratch source archives, closures,
generated targets, executables, raw reports, overlays, and logs were removed.
