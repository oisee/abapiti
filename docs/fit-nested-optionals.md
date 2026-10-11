# Optional map values and reference-only local sets

Based on `6d72bfa`, on `feat/fit-nested-optionals`. The original abaplint
sources remain pinned to `577f875ebec44cfaf64841cfe71c8ab8dc32622e`
(@abaplint/core 2.120.56).

## Roots and semantics

The `unused_variables` refusal is in `buildIssues`: its
`Map<string, ABAPFile | undefined>` caches both successful and unsuccessful
file searches. The old `map.get` signature introduced
`optional<optional<ABAPFile>>`. This is a lookup representation problem;
the optional-chain expression in another method is not its root.

Frontend map reads now request one optional result, including computed and
dotted record reads. Optional record fields retain their absence instead of
being narrowed after lookup. Optional map insertion supplies the value's
context to `undefined`. HIR's map lookup signature uses the same absence
flattening already used by array lookup. Go discards the outer lookup box;
ABAP guards the physical lookup box and reads its stored optional at the call
site. The existing ABAP getter ABI is preserved. These are small emitter adaptations
for the TS→HIR representation; no new operation or source override is added.
The verifier continues to reject nested optional types.

A missing key and a key storing `undefined` both yield `undefined` from `get`.
The independent membership operation distinguishes them. Zero, false, and the
empty string remain present values, and object identity is preserved. Existing
optional-chain guards retain receiver evaluation once and skip computed keys
and call arguments when the receiver is absent. Node-oracle regressions cover
these behaviors, dotted/computed optional fields on optional records, and Go
execution. ABAP emission checks cover the corresponding lookup methods;
these frontend regressions do not claim an ABAP runtime execution result.

The `sy_read_restriction` root is the local `new Set<unknown>()` holding tokens.
A conservative frontend proof checks every symbol use in its enclosing
function, including nested scopes and closures. It requires direct reference
arguments to `add` and `has`, discarded `add` results, and no escape except size
reads. Such sets use HIR's existing root-object key type, preserving identity
across concrete classes and interface views. Aliases, returned sets, fluent
result escapes, unknown/optional inputs, and primitive keys retain refusal.
There is no rule-name check or generic dynamic-key implementation.

## Reproduction

Use Node from `~/.local/opt/node-v26.9.0-linux-x64/bin`, `GOMAXPROCS=2`,
and `GOFLAGS='-p=2 -buildvcs=false'`. Build the ordinary base and branch
executables from their respective sources. The full flag is present in
origin/main:

```sh
"$BASE_ABAPITI" abaplint --full -o "$RUN/full-base"
"$AFTER_ABAPITI" abaplint --full -o "$RUN/full-after"
```

Rules runs reuse only the existing measurement-only declaration-index overlay
from `docs/evidence/train-2-verifier-index.patch`. Body verification and
refusal semantics remain unchanged. Ordinary full/default and gate builds
have no measurement overlay. Select all sorted `test/rules/**/*.ts` roots
from a clean checkout at the upstream pin and build a fresh original Mocha
oracle:

```sh
go build -overlay "$BASE_OVERLAY" -o "$BASE_TEST_ABAPITI" ./cmd/abapiti
go build -overlay "$AFTER_OVERLAY" -o "$AFTER_TEST_ABAPITI" ./cmd/abapiti
node tools/abaplint-tests-closure.mjs "$CLOSURE" "$UPSTREAM" $files
node tools/abaplint-tests-oracle.mjs "$NODE_JSON" "$UPSTREAM" $files
"$TEST_ABAPITI" abaplint-tests "$CLOSURE" -o "$OUT" > "$GO_JSON" 2> "$LOG"
node tools/abaplint-tests-report.mjs "$NODE_JSON" "$GO_JSON" "$REPORT_JSON"
```

## Full-closure measurement

| Measure | Before | After |
| --- | ---: | ---: |
| Blocking diagnostics | 35 | 35 |
| Root / cascade blockers | 29 / 6 | 29 / 6 |
| HIR errors | 150 | 138 |
| Nested optional errors | 3 | 0 |
| Related optional-string narrowing errors | 2 | 0 |
| Dynamic collection-key errors | 5 | 0 |
| Related token-to-dynamic key argument mismatches | 2 | 0 |
| Files with errors | 41 | 38 |

The exact blocking-message histogram is unchanged. Neither target category
was a frontend blocking diagnostic: both were HIR verification errors.
All other HIR message counts are unchanged. This removes the full-core roots
in `unused_variables`, `uncaught_exception`, and `sy_read_restriction`.
The JSON companion records both complete blocking-message and HIR-message
histograms. The remaining 35 blockers and 138 HIR errors concern the existing
unsupported operations, representations, numeric proofs, and type mismatches
outside this task.

## Rules-shard measurement

The complete 192-file rules shard reconciles against a fresh original Mocha
oracle: 3,159 tests, 3,149 passed, ten pending, zero failed. An initial oracle
attempt hit the upstream 250 ms timeout in its first cold 7bit_ascii case;
the unchanged oracle rerun passed. Both attempts have identical ordered titles.
Neither assertions nor original test sources/timeouts were changed.

| Measure | Before | After |
| --- | ---: | ---: |
| Pass | 1,587 | 1,782 |
| FAIL | 0 | 0 |
| Trap | 1,550 | 1,355 |
| Not-run | 22 | 22 |
| Registered | 3,147 | 3,147 |
| Async pending | 0 | 0 |

Both Go runs have identical ordered titles, reconcile without unmatched titles,
and preserve every previously passing test. All 195 extra passes are original
abaplint tests. This measures 3,159 of the full suite's 11,060 tests; no other
shard is touched by these roots.

All **147 async unused-variable cases freed by the registry fix now PASS**.
The whole shard previously reached **150** unused-variable nested-optional
traps: the other three are synchronous quickfix cases, which use
`testRuleFixSingle` and `Registry.parse()`. Those now reach
`src/edit_helper.ts:227:5: HIR verification: foreach requires matching array`
in `EditHelper.deleteStatement`. They are:

- `Rule: unused_variables, single file test, quickfix simple`
- `Rule: unused_variables, single file test, quickfix with TYPE`
- `Rule: unused_variables, single file test, quickfix, chained first`

All **48** reached `sy_read_restriction` dynamic-key traps now PASS.
`uncaught_exception` loses its full-core HIR errors, but its 21 reached traps
remain at the earlier `src/ddic.ts` Go regexp refusal for `/^.?cx_.*$/i`;
two additional tests remain not-run. That excluded regexp work is unchanged.
The remaining 1,355 rules traps and 22 not-run tests belong to existing refusal
and registration categories; no assertion failure was reclassified.

The rules executables were built with the final TS→HIR, HIR contract, and Go
emitter sources. The subsequent ABAP-only physical-ABI correction affects
`hir/abap` emission, which `abaplint-tests` does not call. The final ordinary
compiler rebuild, full-closure rerun, short/Grace/backend gate, and default
output diffs cover that correction. The JSON companion records the measured
executables separately from the final ordinary executable.

## Gates

```sh
go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 go test -short -p 2 -timeout 30m \
  ./hir/... ./tsfront/... ./internal/... ./cmd/...
```

The short suite includes
`TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`. Compare ordinary
executables from `6d72bfa`, origin/main, and the branch using identical working
directory and output spelling, moving each completed output aside:

```sh
"$ABAPITI" abaplint -o out --target all,go
diff -r "$RUN/default-base" "$RUN/default-after"
diff -r "$RUN/default-main" "$RUN/default-after"
```

The main compiler is built from the pinned `23b3640` source snapshot using a
source-only Go overlay over the isolated base checkout, to reuse unchanged
compiler dependency builds. That overlay contains the exact differing files
from origin/main, including deletions; it adds no instrumentation. The base
and branch ordinary builds use their respective source trees directly.

All final gates pass. Both final default diffs are empty across all **6,348
files**, including the Go executable and A4H zip. No golden or oracle fixture
was changed.
