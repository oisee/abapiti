# Settled async tests on the rebased TS-HG@Go harness

`feat/harness-async-v2` starts at `f51056b` (`f51056bd2139b3e2dc779079773fbd6d1538cd7c`) and ports the semantics
of `56bf815`, without merging the old harness or prerequisite branches.
Tested implementation: `4e87240d652e37f3933e3d869a531aa70221064a`. The prerequisite async implementation is
already inherited from the rebased harness. Documentation from `29d913e` is
refreshed here with fresh runs of all four shards.

The test-only thunk interface returns a settled `Promise<void>`. A separate
producer method retains the callback's fulfillment type, including non-void
Promise values. Both the thunk and Go reporter drain that result before
recording success. Synchronous callbacks returning a Promise are covered.
Error and primitive rejections retain their messages. Uncertified pending
work is `async-pending`, never pass, and causes a nonzero executable exit.
Parameterized `done` callbacks remain `not-run` under `unsupported-test-done`.

The mocha shim registers suite-scoped `before`, `beforeEach`, `afterEach`, and
`after` hooks. Setup runs from outer to inner suites; teardown reverses that
order. Before/after hooks run once per selected suite. Setup failure is replayed
for affected tests without invoking their bodies. Teardown runs for entered
suites, and an earlier failure remains primary. Hooks receive `TestContext`.
Skipped and focus-excluded tests do not activate hooks.

The port retains the rebased harness's independent initialization actions,
readiness guards, contextual registration layouts, boxed chai comparisons,
and non-suspending async suites. Only deferred callbacks belonging to the owned
mocha shim are excluded from module-initialization static-read checks. Suite
registration and user functions with the same spelling remain inspected.
The original test and assertion sources are copied and hashed without rewrites.

## Fresh four-shard measurements

Pinned upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`
(@abaplint/core 2.120.56). Both builds select every `.ts` file in sorted order
from each shard, including helpers, and supply no reachability manifest.
The before compiler and materializer are archived from `f51056b`; the after
compiler and materializer use the port. Fresh original mocha/chai runs provide
a common full-title multiset oracle for both builds, including duplicate titles.

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

These are **661/8,461** passing selected tests, not all
11,060 tests or a claim that all core bodies are translated. No shard is counted
twice. Node oracle results and all initialization failures are in
[`abaplint-tests-async.json`](abaplint-tests-async.json).

Rules retain **3,074 registered tests** and all 11 recorded initialization
failures. All **1,458** formerly not-run async callbacks now have explicit
outcomes: **313 reach traps**, **1,145 are async-pending**. None becomes a pass,
and no previously registered title is lost. Rules not-run is 10 skipped tests
plus 85 unregistered tests. The settlement proof still refuses uncertified
producers, including virtual `Registry.parseAsync` through local receivers.

Reached traps continue to expose existing core and test-helper gaps: Config's
JSON round trip, downport's JSON/conf702 paths, the unpinned dense `forEach` in
`test/rules/_utils.ts`, and plain-row/generic reference ABI issues. Remaining
registration failures are recorded rather than fabricated. The full raw trap
ranking and per-shard failures are retained in the JSON; reserved roots were
not changed by this port. General pending promises, timers, arbitrary chai APIs,
and parameterized done callbacks still need separate implementation/proofs.

## Strict core gap and gates

Strict `abaplint --full` blocking diagnostics: **168 -> 168**;
`unsupported-async`: **0 -> 0**.
Root/cascade counts: **105/63 -> 105/63**;
HIR verification errors: **258 -> 258**.
Exact blocking counts by category and message are recorded in the JSON.
Harness normalization and hook execution are opt-in via `TrapOnFailure`, so
this port does not close additional strict-core diagnostics.

Passed: generated-Go harness regressions (including initialization recovery
with an async suite), Node report/summary contracts, all eight Node runtime
scenarios, requested vet, and short tests with `-p 2` on
`./hir/... ./tsfront/... ./internal/... ./cmd/...`.
The Node shim awaits native promises; that does not certify arbitrary producers
for TS-HG@Go.

Fresh default `abapiti abaplint -o "$RUN/default" --target all,go` builds use the
same output path spelling for the two compilers. Their saved trees compare
byte-identically: `diff -r` is empty, including ABAP, zip, Go executable, and
site metadata. No string-repeat or other reserved implementation was changed.

## Reproduction and retained evidence

Raw artifacts are in `/tmp/harness-async-v2`: `node-*.json`,
`before/after-go-*.json/log`, `before/after-report-*.json`,
`before/after-*/closure.json`, generated `*-out/go/`, lowering inventories,
`full-before/after/evidence/`, gate logs, and zero-byte `default.diff`.
The committed JSON records the exact measurement script and compiler SHAs.
Use a clean checkout of the pinned upstream with installed npm dependencies;
the materializer refuses existing closure directories.

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"
export GOMAXPROCS=2 GOFLAGS='-p=2 -buildvcs=false'
RUN=/tmp/harness-async-v2-repro
UPSTREAM=~/src/abaplint-pinned
# Run from the tested implementation recorded above; use fresh artifact paths.
mkdir -p "$RUN/base"
git archive f51056b | tar -x -C "$RUN/base"
(cd "$RUN/base" && go build -o "$RUN/base-abapiti" ./cmd/abapiti)
go build -o "$RUN/branch-abapiti" ./cmd/abapiti
for spec in rules abap/lexer abap/statements abap/structures; do
  name=${spec##*/}
  mapfile -t files < <(cd "$UPSTREAM/packages/core" && rg --files "test/$spec" -g '*.ts' | sort)
  node tools/abaplint-tests-oracle.mjs "$RUN/node-$name.json" "$UPSTREAM" "${files[@]}"
  for stage in before after; do
    if [[ $stage == before ]]; then
      compiler="$RUN/base-abapiti"
      materializer="$RUN/base/tools/abaplint-tests-closure.mjs"
    else
      compiler="$RUN/branch-abapiti"
      materializer=tools/abaplint-tests-closure.mjs
    fi
    node "$materializer" "$RUN/$stage-$name" "$UPSTREAM" "${files[@]}"
    "$compiler" abaplint-tests "$RUN/$stage-$name" -o "$RUN/$stage-$name-out" \
      >"$RUN/$stage-go-$name.json" 2>"$RUN/$stage-go-$name.log" || test "$?" = 1
    node tools/abaplint-tests-report.mjs "$RUN/node-$name.json" \
      "$RUN/$stage-go-$name.json" "$RUN/$stage-report-$name.json"
  done
done
# Exact strict-core commands, also recorded in the JSON:
"$RUN/base-abapiti" abaplint --full -o "$RUN/full-before"
"$RUN/branch-abapiti" abaplint --full -o "$RUN/full-after"
```

Exit 1 from strict full-closure measurement or reached test failures/traps is
expected. All five reconciled outcome counts sum to each Node denominator;
unregistered oracle titles remain explicit not-run evidence.
