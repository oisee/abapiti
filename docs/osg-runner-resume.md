# OSG test runner: host dump / resume

**2 real host dumps → 2 host-trap records → 3 binary invocations → all 6
fixture registrations reported.** The fixture retains **2 passes, 2 genuine
FAILs, 1 typed test trap, 1 host test trap**, plus **1 host initialization
trap**. The test following the dumping test passes. Both host traps retain
`NOT_COMPILED`; the test trap includes the native `find.go` location.

## Corpus measurements

OSGO `368baab28111a072c3f5779a0e07d259305f836f`, original abaplint
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`, Go 1.26.0, Node 26.
Measured against the #139/#140 base, with the new runner and bridge.

| Shard | Node active | Node pending | Registered | Pass | FAIL | Trap | Not-run / unregistered |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap/types` | 29 | 0 | 29 | 0 | 0 | 29 | 0 |
| `abap` | 216 | 4 | 51 | 4 | 0 | 47 | 169 |
| `abap/lexer` | 200 | 0 | 200 | 200 | 0 | 0 | 0 |
| Total | 445 | 4 | 280 | 204 | 0 | 76 | 169 |

All three shards produced reports, with no infrastructure failure, unmatched
result, or async-pending result. Every surviving registration has exactly one
result; duplicate title strings retain their original multiplicity. Lexer
remains **200/200**.

**Verification limitation:** these corpus runs did not reach a host
`NOT_COMPILED`. They do not demonstrate the originally requested bulk
newline-PCRE dump recovery. At this base, `abap` has eight caught initialization
refusals: seven `array.has` catalogue refusals and one HIR narrowing refusal.
They prevent 165 active and four pending registrations. `abap/types` has a
caught initialization class-view refusal but retains all 29 registrations.
The test traps in these shards are native class-view / checked-operation
exceptions, not host dumps. Registration parity is therefore false for
`abap`; missing titles remain explicitly unregistered, never synthesized or
credited as passes.

The initial default-target-pass `abap` run and the subsequent
`ABAPITI_INLINE=0 ABAPITI_SINGLETON=0` run both produced the same 51
registrations and 4 pass / 47 trap counts. Disabling those passes does not
repair the existing registration refusals. The table is the complete
no-inline three-shard run. The first directory run was stopped after exposing
the registration gap, not counted as a successful three-shard verification.

## Recovery protocol

`tsfront/abaplint_tests_osg.go` emits a runner with two explicit inputs:

- `start_index`: one-based first test to execute; earlier bodies are not rerun.
- `skip_init`: comma-delimited indices of initialization actions that have
  already dumped. Other actions, including later file registrations, replay
  in the original manifest order.

The ABAP runner calls its deliberately empty `checkpoint(value)` output
hook before each initialization action and test, and after each completed
result. It also emits an initialization-action count and the complete title
vector before executing tests. JSON escaping protects multiline titles and
messages. Successful initialization actions emit completion checkpoints;
caught initialization diagnostics are checkpointed separately.

OSGO ignores ordinary ABAP `WRITE`. `wireCheckpoints` in
`tools/osg-runner-resume.mjs` therefore adapts **only** the generated empty
checkpoint method to a Go stderr writer. It validates the method signature
and empty-body ABI, failing closed on drift. No test method, runtime refusal,
or OSG checkout file is rewritten.

The private stderr protocol is one JSON object per line, prefixed with
`ABAPITI_OSG_CHECKPOINT `. Generated Go main reads
`ABAPITI_OSG_START_INDEX` and `ABAPITI_OSG_SKIP_INIT`, passes them into ABAP,
and reports a recovered host panic with its first message line and native
source location. The stack walk follows re-panics through generated ABAP
catchers to the original panic site. `NOT_COMPILED` remains a host panic;
this does not make it catchable by `CATCH cx_root`.

`resumeRunner` runs the same built binary in fresh processes. It preserves
completed records, attributes an abnormal exit to the active checkpoint,
records a `trapped` result with `passed:false`, then starts at the next index.
An initialization dump becomes a separate initialization trap and its action
is skipped on restart. Replayed initialization diagnostics are deduplicated;
a previously checkpointed initialization FAIL is retained even if a host
dump follows it.

The initialization plan and registration vector must remain stable across
restarts. Test indices must advance exactly once through that vector. Each
retry consumes a previously unconsumed test or initialization action, so
there can be at most **tests + initialization actions + 1 invocations**.
A dump without an active checkpoint, an out-of-range/duplicate checkpoint,
changed registrations, spawn/buffer errors, or an incomplete final report
fails closed as infrastructure failure. Such failures never manufacture a
registration or a pass.

The bridge emits exactly one stdout JSON report with the original keys:
`asyncPending`, `passed`, `failed`, `trapped`, `notRun`, `results`, and
`initialization`. Counts are recomputed from the merged records. Assertion
FAILs are not relabeled; `abaplint-tests-all.mjs` keeps its existing fail-stop
and independent Node ownership checks. Code caches include the resume helper
bytes; verdicts are still never cached.

## Tests and limits

- The fixture-collecting `TestAbaplintTestHostOSGEmission` uses an original
  synthetic TS closure with a lazy regex on newline text, once during file
  initialization and once inside a test. It exercises real gogen/OSGO dumps,
  checks their messages, checks the later pass, and preserves both earlier
  assertion FAILs. It does not patch emitted test bodies to manufacture dumps.
- Node regression tests cover first/middle/last and consecutive dumping tests,
  duplicate and multiline titles, all initialization actions dumping,
  initialization FAIL preservation, stable registration checks, malformed
  protocol/infrastructure failures, and actual subprocess restarts.
- Gates: `go vet ./...`; `go test -short -p 2 ./tsfront/ ./cmd/...`;
  `node --test tools/*-test.mjs` (**33/33**); lint gate against `origin/main` with **0 new
  issues**, canary **5/5**.

A restart replays initialization, not earlier test bodies or their heap
mutations. Tests that require previous bodies' state cannot have that state
restored from a dead process. Initialization actions that dump midway are
skipped as units: partial registrations/state from the dead action cannot be
reconstructed. The independent oracle exposes missing registrations as
not-run. A failure before any checkpoint, or a kill of the bridge itself,
is still an infrastructure failure.

## Reproduction

Set the caller-owned paths for `ABAPITI_OSG_HOME`, `ABAPLINT_HOME`,
`OSG_HEAVY_LOCK`, `EVIDENCE`, `GOCACHE`, `TMPDIR`, `TRANSPILER`, and
`npm_config_cache` as described in [the speed/cache notes](osg-harness-speed.md).
Use the checkout matching `.github/ci/osgo.ref`, without modifying OSG
sources. Start only at load < 12; hold the shared lock for the entire heavy
step. Do not set `ABAPITI_OSG_LOCK` when the caller already holds it.

```sh
mkdir -p "$TMPDIR" "$EVIDENCE"
flock "$OSG_HEAVY_LOCK" systemd-run --user --scope -p MemoryMax=48G nice -n 10 \
  env GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false \
  ABAPITI_TESTS_TARGET=osg ABAPITI_INLINE=0 ABAPITI_SINGLETON=0 \
  ABAPITI_TESTS_SHARDS=abap/types,abap,abap/lexer \
  ABAPITI_TESTS_SCRATCH="$TMPDIR/shards" \
  node tools/abaplint-tests-all.mjs "$ABAPLINT_HOME" "$EVIDENCE/report.json"
```

The real-dump fixture can be run under the same lock/scope/environment with
`go test -short -p 2 ./tsfront -run TestAbaplintTestHostOSGEmission -count=1`.
After verification, remove the caller's scratch directory and the specified
OSG `tools/gogen/.out/go-cache` artifact under the shared lock. No OSG source
changes are required.
