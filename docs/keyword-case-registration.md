# keyword_case registration and exact-title accounting

Upstream: abaplint `577f875e` (`test/rules/keyword_case.ts`). Host: OSG
`b32d0f60`, the unchanged CI pin.

## Before

The focused original compiler run has 114 Node-active titles, 104 OSG passes,
no FAILs or test-body traps, and **10 unregistered** titles. The corrected
harness exits 1. The full-map input supplied for the investigation likewise
reconciles to 3,149 active titles: 3,133 passes, six traps, and ten unregistered.
Its ten `not-run` results are all Node-pending, not the missing active tests.

The host retains two initialization refusals, at `test.initialize.26` and
`test.initialize.29`: the two `testRule` calls for `keywordCase: lower` and
`keywordCase: lower + ignore boundaries`. Their config initializers succeed;
registration fails when `setConfig` crosses the erased interface boundary.

## Cause and repair

Object spread constructed a synthesized structural class by copying typed
fields, but `literalOwnKeys` deliberately returned no static proof for a spread.
Consequently the generated instance had no own-property presence/order metadata.
The ABAP structural-storage `get` adapter refused that source when the config
view read it. Initialization recovery retained the exceptions and continued;
no `it` calls for either five-case suite were reached. Go's typed source reads
did not have that metadata guard, so the original Go run registered the suites.
This was not title escaping, newline normalization, or deduplication.

Structural spread literals now build a runtime record snapshot and expose its
typed data view. Each source is evaluated once; enumerable own keys and values
are copied in runtime order, before later properties override values. Nullish
optional sources contribute no keys. Explicit undefined remains a present key,
and nested reference values retain shallow-copy identity. Unknown native own-key
layouts still refuse rather than guessing from destination fields.

The Node-oracle regression covers inherited config defaults, both lower-style
config shapes, erased interface assignment, shallow aliasing, optional spreads,
present-undefined keys, source/override evaluation order, and map-built rows with
quoted multi-line text. It also exports an ABAP host fixture.

## Accounting

`reconcileResults` is shared by the report CLI and summary. Summary recomputes
from raw Node/host JSON rather than trusting saved report counts. Exact titles
are multiset keys, including duplicates, quotes, whitespace and newlines.
Missing occurrences are unregistered, never manufactured host results. The
all-tests command rejects failed registration parity. Its regression includes
an equal-count host report with one extra result and one missing occurrence of
a quoted multi-line title; even a stale report claiming zero cannot hide it.

`ABAPITI_TESTS_KEEP_ARTIFACTS=1` now retains focused closure/build files as well
as reports, so registration failures remain inspectable.

## Verification

| Measurement | Pass / active | FAIL | Active unregistered | Initialization refusals |
| --- | ---: | ---: | ---: | ---: |
| Original OSG keyword_case | 104 / 114 | 0 | 10 | 2 |
| Repaired OSG keyword_case | 114 / 114 | 0 | 0 | 0 |
| Original Go keyword_case | 114 / 114 | 0 | 0 | 0 |
| Repaired Go keyword_case | 114 / 114 | 0 | 0 | 0 |
| Repaired OSG abap/lexer (all three files) | 200 / 200 | 0 | 0 | 0 |
| Repaired Go abap/lexer (all three files) | 200 / 200 | 0 | 0 | 0 |

Both hosts reconcile exact title multisets. The standalone spread regression
also passes its one OSG test; its 23 generated ABAP files lint with zero issues.
The repaired OSG measurement uses compiler commit `7db2792`. The preceding Go
measurement compiled the identical lowering worktree before that commit;
its summary's HEAD field still identifies the accounting commit `3648491`.

An extra run with the opt-in `ABAPITI_OSG_HOME` set during the short suite exposed
an existing failure in `TestParsedNestedObjectABAPHost`: native nil dereference
in `runtime.dynamic.get`. That probe's TypeScript has no object spread or object literal expressions
(the records are JSON strings), so none of the changed literal-lowering paths
are entered. It is not certified green here. Its failure log is retained; the
standard short CI suite is run without the optional host-integration environment.
The dedicated keyword_case and lexer host measurements retain the pinned OSG
opt-in. No test was disabled or changed to hide this separate failure.

Required CI checks:

- `go vet ./...`: exit 0.
- `GOFLAGS=-buildvcs=false go test -short -p 2 ./hir/... ./tsfront/ ./cmd/... -count=1`
  in standard short-suite mode (no optional host integration): exit 0.
- Each of the 15 `tools/*-test.mjs` scripts individually, with
  `GOTOOLCHAIN=go1.26.0`: exit 0.
- `HIR_RUNTIMES=" " GOFLAGS=-buildvcs=false .github/ci/hir-unit.sh "$w"`, with
  `open-steamgate` linked to the unchanged CI pin: exit 0; every lint invocation
  reports zero issues, including v702.
- `.github/ci/lint.sh gate origin/main`: zero new issues, canary 5/5,
  golangci-lint 2.13.2.

Heavy measurements/checks use the shared flock, 48 GiB scope, nice 10, and
load admission below 12. Evidence directories are retained in RAM; the temporary
workspace and OSG's generated Go cache are cleaned. OSG source files are unchanged.
