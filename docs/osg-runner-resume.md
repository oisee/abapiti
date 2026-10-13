# OSG test runner: host dump / resume

**4 real host dumps → 4 host-trap records → 5 binary invocations → all 14
fixture registrations reported.** At OSG **16c024c8**, the merged fixture
expects **7 passes, 2 genuine FAILs, 3 typed test traps, 2 host test traps**,
plus **2 host initialization traps and 1 subsequent binding trap**. This
retains the pin-bump baseline of **6 pass / 2 FAIL / 4 trap** and adds one
host-body trap and one subsequent pass. Both multiline JS whitespace and
ASCII word/digit replacements must pass. Every registration is checked in
order, once; both earlier assertion FAILs survive recovery.

The deliberate initialization and body dumps now use the reviewed DDL
`/\bdefine\s+(?:\w+\s+)*?behavior\s+for\s+([\w/]+)(?:\s+alias\s+(\w+))?/gi`
exec pattern on `"define behavior for foo"`. Its leading `\b` lowers to
`(?<![A-Za-z0-9_])`. OSG's `tools/gogen/go/abap/find.go` `compilePCRE` still
emits `NOT_COMPILED in FIND PCRE` for negative lookbehind, which is not in
Go's RE2. Unlike absolute anchors, `adaptTestRegex` does not rewrite this
assertion. Tests require that exact reason and the native `find.go` location.
OSG #726 accepts unanchored newline PCRE at this pin, so the old
`"a\nb".replace(/x+/g, "")` is no longer a dump trigger. No OSG sources or
emitted test bodies are patched.

## Merge re-verification at 16c024c8

Merged `origin/main` at `b160f52` (including #143, #144 and the CI changes),
retaining OSG pin `16c024c8fc175b5656d643034b0fe8dde484740f`. Verified against
original abaplint `577f875ebec44cfaf64841cfe71c8ab8dc32622e`, Go 1.26.0,
Node 26, with `ABAPITI_INLINE=0 ABAPITI_SINGLETON=0`.

| Shard | Node active | Node pending | Registered / reported | Pass | FAIL | Trap | Not-run / unregistered |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap/lexer` | 200 | 0 | 200 | 200 | 0 | 0 | 0 |
| `abap/types` | 29 | 0 | 29 | 6 | 0 | 23 | 0 |
| `abap` | 216 | 4 | 216 | 120 | 0 | 96 | 4 |
| Total | 445 | 4 | 445 | 326 | 0 | 119 | 4 |

Lexer remains **200/200**. Because the merge changes the runner, types and
abap were also rerun; all three shards retain the previous pin-bump counts.
Every registered test has one result, with **0 infrastructure failures,
unmatched results, active unregistered tests or async-pending results**.
The four not-run titles are Node-pending cases absent from the OSG registry,
not active tests or credited passes. Corpus counts are not proof of host-dump recovery: the
real emission fixture separately verifies the four dumps described above.

Passed `go vet ./...`;
`GOFLAGS=-buildvcs=false go test -short -p 2 ./hir/... ./tsfront/ ./cmd/... -count=1`;
`node --test tools/*-test.mjs` (**37/37**); and
`TestAbaplintTestHostOSGEmission` with `ABAPITI_OSG_HOME` at the pin.
The lint gate against `origin/main` reports **0 new issues**, canary **5/5**,
golangci-lint **2.13.2**. Heavy runs held the shared flock in a 48 GiB systemd
scope at nice 10, admitted at load <12. Caller scratch and the OSG gogen
Go-cache artifact are removed after verification; OSG sources are unchanged.

## Historical merge re-verification after #141

Verified code `4944b08`, merged `origin/main` at `b874d6d`, with clean OSGO
`368baab28111a072c3f5779a0e07d259305f836f` and original abaplint
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. Go 1.26.0, Node 26;
`ABAPITI_INLINE=0 ABAPITI_SINGLETON=0`.

| Shard | Node active | Registered / reported | Pass | FAIL | Trap | Not-run / unregistered |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap/lexer` | 200 | 200 | 200 | 0 | 0 | 0 |
| `abap/types` | 29 | 29 | 0 | 0 | 29 | 0 |
| Total | 229 | 229 | 200 | 0 | 29 | 0 |

Both reports have **0 infrastructure failures, unmatched results, async-pending
results and pending registrations**. Lexer remains **200/200**; types retains
**29/29 registrations**, not 29 passes. The broader `abap` shard was not rerun
for this merge; its figures below remain historical. The real fixture separately
verifies two initialization dumps and two body dumps, with all 12 registrations
reported. All heavy steps held the shared flock in a 48 GiB systemd scope at
nice 10, admitted at load <12. Scratch and the OSG gogen Go cache are removed
after verification; no OSG source files are changed.

## Corpus measurements before #141

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

`tsfront/abaplint_tests_osg.go` emits a runner with three explicit inputs:

- `start_index`: one-based first test to execute; earlier bodies are not rerun.
- `skip_init`: comma-delimited indices of initialization actions that have
  already dumped. Other actions, including later file registrations, replay
  in the original manifest order.
- `shard`: the runtime selector passed from `ABAPITI_OSG_SHARD`. Union builds
  execute only that shard's original dependency-first initialization order;
  legacy per-shard builds ignore this input. Unknown union selectors fail
  closed before producing a plan or report.

The all-shard OSG harness now builds the union once and invokes the same binary
separately for each selected shard. `osg-runner-execute.mjs` creates a fresh
`resumeRunner` state for every shard and retains its selector on every restart.
Initialization skip indices and test start indices are local to that shard;
registration still happens through original module actions, not title filtering.
The five-shard `98fa0675` comparison reports identical ordered title/status
vectors: **382 registered, 239 pass, 143 trap, 0 FAIL, 0 unregistered** on both
paths. See [the wall-time/RSS measurement](osg-harness-speed.md).

The real union fixture also passes at this pin: one build, alternating a/b/a/b
selections, **5 registrations in a and 2 in b** each time. Each a retains
**3 passes, 1 genuine FAIL, 1 host test trap and 1 host initialization trap**
after two real dumps and three subprocess invocations. Each b remains clean
with **2 passes** in one invocation. The duplicated `duplicate` title occurs
exactly twice in either selection (one borrowed, one owned); no initialization
skip or test-start state crosses shard boundaries. A ninth invocation rejects
an unknown selector without a report.

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
`ABAPITI_OSG_START_INDEX`, `ABAPITI_OSG_SKIP_INIT` and `ABAPITI_OSG_SHARD`, passes them into ABAP,
and reports a recovered host panic with its first message line and native
source location. The stack walk follows re-panics through generated ABAP
catchers to the original panic site. `NOT_COMPILED` remains a host panic;
this does not make it catchable by `CATCH cx_root`.

`resumeRunner` runs the same built binary in fresh processes. It preserves
completed records, attributes an abnormal exit to the active checkpoint,
records a `trapped` result with `passed:false`, then starts at the next index.
An initialization dump becomes a separate initialization trap and its action
is skipped on restart. Each `host_init_N` boundary, including the forced runtime
initializer, is one action. In a legacy per-shard build its suffix, plan index,
pre-action checkpoint and `skip_init` token are the same stable one-based number.
In a union, host boundary suffixes are unique across the whole module, while
plan/checkpoint/skip indices restart at one in each shard's branch. Those local
indices remain stable on every restart; a suffix is not a cross-shard skip token. `host_body` remains the
test-call boundary. The bridge uses `protectTestBoundaries` in resume mode:
boundary ABIs and native exception-name diagnostics are retained, but host
panics unwind to main rather than being recovered in-process. This lets the
parent record and skip the dumping action in a fresh session. The default
in-process boundary adapter remains separately regression-tested.

Replayed initialization diagnostics are deduplicated; a previously
checkpointed initialization FAIL is retained even if a host
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
and generated-main bytes; verdicts are still never cached. The generated Go
executable also exits **1**
for a caught initialization FAIL or trap, even when every registered test
passes, matching `reportStatus`. Executable regressions cover initialization-only
refusals, test refusals and a clean report; the parent still returns **1** when
only a previous invocation dumped and the final invocation is clean.

## Tests and limits

- The fixture-collecting `TestAbaplintTestHostOSGEmission` uses an original
  synthetic TS closure with the reviewed DDL exec pattern, once during file
  initialization and once inside a test, plus #141's native-PCRE refusal
  cases. It exercises four real gogen/OSGO dumps, checks their lookbehind
  reasons, checks the later pass, and preserves both earlier assertion FAILs.
  It does not patch emitted test bodies to manufacture dumps. The Node resume
  tests use a matching simulated lookbehind diagnostic; the resume helper
  itself has no newline-specific assumption.
- The fixture-collecting `TestAbaplintTestShardsOSGEmission` compiles one union
  and alternates a/b/a/b selections. It checks duplicate-title multiplicity,
  borrowed registrations, initialization/body dumps, preservation of an earlier
  genuine FAIL, a following pass, and no skip/index/state leakage into another
  shard. An unknown selector must fail without manufacturing a report.
- Node regression tests cover first/middle/last and consecutive dumping tests,
  duplicate and multiline titles, all initialization actions dumping,
  initialization FAIL preservation, stable registration checks, malformed
  protocol/infrastructure failures, and actual subprocess restarts.
- Gates: `go vet ./...`; `go test -short -p 2 ./tsfront/ ./cmd/...`;
  `node --test tools/*-test.mjs` (**37/37**); lint gate against `origin/main`
  with **0 new issues**, canary **5/5**, golangci-lint **2.13.2**.

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
  ABAPITI_TESTS_SHARDS=abap/types,abap/lexer \
  ABAPITI_TESTS_SCRATCH="$TMPDIR/shards" \
  node tools/abaplint-tests-all.mjs "$ABAPLINT_HOME" "$EVIDENCE/report.json"
```

The real-dump fixture can be run under the same lock/scope/environment with
`go test -short -p 2 ./tsfront -run TestAbaplintTestHostOSGEmission -count=1`.
After verification, remove the caller's scratch directory and the specified
OSG `tools/gogen/.out/go-cache` artifact under the shared lock. No OSG source
changes are required.
