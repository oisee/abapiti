# abaplint's own tests on TS-HG@Go

## Integration train 3 — current full measurement

**9,021/11,060 active tests passed (81.56%).** All 17 shards completed with
one assertion mismatch, 1,765 traps, zero async-pending executions, and 305
not-run cases. Table totals include the original Node oracle’s 32 pending tests.
No infrastructure failures or unmatched titles remain.

See [train-3 gates, per-shard counts, every assertion mismatch, top 15 traps,
full-closure gap and default-output comparison](train-3.md) and the
[latest JSON evidence](abaplint-tests-latest.json). The earlier measurements
below are retained as history.

## CDS silent-miscompile fix

The grouped-switch-label fix changes the fresh CDS result from **3 passed,
10 failed, 345 trapped, one not-run** to **12 passed, zero failed, 346 trapped,
one not-run**, with all 359 titles reconciled against original Node (358 active,
one pending). Nine lexer mismatches now pass; the parser token-count mismatch
now reaches an existing unsupported-regexp trap. Genuine CDS assertion
mismatches are **zero**. The whole-suite baseline below is retained as history; the current train-3
measurement above reruns every directory.

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

The old 340-test annotation word-regexp trap, 66-test XML-array conversion trap,
and later 292-test quote-language trap are gone. That does **not** imply every
newly reached case passes: DDIC integer-range verification, helper captures,
LSP own-key enumeration and further explicit runtime domains still block cases.
In particular, the `Math.ceil` root now reaches DDIC's independent generated-local
verification refusal; it is not claimed to complete DDIC conversion.

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
