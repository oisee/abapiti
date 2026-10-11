# Empty array expressions in Domain

The shared TS→HIR lowering now constructs empty arrays in typed and tagged
expression contexts. A contextual `any`/`unknown` uses tagged elements; a
literal without an inferable element context still refuses explicitly. The
result is an ordinary fresh HIR array consumed by both backends, with no new
runtime operation. Tagged `for-of` operands receive a checked array view;
non-array payloads refuse instead of reaching an invalid HIR loop.

The actual Domain roots are `src/objects/domain.ts:114` and `:122`, the empty
fallbacks in JSON fixed-value and interval loops. Its annotated arrays, optional
array property initializer, and typed returned fallbacks were already supported.
The regression compares emitted Go with the original TypeScript executed by
Node, checks fresh returns/defaults, arguments, conditionals, optional fallbacks,
tagged boxing and JSON iteration, and verifies explicit refusals for an untyped
literal and a non-array iterable. The same HIR also emits ABAP successfully.

## Merged base

Started at `origin/wip/train-3-finish` (`a0bee5c`), then merged with `--no-ff`
in the requested order: `0de990c`, `26bcfb6`, `29f352e`, `569f496`.
Merged base: `3f94f0e`.

Code conflicts retain the union of unsupported backend operations, both chai
shape emission and live structural records, receiver-context isolation together
with pinned receiver lowering, both qualified-cast regression assertions, and
partial-shard evidence together with existing async accounting. The duplicate
SHA-1 effect declaration uses the newer pure catalogue entry. Grace catalogue
counts were regenerated and inspected; method/call-site facts are unchanged.
The first short-test run exposed a mixed related/unrelated qualified-cast
fixture in the merge resolution. `9aa9899` separates the two original cases
and verifies both; executable compiler sources are unchanged. That run also
hit the default ten-minute package timeout under host load; the final gate
uses `-timeout 60m`.

Incoming measurement documents are retained separately as
`fit-{traps-e,live-class-view,downport-args,ddic-locals}-merged-history.*`;
the train-3 measurements remain in their original files.

The baseline binary was built at `dcabc8f`, immediately before the final golden
amendment. Its executable compiler sources equal `3f94f0e`; their only tree diff
is the five regenerated catalogue counts in `lexer.facts.golden`. The final changed
compiler sources are recorded at `9cf42a9` (the initial fix is `8e7cfdf`). The parallel runner is a separate
measurement change; its default remains one worker, and it drains in-flight
work before cleanup when a genuine FAIL stops scheduling.

## Reproduce

Upstream is the clean original repository at
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`, with its locked root/core npm
dependencies. The Node oracle rebuilds the original TypeScript (6.0.3).
Set `BASE_ABAPITI` and `AFTER_ABAPITI` to binaries built from the revisions above,
and `UPSTREAM` to the pinned checkout. Measurements use two shard workers,
`GOMAXPROCS=4`, and the harness's existing `-p=2` Go build limit per worker.

```sh
export GOMAXPROCS=4 ABAPITI_TESTS_WORKERS=2
ABAPITI_TESTS_COMPILER="$BASE_ABAPITI" ABAPITI_TESTS_COMPILER_COMMIT=dcabc8f \
  node tools/abaplint-tests-all.mjs "$UPSTREAM" /tmp/empty-array/before-all.json
ABAPITI_TESTS_COMPILER="$AFTER_ABAPITI" ABAPITI_TESTS_COMPILER_COMMIT=9cf42a9 \
  node tools/abaplint-tests-all.mjs "$UPSTREAM" /tmp/empty-array/after-all.json
# Focused measurement uses the same commands with ABAPITI_TESTS_SHARDS=objects.

"$BASE_ABAPITI" abaplint --full -o /tmp/empty-array/full-base
"$AFTER_ABAPITI" abaplint --full -o /tmp/empty-array/full-after
"$BASE_ABAPITI" abaplint -o /tmp/empty-array/default-base --target all,go
cp -a /tmp/empty-array/default-base /tmp/empty-array/default-base-snapshot
"$AFTER_ABAPITI" abaplint -o /tmp/empty-array/default-base --target all,go
diff -r /tmp/empty-array/default-base-snapshot /tmp/empty-array/default-base

go vet -p 8 ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 go test -short -p 8 -timeout 60m ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 GRACE_FACTS_OUT=/tmp/empty-array/lexer-after.facts \
  go test -short -p 8 ./tsfront -run '^TestLexerFactsReport$' -count=1
go test -short -p 8 ./hir/abap -run '^TestRuntimeOpsSupportedByEveryBackend$' -v
./.github/ci/lint.sh gate 3f94f0e
node tools/abaplint-tests-all-test.mjs
```

`--full` exists in `origin/main` and measures without a reachability manifest.
Its raw blocking diagnostics and histograms, and both complete test summaries,
are recorded in `fit-empty-array-literal.json`.

## Full-closure gap

| Measure | Merged base | Changed compiler |
| --- | ---: | ---: |
| Empty-array blocking diagnostics | 2 | 0 |
| All blocking diagnostics | 48 | 46 |
| Root / cascade blockers | 35 / 13 | 33 / 13 |
| HIR errors | 117 | 117 |
| Files with blocking/HIR diagnostics | 30 | 29 |

Both measurement commands exit successfully; neither claims that full-core
emission is complete. The empty-array diagnostic category is eliminated from
this closure. The remaining HIR errors are outside Domain and require their
own lowering repairs; this change does not change their classification.

## Default output and oracle timing

The final default-output `diff -r` is empty, including generated Go, its built
binary, ABAP objects, source maps, readmes and the abapGit zip. Both versions
were generated at the same output path before comparing the base snapshot:
readmes and Go debug paths legitimately embed that path. The initial comparison
also caught a typed-array allocation in `RulesRunner` becoming tagged because
an outer hint displaced the checker's precise context. `9cf42a9` restores
context priority and adds an isolated annotated JSON fallback regression.

The first independent fresh whole-suite original-Node run passed all 11,060
active tests with 32 pending. Later whole-suite runs under load hit a few
upstream per-test 250 ms timeouts, with no assertion mismatches. Their raw
records were retained separately. The successful compiler-independent whole
reference was reused after verifying the same pin, exhaustive roots and full
title multiset. No Go result, registration, assertion or refusal was synthesized
or relabeled. Any affected shard oracle is rerun before final reconciliation.

## Local gates

All local gates passed on the final frontend sources: vet, short tests with
`-p 8` and `ABAPITI_GRACECHECK=1`, the Grace golden, the backend catalogue guard,
parallel harness accounting/drain tests, and lint against `3f94f0e` (zero new
issues, canary 5/5, golangci-lint 2.13.2). The final tsfront short-test package
took 864 seconds under load and passed with the explicit 60-minute timeout.
The default release output comparison is byte-identical.
