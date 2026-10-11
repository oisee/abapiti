# abaplint's own tests on TS-HG@Go

## Batch C — partial, integration not certified

The [reviewed whitespace regexp root](traps-c-whitespace.md) passes its Node
oracle and focused DDL shard: **17/32 passed, FAIL 0, 15 trapped** (+13 over
batch B). This is not a whole-suite total. Concurrent edits from another process
in this worktree prevented safe completion of batch-C integration; the last
completed all-17-shard measurement below is still batch B. No new all-suite,
default-output or zabapgit gate is claimed.

## Final batch B — all 17 shards

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

Machine-readable final evidence: [`abaplint-tests-latest.json`](abaplint-tests-latest.json).
Complete repaired-base evidence, including its **top 20 reached traps** and empty
failure list: [`abaplint-tests-traps-b-base.json`](abaplint-tests-traps-b-base.json).
Both report actual whole-suite executions; the initial stopped run is not
presented as a completed base measurement.

## Top 15 reached traps across the final whole suite

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

**All gates passed.** Compiler/runtime unit tests include independent Node
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
