# abaplint's own tests on TS-HG@Go

## CDS silent-miscompile fix

The grouped-switch-label fix changes the fresh CDS result from **3 passed,
10 failed, 345 trapped, one not-run** to **12 passed, zero failed, 346 trapped,
one not-run**, with all 359 titles reconciled against original Node (358 active,
one pending). Nine lexer mismatches now pass; the parser token-count mismatch
now reaches an existing unsupported-regexp trap. Genuine CDS assertion
mismatches are **zero**. The whole-suite baseline below is historical until a
new full measurement replaces it; do not add focused results to it as though
all directories were rerun.

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
node tools/abaplint-tests-all.mjs /home/alice/dev/abaplint docs/abaplint-tests-latest.json
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

## Historical measurement — traps batch A (partial)

The remainder documents batch A's four-shard measurement, not a full-suite rerun.

Pinned upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`
(@abaplint/core 2.120.56). Measured compiler/runtime: **`a1c19a7`**, based on
train-2 **`ce2f789`**. All four shards were freshly rebuilt and executed against
fresh original-Mocha Node oracles; selections include every `.ts` file in sorted
path order. These are actual executions, not estimates or smoke-subset totals.

| Shard | Files | Passed | FAIL | Trapped | Not-run | Registered | Node total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `test/rules/` | 192 | 1,415 | 0 | 1,722 | 22 | 3,147 | 3,159 |
| `test/abap/lexer/` | 3 | 200 | 0 | 0 | 0 | 200 | 200 |
| `test/abap/statements/` | 286 | 4,942 | 0 | 20 | 0 | 4,962 | 4,962 |
| `test/abap/structures/` | 30 | 138 | 0 | 0 | 2 | 139 | 140 |
| **Unique total** | **511** | **6,695** | **0** | **1,742** | **24** | **8,448** | **8,461** |

**6,695/8,461**, up **180** from train-2's 6,515. **FAIL stays zero.**
Unsupported lowering and runtime domains remain traps, never fabricated passes.
Passed + FAIL + trapped + not-run equals Node total. Every shard has zero
unmatched multiset titles and zero async-pending tests. Node has 8,450 passes,
11 pending, and zero failures. The denominator remains 8,461: the isolated
Downport oracle's 303 registrations did not change the complete-shard totals.

This is **partial batch A**, not completion of every assigned root. The
structural-config representation, explicit version conversions and lost
registrations remain unsupported. The 2,599 tests outside these four shards
were not measured. Compact execution evidence is
[`abaplint-tests-latest.json`](abaplint-tests-latest.json). Historical baselines
and integration details are in [`train-1.md`](train-1.md) and
[`train-2.md`](train-2.md).

## Implemented roots and exactness boundaries

### JSON helper and live parsed fields

The first assigned site is **`test/rules/downport.ts:13`**, not the rule's
source import at `src/rules/downport.ts:13`. `buildConfig` changes
`Config.getDefault().get().syntax.version`, serializes the whole graph, and
constructs a new Config. The generic frontend now recognizes the default-library
JSON binding, with exactly one argument. Shadowed bindings are not intrinsics.
String parsing uses the existing strict JSON adapter; input coercion, revivers,
replacers and indentation explicitly refuse.

The usage-gated Go writer preserves tagged-record insertion order (numeric
index keys first), array positions including holes, undefined-property omission,
UTF-16 escaping, primitive encoding and recursive data graphs. Unsupported root
undefined values, cycles, toJSON hooks and unproven native own-key order trap.
ABAP explicitly refuses `json.stringifySubset`; this is not an identity clone
or debug-string serializer.

A newly reached bug replayed a parsed shape's stale input graph after direct
native field writes. That lost the requested Downport release and caused issue
mismatches in an intermediate experiment. Serialization now retains source key
order but reads **live fields**, preserving unprojected properties. Multiple
new keys without write-order metadata refuse. Boxing a changed parsed native
projection also refuses rather than expose stale source values to a later
serializer. Node/Go and runtime regressions cover mutation, key order, holes,
Unicode escaping and native-order refusal. The final reruns have no mismatches.

The JSON/conf702 helper trap is gone. Downport now reaches later, independently
unsupported syntax/crypto/EditHelper/registry paths; this is not 296 new passes.

### Artifacts inventory and dynamic constructor arguments

The five-type factory remains the frozen deployment adapter. Unpruned lowering
executes the pinned source's **complete Objects namespace inventory**: ordered
export enumeration, one dummy construction per export, `getType`, last value
wins for duplicate type keys, lazy publication, then construction with the
requested name. Unknown ordinary keys construct UnknownObject. Inherited
Array/Object properties and unsupported argument domains explicitly refuse
rather than pretending they are absent inventory keys.

General descriptor-valued `new` now supports evaluated argument arrays,
constructor defaults, inherited constructors and ignored extra arguments.
Receiver and arguments are evaluated once in source order. Required storage
conversions are checked; unsupported rest/spread/storage domains trap. Go
factories are usage-gated; ABAP explicitly refuses `classvalue.newArgs`.
A Node/Go differential exercises inherited constructors, defaults and extras.
The pinned artifacts refusal is no longer a reached trap in the four shards.

### Optional references and EmptyStructure

Known undefined literals retain absence when entering a different optional
storage type. Optional boxed reference projections check presence before
unboxing, including map references, instead of forcing absence through a
present-only conversion. This removes ObsoleteStatement's text-edit-map HIR
verification barrier, **but 72 executions still reach runtime conversion
refusals**; that root is not claimed complete.

Reference equality against an optional accepted interface now converts the
present operand to that interface without narrowing the optional operand.
Undefined compares unequal rather than throwing, and same/different instance
identity is retained. This removes EmptyStructure's StatementNode versus
optional union-interface comparison barrier. Generated-Go regressions cover
absence, live map values and interface identity.

### DDIC and ABAP Doc regexps

A small general regexp grammar translates literal ASCII atoms, dot,
non-folded character classes, anchors and `?`, `*`, `+`. Dot is one UTF-16 unit
excluding LF, CR, U+2028 and U+2029; non-u ASCII case folding is expressed with
explicit ASCII alternatives, not Go Unicode case folding. The same source
implementation certifies literal patterns and handles runtime patterns.

Groups, alternation, backreferences, Unicode/class escapes, folded classes,
lookarounds, unsupported flags and empty-source canonicalization refuse.
Node/Go exec differentials include both DDIC exception-name patterns and all
three ABAP Doc patterns, match text/index, astral input and line terminators.
The DDIC/ABAP Doc regexp traps are gone. The new runtime is usage-gated; ABAP
retains its explicit reviewed-pattern refusal for these additional patterns.

## Remaining assigned roots and lost registrations

| Root | Final reached tests / lost registrations | Status |
| --- | ---: | --- |
| `src/rules/7bit_ascii.ts:38` | 121 | Plain JSON rule-config values cannot become nominal config references without a live identity/alias-aware structural view; still refuses |
| `src/rules/obsolete_statement.ts:157` | 72 | Verification barrier removed; checked runtime conversion still refuses |
| `src/config.ts:11` | 20 statements | Explicit release/language object conversion remains unsupported |
| `test/rules/keyword_case.ts:289`, `:304` | 10 lost | Spread plain objects cannot become nominal KeywordCaseConf references |
| `test/rules/line_length.ts:17` | 2 lost | Assignment expression correctly yields scalar 400; config storage cannot retain that scalar |
| structures `_combi.ts:276` | 1 lost | Block-local class needs fresh per-invocation constructor identity/capture ABI |

No nominal config object was manufactured from a plain object or scalar, no
local class was hoisted, and no assertion was suppressed. The 24 not-run tests
are exactly **11 skipped + 13 unregistered**. Registry `findIssues` and
EditHelper's forEach were left untouched, as requested.

## New top 10 reached traps

Counts are actual trapped test executions, excluding initialization failures;
normalization and representative full messages are retained in the JSON.

| Rank | Tests | Reached trap |
| ---: | ---: | --- |
| 1 | 986 | `src/registry.ts:370`: HIR assignment type mismatch (parallel workstream) |
| 2 | 194 | `src/rules/downport.ts:713`: ExpressionNode versus ExpressionNode-array runtime argument mismatch |
| 3 | 187 | `src/edit_helper.ts:220`, `:227`: foreach requires matching array (parallel workstream) |
| 4 | 121 | `src/rules/7bit_ascii.ts:38`: config reference conversion refusal |
| 5 | 72 | `src/rules/obsolete_statement.ts:157`: runtime conversion refusal |
| 6 | 30 | `src/xml_utils.ts:1`: runtime conversion refusal |
| 7 | 29 | `src/rules/downport.ts:3161`: unsupported crypto/hash values |
| 8 | 20 | `src/config.ts:11`: explicit release/language conversion refusal (statements) |
| 9 | 18 | `src/rules/line_only_punc.ts:46`, `:61`: regexp groups outside the supported grammar |
| 10 | 17 | `src/rules/avoid_use.ts:72`: runtime conversion refusal |

Registry/EditHelper counts grow because execution reaches them through removed
barriers, not because their implementations changed.

## Gates, reproduction and cleanup

All Go tests used `GOCACHE=$HOME/.cache/go-build`, `GOTOOLCHAIN=go1.26.0`,
`GOFLAGS=-buildvcs=false`, and `ABAPITI_GRACECHECK=1`.

| Gate | Result |
| --- | --- |
| `go test ./... -short` | PASS |
| Grace-enabled `go test ./tsfront/... ./hir/... ./cmd/...` (non-short) | PASS |
| `go vet ./...` | PASS |
| Node report/summary multiset contract | PASS |
| Node test-host runtime contract | PASS, eight scenarios; not TS-HG@Go |
| Default `abaplint --target all` against `origin/integ/train-2` **236da86** | PASS: empty `diff -r`, 6,339 byte-identical files |
| `abaplint --full` | Measurement only: 33 blocking (27 root + 6 cascade), 143 HIR errors in 39 files |

The branch base is ce2f789; origin/integ/train-2 advanced during this work.
Default output was also compared against the earlier reference executable, then
freshly compared with 236da86. Both comparisons were empty. No reference-source
changes were merged into this branch. Full closure numbers above describe this
branch, not the subsequently advanced integration reference; the supplied base
had 35 blocking / 151 HIR errors.

Grace golden updates change **only operation-catalogue counts** for the JSON
writer and constructor-argument operation. Lexer method/call-site facts remain
unchanged. Frozen deployment lowering retains its original serials and output;
new generic semantics apply to unpruned lowering. ABAP refuses unsupported new
operations explicitly.

Reproduction follows the train-2 recipe: materialize each sorted complete shard
with `tools/abaplint-tests-closure.mjs`; build a fresh original oracle with
`tools/abaplint-tests-oracle.mjs`; execute `abapiti abaplint-tests`; reconcile with
`tools/abaplint-tests-report.mjs`; aggregate with
`tools/abaplint-tests-summary.mjs`. Harness-only builds reuse the measurement-only
declaration index described in `docs/evidence/train-2-verifier-index.patch`:
all original bodies are checked in order, with no production verifier changes.
Code gates and default/full measurements use ordinary source-built executables.

Generated Go/ABAP, binaries, copied closures, oracle builds, raw results, debug
logs and baseline source extractions were deleted after execution/summary.
Only compact JSON/markdown evidence is retained. No push or transpiler clone
was performed.
