# Integration train 2

Branch `integ/train-2`, based on train-1 `bbb7344`.

## Ordered merges and conflict resolution

| Order | Source | Merge |
| ---: | --- | --- |
| 1 | `feat/fit-settled-transitive` at `88f8873` | `113b69d` |
| 2 | `feat/harness-async-v2` at `90cdeec` | `a6ab6f0` |
| 3 | `feat/fit-parsetype` at `d539061` | `ade9d41` |
| 4 | `feat/fit-fail-triage` at `6f5236b` | `7d1b430` |

All four use `git merge --no-ff` in the requested order.

- Settledness: `tsfront/lower.go` retains both generator state and the new
  transitive dispatch/proof/world state.
- Async harness: the source branch carries the older `f51056b` harness stack.
  Resolved its conflicts by applying its complete `f51056b..90cdeec` feature
  delta onto the integrated tree, retaining the merge ancestry. This preserves
  train-1's runtime/emitter/frontend behavior in `hir/golang/{emit,ops}.go`,
  `hir/runtime.go`, `tsfront/lower_{class2,expr,hof}.go`, and the lexer golden.
  The async delta combines cleanly with train-1's reporting scripts,
  `abaplint_tests{,_test}.go`, `lower_closure.go`, `static_init.go`, mocha shim,
  `tests_calls.go`, and registration tests. Historical `abaplint-tests.md` and
  `abaplint-tests-latest.json` retain train-1's measurements; separate async
  documents retain the feature's own measurements. This report records the
  integrated results.
- Parsetype: `lower_closure.go` retains the property-thunk and Mocha paths,
  Promise draining, and test-only thunk result normalization; ordinary thunk
  calls evaluate receivers and all extra arguments through the arity adapter.
  `lower_hof.go` retains the new default/rest/omitted-tail callback adapter and
  train-1's branching-return lifting. The existing single fallback for unused
  element parameters replaces the earlier redundant fallback.
- Fail triage: retained the integrated lexer golden for one final facts export.
  The branch supplies an isolated patch rather than changing the live harness.
  Applied its `rangeFault` classification at initialization and body/hook
  recovery boundaries, preserving async-pending and done-callback distinctions.
  Assertion exceptions retain FAILED status.

## Environment and regeneration

Node is on PATH via `~/.local/opt/node-v26.9.0-linux-x64/bin`.
All Go commands use `GOMAXPROCS=2` and
`GOFLAGS='-p=2 -buildvcs=false'`.

The disposable upstream checkout is pinned to
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`, using installed upstream npm
packages through dependency symlinks. The original source checkout is untouched.

Regenerated fixture oracles once against that checkout:

```sh
node tools/syntax-features-oracle.mjs "$UPSTREAM"
node tools/registry-features-oracle.mjs "$UPSTREAM"
node tools/string-repeat-match-oracle.mjs "$UPSTREAM"
node tools/regexp-exec-oracle.mjs "$UPSTREAM"
ABAPITI_GRACECHECK=1 GRACE_FACTS_OUT=/tmp/train2-evidence/lexer.facts \
  go test -short -p 2 ./tsfront -run '^TestLexerFactsReport$'
```

All fixture oracles and the Grace lexer facts are byte-identical to train-1.

## Gates and default output

| Gate | Result |
| --- | --- |
| `go vet ./...` | PASS |
| `go test -short -p 2 ./...` | PASS |
| `ABAPITI_GRACECHECK=1 go test -short -p 2 -timeout 30m ./tsfront -run '^(TestEmitRegistry\|TestLexerFactsReport$)'` | PASS |
| `ABAPITI_GRACECHECK=1 go test -short -p 2 -timeout 30m ./hir/rewrite/... ./internal/gracecheck/... ./internal/hirclone/...` | PASS |
| `go test -short -p 2 ./cmd/abapiti -run '^TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend$' -v` | PASS, executed rather than skipped |
| Node report/summary contract | PASS |
| Node refusal contract and Go refusal/assertion execution | PASS |
| Node test-host runtime contract | PASS, eight scenarios |
| Lexer facts and fixture oracle stability | PASS, no golden/oracle changes |
| Default all-target and Go output against `bbb7344` | PASS, empty `diff -r`, 6,348 files including Go executable |

Reference sources were extracted with `git archive bbb7344` and
`git archive origin/main`; each executable was built with `go build`.
All three executables used the same working directory and output spelling:

```sh
"$ABAPITI" abaplint -o out --target all,go
# Move out to the corresponding baseline/after/main snapshot after each run.
diff -r "$BASE_DEFAULT" "$AFTER_DEFAULT"
```

The extra origin/main comparison is **not empty**. Main is still `7d6e5d9`
and does not contain train-1. Its differences are inherited unchanged from
`bbb7344`: closure `106339` versus `106334` and its generated names/references,
train-1's repeat/RegExp runtime and match metadata, associated profiling/source
metadata, the A4H archive, and Go executable. Because every train-2 output byte
matches train-1, none of these differences is introduced by train-2. The
requested train-specific default-output gate uses `bbb7344` and passes.

## Full closure

Commands, each with the corresponding exact source-built executable:

```sh
"$BASE_ABAPITI" abaplint --full -o /tmp/train2-evidence/full-base
"$AFTER_ABAPITI" abaplint --full -o /tmp/train2-evidence/full-after
```

Both exit 0 in measurement mode; the remaining diagnostics prevent full-core
emission. `docs/train-2.json` records exact blocking-message histograms.

| Measure | Before `bbb7344` | After `c46cfb2` |
| --- | ---: | ---: |
| Blocking diagnostics | 35 | 35 |
| Root blocking | 29 | 29 |
| Cascade blocking | 6 | 6 |
| HIR verification errors | 173 | 151 |
| Source files with blocking/verification errors | 47 | 41 |
| Call-arity verification errors | 22 | 0 |
| Async blocking diagnostics | 0 | 0 |

The exact blocking-message histogram is unchanged. All 22 arity failures are
removed from eight source roots:

| Root | Removed arity errors |
| --- | ---: |
| `src/ddic.ts` | 10 |
| `src/abap/5_syntax/expressions/select.ts` | 6 |
| `src/abap/5_syntax/expressions/_check_database_fields.ts` | 1 |
| `src/abap/5_syntax/expressions/sql_compare.ts` | 1 |
| `src/abap/5_syntax/global_definitions/find_global_definitions.ts` | 1 |
| `src/lsp/help.ts` | 1 |
| `src/objects/lock_object.ts` | 1 |
| `src/objects/maintenance_and_transport_object.ts` | 1 |

The remaining full-closure diagnostics are existing unsupported operations and
representation checks: LSP/vscode types and helpers, generic JSON/Object and
XML/crypto helpers, ToPrimitive coercions, numeric integral-result proofs,
primitive-array absence handling, dynamic calls and shape/collection typing.
Verification still finds checked numeric conversion failures, assignment and
call type mismatches, unresolved fields, missing expressions, and other
unsupported representations. These roots belong to other workstreams and remain outside this integration
task.

## Harness measurement method

All four shards use sorted complete file selections, fresh original Mocha/chai
oracles, and independent generated-Go binaries. The oracle pin is the same for
before and after. Full titles are reconciled as a multiset, including duplicate
occurrences; unregistered titles count as not-run. Shard builds are sequential.
These four shards contain 8,461 original tests, not the entire 11,060-test suite.

For the harness only, disposable Go build overlays reuse the declaration-name
maps in `VerifyBody` after signature repair, as suggested by
[the feature measurement recipe](fail-triage.md). `verifier.body` is unchanged
and runs for every original body in the original order. Class/interface entries
remain live pointers, so field and method removals are still observed. The index
is local to one `repairFailedBodies` invocation; it is rebuilt after each later
emitter refusal. Fresh/cached verification matched in 1,158 checks on real HIR
fixtures, including field/method removals. The first unoptimized baseline rules
attempt was stopped before execution; it contributes no counts.

The overlays, equivalence probe and measurement executables are scratch files.
The exact [measurement-only patch](evidence/train-2-verifier-index.patch) is
retained for reproducibility (`git apply --unidiff-zero` in a disposable
archive); it is not applied to the published compiler. All code gates and default/full-closure measurements use
the original source-built executables without overlays. The measured production
code revision is `c46cfb2`.

Harness build and execution commands follow this pattern:

```sh
go build -overlay "$BASE_OVERLAY" -o "$BASE_TEST_ABAPITI" ./cmd/abapiti
go build -overlay "$AFTER_OVERLAY" -o "$AFTER_TEST_ABAPITI" ./cmd/abapiti
# Base tools and runtime come from the bbb7344 source archive.
node "$SOURCE/tools/abaplint-tests-closure.mjs" "$CLOSURE" "$UPSTREAM" $files
node tools/abaplint-tests-oracle.mjs "$NODE_JSON" "$UPSTREAM" $files
"$TEST_ABAPITI" abaplint-tests "$CLOSURE" -o "$OUT" \
  > "$GO_JSON" 2> "$LOG"
node tools/abaplint-tests-report.mjs "$NODE_JSON" "$GO_JSON" "$REPORT_JSON"
node tools/abaplint-tests-summary.mjs "$RUN" "$SUMMARY_JSON" "$PREFIX"
```

The generated-Go command may exit 1 when tests fail, trap, or remain
async-pending; counts come from its actual JSON result and independent oracle
reconciliation. A harness build/refusal error without that JSON is not a
measured shard.


## Four-shard N table

Every count below comes from the fresh executions above. FAIL means an
issue/assertion or observable test-behavior mismatch, not a compiler/domain
refusal. The baseline's 20 raw FAILED statement records all have exactly
`src/config.ts:11:3: value outside supported numeric, ordering or conversion domain`,
the emitted `rangeFault` guard before assertions. They are shown as TRAP here.
`baselineRawTestCounts` and the original refusal records in `train-2.json`
retain the raw baseline counters (668 passed / 20 mislabeled refusals / 6,291
raw traps / 1,482 not-run). Train-2 directly reports those refusals as trapped.
No refusal becomes a pass.

Cells show **before → after**. The four-shard total is unique; no smoke subset
or compiler-regression tests are added to N.

| Shard | Files | Passed | FAIL | Trapped | Async-pending | Not-run | Node total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| rules | 192 | 10 → 1,235 | 0 → 0 | 1,669 → 1,902 | 0 → 0 | 1,480 → 22 | 3,159 |
| abap/lexer | 3 | 200 → 200 | 0 → 0 | 0 → 0 | 0 → 0 | 0 → 0 | 200 |
| abap/statements | 286 | 434 → 4,942 | 0 → 0 | 4,528 → 20 | 0 → 0 | 0 → 0 | 4,962 |
| abap/structures | 30 | 24 → 138 | 0 → 0 | 114 → 0 | 0 → 0 | 2 → 2 | 140 |
| **Unique total** | **511** | **668 → 6,515** | **0 → 0** | **6,311 → 1,922** | **0 → 0** | **1,482 → 24** | **8,461** |

**5,847 additional passes.** All 1,458 formerly excluded async callbacks now
execute; none is async-pending. Registration is unchanged at 8,448, with zero
unmatched multiset titles in every shard. Node has 8,450 passes, 11 pending,
and zero failures. Structures improve from 24 to 138 passes.

The generated-Go rules and statements commands exit 1 because their summaries
contain traps, not assertion failures. Lexer exits 0 before/after; structures
exits 1 before and 0 after. All eight runs produce valid, reconciled N reports.
These measurements do **not** claim all abaplint tests pass or all core bodies
are translated.

Not-run after comprises **11 skipped tests and 13 unregistered tests**:
10 lost keyword-case registrations at `test/rules/keyword_case.ts:289` and
`:304` (plain spread config values cannot become nominal config references),
two at `test/rules/line_length.ts:17` (the scalar assignment result cannot become
a config reference), and one at `test/abap/structures/_combi.ts:276`
(block-local class `Normal` needs constructor identity/capture support).
Initialization refusals remain explicit; no async initialization failures occur.

## Top 10 reached traps after

Counts are actual affected test executions, excluding initialization failures.
Messages are grouped with the existing summary tool's source/diagnostic
normalization. The JSON retains full representative messages and shard counts.

| Rank | Tests | Reached trap |
| ---: | ---: | --- |
| 1 | 736 | `src/registry.ts:370` — `findIssues` body: HIR assignment type mismatch |
| 2 | 296 | `test/rules/downport.ts:13` — helper cannot lower `JSON` / `conf702` |
| 3 | 281 | `src/artifacts_objects.ts:8` — pinned artifacts-object refusal |
| 4 | 139 | `src/edit_helper.ts:220`, failure at `:227` — foreach requires matching array |
| 5 | 104 | `src/rules/7bit_ascii.ts:38` — JSON rule-config reference conversion refusal |
| 6 | 79 | `src/rules/obsolete_statement.ts:157`, failure at `:237` — optional dynamic cannot become optional text-edit map |
| 7 | 22 | `src/ddic.ts:30`, failure at `:63` — unsupported Go regexp `/^.?cx_.*$/i` |
| 8 | 20 | `src/config.ts:11` — explicit version/language config conversion refusal (statements) |
| 9 | 20 | `src/rules/abapdoc.ts:45` — unreviewed JavaScript regexp |
| 10 | 20 | `src/rules/empty_structure.ts:68`, failure at `:132` — StatementNode / optional union-interface type mismatch |

`selection_screen_naming` also has 20 reached traps and falls just outside the
top ten under the summary tool's deterministic tie ordering. Remaining work
includes these typing/projection and reviewed-regexp gaps, generic JSON helpers,
artifacts coverage, and the lost registrations above. They are preserved
refusals belonging to subsequent workstreams. The other 2,599 tests outside
these four selected shards were not measured by this train.

Compact evidence is in [train-2.json](train-2.json). All requested code gates
pass; the default output against `bbb7344` is unchanged. The full-core and
original-test measurements remain partial as reported above.
