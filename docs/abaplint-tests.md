# abaplint's own tests on TS-HG@Go

## Final batch E — merged C + registrations base, all 17 shards

**8,876/11,060 active Node tests (80.25%), FAIL 0: +16 over the merged
base's 8,860.** Fresh execution of all **625 files**, no exclusions, all **17**
modules rebuilt and executed after the final semantic repair. Compiler/runtime
source is `9f479b6`; reported source `bd86393` additionally certifies the bounded
thunk catalogue and conservative callback effects (tests/documentation only).
Original upstream remains read-only at `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

**Step 0 passed:** all 17 shards on merged base `f0e8efe` measured
**8,860/11,060, FAIL 0**, 2,091 trapped and 141 not-run. Exact base evidence:
[abaplint-tests-traps-e-base.json](abaplint-tests-traps-e-base.json).
A second complete run after the first two harness repairs measured
**8,867/11,060, FAIL 0**; its new top 20 supplied the additional `.throw` item.
These measurements are independent complete executions, not summed gains.

**A subsequent genuine mismatch stopped the run at the rules shard:** two
Downport ELSEIF cases expect primitive-string throws, but the initial `.throw`
shim's Error-typed catch let them escape. Reported before repair, never relabeled
as trapped. Actual-Chai regressions now cover the exact string in ordinary and
existing normalized-thunk modes; the focused original Downport run confirms both
cases pass (**3 passed, 295 trapped, 5 not-run, FAIL 0**). Only the fresh final
all-shard execution above is the final measurement. An independent Chai oracle
also exposed and repaired the old shallow containment's NaN/sparse-hole mismatch.
[Root details, explicit refusal domains, and precise Dell handoffs](traps-e-roots.md).

Owned repairs remove `expect.fail`, the seven Chai containment casts, the
DataDefinition Set-input conditional, and missing Chai `.throw`. The last uses a
harness-only string/native-Error observation boundary, **not** a generic catch,
callable, class-view or async ABI change. Downport's dynamic method result,
AmbiguousStatement's interface narrowing, RenameGlobalInterface's **synthetic
WorkspaceEdit return bridge**, and NameValidator's nominal config shape remain
explicit refusals handed to Dell. Existing registry/EditHelper/config/LSP,
captured closures, DDIC, local-class, async and panic roots are untouched.

Fresh original-Mocha whole-suite oracle: **11,060 passed, 0 failed, 32 pending
(11,092 total)**. Owned titles reconcile as multisets with no unmatched titles,
wiring/infrastructure failures or async-pending records. **Registration parity
is still false:** 115 Node cases remain unregistered on Go; no passes are
estimated for them.

| Shard under `test/` | Files | Merged base passed | Final E passed | FAIL | Trapped | Not-run | Registered | Node active + pending |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap` | 18 | 158 | 160 | 0 | 56 | 4 | 216 | 216 + 4 |
| `abap/expressions` | 11 | 41 | 42 | 0 | 8 | 57 | 50 | 107 + 0 |
| `abap/lexer` | 3 | 200 | 200 | 0 | 0 | 0 | 200 | 200 + 0 |
| `abap/nodes` | 1 | 2 | 2 | 0 | 0 | 0 | 2 | 2 + 0 |
| `abap/statements` | 286 | 4,942 | 4,942 | 0 | 20 | 0 | 4,962 | 4,962 + 0 |
| `abap/structures` | 30 | 138 | 138 | 0 | 0 | 2 | 139 | 139 + 1 |
| `abap/syntax` | 8 | 1,302 | 1,309 | 0 | 15 | 49 | 1,332 | 1,363 + 10 |
| `abap/types` | 6 | 22 | 23 | 0 | 6 | 0 | 29 | 29 + 0 |
| root `*.ts` | 12 | 35 | 38 | 0 | 56 | 0 | 94 | 94 + 0 |
| `cds` | 2 | 334 | 334 | 0 | 24 | 1 | 359 | 358 + 1 |
| `ddl` | 1 | 32 | 32 | 0 | 0 | 0 | 32 | 32 + 0 |
| `lsp` | 16 | 64 | 64 | 0 | 124 | 5 | 193 | 188 + 5 |
| `objects` | 26 | 35 | 35 | 0 | 109 | 1 | 145 | 144 + 1 |
| `objects/rename` | 9 | 0 | 0 | 0 | 31 | 0 | 31 | 31 + 0 |
| `pretty_printer` | 1 | 0 | 0 | 0 | 31 | 0 | 31 | 31 + 0 |
| `rules` | 192 | 1,555 | 1,557 | 0 | 1,580 | 22 | 3,147 | 3,149 + 10 |
| `utils` | 3 | 0 | 0 | 0 | 15 | 0 | 15 | 15 + 0 |
| **Unique total** | **625** | **8,860** | **8,876** | **0** | **2,075** | **141** | **10,977** | **11,060 + 32** |

**Final FAIL list: `[]`.** Not-run is **26 skipped/focused + 115 unregistered**.
Each row's four outcome counts sum to its Node active + pending total. Exact
locations, normalized messages, initialization refusals and per-shard counts:
[abaplint-tests-latest.json](abaplint-tests-latest.json).

### Top 15 reached traps — final batch E

Initialization failures are excluded. Counts are actual reached tests, not
lowering diagnostic counts or inferred aggregate root sizes.

| Tests | Reached trap / representative location |
| ---: | --- |
| 1,146 | `src/registry.ts:370`: assignment type mismatch — Dell-owned |
| 435 | `src/edit_helper.ts:227`: foreach requires matching array — Dell-owned |
| 189 | `src/rules/7bit_ascii.ts:38`: conversion/domain refusal — config live-view ABI |
| 55 | `src/lsp/_lookup.ts:272`: `Object.keys` lacks native own-key presence/order ABI |
| 31 | `test/abap/nesting.ts:253`: conversion/domain refusal after registration recovery |
| 22 | `src/config.ts:11`: release/language conversion/domain refusal — Dell-owned |
| 15 | `test/cds/cds_parser.ts:3879`: captured `parseView` has no lowered type |
| 13 | `src/rules/downport.ts:3556`: `getParameters` on dynamic; unresolved `importing` cascade |
| 10 | `src/rules/downport.ts:730`: unresolved captured local `fix` — Dell-owned |
| 9 | `src/rules/ambiguous_statement.ts:95`: runnable-to-statement interface narrowing |
| 7 | `src/objects/rename/rename_global_interface.ts:18`: synthetic return-bridge assignment mismatch |
| 7 | `test/pretty_printer/pretty_printer.ts:161`: live `keyword_case` property assignment unsupported |
| 7 | `test/cds/cds_parser.ts:3822`: captured `parse` has no lowered type |
| 7 | `test/utils/name_validator.ts:7`: `patternKind` outside nominal NamingRuleConfig shape |
| 6 | `src/objects/rename/rename_global_class.ts:18`: assignment type mismatch |

### Final E gates and output audit

With Go 1.26.0, `GOFLAGS=-buildvcs=false`, `GOCACHE=$HOME/.cache/go-build`,
`ABAPITI_GRACECHECK=1`: **`go test ./... -short`, full tsfront/hir/cmd tests,
`go vet ./...` and both all-shard/report self-tests pass**. The inspected Grace
golden changes only operation catalogue counts for the two harness operations;
no lexer program facts change. Callback effects are conservatively unknown,
never used to fabricate purity.

Fresh default `abaplint` output compared with
**`origin/feat/fit-traps-c@b11e522` by raw `diff -r`: empty, exit 0**. All **6,312
files**, including every ABAP mirror, names, READMEs and zip, are byte-identical.
Harness operations/shims are tests-only; the Set-input context change has no
emitted default-closure change; registration callables are unpruned-only.
No changed path or output exception is accepted.

Fresh final-source generated Go and freshly built original Node agree
byte-for-byte on zabapgit **clean (0 issues)** and **seeded (5 issues)**, also
matching the SHA-256-pinned release kit. `go-target-check.py` independently passes
both. No execution of newly emitted ABAP is claimed. Compact gates, inventory
hash, stopped genuine failures and focused repair evidence:
[evidence/traps-e-final.json](evidence/traps-e-final.json).
Generated builds/closures, snapshots, kit, Node builds, binaries and profiles were
removed. No upstream edits, transpiler clone or push.

## Historical final batch C — third round, all 17 shards

**8,756/11,060 active Node tests (79.17%), FAIL 0: +180 over batch B's 8,576.**
Fresh execution of all **625 files**, no exclusions, all **17** modules built
and executed against source **`017c6ba`**. Original upstream remains read-only at
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. Every final shard was rebuilt after
the last semantic fix; focused gains are not summed into this total.

**A genuine Chai shim mismatch was found, reported, and fixed before continuing:**
`.deep` did not persist on reused assertions. The incomplete run was stopped;
actual-Chai Node regressions now check persistent flags, distinct accessor
wrappers and `.eql` not setting `.deep`. No failed record was reclassified.
See [root details, explicit refusal domains and the LSP ABI handoff](traps-c-roots.md).
The 17 selection-screen assignments and all 32 DDL tests now pass. LSP's own-key
root is **type/ABI work handed to Dell**, not a fabricated enumeration fix;
rule-config live views and other Dell-owned roots remain untouched.

The independent whole-suite original-Mocha oracle reports **11,060 passed,
0 failed, 32 pending (11,092 total)**. Observed owned titles reconcile as
multisets with no unmatched titles, wiring failures, infrastructure failures or
async-pending cases. **Registration parity is still false:** 279 Node cases do
not register on Go; they remain not-run, never estimated as passes.

| Shard under `test/` | Files | Batch B passed | Final C passed | FAIL | Trapped | Not-run | Registered | Node active + pending |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap` | 18 | 52 | 54 | 0 | 27 | 139 | 81 | 216 + 4 |
| `abap/expressions` | 11 | 41 | 41 | 0 | 9 | 57 | 50 | 107 + 0 |
| `abap/lexer` | 3 | 200 | 200 | 0 | 0 | 0 | 200 | 200 + 0 |
| `abap/nodes` | 1 | 2 | 2 | 0 | 0 | 0 | 2 | 2 + 0 |
| `abap/statements` | 286 | 4,942 | 4,942 | 0 | 20 | 0 | 4,962 | 4,962 + 0 |
| `abap/structures` | 30 | 138 | 138 | 0 | 0 | 2 | 139 | 139 + 1 |
| `abap/syntax` | 8 | 1,274 | 1,302 | 0 | 22 | 49 | 1,332 | 1,363 + 10 |
| `abap/types` | 6 | 21 | 22 | 0 | 7 | 0 | 29 | 29 + 0 |
| root `*.ts` | 12 | 35 | 35 | 0 | 59 | 0 | 94 | 94 + 0 |
| `cds` | 2 | 334 | 334 | 0 | 24 | 1 | 359 | 358 + 1 |
| `ddl` | 1 | 4 | 32 | 0 | 0 | 0 | 32 | 32 + 0 |
| `lsp` | 16 | 55 | 64 | 0 | 124 | 5 | 193 | 188 + 5 |
| `objects` | 26 | 30 | 35 | 0 | 109 | 1 | 145 | 144 + 1 |
| `objects/rename` | 9 | 0 | 0 | 0 | 31 | 0 | 31 | 31 + 0 |
| `pretty_printer` | 1 | 0 | 0 | 0 | 2 | 29 | 2 | 31 + 0 |
| `rules` | 192 | 1,448 | 1,555 | 0 | 1,582 | 22 | 3,147 | 3,149 + 10 |
| `utils` | 3 | 0 | 0 | 0 | 15 | 0 | 15 | 15 + 0 |
| **Unique total** | **625** | **8,576** | **8,756** | **0** | **2,031** | **305** | **10,813** | **11,060 + 32** |

**FAIL list: `[]`.** Not-run is unchanged: **26 skipped/focused + 279
unregistered**. Each row's four outcome counts sum to its Node active + pending
total. Typed lowering/runtime refusals remain trapped, never genuine assertion
failures. Historical exact messages, initialization refusals and per-shard counts are
retained at `origin/feat/fit-traps-c:docs/abaplint-tests-latest.json`; the current
file records batch E.

### Top 15 reached traps — final batch C

Initialization failures are excluded. These are actual reached test counts,
not diagnostic counts or inferred aggregate root sizes. Newly reached cases
explain increases in surviving buckets after earlier traps disappear.

| Tests | Reached trap / representative location |
| ---: | --- |
| 1,124 | `src/registry.ts:370`: assignment type mismatch — Dell-owned |
| 427 | `src/edit_helper.ts:227`: foreach requires matching array — Dell-owned |
| 186 | `src/rules/7bit_ascii.ts:38`: conversion/domain refusal — config ABI handoff |
| 55 | `src/lsp/_lookup.ts:272`: `Object.keys(ref.extra)` lacks native own-key presence/order ABI |
| 22 | `src/config.ts:11`: release/language conversion/domain refusal — Dell-owned |
| 15 | `test/cds/cds_parser.ts:3879`: captured `parseView` has no lowered type — Dell-owned |
| 13 | `src/rules/downport.ts:3556`: `getParameters` on dynamic; unresolved `importing` |
| 9 | `src/rules/ambiguous_statement.ts:95`: invalid runnable-to-statement interface narrowing |
| 9 | `src/rules/downport.ts:730`: unresolved captured local `fix` — Dell-owned closure work |
| 9 | `test/rules/_utils.ts:154`: unresolved `expect` |
| 8 | `src/objects/data_definition.ts:221`: conditional type mismatch |
| 7 | `runtime/chai.ts:43`: conversion/domain refusal (not the removed `.deep` property trap) |
| 7 | `src/objects/rename/rename_global_interface.ts:18`: assignment type mismatch |
| 7 | `test/cds/cds_parser.ts:3822`: captured `parse` has no lowered type — Dell-owned |
| 7 | `test/utils/name_validator.ts:7`: `patternKind` outside NamingRuleConfig shape; unresolved helper |

### Final C gates and output audit

With Go 1.26.0, `GOFLAGS=-buildvcs=false`, `GOCACHE=$HOME/.cache/go-build` and
`ABAPITI_GRACECHECK=1`: **`go test ./... -short`, full tsfront/hir/cmd tests,
`go vet ./...` and both all-shard/report self-tests pass**. Lint was attempted
against `origin/feat/fit-traps-b`, but **unavailable**: `golangci-lint` is not on
PATH; no lint verdict is claimed. The Grace golden changes only reviewed
operation catalogue counts, not lexer program facts.

Raw default `abaplint` **`diff -r` is nonempty and fully accounted for**, not
claimed identical: unused ancestry-only union interfaces/adapters disappear,
ancestry operands no longer take pre-test flow casts, one closure is renamed,
and default assignments construct optional storage explicitly in 18 classes.
All three ABAP mirrors agree; `names.json`, object-count README and containing
zip change accordingly. [The compact inventory](evidence/traps-c-final.json)
and [isolation builds/root explanations](traps-c-roots.md) cover every change.
A fresh final-source rebuild matches the audited tree byte-for-byte.

Fresh original Node and freshly generated Go agree byte-for-byte on zabapgit
**clean (0 issues)** and **seeded (5 issues)**, also matching the pinned release
kit; `go-target-check.py` independently passes both. No execution of newly
emitted ABAP is claimed. Generated builds/closures, temporary snapshots, Node
builds, kit, binaries and profiles were deleted. Only compact JSON/Markdown
remains; no upstream edits, transpiler clone or push.

## Historical final batch B — all 17 shards

**8,576/11,060 active Node tests (77.54%), FAIL 0.** This is **+434** from the
repaired integration base's 8,142, not a sum of focused reruns. All **625 files**
are selected, no files are excluded, and all 17 Go modules built and executed.
Upstream remains read-only at `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.
Final emission/runtime source: `db05a5b`; subsequent `86a196b` regenerates the
runtime-effect golden and strengthens the retained critic-fixture tests without
changing emitted programs. Every shard was rebuilt after the final root fix.

**A genuine assertion mismatch was found on the initial merged base, and that
run was stopped.** Inferred character variables incorrectly retained literal-only
assignment relaxations because object rest copied an excluded optional field.
The shared frontend was repaired before trap work; the test now passes. See
[the root cause and independent Node/Go regression](traps-b-object-rest.md).
No failing assertion was relabeled as trapped.

The fresh whole-suite original-Mocha oracle reports **11,060 passed, zero failed,
32 pending: 11,092 total**. The denominator is active tests, not total including
pending. Each row's Go passed + FAIL + trapped + not-run equals its Node total.
All owned titles reconcile as a multiset, preserving legitimate duplicates.
Zero unmatched titles, zero wiring failures, zero async-pending cases.

| Shard under `test/` | Files | Base passed | Final passed | FAIL | Trapped | Not-run | Registered | Node active + pending |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap` | 18 | 52 | 52 | 0 | 29 | 139 | 81 | 216 + 4 |
| `abap/expressions` | 11 | 41 | 41 | 0 | 9 | 57 | 50 | 107 + 0 |
| `abap/lexer` | 3 | 200 | 200 | 0 | 0 | 0 | 200 | 200 + 0 |
| `abap/nodes` | 1 | 2 | 2 | 0 | 0 | 0 | 2 | 2 + 0 |
| `abap/statements` | 286 | 4,942 | 4,942 | 0 | 20 | 0 | 4,962 | 4,962 + 0 |
| `abap/structures` | 30 | 138 | 138 | 0 | 0 | 2 | 139 | 139 + 1 |
| `abap/syntax` | 8 | 1,243 | 1,274 | 0 | 50 | 49 | 1,332 | 1,363 + 10 |
| `abap/types` | 6 | 4 | 21 | 0 | 8 | 0 | 29 | 29 + 0 |
| root `*.ts` | 12 | 35 | 35 | 0 | 59 | 0 | 94 | 94 + 0 |
| `cds` | 2 | 12 | 334 | 0 | 24 | 1 | 359 | 358 + 1 |
| `ddl` | 1 | 1 | 4 | 0 | 28 | 0 | 32 | 32 + 0 |
| `lsp` | 16 | 54 | 55 | 0 | 133 | 5 | 193 | 188 + 5 |
| `objects` | 26 | 3 | 30 | 0 | 114 | 1 | 145 | 144 + 1 |
| `objects/rename` | 9 | 0 | 0 | 0 | 31 | 0 | 31 | 31 + 0 |
| `pretty_printer` | 1 | 0 | 0 | 0 | 2 | 29 | 2 | 31 + 0 |
| `rules` | 192 | 1,415 | 1,448 | 0 | 1,689 | 22 | 3,147 | 3,149 + 10 |
| `utils` | 3 | 0 | 0 | 0 | 15 | 0 | 15 | 15 + 0 |
| **Unique total** | **625** | **8,142** | **8,576** | **0** | **2,211** | **305** | **10,813** | **11,060 + 32** |

**FAIL list: `[]`.** Typed lowering/runtime refusals count as trapped; genuine
assertion mismatches and other non-refusal throws count as failed. Reporting
never changes a failed record's classification. The runner now stops immediately
after a shard with FAIL, prints its actual failures, and cleans generated builds.
Not-run is **26 skipped/focused + 279 unregistered**, not omitted source files.
Registration losses remain unsupported; they are not estimated as passes.

Historical machine-readable final evidence is retained at
`origin/feat/fit-traps-b:docs/abaplint-tests-latest.json`; the current file now
records the fresh batch-E run above. Complete repaired-base evidence, including its **top 20 reached traps** and empty
failure list: [`abaplint-tests-traps-b-base.json`](abaplint-tests-traps-b-base.json).
Both report actual whole-suite executions; the initial stopped run is not
presented as a completed base measurement.

## Historical batch-B top 15 reached traps

Initialization failures are excluded. These are reached test counts, not counts
of lowering diagnostics. Messages are shortened here; exact normalized messages,
representative TS locations and per-shard counts are in the final JSON.
The existing normalization retains generated `range_local_*` names; the DDIC
rows below are separate reported keys, not an invented aggregate root count.

| Tests | Reached trap / location |
| ---: | --- |
| 1,119 | `src/registry.ts:370`: HIR assignment type mismatch — Dell-owned, skipped |
| 208 | `src/edit_helper.ts`: HIR foreach requires matching array — Dell-owned, skipped |
| 194 | `src/rules/downport.ts`: ExpressionNode argument versus array argument |
| 173 | `src/rules/7bit_ascii.ts`: runtime conversion/domain refusal |
| 72 | `src/rules/obsolete_statement.ts`: runtime conversion/domain refusal |
| 53 | `src/lsp/_lookup.ts`: `Object.keys(ref.extra)` has no lowered own-key proof |
| 29 | `src/rules/downport.ts`: crypto/hash not lowered |
| 27 | `src/ddic.ts`: unresolved `range_local_291037` (syntax shard) |
| 22 | `src/config.ts`: explicit release/language conversion/domain refusal |
| 17 | `src/rules/avoid_use.ts`: runtime conversion/domain refusal |
| 17 | `src/rules/selection_screen_texts_missing.ts`: HIR assignment type mismatch |
| 15 | `test/cds/cds_parser.ts`: captured `parseView` has no lowered type |
| 14 | `src/ddic.ts`: unresolved `range_local_314269` (rules shard) |
| 13 | DDL: unreviewed JavaScript regexp `/\s+/g` |
| 13 | `test/ddl/tabl.ts`: chai `deep` property not lowered |

The old 340-test annotation word-regexp trap, 66-test XML-array conversion trap,
and later 292-test quote-language trap are gone. That does **not** imply every
newly reached case passes: DDIC integer-range verification, helper captures,
LSP own-key enumeration and further explicit runtime domains still block cases.
In particular, the `Math.ceil` root now reaches DDIC's independent generated-local
verification refusal; it is not claimed to complete DDIC conversion.

## Implemented roots and exactness boundaries

[Root-by-root implementation, focused checks, commits, integration/default-output
comparisons and full zabapgit follow-up](traps-b-roots.md):

- **Object rest:** excluded optional keys receive absence even when checker/HIR
  storage reuses a wider shape; other fields copy into fresh storage. Shared Go
  and ABAP HIR, with a Node/Go regression and ABAP emission check.
- **Parsed XML/JSON arrays:** one canonical Go reference-array ABI; native views
  retain identity and live writes instead of snapshot conversion. ABAP's existing
  tagged-to-native array boundary still refuses explicitly.
- **Regexp grammar:** ASCII word/digit classes, ASCII folded classes, alternation,
  noncapturing groups and nonrepeated/optional captures. Repeated captures,
  backreferences, general lookarounds, unsupported flags and other unmodeled
  syntax refuse. Match text/index and capture absence are Node/Go checked.
- **`Math.ceil`:** numeric-only default-library binding, once-only argument
  evaluation, IEEE-754 signed-zero/NaN/infinity behavior; input coercion refuses.
  ABAP explicitly refuses the new runtime operation.
- **Nominal class views:** related valid Go views retain the original instance;
  mismatched runtime types refuse outside JS catch. ABAP explicitly refuses
  unproven checked views; pruned lowering/pinned proofs are unchanged.
- **Quote-delimited regexp language:** an exact whole-language translation,
  exhaustively compared with Node on **349,525** strings, plus emitted-Go Unicode
  exec tests. No claim of general lookahead support; ABAP still refuses this
  additional unreviewed pattern.

Dell's registry assignment, EditHelper forEach, qualified-type panic, nested
unused-variable optionals and dynamic `sy_read_restriction` keys were not edited.
Rules/lexer/statements/structures were rerun but not selected for batch-B root
work. Their incidental gains come from shared fixes needed by the new shards.

## Directory ownership and title accounting

`tools/abaplint-tests-all.mjs` recursively discovers every `.ts` file and groups
roots by their **immediate parent directory**. `objects` does not also select
`objects/rename`; `root` contains only `test/*.ts`. Helpers remain selected.
There is no hard-coded four-shard selection or smoke subset.

Syntax stays in one binary: `modify.ts` and `fugr_import_export.ts` import the
test-bearing `syntax.ts`. Syntax and LSP also import six tests owned by
`test/rules/sql_value_conversion.ts`. Independent original-Mocha runs establish
the borrowed multiset; only those identified cases are removed from the borrowing
shards' Node and Go records. Ambiguous own/borrowed collisions refuse. The final
owned multiset/totals agree with an independent whole-suite Node run. JSON retains
borrowed and raw registration counts. No registrations are manufactured from
oracle titles.

## Reproduce and gates

```sh
export GOCACHE="$HOME/.cache/go-build" GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false
export ABAPITI_GRACECHECK=1 GOMAXPROCS=4
node tools/abaplint-tests-all.mjs /home/alice/dev/abaplint docs/abaplint-tests-latest.json

go test ./... -short
go test ./tsfront/... ./hir/... ./cmd/...
go vet ./...
node tools/abaplint-tests-all-test.mjs
node tools/abaplint-tests-report-test.mjs
```

**Batch-B gates passed.** Final batch-E gate results are recorded above; the batch-C local lint
limitation is retained in its historical section. Compiler/runtime unit tests include independent Node
oracles and explicit refusal/catch boundaries. Runtime-effect goldens were
regenerated through `GRACE_FACTS_OUT` and inspected: only the two new catalogue
operations' counts and one conservative record change.

The merged integration base's default `abaplint` output equals train-2 except
[the documented grouped-switch-fix paths](cds-lexer-miscompile.md). Batch B's
final output adds only corrected InlineData (the same class in `classes/`,
`osg/`, `native/lib/`) and its containing zip, due to the genuine object-rest
miscompile repair. A fresh full zabapgit check compared original Node, generated
Go and the release kit: clean **zero issues**, seeded **five issues**, outputs
byte-identical. `tools/go-target-check.py` independently passed both variants.
This is a Go workload check, not execution of newly emitted ABAP.

Generated Go, binaries and closures are deleted after each shard. Original-Node
builds and transient records are disposed after summarization. Release kit,
workload builds and profiles were removed. Only compact JSON/Markdown evidence
is committed. No upstream source edits, transpiler clone or push.

## Historical measurements

- [Grouped-switch silent miscompile and historical CDS repair](cds-lexer-miscompile.md).
- Batch A's four-shard measurement: **6,695/8,461; FAIL 0** (`a1c19a7`), not a
  whole-suite denominator. Its implementations are included in the batch-B base.
- [Integration train 1](train-1.md), [train 2](train-2.md), and
  [train-2 ABAP config integration](train-2-config-json-merge.md).

Historical 769/11,060 and four-shard-only tables are superseded by the complete
fresh executions above; none is added to this final total.
