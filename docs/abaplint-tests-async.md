# Settled async tests on TS-HG@Go

`feat/harness-async` extends `origin/feat/abaplint-tests-harness` with
`origin/feat/fit-async` merged in. The merge preserves the harness's optional
`typeof` handling and combines its shim dispatch with Promise dispatch.

The test-only thunk interface returns a settled `Promise<void>`. A producer
method keeps the callback's actual fulfillment type, including non-void
Promise values, and the thunk drains that result before returning its own
fulfillment. The Go reporter drains the thunk before recording success.
Normal compilation keeps the existing void thunk interface.

Async arrow/function callbacks and synchronous callbacks returning a Promise
use the producer certification from the async prerequisite. Rejected promises
produce failed records with the original Error message or primitive rejection
reason. A lowering refusal for pending work or an uncertified provider produces
`async-pending`, its own counter, and a nonzero executable exit. It never counts
as a pass. Parameterized `done` callbacks remain not-run under
`unsupported-test-done`; they are not misclassified as pending promises.

The mocha shim registers suite-scoped `before`, `beforeEach`, `afterEach`, and
`after` hooks. The Go host drains setup from outer to inner suites and teardown
from inner to outer suites. Before/after hooks run once per selected suite;
skipped and focus-excluded tests do not activate hooks. A failed before hook
is replayed for every affected test without invoking its body. Teardown still
runs for suites already entered. A teardown failure is assigned to the affected
test (the last selected test for an after hook); an earlier failure remains the
primary result. Hook callbacks receive `TestContext`, including its no-op timeout.

Module-initialization safety skips the deferred bodies of actual owned mocha
`it` and hook callbacks. It continues inspecting immediate suite-registration
code and user functions with the same spelling. Upstream test sources and
assertion bodies remain unchanged.

Generated-Go regressions cover async and returned-Promise fulfillment,
Error/string/undefined rejection, pending callbacks and hooks, setup failure
replay, teardown rejection, skipped hooks, context binding, and loop captures.
Native Node contracts independently check fulfillment and hook ordering.
The Node shim uses native await; its ability to run arbitrary native promises
is not evidence that those producers are certified by TS-HG@Go.

Measurements and reproduction commands are recorded in
`docs/abaplint-tests-async.json`.

## Measured rules shard

Both hosts selected the same 192 rule files (1,732 files in each materialized
closure), without a reachability manifest. Original mocha registered 3,159 tests:
3,148 passed, 10 pending, and one exceeded its original 250 ms timeout under
host load. That timeout does not change the title-multiset denominator.

| Outcome | Harness parent | Async integration |
| --- | ---: | ---: |
| Passed | 0 | 5 |
| Failed | 0 | 0 |
| Trapped | 1,338 | 1,610 |
| Async-pending | 0 | 1,134 |
| Not-run | 1,821 | 410 |
| Total | 3,159 | 3,159 |
| Registered | 2,730 | 2,759 |

Match the original 1,382 async records by full title and occurrence ordinal:
**308 are now invoked and reach a trap**, 1,058 are async-pending, and 16 are
unregistered. None of those 1,382 is counted as a pass. The five passes are the
`blank line matching` tests. Remaining not-run is 10 skipped tests plus 400
unregistered tests.

The settlement proof still refuses many virtual `Registry.parseAsync` producers
through local receivers. Invoked async tests reach existing Config JSON-round-trip
and downport JSON/conf702 traps. The 16 lost registrations come from the merged
prerequisite's object-array typing: `definitions_top` (5), `obsolete_statement`
(4), and `unnecessary_pragma` (7) fail module verification because `testRule`
expects an array of its row shape but receives `array<builtin.object>`. Those
object/type ABI roots are reserved for another owner.

The strict core `abaplint --full` comparison is **373 -> 250 blocking diagnostics**,
including **1 -> 0 unsupported-async**. Root/cascade counts are 155/218 -> 145/105;
HIR verification errors are **291 -> 325**. These measurements include both
prerequisite branches; the Registry settlement fix is inherited from the async
branch. Exact diagnostic messages and verification-error counts are in the JSON.
They do not certify all core bodies or all abaplint tests as translated.

Vet, short tests with `-p 2`, generated-Go harness regressions and both Node
contracts passed. The required identical-path default `all,go` comparison found
only an inherited unused `string.repeat` helper in `go/runtime.go` and the resulting
Go executable difference. All other outputs match. Its emission-only patch is
prepared at `/tmp/harness-async/repeat-emission.patch`; applying it requires an
exception to the instruction reserving repeat for another owner. The helper's
implementation has not been changed.

## Reproduction

Use a clean checkout of the recorded upstream pin with dependencies installed.
The before compiler is built from an archive of the recorded harness-parent SHA;
the main compiler is built from an archive of the recorded main SHA. Materialize
the before closure with the archived harness tools, so its shim stays unchanged.
The JSON records the exact compiler commands and artifact paths used for these
runs. Exit 1 from either test binary is expected for reached failures/traps.

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"
export GOMAXPROCS=2
export GOFLAGS='-p=2 -buildvcs=false'
UPSTREAM=~/src/abaplint-pinned
RUN=/tmp/harness-async
mapfile -t files < <(cd "$UPSTREAM/packages/core" && rg --files test/rules -g '*.ts' | sort)
node "$RUN/base/tools/abaplint-tests-closure.mjs" "$RUN/rules-before" "$UPSTREAM" "${files[@]}"
node tools/abaplint-tests-closure.mjs "$RUN/rules-after" "$UPSTREAM" "${files[@]}"
node tools/abaplint-tests-oracle.mjs "$RUN/node-rules.json" "$UPSTREAM" "${files[@]}"
"$RUN/base-abapiti" abaplint-tests "$RUN/rules-before" -o "$RUN/before-out" \
  >"$RUN/go-before.json" 2>"$RUN/go-before.log" || test "$?" = 1
"$RUN/branch-abapiti" abaplint-tests "$RUN/rules-after" -o "$RUN/after-out" \
  >"$RUN/go-after.json" 2>"$RUN/go-after.log" || test "$?" = 1
for name in before after; do
  node tools/abaplint-tests-report.mjs "$RUN/node-rules.json" \
    "$RUN/go-$name.json" "$RUN/report-$name.json"
done
```

The materializer refuses an existing closure directory; use fresh artifact paths
when reproducing. Original sources are copied and hashed, without test-body or
assertion rewrites. The raw Go records, Node oracle, reconciliation reports,
lowering inventories, generated modules and default comparison are retained
under the artifact directory.
