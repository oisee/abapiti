# abaplint's own tests on TS-HG@Go: spike

## Status: acceptance is still blocked

At upstream `577f875ebec44cfaf64841cfe71c8ab8dc32622e`:

| Run | Passed | Failed | Status |
| --- | ---: | ---: | --- |
| Original `7bit_ascii.ts`, fresh Node build + original mocha/chai | 7 | 0 | Verified |
| Same upstream test closure on TS-HG@Go | — | — | Refused before emission: 441 blocking diagnostics, 315 HIR errors |
| Synthetic Go-host test | 1 | 1 (deliberate) | JSON, registration, per-test records and exit status verified |

**The requested seven TS-HG@Go passes have not been achieved.** No upstream
mocha tests executed on Go. A blocked translation is not seven failed tests,
and neither the Node oracle nor the synthetic host test is TS-HG acceptance.
The TypeScript runtime is a Node-validated draft; callback `this` binding and
function/namespace merging still prevent lowering it as a whole.

The complete closure has **1,542 files** (1,538 upstream source files, two
upstream test files, two runtime files), **2,058 classes**, **148 interfaces**,
and **zero coverage traps**. `_utils.ts` imports the core barrel and Registry,
so even this seven-case file reaches essentially the entire core. Bodies in
that import closure must all lower; most blockers are outside SevenBitAscii.

## Run

Run from the abapiti repository/worktree. The spike requires the read-only
pinned upstream checkout with its installed npm dependencies; test files are
not embedded in the abapiti executable. No npm downloads are performed.

```sh
export GOCACHE="$HOME/.cache/go-build"
export GOTOOLCHAIN=go1.26.0
export GOFLAGS=-buildvcs=false
SCRATCH=/tmp/claude-1000/-home-alice-dev-abapiti/3f37988e-1ca8-4480-a1b0-02daa31f268f/scratchpad/tests
export TMPDIR="$SCRATCH"
UPSTREAM=/home/alice/dev/abaplint

# Use a fresh closure directory: the materializer refuses an existing one.
node tools/abaplint-tests-closure.mjs \
  "$SCRATCH/my-closure" "$UPSTREAM" test/rules/7bit_ascii.ts

go run ./cmd/abapiti abaplint-tests "$SCRATCH/my-closure" \
  -o "$SCRATCH/my-results"
```

Additional test files are additional arguments to the materializer, e.g.
`test/rules/7bit_ascii.ts test/rules/abapdoc.ts`. Their union is translated as
one closure, with shared runtime state and mocha-style `.only` selection.
There is no separate `--file` CLI option: selection happens at materialization.

The materializer reuses the exported closure walker in
`tools/registry-closure.mjs`. It verifies the upstream commit/clean checkout,
copies original sources without edits, records SHA-256 hashes, maps `chai`
and `mocha` to `runtime/*.ts`, includes mocha globals, and disables ambient
npm `@types`. DFS postorder records module evaluation order. The original
Registry command's closure manifest was checked to remain identical.

`tsfront.LowerAbaplintTests` verifies the materialized hashes and reuses
`LowerRegistryWithOptions` with `RegistryOverrides()`, **nil reachability**
and an explicit binary64 numeric policy. `ABAPITI_ASSUME_INT` does not change
this harness's policy. There is no body pruning and no test-specific override
of `_utils.ts`, SevenBitAscii or Registry.

The CLI always writes:

- `evidence/registry-blocking.json`: individual diagnostics and TS locations;
- `evidence/registry-verify.txt`: individual HIR errors;
- `evidence/registry-overrides.json` and `registry-traps.json`;
- `evidence/test-gate.json`: totals and descending diagnostic-category counts.

It exits nonzero on a blocked lowering and **does not emit partial Go**.
After a successful gate it emits `go/{hir.go,runtime.go,main.go,go.mod}`, builds
`go/abaplint-tests-go`, and runs it. Per-test `{title,passed,message}` records
go to stderr; stdout is one JSON object:

```json
{"passed":7,"failed":0,"failures":[]}
```

That example is the target schema, **not the current Go result**. The host
exits 1 for assertion failures, 2 for a harness panic or no executed tests,
and 0 for a nonempty passing run. Use a fresh output directory to avoid
mistaking a previous module for a newly emitted one.

### Go gates and Node comparison

```sh
go test ./tsfront ./hir/... ./cmd/abapiti

# Opt-in acceptance gate: currently FAILS, with the same strict refusal.
ABAPLINT_TEST_CLOSURE="$SCRATCH/my-closure" \
ABAPITI_TEST_OUT="$SCRATCH/test-evidence" \
  go test ./tsfront -run '^TestAbaplintOwnTestsGo$' -count=1 -v

# Independent contract checks for the runtime draft (Node, not TS-HG).
node tools/abaplint-tests-runtime-test.mjs "$UPSTREAM"

# Fresh pinned original sources + original mocha/chai, not our runtime.
node tools/abaplint-tests-oracle.mjs "$SCRATCH/node.json" \
  "$UPSTREAM" test/rules/7bit_ascii.ts
```

The regular Go package gates pass. The runtime's six Node scenarios cover
success/failure messages, fluent forms, nested suites, timeout context,
multiple focused tests/suites, skip, and empty/skipped focused suites. The
synthetic Go host gate also checks source-hash rejection, binary64 policy,
actual generated `main.go`, stdout/stderr separation, and failure exit status.
Compiler regression tests execute lowered getter reads, stored thunks,
optional `typeof` comparisons and `String`/`repeat` on Go. Repeat edge tests
include fractional/NaN counts, negative/infinite counts, empty strings,
UTF-16 lone-surrogate copying, and the runtime's string-length bound.

The original seven full titles (escapes show control characters) are:

```text
test 7bit_ascii rule "WRITE: / 'æøå'." should have 1 issue(s)
test 7bit_ascii rule "WRITE: / 'abc'." should have 0 issue(s)
test 7bit_ascii rule "" should have 0 issue(s)
test 7bit_ascii rule "  " should have 0 issue(s)
test 7bit_ascii rule "method( \r)." should have 1 issue(s)
test 7bit_ascii rule "method( \r\n)." should have 0 issue(s)
test 7bit_ascii rule "method( \n)." should have 0 issue(s)
```

## Runtime API and limitations

`tsfront/testdata/abaplint-tests/runtime/mocha.ts` implements synchronous
registration, nested full titles, `describe`, `describe.skip`, `describe.only`,
`it`, `it.skip`, `it.only`, global focus selection, and a timeout context
whose `timeout()` is a no-op. `TestRuntime.run()` returns a named summary
shape and stores every executed test's result in `TestRuntime.results`.
Skipped tests are not counted in `passed` or `failed`.

`runtime/chai.ts` implements:

- `expect(value, message?)`, `.to.equal`/`.equals`, negation and optional
  assertion messages;
- `.to.be.instanceof` and `.to.not.be.instanceof`;
- `.contain`/`.include`, including negation, for strings and arrays;
- `.to.have.length(n)`;
- `.to.be.undefined`/`.to.not.be.undefined`;
- `expect.fail(message)`.

The chain words are ordinary fields, not Proxy machinery. `undefined` is a
getter, so it asserts only when read; the compiler now lowers getter reads.

**Not yet TS-HG-lowerable:** merged function namespaces (`it.only`,
`describe.skip`, `expect.fail`), and binding a suite callback's `this` with
`Function.call`. Three `_utils.ts` callbacks fail the existing lexical-thunk
`this` restriction. These are recorded blockers, not source rewrites or
no-op replacements of upstream assertions. The `mocha` path is a globals
script, suitable for the current suite's ambient/side-effect use; named
module imports would need a re-export shim.

Not implemented: asynchronous tests/Promise waiting/done callbacks, hooks,
retries, real timers, deep equality or `deep.include`, arbitrary chai chains,
and mocha's complete pending/reporting API. The runtime is synchronous-only;
do not interpret an async callback as a valid passing test. Arbitrary object
string coercion is not supported by the compiler. String/array containment
also remains subject to existing Go-runtime operation limits.

## Blockers met and ranked

Before the general fixes, this test closure reported **661 diagnostics and
397 HIR errors**. After them it reports **441 diagnostics and 315 HIR errors**:
220 fewer diagnostics and 82 fewer HIR errors. These totals include cascades
from failed declarations; they are not counts of independent missing features.

### Fixed on this branch

| Blocker | Initial occurrences / scope | Fix |
| --- | ---: | --- |
| Missing RuntimeOp receiver dereferenced by numeric passes | 2 unsafe pass sites | Guard both integer-assumption and range-rewrite signature lookups; verification still rejects incomplete HIR |
| `string.repeat` | 40 diagnostics | Numeric/omitted count lowering, HIR signature/effects and Go runtime; JS truncation/NaN, negative/infinite-count rejection and UTF-16 copying |
| Optional `typeof` comparisons | 6 diagnostics | Presence-aware comparisons; evaluate operand once; include I64 tagging |
| Assertion getter | 2 diagnostics (signature/body passes) | Lower class getter reads as method invocations; setters remain blocked |
| Unimplemented ABAP string operation | New repeat op would leave an initial result | Fail closed on unhandled string ops; test explicit ABAP refusal |
| Primitive `String(...)` calls | Includes 4 runtime coercion calls | Reuse primitive coercion, including I64 and builtin Error formatting |
| Stored thunk invocation | 1 runtime `test.body()` diagnostic | Invoke thunk-valued fields; allow always-throwing `never` callbacks |

`string.repeat` is implemented for **Go** in this spike. The ABAP backend
still rejects this new operation explicitly; the existing fingerprinted
`repeatIndent` operation and its deployments are unchanged. Negative/infinite
counts use the existing Go `rangeFault` refusal, not a JS-catchable RangeError
object; the edge probes do not certify that exception-object ABI. No broad support
for RegExp results or external namespace constructors was approximated.

### Remaining diagnostic categories (descending)

| Category | Count |
| --- | ---: |
| unsupported-expr | 186 |
| unsupported-call | 92 |
| unsupported-top-level | 84 |
| unsupported-type | 33 |
| unsupported-binding | 16 |
| unsupported-number | 9 |
| unsupported-callback | 4 |
| unsupported-regex | 4 |
| unsupported-closure | 3 |
| unsupported-new | 3 |
| unsupported-statement | 3 |
| unsupported-assignment | 2 |
| unsupported-async | 1 |
| unsupported-shortcircuit | 1 |
| **Total** | **441** |

Largest identifiable remaining root-message groups, rather than cascaded
unknown-local errors:

| Root message / group | Count | Why not approximated |
| --- | ---: | --- |
| Namespace declarations | 84 | 81 external vscode-languageserver-types declarations; 3 runtime merged namespaces; need actual functions/shapes, not declaration execution |
| Namespace `create`/`replace` calls | 33 | `create` on maps 15, class values 13, `replace` on maps 5; external constructor ABI |
| `Date` as a value | 32 | Runtime/time API and external declaration handling |
| `string.match` calls | 31 | Requires null-preserving match arrays, captures and global behavior; 5 additional RegExpMatchArray type errors |
| Object literal without named shape | 17 | Reusable anonymous shape identity/materialization needed |
| Void locals | 16 | Need a value-preserving result ABI, not erased initializer effects |
| `RegExp.exec` result | 10 | 5 calls + 5 RegExpExecArray type errors; match object/index ABI |
| Binary64 Math.min/max | 9 | Frontend's integer-only certification remains; exceptional numbers/signed zero contract |
| `JSON` as a value | 7 | General serialization paths beyond existing pinned serialization overrides |
| `DocumentSymbol` type | 5 | External named shapes |
| Non-literal regex replacement | 4 | Pattern/replacement semantics |
| Suite callback `this` | 3 | `_utils.ts` lines 36, 72, 169; lexical thunk ABI is not a JS call-bound context |
| Callback with multiple returns | 3 | Existing inline-callback restriction |
| Primitive-array `find` absence | 3 | Absence-preserving callback ABI |
| `Function.call` context argument | 1 | Runtime scope callback needs explicit `this` binding |

There are also smaller expression, new/statement, assignment, async and
short-circuit gaps listed with exact locations in the evidence.

SevenBitAscii itself has **two exec calls**: `/[\u007f-\uffff]/` and `/\r.+$/`.
Both read `.index` after a null check; the current truthiness-only
exec-to-test optimization cannot handle them. A correct implementation must
preserve null, UTF-16 indexes and the match-result object. That is the first
rule-local blocker, but fixing it alone cannot make the unpruned closure
pass. Next priorities are the suite-context/namespace ABI and the shared
full-core gaps above. Further Go backend/build blockers cannot be measured
until the strict frontend/HIR gate passes.
