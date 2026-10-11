# abaplint's own tests on TS-HG@Go

## Reconciled trap batches F + G — certified

**10,399/11,060 active tests passed (94.02%), FAIL 0.** All **17 shards / 625
files** completed against fresh original Node at
`577f875ebec44cfaf64841cfe71c8ab8dc32622e` (11,060 passed, zero failed,
32 pending). Go has **601 typed/domain traps, zero async-pending, 92 not-run**.
No exclusions, infrastructure/wiring failures or unmatched Go titles. Owned
and whole-suite title multisets reconcile; borrowed executions are checked
for genuine FAIL before ownership subtraction. Registration parity is still
false because the 64 explicit missing titles are not synthesized.

Not-run: **0 async/done, 28 skipped/focused, 64 unregistered**. Four missing
titles are Node pending; the other 28 Node pending titles register as skipped.
The 60 missing active titles are the 57 matcher-union cases, one block-local
class case and two primitive-config assignments. All 41 STRUCTURE titles now
register, with 39 active bodies passing and two pending.

The measured semantic source is `ac868c0`, the merge of G `dd140d6` and F
`6a20ee4`; the final amendment adds only documentation/evidence, not compiler,
runtime or tests. [Compact gates and semantic tree identities](evidence/traps-fg-gates.json),
[all-shard counts, exact missing titles and top 20](evidence/traps-fg-final.json),
and [conflict decisions / oracle audit](traps-fg-roots.md) retain the certificate.
Both parent measurements below are historical.

### Actual gains, not the additive forecast

The **~10,500 forecast is not reached**. The measured result is **+82 over G**
and **+23 over F**, not G + F's earlier +209: both branches repair the same
regex split/construction/naming callers, and removing a first trap often
exposes another ABI refusal in the same test. F's full regex grammar already
covers G's exact naming reviews; G's broader capture/empty-match/limit split
semantics do not add extra passes at the two existing capture-free split sites.
All non-overlapping code and both oracle test sets remain; no missing cases
were hidden or tests weakened to inflate this number.

| Changed shard | F passed | G passed | FG passed | FG − G |
| --- | ---: | ---: | ---: | ---: |
| `abap` | 168 | 163 | 168 | +5 |
| `abap/syntax` | 1,353 | 1,310 | 1,353 | +43 |
| `lsp` | 74 | 97 | 97 | 0 |
| `objects` | 105 | 98 | 105 | +7 |
| `rules` | 2,879 | 2,852 | 2,879 | +27 |
| **All shards** | **10,376** | **10,317** | **10,399** | **+82** |

The largest remaining root illustrates why trap counts are not pass estimates:
F had 211 ClearExportingParameters refusals; FG moves beyond that live-array
boundary but still reaches **195 Recommendations config/conversion refusals**
(up from G's 173 as F's erased regex pattern reads advance further). Domain's
empty-array lowering now executes, but 23 native-field conversion refusals
remain. These are unrepaired ABI domains, not genuine assertion mismatches.

### All 17 shards

| Shard | Files | Passed | FAIL | Trapped | Not-run | Registered | Node active + pending |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap` | 18 | 168 | 0 | 48 | 4 | 216 | 216 + 4 |
| `abap/expressions` | 11 | 42 | 0 | 8 | 57 | 50 | 107 + 0 |
| `abap/lexer` | 3 | 200 | 0 | 0 | 0 | 200 | 200 + 0 |
| `abap/nodes` | 1 | 2 | 0 | 0 | 0 | 2 | 2 + 0 |
| `abap/statements` | 286 | 4,942 | 0 | 20 | 0 | 4,962 | 4,962 + 0 |
| `abap/structures` | 30 | 138 | 0 | 0 | 2 | 139 | 139 + 1 |
| `abap/syntax` | 8 | 1,353 | 0 | 10 | 10 | 1,373 | 1,363 + 10 |
| `abap/types` | 6 | 23 | 0 | 6 | 0 | 29 | 29 + 0 |
| root | 12 | 66 | 0 | 28 | 0 | 94 | 94 + 0 |
| `cds` | 2 | 334 | 0 | 24 | 1 | 359 | 358 + 1 |
| `ddl` | 1 | 32 | 0 | 0 | 0 | 32 | 32 + 0 |
| `lsp` | 16 | 97 | 0 | 91 | 5 | 193 | 188 + 5 |
| `objects` | 26 | 105 | 0 | 39 | 1 | 145 | 144 + 1 |
| `objects/rename` | 9 | 0 | 0 | 31 | 0 | 31 | 31 + 0 |
| `pretty_printer` | 1 | 10 | 0 | 21 | 0 | 31 | 31 + 0 |
| `rules` | 192 | 2,879 | 0 | 268 | 12 | 3,157 | 3,149 + 10 |
| `utils` | 3 | 8 | 0 | 7 | 0 | 15 | 15 + 0 |
| **Total** | **625** | **10,399** | **0** | **601** | **92** | **11,028** | **11,060 + 32** |

### Top 20 reached traps

Exact normalized messages, representative locations, shard contributions and
separate initialization refusals are in the compact evidence. The generic
conversion diagnostic does not by itself identify a numeric fault; the parent
root audits preserve the concrete ABI handoffs.

| Tests | Root / remaining boundary |
| ---: | --- |
| 195 | `function_module_recommendations.ts:125`: live config/row conversion domain |
| 37 | `_abap_object.ts:148`: native string-key map receives Dynamic `id` |
| 33 | `lsp/_lookup.ts:238`: original structural own/enumerable/presence storage |
| 31 | `nesting.ts:253`: Program asserted as sibling Class; common-method view |
| 27 | `downport.ts:730`: unresolved captured `fix` |
| 23 | `domain.ts:100`: native value/optional field conversion domain |
| 22 | `config.ts:11`: version/language config conversion |
| 16 | `downport.ts:3556`: dynamic getParameters/importing bridge |
| 15 | `pretty_printer.ts:16`: constructor/config bridge |
| 15 | `rules_runner.ts:133`: conversion domain |
| 15 | `cds_parser.ts:3879`: captured parseView |
| 12 | `lsp/references.ts:40`: property-access callback invocation ABI |
| 9 | `identical_conditions.ts:23`: absence-preserving primitive find callback ABI |
| 7 | `rename_global_interface.ts:18`: assignment/return view |
| 7 | `pretty_printer.ts:161`: live erased keyword_case assignment |
| 7 | `cds_parser.ts:3822`: captured parse |
| 7 | `name_validator.ts`: patternKind structural config field; ruleViolated cascades |
| 6 | `rename_global_class.ts:18`: assignment/return view |
| 6 | `registry.ts`: captured getConfig |
| 5 | `types/form_definition.ts`: module reg binding failed readiness |

### Oracle decisions, goldens and gates

One typed split operation uses **G's implementation**. F's original capture,
word-boundary and empty-input probes now assert Node's exact supported results,
not F's former narrower-domain refusals. Retain F's bounded capture-safe grammar
and erased-string constructor arguments on the same translator, plus G's `$^`
and naming reviews. G's old `a{2}` refusal becomes a Node-certified acceptance
case; every other distinct negative and every original feature case remains.
A new bounded-split Node differential checks the combined grammar/split path.
F's helper, concat, dynamic-loop and Chai work and all G/base live-view work stay.
No genuine Go/Node value mismatch occurred in final certification.

```sh
export GOCACHE="$HOME/.cache/go-build" GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false
export ABAPITI_GRACECHECK=1 GOMAXPROCS=4
go test ./... -short -timeout=30m
go test ./tsfront/... ./hir/... ./cmd/... -timeout=30m
go vet ./...
node tools/abaplint-tests-report-test.mjs
node tools/abaplint-tests-all-test.mjs
GRACE_FACTS_OUT=<scratch>/lexer.facts go test ./tsfront -run '^TestLexerFactsReport$' -count=1
ABAPITI_TESTS_FIRST=rules,abap/syntax nice -n 10 \
  node tools/abaplint-tests-all.mjs ~/dev/abaplint docs/evidence/traps-fg-final.json
```

All gates pass. The documented facts export is byte-identical to G's committed
golden: **no golden changes**, no manual regeneration or lexer oracle edits.
Default output from an archived `origin/feat/fit-traps-g` compiler versus final
source, using the **same absolute output path**, has raw **`diff -r` exit 0**:
all **6,312 files**, including the A4H zip, are identical. There are no default
output changes requiring justification.

Fresh separately emitted Go and fresh original pinned Node are byte-identical
by direct `cmp` on zabapgit **clean 0 / seeded 5** issues. Both also match the
release kit, independently checked by `tools/go-target-check.py`. Earlier
pre-resolution gate attempts/test-harness errors are disclosed in the root
audit and discarded, not counted as certification. Only compact Markdown/JSON
is retained; generated builds, closures, binaries, kit, raw records and logs
are deleted. One merge commit, clean worktree, no push or transpiler clone.

## Trap batch G — historical, after live typed arrays

**10,317/11,060 active tests passed (93.28%), FAIL 0.** All 17 shards and
625 selected files completed on compiler **`b0570c4`**, based on **`350034c`**
(yolo-4 + live views 2). Upstream remains
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. Fresh original Node: 11,060 passed,
zero failed, 32 pending. Go: 644 typed/domain traps, zero async-pending,
131 not-run. No infrastructure failures, exclusions or unmatched titles.

Not-run is **0 async/done, 26 skipped/focused, 105 unregistered**. It includes
all 32 Node pending titles and 99 active titles. Registration loss is not a
file-selection omission. Owned-title accounting still subtracts the six
independently identified borrowed SQL-value-conversion tests from syntax/LSP;
all borrowed executions are checked for FAIL before subtraction.

| Shard | Files | Passed | FAIL | Trapped | Not-run | Registered | Node active + pending |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap` | 18 | 163 | 0 | 53 | 4 | 216 | 216 + 4 |
| `abap/expressions` | 11 | 42 | 0 | 8 | 57 | 50 | 107 + 0 |
| `abap/lexer` | 3 | 200 | 0 | 0 | 0 | 200 | 200 + 0 |
| `abap/nodes` | 1 | 2 | 0 | 0 | 0 | 2 | 2 + 0 |
| `abap/statements` | 286 | 4,942 | 0 | 20 | 0 | 4,962 | 4,962 + 0 |
| `abap/structures` | 30 | 138 | 0 | 0 | 2 | 139 | 139 + 1 |
| `abap/syntax` | 8 | 1,310 | 0 | 14 | 49 | 1,332 | 1,363 + 10 |
| `abap/types` | 6 | 23 | 0 | 6 | 0 | 29 | 29 + 0 |
| root | 12 | 66 | 0 | 28 | 0 | 94 | 94 + 0 |
| `cds` | 2 | 334 | 0 | 24 | 1 | 359 | 358 + 1 |
| `ddl` | 1 | 32 | 0 | 0 | 0 | 32 | 32 + 0 |
| `lsp` | 16 | 97 | 0 | 91 | 5 | 193 | 188 + 5 |
| `objects` | 26 | 98 | 0 | 46 | 1 | 145 | 144 + 1 |
| `objects/rename` | 9 | 0 | 0 | 31 | 0 | 31 | 31 + 0 |
| `pretty_printer` | 1 | 10 | 0 | 21 | 0 | 31 | 31 + 0 |
| `rules` | 192 | 2,852 | 0 | 295 | 12 | 3,157 | 3,149 + 10 |
| `utils` | 3 | 8 | 0 | 7 | 0 | 15 | 15 + 0 |
| **Total** | **625** | **10,317** | **0** | **644** | **131** | **10,987** | **11,060 + 32** |

### Top 15 reached traps (initialization failures excluded)

Exact normalized messages, representative locations, per-shard contributions
and initialization refusals are in [compact evidence](evidence/traps-g-final.json).
These are remaining ABI/domain refusals, not estimated future passes.

| Tests | Message / location |
| ---: | --- |
| 173 | `function_module_recommendations.ts:125`: conversion/domain refusal |
| 50 | `object_naming.ts:126`: RegExp pattern not a lowered string; missing regex local cascades |
| 38 | `objects/domain.ts:114/122`: contextual empty-array fallback |
| 37 | `objects/_abap_object.ts:148`: string <- dynamic |
| 33 | `lsp/_lookup.ts:238`: own-key enumeration requires original structural storage |
| 31 | `test/abap/nesting.ts:253`: config conversion/domain refusal |
| 27 | `rules/downport.ts:730`: unresolved captured fix |
| 22 | `config.ts:11`: conversion/domain refusal |
| 16 | `rules/downport.ts:3556`: getParameters on dynamic; importing local cascades |
| 15 | `pretty_printer.ts:16`: conversion/domain refusal |
| 15 | `test/cds/cds_parser.ts:3879`: captured parseView lacks a lowered type |
| 12 | `lsp/references.ts:40`: property-access callback is not an arrow |
| 9 | `identical_conditions.ts:23`: primitive find needs absence-preserving callback ABI |
| 7 | `rename_global_interface.ts:18`: generated bridge return assignment mismatch |
| 7 | `test/pretty_printer/pretty_printer.ts:161`: keyword_case property assignment |

### Scope, oracle regressions and gates

Regexp split has its own typed signature, numeric ToUint32 limits, original
anchor context, JS empty-match suppression and capture splicing. Both regexp
split sites in abaplint src were enumerated with the TypeScript AST and use the
same operation. Dynamic Go RegExp construction uses the same checked subset as
literals, including JS whitespace and ASCII boundaries. XML version patterns,
`/$^/i`, and two exact bounded naming grammars are certified against live Node.
Unsupported patterns/flags still refuse; this is not arbitrary RE2 fallback.

An oracle exposed undefined captures becoming empty strings in ordinary
string-array reads. That prototype was rejected: an actually unmatched split
capture now produces a typed nullable-element ABI refusal at insertion, before
slice/iteration/escape can lose absence. Mandatory/empty captures and limits
stopping before an absent capture are supported. Repeated captures outside the
capture-safe grammar refuse. The splitter also avoids exec's bounded-repeat
cursor wrapper; a 1101-unit Node differential covers long input. See
[owned roots, mismatch diagnosis and dell's ABI handoff](traps-g-roots.md) and
[the certified domain](../tsfront/regexp_exec.md). No broad string-array,
closure, config, LSP presence, rename, DDIC or async ABI repair is included.

Passed on the final implementation:

```sh
export GOCACHE="$HOME/.cache/go-build" GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false
ABAPITI_GRACECHECK=1 go test ./... -short
ABAPITI_GRACECHECK=1 go test ./tsfront/... ./hir/... ./cmd/...
go vet ./...
node tools/abaplint-tests-report-test.mjs
node tools/abaplint-tests-all-test.mjs
nice -n 10 node tools/abaplint-tests-all.mjs ~/dev/abaplint docs/evidence/traps-g-final.json
```

Only the new op's runtime-catalogue counts change the Grace golden; method
facts and 61 lexer call sites are unchanged. Default `abaplint -o` output was
compared to an executable built from `git archive 350034c`, using the same
absolute output path before moving the baseline aside: **all 6,312 files
byte-identical by `diff -r`**, including the A4H zip. The path/content aggregate
SHA-256 is `2f910f53ad936abb3291a90e3bab05bbb19ff5c38dd184f105a8b94cd1b0af43`.
With `--target all,go` added, only `go/runtime.go` and its compiled binary differ:
the dynamic checked translator is now emitted, and the two exact naming reviews
join its table. `go/hir.go` and every ABAP-target file remain identical.

Fresh final-source Go and original Node are byte-identical on zabapgit **clean
(0 issues)** and **seeded (5 issues)**, also matching the release kit. The compact
JSON retains the gate audit; generated builds, source closures, binaries,
profiles, kit copies and raw/intermediate records were deleted. No transpiler
checkout or push. Earlier measurements below are historical, not this base's N.

## Trap batch F — historical frozen-source full measurement

**10,376/11,060 active tests passed (93.82%), FAIL 0.** All 625 files and
17 shards built and executed at **`98c43e3`**, against fresh original Node at
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. There are **624 typed traps**,
zero async-pending executions, and **92 not-run = 64 unregistered + 28 skipped**
(zero async/done). Node has 32 pending tests: four remain unregistered and 28
register as skipped. No files are excluded, no wiring failures or unmatched Go
titles occur, and the complete title **multiset**, including duplicates and
borrowed registrations, reconciles. No passes are estimated or synthesized.

See [owned roots and precise Dell handoffs](traps-f-roots.md),
[all-shard evidence and exact remaining missing titles](evidence/traps-f-final.json),
[base registration audit](evidence/traps-f-registration-base.json), and
[gate/default/workload evidence](evidence/traps-f-gates.json).
The supplied yolo-4 base is pinned at `7c2d33f`; only five base shards were run
here. Their actual pass count improves **4,378 → 4,580 (+202)**, and their
unregistered count falls **105 → 64**. The historical 115 included ten
keyword-case cases already recovered on this base; F does not credit them again.
All 41 STRUCTURE titles now register: **39 active bodies pass, two stay pending**.

| Shard under `test/` | Files | Passed | Failed | Trapped | Not-run | Registered | Node active + pending |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap` | 18 | 168 | 0 | 48 | 4 | 216 | 216 + 4 |
| `abap/expressions` | 11 | 42 | 0 | 8 | 57 | 50 | 107 + 0 |
| `abap/lexer` | 3 | 200 | 0 | 0 | 0 | 200 | 200 + 0 |
| `abap/nodes` | 1 | 2 | 0 | 0 | 0 | 2 | 2 + 0 |
| `abap/statements` | 286 | 4,942 | 0 | 20 | 0 | 4,962 | 4,962 + 0 |
| `abap/structures` | 30 | 138 | 0 | 0 | 2 | 139 | 139 + 1 |
| `abap/syntax` | 8 | 1,353 | 0 | 10 | 10 | 1,373 | 1,363 + 10 |
| `abap/types` | 6 | 23 | 0 | 6 | 0 | 29 | 29 + 0 |
| root `*.ts` | 12 | 66 | 0 | 28 | 0 | 94 | 94 + 0 |
| `cds` | 2 | 334 | 0 | 24 | 1 | 359 | 358 + 1 |
| `ddl` | 1 | 32 | 0 | 0 | 0 | 32 | 32 + 0 |
| `lsp` | 16 | 74 | 0 | 114 | 5 | 193 | 188 + 5 |
| `objects` | 26 | 105 | 0 | 39 | 1 | 145 | 144 + 1 |
| `objects/rename` | 9 | 0 | 0 | 31 | 0 | 31 | 31 + 0 |
| `pretty_printer` | 1 | 10 | 0 | 21 | 0 | 31 | 31 + 0 |
| `rules` | 192 | 2,879 | 0 | 268 | 12 | 3,157 | 3,149 + 10 |
| `utils` | 3 | 8 | 0 | 7 | 0 | 15 | 15 + 0 |
| **Unique total** | **625** | **10,376** | **0** | **624** | **92** | **11,028** | **11,060 + 32** |

### F top 15 reached traps

Counts exclude initialization failures; exact messages, locations and shard
splits are in the evidence JSON. These are reached refusals, not estimated wins.

| Tests | Root / remaining boundary |
| ---: | --- |
| 211 | `clear_exporting_parameters.ts:78`: parsed skipNames → native string-array bridge |
| 55 | `lsp/_lookup.ts:272`: Object.keys/JSON of native IReferenceExtras own properties |
| 37 | `_abap_object.ts:148`: native string-key map receives Dynamic `id` |
| 31 | `nesting.ts:253`: Program asserted as sibling Class, common-method view |
| 27 | `downport.ts:728`: unresolved captured `fix` |
| 23 | `domain.ts:100`: remaining native value/field conversion domain |
| 22 | `config.ts:11`: explicit version/language config conversion |
| 16 | `downport.ts:3520`: dynamic getParameters/importing bridge |
| 15 | `pretty_printer.ts:16`: constructor/config bridge |
| 15 | `cds_parser.ts:3879`: captured parseView |
| 12 | `lsp/references.ts:40`: non-arrow callback invocation ABI |
| 9 | `identical_conditions.ts:23`: absence-preserving primitive find callback ABI |
| 7 | `rename_global_interface.ts:18`: assignment/return view |
| 7 | `pretty_printer.ts:161`: live erased keyword_case assignment |
| 7 | `cds_parser.ts:3822`: captured parse |

### F reproduction and gates

```sh
export GOCACHE="$HOME/.cache/go-build" GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false
ABAPITI_GRACECHECK=1 go test ./... -short -timeout=30m
ABAPITI_GRACECHECK=1 go test ./tsfront/... ./hir/... ./cmd/... -timeout=30m
go vet ./...
node tools/abaplint-tests-report-test.mjs
node tools/abaplint-tests-all-test.mjs
ABAPITI_GRACECHECK=1 ABAPITI_TESTS_FIRST=rules,abap/syntax nice -n 10 \
  node tools/abaplint-tests-all.mjs ~/dev/abaplint docs/evidence/traps-f-final.json
```

All gates pass. The initial package-wide 10-minute short-test deadline expired
while tests were progressing; the 30-minute rerun excluded no tests. Shard
priority changes only execution order, never selection or accounting. Interrupted
pre-final diagnostic runs are disclosed in the roots document and not used as
whole-suite evidence. Generated binaries, closures, raw records and profiles are
removed; only compact evidence remains. No transpiler was cloned and nothing
was pushed.

Default `abaplint` output versus the supplied yolo-4 snapshot: **all 6,312 files
byte-identical**, including zip output, using the same absolute output path and
`diff -r`. The optional `--target all,go` comparison changes only
`go/runtime.go` (dynamic RegExp subset certification) and its rebuilt executable;
all other bytes match. Fresh original Node, Go and release-kit checks agree on
zabapgit **clean 0 / seeded 5** issues byte-for-byte. The lexer golden changes
only six runtime-catalogue counts, including inherited missing class-view facts;
no lexer method, call-site or oracle facts are altered.

## Integration train 3 — historical full measurement

**9,021/11,060 active tests passed (81.56%).** All 17 shards completed with
one assertion mismatch, 1,765 traps, zero async-pending executions, and 305
not-run cases. Table totals include the original Node oracle’s 32 pending tests.
No infrastructure failures or unmatched titles remain.

See [train-3 gates, per-shard counts, every assertion mismatch, top 15 traps,
full-closure gap and default-output comparison](train-3.md) and the
[train-3 JSON evidence](abaplint-tests-latest.json). The earlier measurements
below are retained as history.

## CDS silent-miscompile fix

The grouped-switch-label fix changes the fresh CDS result from **3 passed,
10 failed, 345 trapped, one not-run** to **12 passed, zero failed, 346 trapped,
one not-run**, with all 359 titles reconciled against original Node (358 active,
one pending). Nine lexer mismatches now pass; the parser token-count mismatch
now reaches an existing unsupported-regexp trap. Genuine CDS assertion
mismatches are **zero**. The whole-suite baseline below is retained as history; the current train-3
measurement above reruns every directory.

See [root cause, regression, other affected sites, default-output diff and full
zabapgit validation](cds-lexer-miscompile.md). The fix is shared frontend HIR,
not a CDS override or a Go-only runtime workaround.

## All-directory baseline — before grouped switch fix

**769/11,060 active Node tests (6.95%).** All **625 files** are selected in
**17 directory shards**, including `test/*.ts` and files directly in `test/abap/`.
Upstream remains `577f875ebec44cfaf64841cfe71c8ab8dc32622e`. Harness/compiler
source: **`67fceb1`**, based on **`e8647a5`** (`integ/train-1`).

The fresh whole-suite original-mocha oracle reports **11,060 passed, zero failed,
32 pending: 11,092 total**, not 11,060 including pending. The table uses
**Node active + pending**; every row's Go passed + failed + trapped + not-run
sums to that row's Node total. Full titles reconcile as a **multiset**, preserving
legitimate duplicates. No Go titles are unmatched. **No files are excluded or
unwired; all 17 Go modules built and executed.** Registration loss is not a
file-selection omission.

| Shard under `test/` | Files | Passed | Failed* | Trapped | Not-run | Registered | Node active + pending |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap` | 18 | 44 | 2 | 20 | 154 | 81 | 216 + 4 |
| `abap/expressions` | 11 | 41 | 2 | 7 | 57 | 50 | 107 + 0 |
| `abap/lexer` | 3 | 200 | 0 | 0 | 0 | 200 | 200 + 0 |
| `abap/nodes` | 1 | 2 | 0 | 0 | 0 | 2 | 2 + 0 |
| `abap/statements` | 286 | 434 | 20 | 4,508 | 0 | 4,962 | 4,962 + 0 |
| `abap/structures` | 30 | 24 | 0 | 114 | 2 | 139 | 139 + 1 |
| `abap/syntax` | 8 | 0 | 0 | 1,324 | 49 | 1,332 | 1,363 + 10 |
| `abap/types` | 6 | 4 | 0 | 25 | 0 | 29 | 29 + 0 |
| root `*.ts` | 12 | 5 | 0 | 18 | 71 | 94 | 94 + 0 |
| `cds` | 2 | 3 | 10 | 345 | 1 | 359 | 358 + 1 |
| `ddl` | 1 | 1 | 0 | 31 | 0 | 32 | 32 + 0 |
| `lsp` | 16 | 1 | 0 | 156 | 36 | 193 | 188 + 5 |
| `objects` | 26 | 0 | 0 | 29 | 116 | 145 | 144 + 1 |
| `objects/rename` | 9 | 0 | 0 | 31 | 0 | 31 | 31 + 0 |
| `pretty_printer` | 1 | 0 | 0 | 0 | 31 | 2 | 31 + 0 |
| `rules` | 192 | 10 | 0 | 1,669 | 1,480 | 3,147 | 3,149 + 10 |
| `utils` | 3 | 0 | 0 | 7 | 8 | 15 | 15 + 0 |
| **Unique total** | **625** | **769** | **34** | **8,284** | **2,005** | **10,813** | **11,060 + 32** |

**\*Failure-classification limitation on this base:** lowering traps are trapped,
but other throws still become failed. Of these 34, **10 are actual CDS assertion
mismatches; 24 are conversion/domain refusals** (22 in Config, two in chai), not
assertion mismatches. Dell's assertion-only fail-triage work is not included.
The table/JSON preserve the existing harness statuses rather than silently
relabeling them; assertion-only triage would make failed **10**, trapped **8,308**,
with N unchanged. The top-trap ranking also follows these legacy statuses.
All titles/messages are in `abaplint-tests-latest.json`.
Not-run comprises **1,700 async/done**, **26 skipped/focused**, and **279
unregistered** cases, including pending cases that never registered.

### Directory ownership and duplicate-import accounting

`tools/abaplint-tests-all.mjs` discovers every `.ts` root recursively and groups
by its **immediate parent directory**. `objects` therefore does not also select
`objects/rename`; `root` contains only `test/*.ts`. Helpers remain selected.
There is no hard-coded list of the four old shards.

Keep syntax in **one binary**: `modify.ts` and `fugr_import_export.ts` import
`syntax.ts` (1,153 tests). Splitting it would execute those tests twice. In
addition, syntax and LSP import the test-bearing
`test/rules/sql_value_conversion.ts`, registering **six borrowed tests each**.
Separate original-mocha runs of foreign test modules establish the borrowed
multiset and statuses. The owning `rules` shard counts those tests; syntax/LSP
subtract only the independently identified borrowed cases from both Node and Go.
The latest JSON includes borrowed counts and original registered counts
(**1,338 syntax; 199 LSP**). Ambiguous own/borrowed title collisions refuse;
arbitrary duplicate-title deduplication is not used. The final owned-title
multiset and passed/failed/pending totals match the whole-suite Node run exactly.

A small compiler crash fix was necessary: `typeRefName` no longer calls
`Node.Text()` on a `QualifiedName` such as `Basic.TableType`. It formats the name
for diagnostics/`as const` recognition only; checker resolution and cast semantics
are unchanged. Regression tests cover identifiers, nested qualified names, and
an emitted Go namespace-qualified cast. The six earlier unaffected shards used
`e8647a5`; syntax and subsequent shards used the corrected compiler source.

### Top 15 reached traps across the whole suite

Initialization failures are excluded. Long checker types are abbreviated here;
exact normalized diagnostics, representative messages and shard counts are in
`abaplint-tests-latest.json`. These are reached traps, not all lowering diagnostics.

| Tests | Message / location |
| ---: | --- |
| 6,659 | `src/abap/5_syntax/global_definitions/find_global_definitions.ts`: HIR verification: call arity mismatch |
| 1,065 | `test/abap/syntax/syntax.ts`: identifier JSON is not a lowered local or module value |
| 339 | not supported in the Go prototype: JavaScript regexp `/^@\w+$/` |
| 91 | `src/artifacts_objects.ts: src/artifacts_objects.ts:8` |
| 18 | `test/rules/_utils.ts`: identifier JSON is not a lowered local or module value |
| 15 | `test/cds/cds_parser.ts`: captured parseView has no lowered type |
| 13 | `test/ddl/tabl.ts`: property deep is not lowered |
| 7 | `test/cds/cds_parser.ts`: captured parse has no lowered type |
| 7 | `test/utils/name_validator.ts`: object literal property patternKind is not in shape `NamingRuleConfig`; identifier ruleViolated is not a lowered local or module value |
| 6 | `test/registry.ts`: captured getConfig has no lowered type |
| 6 | `test/abap/syntax/syntax_atselection.ts`: identifier JSON is not a lowered local or module value |
| 5 | `test/abap/expressions/sql_set_op.ts`: recursive tuple-union checker type is not a lowered declaration; empty array literal outside a statement context |
| 4 | `test/abap/native_sql_colon.ts`: captured nativeOf has no lowered type |
| 3 | `test/objects/class.ts`: type assertion requires checker-proven narrowing of its operand; identifier clas is not a lowered local or module value |
| 3 | `test/objects/malformed_xml.ts`: captured wrapValues has no lowered type |

### Reproduce the entire suite with one command

```sh
export GOCACHE="$HOME/.cache/go-build" GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false
node tools/abaplint-tests-all.mjs ~/dev/abaplint docs/abaplint-tests-latest.json
```

The command prints one table, checks the pinned active denominator and exact Node
multiset/totals, and writes the compact summary. It compiles original upstream
once into disposable storage, with a separate original-mocha process for each
oracle; no runtime shims enter Node. One generated Go binary is built per
directory. Generated Go, binaries and source closures are deleted after each
shard, and transient records after summarization. Only compact committed
JSON/markdown evidence is retained. No transpiler checkout was cloned.

During this measurement the initial split and cross-directory import overlaps
were detected, not ignored. After all generated modules had built/executed,
the final reconciliation re-linked and executed their **actual Go build-cache
archives**, using fresh whole-suite/per-shard/borrowed-module Node oracles. No
passes or registrations were estimated or synthesized. The discarded syntax
split is not counted. Builds, re-linked binaries, closures and raw records were
then removed; the JSON records this provenance.

Passed at this revision with the specified environment:

```sh
ABAPITI_GRACECHECK=1 go test ./tsfront/... ./hir/... ./cmd/...
go vet ./...
node tools/abaplint-tests-report-test.mjs
node tools/abaplint-tests-all-test.mjs
```

Default `abapiti abaplint -o out` builds against **`e8647a5`** and this branch were
compared with `diff -r`: **all 6,339 files byte-identical**, including the A4H zip.
Both executions used the **same absolute output path**, moving the first output
aside before the second, because READMEs embed resolved paths. The aggregate
path/content SHA-256 is
`54fdd8ad3fad9614ece559ba20b30bf772678365d3f8b576b3eb1572f7c300d4`.

## Historical four-shard N/total — forEach round

The remainder documents the older four-shard round and its implementation notes,
not the current whole-suite counts above.

Pinned upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`
(@abaplint/core 2.120.56). Compiler/runtime revision: **`8acc4f2`**, based on
harness revision `5a373f3` (PR #107). These are fresh **original-mocha Node
runs and generated Go executions**, not estimates. Every selected `.ts` file,
including helpers, was supplied in sorted path order.

| Shard | Selected files | Passed | Failed | Trapped | Not-run | Go registered | Node total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `test/rules/` | 192 | 5 | 0 | 1,674 | 1,480 | 3,147 | 3,159 |
| `test/abap/lexer/` | 3 | 200 | 0 | 0 | 0 | 200 | 200 |
| `test/abap/statements/` | 286 | 434 | 20 | 4,508 | 0 | 4,962 | 4,962 |
| `test/abap/structures/` | 30 | 24 | 0 | 114 | 2 | 139 | 140 |
| **Unique total** | **511** | **663** | **20** | **6,296** | **1,482** | **8,448** | **8,461** |

**663/8,461**, up from **661/8,461**. This is not coverage of the entire upstream
test suite; the four shards are separate binaries, with no smoke subset counted
twice. Node: rules 3,149 passed / 10 pending; lexer 200 passed; statements 4,962
passed; structures 139 passed / one pending. No Node failures or unmatched Go
titles. Go's not-run total is **1,458 async/done callbacks**, **11 skipped tests**,
and **13 unregistered tests** (12 rules, one structure).

Registration improves by **73 tests**: 3,074 → 3,147 rules; other counts are
unchanged. Lexer and statements have registration parity. None of the new
registrations is invented from the oracle. Most unlocked callbacks now reach
Config, so fixing forEach does not itself produce 1,321 new passes.

### Reached traps, ranked by tests

Only **two distinct reached trap messages** remain. Initialization failures are
excluded. `docs/abaplint-tests-latest.json` records per-shard counts, representative
messages, every remaining initialization failure, and every ordinary failure.

| Tests | Reached trap / location |
| ---: | --- |
| 6,278 | `src/config.ts:69`, via `src/config.ts:11:3`: pinned default-config JSON round-trip trap; 1,656 rules, 4,508 statements, 114 structures |
| 18 | `test/rules/_utils.ts:13:21`: `identifier JSON is not a lowered local or module value`, in `fullErrorNamespace` |

The **1,321 forEach traps and the one uninitialized `crlf` trap are gone**.
Config and async implementations were deliberately not changed.

## forEach/every: length snapshot, live membership and values

The existing fingerprinted dense/stable proofs remain fast paths. Unpinned
array `forEach` and `every` now lower to presence-sensitive indexed loops:

- evaluate the receiver once and read length once;
- check membership immediately before each visit and read the current value;
- skip holes, including holes created by deletion or out-of-range assignment;
- visit explicit `undefined` values (they are not holes);
- observe replacements, removals, shifts and splices; do not visit later pushes;
- pass the original array and original numeric index to callbacks;
- return `undefined` for forEach; short-circuit every, with true for an empty array.

This is a **length snapshot, not an element-copy snapshot**. The presence-aware
Go array ABI tracks holes separately from values, preserving membership through
slice, concat, reverse, unshift, splice, pop/shift and reference views. It is
emitted only when presence/deletion is used; ordinary dense-runtime output stays
unchanged. `find` keeps its existing separate semantics: it visits missing slots
as undefined, rather than borrowing forEach's hole-skipping rule.

Inline callbacks accept zero through three simple parameters. A forEach callback
with returns is lifted into its own invocation, so a return cannot escape the
surrounding method. Unsupported mutable lifted captures still refuse. A
non-arrow callback using its own `this`, a supplied `thisArg`, arbitrary callable
values, prototype mutation and other unmodeled constructs remain fail-closed;
they are not approximated by binding the surrounding method's receiver.

Node/Go differentials exercise holes versus explicit undefined, deletion,
out-of-range writes, replacements, pushes, shrinking arrays, original index and
receiver, early return, zero-argument callbacks, sparse slice/reverse/splice,
empty every and every short-circuiting. The existing source-fingerprint mutation
check still passes; its unpinned every case now also executes successfully.

## Small initialization fixes and remaining registration barriers

Fixed generally:

- boxed plain registration rows expose their stored fields to `dynamic.get`,
  including absent optional fields; this unlocks `check_comments`,
  `fully_type_constants`, `method_length` and `newline_between_methods`;
- map callbacks ending in returning branches use an ordinary lifted invocation,
  rather than requiring a final syntactic return;
- property assignment expressions evaluate the reference and RHS once, assign,
  and yield the **RHS value**, not the receiver;
- `keyof T` maps through the checker, not to the nominal type of T;
- dynamic reads/writes of supported class fields and `Object.keys` on
  zero-argument field-storage classes preserve the existing instance;
- literal `/\n/g` replacement with a literal, dollar-free replacement lowers to
  exact global newline replacement. `crlf` failed because this regexp was
  unsupported, **not** because module initializers ran out of order. The original
  readiness guard remains.

Remaining actions, recorded exactly in the JSON:

| File / action | Lost tests | Remaining blocker |
| --- | ---: | --- |
| `keyword_case.ts:289`, `:304` | 5 + 5 | Plain spread objects passed through `any` cannot become nominal `KeywordCaseConf` references; refusal in `src/rules/keyword_case.ts:129` |
| `line_length.ts:17` | 2 | `new LineLengthConf().length = 400` correctly yields the number **400**; `setConfig`'s nominal config ABI cannot store that number (`src/rules/line_length.ts:35`) |
| structures `_combi.ts:276` | 1 | Block-local class `Normal` at `:277`; fresh per-invocation constructor identity / local-class capture ABI is not implemented |

No lexer or statement initialization failures remain. No async suite
initialization failures remain. Local class hoisting and fabricating a nominal
config instance from a spread object would hide missing runtime semantics; they
were not used to manufacture registrations.

### The 18 JSON serialization cases, precisely

All 18 reach the same **whole-body lowering trap** in `fullErrorNamespace`:
`Config.getDefault().get()` produces `IConfig`, its `syntax.errorNamespace` is
changed to `"."`, and the helper calls `new Config(JSON.stringify(conf))`.
`JSON.stringify` has no generic frontend lowering here, so this helper traps
before executing its statements. This is not 18 different serialization domains
and is not evidence that a serializer executed and rejected 18 values.

The required input is the **entire nested config graph**, not a primitive string:
`global`, the dependency-object array, `syntax` (including its version form and
string arrays), and `rules` containing rule-config field objects; leaves include
strings, booleans, numbers, enum values and optional undefined properties.
Correct support must serialize own enumerable fields recursively, omit undefined
object properties, preserve array positions and JS numeric/string encoding, and
feed the resulting text through the Config constructor. A Go debug rendering,
nominal cast, or bypassing that constructor is not JSON.stringify. This graph
serialization remains unsupported and is reported, not implemented as an
identity shim; the separate Config round-trip work is left untouched.

## Ordinary failures: two runtime bugs fixed, 20 Config refusals remain

- **Structure `sta1 match` now passes.** `DummyNode extends
  AbstractNode<TokenNode>` actually contains a `StatementNode`. In the unpruned
  tests path, a common method invoked on an erased generic array element now
  dispatches through the erased `INode` interface and preserves its result type.
  It no longer makes a false TokenNode/AbstractToken cast before chai. Default
  lowering is unchanged. A generated-Go regression covers this mixed-node case.
- **LOOP AT SCREEN's version-failure test now passes.** Its expected parser
  failure throws `FailCombinatorError`; the Go catch previously required exactly
  `payload[Error]`, missing invariant `payload[FailCombinatorError]`. Reference
  catches now match the TS payload's reference ancestry. Host faults, lowering
  traps and return/loop-control signals do not implement the payload interface
  and cannot be swallowed by TypeScript catch.
- The remaining **20 statement failures** are Config constructor conversion
  refusals for explicit release/language object forms, not chai mismatches.
  Their full titles and messages remain in the JSON; no Config changes were made.

Chai strict equality, instanceof, negation and registration readiness/recovery
from PR #107 remain intact. Unsupported assertions are not no-ops. Deep chains,
hooks, arbitrary chai APIs and general async/Promise draining remain uncertified.

## Reproduce, compact evidence and gates

Use the unchanged pinned upstream checkout with installed npm dependencies.
Materialization refuses an existing closure directory. Oracle builds run original
upstream mocha/chai, never our shims. Keep raw data transient; delete generated
Go, binaries and copied source after each execution, and the remaining transient
JSON after producing the compact summary.

```sh
set -e
export GOCACHE="$HOME/.cache/go-build"
export GOTOOLCHAIN=go1.26.0
export GOFLAGS=-buildvcs=false
UPSTREAM=~/dev/abaplint
mkdir -p .local
RUN=$(mktemp -d "$PWD/.local/tests-run.XXXXXX")
trap 'rm -rf "$RUN"' EXIT
go build -o "$RUN/abapiti" ./cmd/abapiti

for spec in rules abap/lexer abap/statements abap/structures; do
  name=${spec##*/}
  files=$(find "$UPSTREAM/packages/core/test/$spec" -name '*.ts' | \
    sort | perl -pe "s@^$UPSTREAM/packages/core/@@")
  node tools/abaplint-tests-closure.mjs "$RUN/$name" "$UPSTREAM" $files
  node tools/abaplint-tests-oracle.mjs "$RUN/node-$name.json" "$UPSTREAM" $files
  "$RUN/abapiti" abaplint-tests "$RUN/$name" -o "$RUN/$name-out" \
    >"$RUN/go-$name.json" 2>"$RUN/run.log" || test "$?" = 1
  node tools/abaplint-tests-report.mjs "$RUN/node-$name.json" \
    "$RUN/go-$name.json" "$RUN/report-$name.json" >/dev/null
  # The summary needs only root count and source pin, not copied TS files.
  node -e 'const f=require("node:fs"),d=process.argv[1];
    const {roots,upstreamPin}=JSON.parse(f.readFileSync(d+"/closure.json"));
    f.rmSync(d,{recursive:true});f.mkdirSync(d);
    f.writeFileSync(d+"/closure.json",JSON.stringify({roots,upstreamPin}));' "$RUN/$name"
  rm -rf "$RUN/$name-out" "$RUN/run.log"
done
node tools/abaplint-tests-summary.mjs "$RUN" docs/abaplint-tests-latest.json
```

The reporter reconciles full titles as a **multiset**, including duplicates.
Passed + failed + trapped + not-run equals Node total. Only the compact committed
JSON/markdown summaries are retained. All transient builds, executables, source
closures, raw records and logs from this round were removed. No transpiler
checkout was cloned. During the final sequential run the command timeout stopped
the structures build; its unchanged generated Go was executed with
`GOMAXPROCS=4`, reconciled against the fresh oracle, then deleted.

Passed with the stated environment:

```sh
ABAPITI_GRACECHECK=1 go test ./tsfront/... ./hir/... ./cmd/...
go vet ./...
node tools/abaplint-tests-report-test.mjs
node tools/abaplint-tests-runtime-test.mjs ~/dev/abaplint
```

The Grace golden changes **only** runtime-catalogue counts for `array.has` and
`array.delete`; the 61 lexer call sites and method facts are unchanged.
`LowerOptions.TrapOnFailure` remains opt-in; test lowering uses nil coverage and
explicit binary64 semantics, unaffected by `ABAPITI_ASSUME_INT`.

Default **all-target** builds were compared against executables built from
`git archive origin/main`, finally pinned at **`ea61e23`**. Identical `-o`
spelling was used because driver READMEs embed it. **All 6,339 files are
byte-identical**, including the A4H zip; both builds exited zero. Earlier checks
against `54a521c` and `67d81fc` also matched. The final reference commit, aggregate
file-hash digest, gates and cleanup policy are recorded in the latest JSON.
