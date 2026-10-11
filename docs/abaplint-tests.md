# abaplint's own tests on TS-HG@Go

## Latest async integration — rebased harness

Fresh before/after measurements against `f51056b` are in
[`abaplint-tests-async.md`](abaplint-tests-async.md) and its companion JSON.

| Shard | Build | Passed | Failed | Trapped | Async-pending | Not-run | Registered | Node total |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `rules` | before | 5 | 0 | 1,601 | 0 | 1,553 | 3,074 | 3,159 |
| `rules` | after | 5 | 0 | 1,914 | 1,145 | 95 | 3,074 | 3,159 |
| `lexer` | before | 200 | 0 | 0 | 0 | 0 | 200 | 200 |
| `lexer` | after | 200 | 0 | 0 | 0 | 0 | 200 | 200 |
| `statements` | before | 433 | 21 | 4,508 | 0 | 0 | 4,962 | 4,962 |
| `statements` | after | 433 | 21 | 4,508 | 0 | 0 | 4,962 | 4,962 |
| `structures` | before | 23 | 1 | 114 | 0 | 2 | 139 | 140 |
| `structures` | after | 23 | 1 | 114 | 0 | 2 | 139 | 140 |
| **Unique total** | **before** | **661** | **22** | **6,223** | **0** | **1,555** | **8,375** | **8,461** |
| **Unique total** | **after** | **661** | **22** | **6,536** | **1,145** | **97** | **8,375** | **8,461** |

The round-two report below is the baseline historical measurement.

## Latest N/total — harness round 2

Pinned upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`
(@abaplint/core 2.120.56). Rebased onto **origin/main `54a521c`**.
Compiler/runtime revision: `069d945` (following `c05dd4f`). These are fresh
**original-mocha Node runs and generated Go executable runs**, not estimates.
Every selected `.ts` file, including helpers, was supplied in sorted path order.

| Shard | Selected files | Passed | Failed | Trapped | Not-run | Go registered | Node total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `test/rules/` | 192 | 5 | 0 | 1,601 | 1,553 | 3,074 | 3,159 |
| `test/abap/lexer/` | 3 | 200 | 0 | 0 | 0 | 200 | 200 |
| `test/abap/statements/` | 286 | 433 | 21 | 4,508 | 0 | 4,962 | 4,962 |
| `test/abap/structures/` | 30 | 23 | 1 | 114 | 2 | 139 | 140 |
| **Unique total** | **511** | **661** | **22** | **6,223** | **1,555** | **8,375** | **8,461** |

This is **661/8,461**, not coverage of abaplint's entire test suite. The source
ABAP directories are `1_lexer`, `2_statements`, and `3_structures`; the test
directories above are separate binaries. No smoke subset is counted twice.

Node: rules 3,149 passed / 10 pending; lexer 200 passed; statements 4,962
passed; structures 139 passed / 1 pending. Node reported no failures.
Go's not-run total is **1,458 async/done callbacks**, **11 skipped tests**, and
**86 unregistered oracle tests** (85 rules, one structure). No unmatched Go
titles and no async suite initialization failures remained.

**Registration parity is achieved for lexer and statements, not yet for rules
and structures.** The initial report registered 21 lexer and zero statement
tests; those registration barriers are removed. Remaining initialization
failures are explicit evidence, not fabricated registrations, passes, or
failed tests. `docs/abaplint-tests-latest.json` replaces the first-run JSON and
contains every remaining initialization failure and ordinary test failure.

### Top 20 reached traps, ranked by tests

Only **four distinct reached trap messages** remain. Counts are actual test
executions reaching a trap, not compiler diagnostic occurrences. Initialization
failures are excluded. Full representative messages and per-shard counts are in
the latest JSON.

| Rank | Tests | Reached trap / location | Next fix |
| --- | ---: | --- | --- |
| 1 | 4,883 | `src/config.ts:69`, via `src/config.ts:11:3` | Pinned `Config.getDefault` override still traps the default-config JSON round trip; 4,508 statements, 261 rules, 114 structures |
| 2 | 1,321 | `test/rules/_utils.ts:47:11`: `forEach needs a source-pinned dense/stable receiver proof` | Issue/fix checks inside the test callback; whole callback traps conservatively even when the fix branch would not execute |
| 3 | 18 | `test/rules/_utils.ts:13:21`: `identifier JSON is not a lowered local or module value` | `fullErrorNamespace` serialization |
| 4 | 1 | `test/rules/line_break_style.ts:40:37`: binding `crlf` was not initialized | Its initializer uses unsupported `/\n/g`; a readiness guard prevents a fabricated zero/undefined value |

PR #100 (`origin/feat/fit-misc-roots`, `f51a5b6`) is **not in the rebased main**.
Its added dense-callback pins cover core sites such as `Conditions`,
`ContainsTab.runParsed`, and `Catch.findCommonSuperclass`; **none covers
`test/rules/_utils.ts:47`**. Checking that branch therefore does not remove the
first run's 1,070-test trap. More tests now reach that same trap (1,321) because
registration improved. This branch was inspected, not cherry-picked wholesale.

## Registration: causes, fixes, and exact remaining failures

The old failure policy could replace an entire module initializer, losing
unrelated later suites. Heterogeneous `const tests = [...]` arrays also widened
to `object`, losing the registration helper's optional row-field context.

The tests-only path now:

- lowers each top-level executable statement into an independent initialization
  action, runs actions in source order, recovers at the Go host, and records
  `{file, action, message, status}` in both raw results and reconciliation reports;
- continues later statements/suites after a failed action, without inventing
  titles from the Node oracle;
- guards test-module bindings with readiness flags, so runtime initializer
  failures cannot expose fabricated zero/undefined values; compile-time failed
  declarations and their retained uses still fail verification;
- recovers typed row contexts from registration calls, preserves contextual
  synthetic-shape storage/identity through checker-only views, and narrows fields
  individually rather than making an invalid nominal cast;
- pins dense/stable registration arrays in the two upstream helpers, lexer
  `count.ts`/`type.ts`, and `method_length.ts` by complete source hashes;
- refines the pinned ABAP helper's otherwise-`any` statement arrays to its actual
  string/typed-row contracts, and boxes erased reference-array elements on reads;
- avoids the compiler panic in the ABAP helper's top-level `legacyReleases` IIFE
  by initializing lifted-function bookkeeping and giving lifted bodies a failure
  boundary;
- permits async `describe` bodies containing only inert constants and synchronous
  registration calls: Mocha ignores their return value and they do not suspend.
  Their async `it` callbacks remain explicitly not-run.

Remaining rule initialization actions (all recorded separately in the JSON):

| File / action | Exact blocker |
| --- | --- |
| `check_comments.ts:152`, `fully_type_constants.ts:131`, `newline_between_methods.ts:199` | `_utils.ts:169` suite callback, at `it(test.description, ...)`: `dynamic.get` cannot read a boxed plain row object's fields (no dynamic field adapter) |
| `method_length.ts:229` | Same plain-row field ABI at its local registration helper (`:8`, reading `test.description`) |
| `keyword_case.ts:289` | Config reference conversion refuses at `src/rules/keyword_case.ts:129` |
| `keyword_case.ts:292`, `:304` | `.map` callback does not end in a value return; subsequently dropped `testLowerCaseGlobalClassSuite2` cannot be read |
| `line_break_style.ts:32` | Unsupported JavaScript regexp `/\n/g` in the `crlf` table initializer |
| `line_length.ts:17` | Assignment inside a table-transform expression is not lowered |
| `no_dynamic_stuff.ts:124`, `:134` | `keyof NoDynamicStuffConf` signature incorrectly becomes a config reference; string argument to `conf(...)` fails HIR verification |

Structures: `_combi.ts:276` contains a local class declaration `Normal` at `:277`;
that suite callback traps at registration and loses one test. Later suites still
register. Statements: the final four missing window-function rows at
`select_loop.ts:393` now register after preserving their contextual map-row
layout. **No statement or lexer initialization failures remain.**

## Chai comparisons and the remaining ordinary failures

`equal`/`equals` use tagged **strict equality**, including numeric equality across
numeric representations, strings, booleans, undefined versus null, and object
reference identity. They never substitute structural/deep equality. Successful
assertions do not eagerly stringify arbitrary objects; failure formatting avoids
unsupported object ToPrimitive conversion. `instanceof` checks object presence
and unboxes the object and constructor descriptor before the ancestry test.
Numeric `greaterThan` covers the lexer's positive-position assertions.

The 17 former lexer runtime refusals are gone: **all 200 lexer tests pass**.
Generated-Go regressions exercise primitive/reference equality, boxed constructor
arguments, negation, ordinary assertion failures, initialization recovery,
readiness guards, contextual map rows, and non-suspending async suites. The Node
shim contract tests also pass.

The remaining 22 **non-trap failures are not all assertion mismatches**:

- 20 statement tests refuse in `Config.getDefault` (`src/config.ts:11:3`) for
  explicit release/language object forms: unsupported conversion domain.
- `LOOP AT SCREEN`'s version-failure test throws `FailCombinatorError`.
- Structure `sta1 match` refuses **before chai is called**, at
  `_combi.ts:44`: `parent.getChildren()[0]` is statically viewed as `TokenNode`
  through `DummyNode extends AbstractNode<TokenNode>`, but contains a
  `StatementNode`. The erased generic reference-array cast fails. A separate
  stack-instrumented diagnostic run confirms `castRef` at generated `hir.go:3261`,
  not a chai comparison. This generic node ABI is still missing; it is not
  reclassified as a pass or silently hidden as an assertion-shim fix.

Deep chains, general pending async execution, real timers, and arbitrary chai
APIs remain unsupported or uncertified. Settled Promise draining and suite hooks
are covered by the async integration linked above. Unsupported constructs
trap; assertions are not no-ops.

## Reproduce and evidence

Use the unchanged pinned upstream checkout with installed npm dependencies.
The materializer refuses an existing closure directory. Fresh Node oracles build
and run original upstream mocha/chai, never our shims.

```sh
export GOCACHE="$HOME/.cache/go-build"
export GOTOOLCHAIN=go1.26.0
export GOFLAGS=-buildvcs=false
UPSTREAM=~/dev/abaplint
mkdir -p .local
RUN=$(mktemp -d "$PWD/.local/tests-run.XXXXXX")
go build -o "$RUN/abapiti" ./cmd/abapiti

for spec in rules abap/lexer abap/statements abap/structures; do
  name=${spec##*/}
  files=$(find "$UPSTREAM/packages/core/test/$spec" -name '*.ts' | \
    sort | perl -pe "s@^$UPSTREAM/packages/core/@@")
  node tools/abaplint-tests-closure.mjs "$RUN/$name" "$UPSTREAM" $files
  node tools/abaplint-tests-oracle.mjs "$RUN/node-$name.json" "$UPSTREAM" $files
  "$RUN/abapiti" abaplint-tests "$RUN/$name" -o "$RUN/$name-out" \
    >"$RUN/go-$name.json" 2>"$RUN/go-$name.log" || test "$?" = 1
  node tools/abaplint-tests-report.mjs "$RUN/node-$name.json" \
    "$RUN/go-$name.json" "$RUN/report-$name.json"
done
node tools/abaplint-tests-summary.mjs "$RUN" "$RUN/summary.json"
```

Current raw evidence is retained **inside this worktree**, under
`.local/harness-round2/`: `node-*.json`, `round2-go-*.json/log`,
`round2-report-*.json`, `round2-*/closure.json`, and `round2-*-out/{go,evidence}/`.
The final summary uses artifact prefix `round2-`. Diagnostic-only stack runs are
`diagnostic-rules.log` and `diagnostic-structures.log`; their instrumented outputs
are **not** used in the N table. Nothing depends on the vanished
`/tmp/tests-first/` evidence, and no transpiler checkout was cloned here.

The reporter reconciles full titles against Node as a **multiset**, including
duplicates. Passed + failed + trapped + not-run equals Node total. Unexpected Go
titles remain explicit `unmatched` evidence. The summary ranks reached traps
from all raw test records, not truncated per-shard diagnostic inventories.

## Policy, rebase, and gates

`LowerOptions.TrapOnFailure` is opt-in. `LowerAbaplintTests` supplies **nil
coverage** and explicit binary64 semantics; `ABAPITI_ASSUME_INT` does not change
the test build. Ordinary abaplint builds, including `--full`, do not enable it.
Body-level lowering/verifier/backend failures become source-attributed HIR traps;
structural errors that cannot be attributed still refuse compilation. Only Go
host boundaries recover traps; TypeScript `try/catch` cannot swallow them.

Main's optional `typeof`, indentation repeat, object/constructor and void-local
implementations were retained. The only extra `typeof` support concerns callable
thunks. General `String.repeat`, still needed outside main's indentation subset,
is emitted **only when used**, leaving the established default runtime unchanged.
Malformed-HIR receiver guards are retained for verification instead of compiler
panics. The Grace golden changes only in runtime catalogue counts for the added
repeat operation; lexer method/call-site facts are unchanged.

Passed with the stated environment:

```sh
ABAPITI_GRACECHECK=1 go test ./tsfront/... ./hir/... ./cmd/...
go vet ./...
node tools/abaplint-tests-report-test.mjs
node tools/abaplint-tests-runtime-test.mjs ~/dev/abaplint
```

Built the reference executable from **`git archive origin/main` (`54a521c`)**
inside the worktree. Both reference and harness generated fresh **default
all-target** outputs with identical `-o` spelling (driver READMEs embed it).
`diff -r` is empty, including the A4H zip: `round2-default.diff` is **zero bytes**.
Both builds exited zero. Test/vet logs and recorded verification metadata are
retained beside the raw runs and summarized in the latest JSON.
