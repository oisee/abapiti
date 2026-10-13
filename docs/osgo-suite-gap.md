# TS-HA@OSGO: new-pin lexer and statements measurements

## Resumed verification: f557e12f4, anchors unchanged

OSG **`f557e12f4217addcec7834b0607234aecf9b723a`** is pinned, with green
kernel-conformance CI (`38221901138`). Verified fix ancestry: #714
(`70b2c0193`), inbox 047 (`6f743d2b7`), #698 (`d858ca022`), #700
(`f4f872892`), #701 (`221cc4981`), and inbox 041 (`e7dd62ce2`).
Compact evidence: [osgo-pin-bump.json](evidence/osgo-pin-bump.json).
The resumed run uses the runtime after anchor withdrawal (`03537a6`), with
successful registry report output added in `46365a2` / `39c5f3f`.

| New-pin shard | Pass / Node active | FAIL | Traps | Registered | Frontend (s) | Emit (s) | Go build (s) | Run (s) | Phase total (s) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap/lexer` | 85 / 200 | 0 | 115 | 200 | 137.302 | 13.374 | 115.785 | 0.056 | 266.517 |
| `abap/statements` | 433 / 4,962 | 0 | 4,529 | 4,962 | 174.029 | 17.460 | 152.118 | 5.074 | 348.681 |

Both original Node shards pass their entire active denominators. OSGO has
**518/5,162 credited passes, FAIL 0, 4,644 traps**, exact title/registration
parity, and zero missing, pending, unmatched or initialization-failure titles.
Statements now returns a trustworthy report: the old bare-PCRE infrastructure
blocker is gone. **4,528 statements traps** are owning-body `chai.box` /
`chai.deepEqual` refusals; **one** is the native message `An exception was
raised.` (not yet attributed). None is credited as a pass. Lexer retains its
100 single-assertion and 15 double-assertion refusal-body reports.
The harness binary exits 1 for explicit traps; the reconciled two-shard
measurement exits 0 with no genuine FAIL. This is not a whole-suite certificate.
Both ABAP envelopes remain at 255 bytes maximum per line, zero overlong lines,
and 29 bytes maximum per object name. Timings include concurrent local gates;
TypeScript/HIR lowering and ABAP emission are outside the OSG phase totals.

### Registry parity, final CI and gates

- Fresh vanilla Node on the existing zabapgit kit: **clean 0**, SHA-256
  `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9`
  (**16.226 s**); **seeded 5**, SHA-256
  `7b9efbf9bf52830aa9868b079f473473f7c4f03d90d3339aed6e94b0081595b7`
  (**18.045 s**). Exact kit output bytes match, without normalizing whitespace.
- Seeded OSGO `ZCL_ABAPITI_REGISTRY_RUN`: **SUCCESS**, expected five-issue hash
  unchanged, **119.013 s execution**, **314.005 s total**, zero retries.
  Emit **17.834 s**, Go build **70.626 s**. Execution includes embedded-input
  decoding/loading, checking and the assertion. The coordinator's independent
  clean run at `40e673d` passed in **78–80 s**, emit **12.6 s**, cold Go build
  **41.8 s**, comparable to old-pin **75 s** (load 60 s + check 14 s).
  The resumed seeded timing was measured under concurrent load, not a serial
  performance comparison. Use `GOGEN_UNIT_TIMEOUT_MS=900000`.
- The unit driver asserts hash parity, clears `ok` on caught exceptions and
  now issues `WRITE / report` even on success; regression-tested. OSGO currently
  ignores ordinary ABAP `WRITE`, so `tools/osgo-registry-report.mjs` surfaces
  the returned report from a supplemental invocation of the same compiled
  driver, only after an unchanged successful unit result. It writes solely
  into the caller's build/scratch directories, never OSG. Live printed result:
  **ok=X, load_us=94885099, run_us=18581313**, seeded SHA above; helper wall
  **116.10 s** including its small Go build. It also rejects unsuccessful units.
  The legacy `want_issues=0` field is CLI diagnostic metadata, not the measured
  issue count; acceptance is the unchanged SHA of Node's five-issue dump.
- `.github/ci/osgo-unit.sh` / `.github/ci/hir-unit.sh` run unchanged as wired in
  **`osd-m1.yml`** (they no longer live in `ci.yml`). OSG unit **4,077/4,077**,
  FAIL 0; HIR checks both hosts, lexer 44/44 oracles and critic/number probes.
  The original narrow OSGO negative-lookbehind exec exemption is unchanged;
  that one native refusal is not a pass. Intentional verifier mutations must
  still produce their expected failures; no workload mismatch is exempted.
- Passed final `go test ./... -short`,
  `ABAPITI_GRACECHECK=1 go test ./tsfront/... ./hir/... ./cmd/...`, and
  `go vet ./...` (all exit 0). No assertion or oracle was weakened.

### Retained reverts and default output

Removed the broad `FIND PCRE` and submatch `LOOP` gaps. Retained FIRST/APPEND
and groups-copy forms; optional simplifications were unnecessary. Scalar
Map/Set keys regain the hashed secondary key; reference keys retain linear
identity lookup because #698 rejects reference key components. Scalar Set
removal uses `DELETE TABLE … WITH TABLE KEY by_k COMPONENTS k`, never the zero
`sy-tabix` of a hashed read. Optional `DELETE … WHERE` was skipped. #701's
move-then-clear optimization is already present; no `<dir>` workaround remains.

Fresh default output versus `git archive c921e37`, generated at the same path:
**6,312 files, 151 changed files, 50 class identities (48 Maps / 2 Sets)** in
`classes`, `osg`, `native/lib`, plus **50 matching zip members**. Every byte
change matches only: (1) scalar `WITH UNIQUE HASHED KEY by_k COMPONENTS k`;
(2) scalar reads `WITH KEY by_k COMPONENTS k`; (3) scalar Set
`DELETE TABLE … WITH TABLE KEY by_k COMPONENTS k`. Reference-key collections,
default regex runtime, names, interfaces, non-runtime classes, drivers and
READMEs are identical. Successful report output affects embedded drivers only,
not the default output. Generated outputs, raw reports and invocation scratch
are removed; OSG remains clean at the new pin, with `.out/go-cache` removed.
No new clone or push.

### Anchor incident and follow-up (migration remains withdrawn)

The earlier mandatory `TestEmitRegExpExec` OSG-JS differential returned a
wrong answer after changing absolute lookarounds to `\A` / `\z`. Work stopped
immediately; no exemption was added. Pinned transpiler/runtime **`ee5c2cd1`**
does not translate these escapes before JavaScript `RegExp`, which reads them
as literal `A` and `z`. OSGO support alone cannot certify both hosts. The unsafe
migration was withdrawn in `03537a6`, and the unchanged OSG-JS fixture now
passes. **Follow-up:** fix and bump the upstream transpiler/runtime pin on the
OSG-JS side, then reapply anchors and rerun both hosts and both shards. Do not
reapply the migration merely because OSGO supports it.

## Historical M0 at 7720f6e29 (unchanged evidence)

**85/200 lexer tests passed; FAIL 0; 115 trapped; 200 registered; no missing,
pending, unmatched, or initialization-failure titles.** Fresh original Node:
**200/200**, zero failures. Fresh resumed OSGO phase total **202.092 s**;
generated-Go build + execution **91.013 s**. This is an ABAP-host measurement,
not the 11,060/11,060 Go-target result and not a whole-suite ABAP certificate.

**Statements: 0 credited passes / 4,962 Node-active titles; all 4,962 not-run
because no trustworthy runner report was returned.** This is an infrastructure
refusal, **not 4,962 traps and not a FAIL-0 certificate** for statements. The
pinned host's RegExp constructor raises native `NOT_COMPILED ... FIND PCRE`
(`z_runtime_rege_8d1cc105868012.clas.abap:825`) which escapes ABAP `CATCH`.
The exact form is **`FIND PCRE posix IN ' '.`**: pinned
`tools/gogen/frontend.mjs:3176` rejects bare PCRE, while `PCRE ... RESULTS`
has separate support. The constructor is consequently replaced by a native
refusal stub. This is a **host statement-form gap**, not evidence of an invalid
regex pattern or a wrong ABAP answer.
Assertions, registration and test-body impact in that aborted shard are
**unmeasured**; no partial results are invented. A second unchanged-host attempt
retains gap and phase-timing evidence before cleanup: **267.908 s** total OSG
phases, **128.255 s** generated-Go build + execution before the native abort.
The Go build succeeded; the host binary exited **2** without a JSON report.

| Shard | Credited pass | FAIL | Traps | Not-run | Node active | OSG phases (s) | Go build + run (s) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap/lexer` | 85 | 0 | 115 | 0 | 200 | 202.092 | 91.013 |
| `abap/statements` | 0 | unmeasured | unmeasured | 4,962 | 4,962 | 267.908 | 128.255 |

The phase total is frontend + Go emission + build + execution; TypeScript/HIR
lowering and ABAP emission are outside this host-phase total. The statements
row is an aborted attempt, not a completed test run. Its printed harness zero
FAIL/trap/registration counters describe **no returned records**, not proven
assertion or registration outcomes.

**Static full closure: 625 roots / 17 shards / 2,166 files / 7,052 classes /
85 interfaces; blocking diagnostics 0; HIR errors 0.** Of the **26** operations
without an ABAP catalogue entry, **24 occur at 192 pre-emission HIR sites**.
The ABAP emitter additionally refuses **76 transformed-HIR sites in five
other groups**. Nonfinite literal sites **0**, line-envelope refusals **0**,
name refusals **0**. Compact evidence: [osgo-suite-m0.json](evidence/osgo-suite-m0.json).

## Runtime-operation inventory and measured shard impact

| Missing operation | Full closure sites | Lexer closure sites | Lexer refusal-body tests |
| --- | ---: | ---: | ---: |
| `array.delete` | 0 | 0 | 0 |
| `array.has` | 23 | 3 | 0 |
| `chai.arrayInclude` | 1 | 1 | 0 |
| `chai.box` | 10 | 10 | 115 |
| `chai.deepEqual` | 2 | 2 | 115 |
| `chai.deepInclude` | 1 | 1 | 0 |
| `chai.members` | 1 | 1 | 0 |
| `chai.throwMatch` | 1 | 1 | 0 |
| `classvalue.newArgs` | 2 | 2 | 0 |
| `dynamic.call0` | 6 | 2 | 0 |
| `dynamic.delete` | 3 | 1 | 0 |
| `json.indent` | 3 | 2 | 0 |
| `json.stringifySubset` | 34 | 4 | 0 |
| `json5.parse` | 1 | 1 | 0 |
| `lsp.isCreateFile` | 1 | 0 | 0 |
| `lsp.isDeleteFile` | 1 | 0 | 0 |
| `lsp.isRenameFile` | 2 | 1 | 0 |
| `lsp.isTextDocumentEdit` | 2 | 1 | 0 |
| `number.ceil` | 9 | 9 | 0 |
| `object.checkedClassView` | 77 | 2 | 0 |
| `string.padStart` | 3 | 0 | 0 |
| `string.parseFloat` | 0 | 0 | 0 |
| `string.parseFloatNumber` | 3 | 3 | 0 |
| `string.sha1Hex` | 1 | 1 | 0 |
| `string.splitRegex` | 2 | 2 | 0 |
| `xml.parseWithErrors` | 3 | 3 | 0 |

The two **115** counts describe the **same 115 tests**, not 230 tests.
There are 100 single-assertion and 15 double-assertion refusal-body reports.
The catalogue gate wins even where a legacy switch arm can print some ABAP
(`array.has`, `number.ceil`); unsupported storage semantics are not accepted.

**Impact limitation:** the unpruned harness supplies no test→function
reachability map. The last column measures reaching an *owning-body refusal*,
not executing every operation in that body. In particular, equality's deep
branch may be inactive; it nevertheless prevents whole-body emission. Zero
means no corresponding refusal-body report in this shard, not that the site is
safe or unreachable in the remaining suite. Impacts for the **15 unrun shards
and the aborted statements shard are unmeasured**, not extrapolated from the shared core closure. Statements has
**1,828 files / 2,367 classes / 75 interfaces**, zero blocking/HIR/coverage
errors, the same **53 runtime sites / 21 present unsupported operations** and
**70 other refusal sites** as lexer. Its site inventory is in the JSON; its
impact counts are **null**, not zero.

## Every other emitter refusal

| Refusal | Full sites | Lexer sites | Lexer refusal-body tests |
| --- | ---: | ---: | ---: |
| Dynamic/nonfinite `parseInt` Number input; requires a finite safe-integer literal | 66 | 66 | 0 |
| `classOf` on root `object` | 3 | 3 | 0 |
| Dynamic boxing of `i64` | 4 | 0 | 0 |
| Erased structural `object.has` | 2 | 1 | 0 |
| Fresh lexical constructor descriptor | 1 | 0 | 0 |

Examples include `_dynpros.ts:27`, `ddl_parser.ts:97`, four Message type
callbacks, both `applyEdit` helpers, and `_combi.ts:277`. The JSON retains
representative original source locations and owners for every group. Synthetic
ID-zero operands are attributed to their owning method, never all ID-zero nodes.

Runtime counts are expression occurrences **before ABAP passes**, including
lowered specializations. Other counts are distinct emitter refusals **after
ABAP passes**; inline copies and shared diagnostic strings are deduplicated
with source/body context. These two inventories must not be added as if they
were one pre-pass source-site census. Full inventory uses the production
`ABAPITI_INLINE=classic` policy. Node/source fingerprints, nil coverage,
`TrapOnFailure`, and explicit binary64 lowering are the same as the Go harness.

### ABAP-specific limits

- **NaN/Infinity:** zero surviving nonfinite HIR literals in all three measured closures.
  ABAP `f` is not an IEEE nonfinite substitute. Nonfinite literals, general
  Number remainder/division, and dynamic `parseInt` retain explicit emitter
  refusals; unsupported producing operations remain in the table above. Zero
  literal sites does **not** certify all runtime inputs finite.
- **255-byte lines:** no unbreakable-line diagnostic. The executed lexer emits
  **2,348 ABAP files**, and statements emits **2,606**; both have maximum line
  **255 bytes**, zero overlong lines, maximum object name **29 bytes**. Long
  literals are chunked and other lines wrapped, not truncated.
- **Names:** default hashed identities are at most **29 bytes** against the
  30-character ABAP envelope; collisions refuse. No naming refusal occurred.
  Fresh lexical identity is a separate ABI refusal, not a long-name problem.

## Harness and failure boundary

`abapiti abaplint-tests --target osg` emits the original core, test modules,
mocha/chai shim, and an ABAP runner. It executes registrations in manifest
initialization order and returns the Go report's JSON keys, titles, booleans,
statuses, messages and counters. The read-only host bridge imports pinned OSG's
frontend/emitter and copies its Go module into the invocation's output directory.
It does not install dependencies, fetch, switch commits, or write in OSG.

Refused owning bodies retain their signatures and raise dedicated coverage
exceptions. Assertion throws alone receive a subtype marker that preserves
source catch/payload semantics, including through inlining. Settled thunk
results are drained and their original rejections re-raised. Other native
exceptions are traps; compiler failures or OSG panics escaping ABAP remain
**infrastructure failures**, never invented assertions or registrations.

**Stopped synthetic miscompile:** an intentional `expect.fail` initially
reported passed because the first runner ignored the thunk's settled rejection.
Shard work was stopped and the root reported before repair. The live pinned-OSG
regression now requires **one pass, one intentional assertion failure, two
traps**, exact failure message, and exit 1. The fixture is not upstream and is
not included in 85/200. No upstream assertion mismatch was accepted.

**Remaining runner limit:** nonempty mocha hook chains explicitly trap before
the body; their lifecycle is not yet emitted. No hooks are silently skipped.
Lexer and statements have no affected hooks. Historical M0 statements was
blocked by the host's native PCRE refusal; the new-pin result above replaces
that infrastructure refusal with measured passes and traps. The remaining 15
shards were not run. Next work is Chai boxing/equality emission, the remaining
native exception, hook execution, runtime ABIs, and complete per-shard impact
measurements—not weakening assertions.

## Reproduction

The current checkout is `f557e12f4217addcec7834b0607234aecf9b723a`
(OSG main CI green). The following invocation reproduces the resumed
measurement with the absolute-anchor migration still withdrawn. Historical
M0 measurements remain bound to their original pin:

```sh
export GOCACHE=/mnt/ssd-510/osg/go-build TMPDIR=/mnt/ram/abapiti/tmp
export TRANSPILER=/mnt/ssd-510/osg/abapiti/transpiler-ee5c2cd1
export npm_config_cache=/mnt/ssd-510/osg/abapiti/npm-cache
export GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false GOMAXPROCS=4
# Full unpruned ABAP closure exceeds Node's default heap during Go emission.
export NODE_OPTIONS=--max-old-space-size=32768
export ABAPITI_OSG_HOME=/mnt/ssd-510/osg/abapiti/open-steamgate
ABAPITI_TESTS_TARGET=osg ABAPITI_TESTS_SHARDS=abap/lexer,abap/statements \
  nice -n 10 node tools/abaplint-tests-all.mjs /home/alice/dev/abaplint result.json
```

For a materialized closure, static measurement needs no OSG invocation:

```sh
ABAPITI_INLINE=classic nice -n 10 abapiti abaplint-tests CLOSURE \
  --target osg --gap-only -o OUT
# OUT/evidence/abap-gap.json contains every inventory/refusal group.
```

The original reporter reconciles Node/OSG titles unchanged. The all-shard tool
accepts `ABAPITI_TESTS_TARGET=osg` and respects the caller's `GOCACHE`; its
historical transient `go-*.json` filenames remain target-agnostic. OSG-only
static/gate/envelope/phase evidence survives cleanup even on infrastructure
failures. No-report impact is marked unmeasured, never inferred as zero.
The two-shard result is measured above, not extrapolated to the other shards.

For a successful embedded registry unit, retain its JSON and build directory
until the timing report is printed (this supplemental run repeats the driver):

```sh
OSG_HOME="$ABAPITI_OSG_HOME" nice -n 10 node \
  "$ABAPITI_OSG_HOME/tools/gogen/unit.mjs" \
  --input OUT/osg --class ZCL_ABAPITI_REGISTRY_RUN --jobs 1 \
  --no-cache --out UNIT_OUT > unit.json
nice -n 10 node tools/osgo-registry-report.mjs unit.json "$ABAPITI_OSG_HOME"
```

## Historical M0 gates and parity

Passed: `ABAPITI_GRACECHECK=1 go test ./... -short`, full Grace-enabled
`go test ./tsfront/... ./hir/... ./cmd/...`, `go vet ./...`, report/discovery
contracts, and the live OSG classification regression. Default `abaplint`
output versus **07714f1**, at the same output path, has **raw `diff -r` exit 0,
zero bytes, 6,312 files including the zip**. Go test emission remains on the
unchanged default path; new emission machinery is ABAP-target-only.

Generated closures, Go modules/binaries, raw reports, baseline output, and
invocation temporary artifacts are removed. OSG stays clean at the CI pin;
its ignored `tools/gogen/.out/go-cache` is removed. No push or clone.
