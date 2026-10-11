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
UPSTREAM=/home/alice/dev/abaplint
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
node tools/abaplint-tests-runtime-test.mjs /home/alice/dev/abaplint
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
