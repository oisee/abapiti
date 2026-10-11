# Downport call receiver context

The merged base is `5767b63737090241e1725edf12ec0bd2a27dd60d`: train-3
`648101e` plus trap batch B `2169c5d`, merged with `--no-ff`. Both operation
lists, summary accounting and historical reports are retained. The qualified
assertion regression is reconciled with batch B's checked class-view domain;
valid qualified/const-alias views retain identity, invalid views refuse in Go,
and ABAP refuses the unproven view. This changes test expectations only.

## Root

`src/rules/downport.ts:738–742` builds an array of the arrays returned by
`findAllExpressionsRecursive(SQLTarget/SQLSource/SQLSourceNoSpace/SQLSourceSimple)`.
The enclosing call result is `ExpressionNode[]`, but its receiver literal is
`ExpressionNode[][]`. Lowering passed the result hint into that receiver,
creating an outer `ExpressionNode[]` and four invalid `array.push` arguments.
Arity and the query-method signatures are correct; the receiver selected the
wrong runtime ABI. The harness refused the whole `downportSQLExtras` body
starting at line 713, even before its early guards could run.

Library and virtual-call receivers now lower without the surrounding result
hint, then restore it for arguments/result handling. The shared HIR fix applies
to both backends. No builtin implementation, overload, callee ABI, source-site
exception or reachability override changes. A Node oracle checks nested array
receivers in return, argument and virtual-call contexts and once-only effects;
the same HIR emits successfully to ABAP. The regression fails on the merged
base with the original argument mismatches and passes with the fix.

## Full closure

[Complete blocking-message histograms and HIR delta](fit-downport-args-gap.json):
blocking **50→50** (36 root, 14 cascade); HIR errors **127→122**; call argument
type mismatches **23→19**; the nested `ExpressionNode` array mismatch **4→0**.
The fifth removed error is a receiver-related narrowing in
`ambiguous_statement.ts:95:29`. No new HIR errors appear.

```sh
$BASE_ABAPITI abaplint --full -o /tmp/downport-evidence/full-base
$AFTER_ABAPITI abaplint --full -o /tmp/downport-evidence/full-after
```

`--full` is present in origin/main and disables the reachability manifest.
These are diagnostic measurements, not successful full-core emissions.

## Baseline tests

[Focused rules evidence](fit-downport-args-base.json): **2,301 passed, FAIL 0,
836 trapped, 22 not-run**, against 3,149 active Node tests and 10 pending.
The merged base reaches this Downport mismatch in **224** tests; the earlier
194 figure predates the combined base's other fixes. All 224 are original
`Rule: downport` titles. The whole Downport title group has 31 passes,
267 traps and five not-run cases before this fix.

```sh
# In the disposable merged-base source snapshot. The temporary runner adds
# ABAPITI_TESTS_SHARDS filtering to plan.shards; committed tooling is unchanged.
ABAPITI_TESTS_SHARDS=rules ABAPITI_TESTS_SCRATCH=/tmp/downport-evidence/rules-base-shards \
  node tools/abaplint-tests-all.mjs ~/.cache/kits/abaplint-577f875e \
  /tmp/downport-evidence/rules-base-tests.json
```

The original Node oracle is rebuilt from the clean pinned source. The selected
shard's owned results reconcile exactly with its Node oracle, with zero
unmatched titles and zero failures. The exhaustive summary correctly rejects a
selected-shard run; focused evidence records the completed report directly and
makes no whole-suite Go claim. The full 17-shard branch result follows.

## Gates

All local gates pass with Node v26.9.0 on PATH, `GOMAXPROCS=2`, and `-p 2`:

```sh
go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -timeout 30m -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 GRACE_FACTS_OUT=/tmp/downport-evidence/lexer.facts \
  go test -short -p 2 ./tsfront -run '^(TestEmitRegistry|TestLexerFactsReport$)' -count=1
ABAPITI_GRACECHECK=1 go test -short -p 2 ./hir/rewrite/... ./internal/gracecheck/... ./internal/hirclone/...
go test -p 2 ./cmd/abapiti -run '^TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend$' -count=1 -v
./.github/ci/lint.sh gate 5767b63737090241e1725edf12ec0bd2a27dd60d
```

Lint reports **0 new issues**, with all five canaries detected. The initial
short run exposed the stale qualified-cast expectation and the default
10-minute timeout; the reconciled test and the complete 30-minute rerun pass.

Both binaries run from the same disposable directory with the same output
spelling and environment. Each completed output is moved to its snapshot:

```sh
$ABAPITI abaplint -o /tmp/downport-evidence/default --target all,go
diff -r /tmp/downport-evidence/default-base /tmp/downport-evidence/default-after
```

The recursive diff is empty, including the Go executable and packaged ABAP.

## Completed branch tests

[Full 17-shard evidence](fit-downport-args-after.json): **9,573/11,060 active
tests passed (86.56%), FAIL 0, 1,214 trapped, 305 not-run**. All 625 files and
17 shards reconcile exactly with the original Node title multiset; no files are
excluded, no titles are unmatched, and no infrastructure failures remain. Node
passes 11,060 active tests with 32 pending. Go not-run is 26 skipped/focused plus
279 unregistered; these are not counted as passes. Async-pending is zero.

| Shard | Passed | FAIL | Trapped | Not-run |
| --- | ---: | ---: | ---: | ---: |
| `abap` | 55 | 0 | 26 | 139 |
| `abap/expressions` | 41 | 0 | 9 | 57 |
| `abap/lexer` | 200 | 0 | 0 | 0 |
| `abap/nodes` | 2 | 0 | 0 | 0 |
| `abap/statements` | 4942 | 0 | 20 | 0 |
| `abap/structures` | 138 | 0 | 0 | 2 |
| `abap/syntax` | 1275 | 0 | 49 | 49 |
| `abap/types` | 21 | 0 | 8 | 0 |
| `root` | 61 | 0 | 33 | 0 |
| `cds` | 334 | 0 | 24 | 1 |
| `ddl` | 4 | 0 | 28 | 0 |
| `lsp` | 65 | 0 | 123 | 5 |
| `objects` | 61 | 0 | 83 | 1 |
| `objects/rename` | 0 | 0 | 31 | 0 |
| `pretty_printer` | 0 | 0 | 2 | 29 |
| `rules` | 2366 | 0 | 771 | 22 |
| `utils` | 8 | 0 | 7 | 0 |
| **TOTAL** | **9,573** | **0** | **1,214** | **305** |

```sh
ABAPITI_TESTS_SCRATCH=/tmp/downport-evidence/after-shards \
  node tools/abaplint-tests-all.mjs ~/.cache/kits/abaplint-577f875e \
  /tmp/downport-evidence/after-tests.json
```

The first full attempt reached every shard but had two measurement failures:
the independent Node statements shard timed out one ADD test at its original
250 ms limit, and the utils harness exited 2 with a truncated runtime stack.
A fresh original-source statements oracle passes all 4,962 tests with unchanged
timeouts. The unchanged compiler freshly lowers/emits/builds/runs utils: eight
passes, seven typed traps, zero failures. The successful retry records replace
only those two records, then the **unchanged** exhaustive summary guards pass.
The first-attempt records are retained in `/tmp/downport-evidence/first-attempt`;
the utils crash cause is not established. No failing assertion is relabeled.

Retry/reconciliation commands (scripts are disposable, not source edits):

```sh
node /tmp/downport-evidence/retry-oracle.mjs
node /tmp/downport-evidence/materialize-utils.mjs
$AFTER_ABAPITI abaplint-tests /tmp/downport-evidence/utils-retry \
  -o /tmp/downport-evidence/utils-retry-out
node tools/abaplint-tests-report.mjs \
  /tmp/downport-evidence/after-shards/all-Dfi333/node-utils.json \
  /tmp/downport-evidence/after-shards/all-Dfi333/go-utils.json \
  /tmp/downport-evidence/after-shards/all-Dfi333/report-utils.json
node tools/abaplint-tests-summary.mjs \
  /tmp/downport-evidence/after-shards/all-Dfi333 \
  /tmp/downport-evidence/after-tests.json
```

## Case delta and next roots

[Per-title transitions and next-trap evidence](fit-downport-args-cases.json):
rules **2,301→2,366 passes**, **836→771 traps**, FAIL **0→0**, not-run **22→22**.
All 2,301 previously passing rules still pass, all original registered titles
match, and no case becomes failed or not-run.

The argument-mismatch category is **224→0**. Of its 224 original cases,
**56 now pass** and 168 reach these later refusals:

| Next trap | Cases | Why it remains |
| --- | ---: | --- |
| `uniqueName` (`downport.ts:3161`): unlowered `crypto`/`hash` | 140 | The Node `crypto.createHash("sha1").update(...).digest("hex")` value/API has no TS→HIR lowering. Its whole method refuses before the class-definition guard. |
| `addFix` (`downport.ts:728:20`): unresolved local `fix` | 18 | The lifted callback’s mutable optional edit capture has an unresolved HIR binding. |
| `outlineNewObject` (`downport.ts:3520`): dynamic `getParameters`, unlowered `importing` | 10 | The method on the dynamic receiver is unsupported; the later locals cascade. |

The whole Downport title group changes from **31 passes / 267 traps / five
not-run** to **87 passes / 211 traps / five not-run**, with FAIL zero. The same
general receiver fix also frees **nine ambiguous-statement rule cases** from
the related narrowing refusal. No source-site exception is introduced.
