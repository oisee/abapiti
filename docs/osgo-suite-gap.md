# TS-HA@OSGO: pin bump blocked; historical M0 suite gap

## New pin: stopped on a mandatory OSG-JS wrong answer

OSG **`f557e12f4217addcec7834b0607234aecf9b723a`** is pinned, with green
kernel-conformance CI (`38221901138`). Ancestry checks include #714
(`70b2c0193`), inbox 047 (`6f743d2b7`), #698 (`d858ca022`), #700
(`f4f872892`), #701 (`221cc4981`), and inbox 041 (`e7dd62ce2`).
Compact evidence: [osgo-pin-bump.json](evidence/osgo-pin-bump.json).

**No new-pin suite or full-CI certificate.** The mandatory, unchanged
`TestEmitRegExpExec` differential returned a **wrong answer on OSG-JS** after
replacing absolute lookarounds with `\A`/`\z`: expected captures/indices beginning
`2:ab:a:b|1:a|4:a|reset:0|sticky:0:2:true`, but actual output began
`|last:0:true:0|anchor:true`. CI, registry and shard processes were stopped as
soon as the wrong answer was inspected; no failure exemption was added.

**Root cause:** the required OSG-JS path uses the pinned transpiler/runtime
`ee5c2cd1`, whose `packages/runtime/src/abap_regex.ts` does not translate
`\A` or `\z` before constructing JavaScript `RegExp`. JavaScript treats these
escapes as literal `A` and `z`. New-pin OSGO supports the anchors, but that
alone does not certify the required second host. The unsafe anchor migration
was **withdrawn**. The external OSG and transpiler sources were not modified.
The next gate needs an upstream, pinned transpiler anchor fix, followed by
reapplying the migration and rerunning both hosts and both shards.

| New-pin shard | Node-active denominator from M0 | Credited pass | FAIL | Traps | Times | Result |
| --- | ---: | ---: | --- | --- | --- | --- |
| `abap/lexer` | 200 | 0 | unmeasured | unmeasured | unmeasured | stopped during generated-Go build; no runner report |
| `abap/statements` | 4,962 | 0 | unmeasured | unmeasured | unmeasured | not started; sequentially follows lexer |

**Statements result: unmeasured, not FAIL 0, not 4,962 traps.** No returned
registrations or assertion results are invented. The old-pin 85/200 lexer
result below must not be presented as a new-pin run.

### Completed checks and retained changes

- Removed both broad gap entries (`FIND PCRE`, submatch `LOOP`). Retained the
  existing FIRST/APPEND and groups-copy forms; optional simplifications were
  not needed. The original narrow OSGO negative-lookbehind exemption in
  `hir-unit.sh` is unchanged; BDEF's real lookbehind remains a host refusal,
  not a passing native exec test. Trying `\b` instead also hit an explicit
  host refusal, so that attempt was withdrawn without changing the fixture.
- Restored scalar-key Map/Set hashed secondary keys. Reference keys retain
  their linear identity lookups: #698 explicitly refuses a secondary-key
  component of type `ref`. Scalar Set deletion uses `DELETE TABLE … WITH TABLE
  KEY by_k COMPONENTS k`, not the zero `sy-tabix` from a hashed read. The optional
  `DELETE … WHERE` change was skipped. #701's move-then-clear optimization is
  already present; no `<dir>` literal workaround remains to revert for inbox 041.
- A completed `osgo-unit.sh` attempt passed **4,077/4,077**, FAIL **0**. The
  final HIR run passed OSGO fixtures apart from the existing narrowly recorded
  exec refusal, then stopped on the OSG-JS assertion mismatch. These scripts
  now live in `.github/workflows/osd-m1.yml`, not current `ci.yml`.
- Fresh vanilla Node on the existing zabapgit check kit: **clean 0**,
  SHA-256 `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9`;
  **seeded 5**, SHA-256
  `7b9efbf9bf52830aa9868b079f473473f7c4f03d90d3339aed6e94b0081595b7`.
  OSGO clean parity genuinely passed: **117.799 s execution**, **262.771 s
  discovery/frontend/emission/build/execution**, zero retries. Execution includes
  embedded input decoding/loading and the check. Seeded OSGO was stopped before
  a report: **unmeasured**. Long registry runs use `GOGEN_UNIT_TIMEOUT_MS=900000`.
- The generated registry unit driver previously failed unconditionally just
  to display its report, including when `ok=X`. It now asserts the returned
  hash-parity boolean and clears it on caught exceptions; a dedicated regression
  checks this contract. No corpus assertion or expected hash was relaxed.
- Default output against `git archive c921e37`, generated at the same path:
  **6,312 files**, **151 changed files**, exactly **50 class identities (48 Maps,
  2 Sets)** copied into `classes`, `osg`, `native/lib`, plus those same **50 zip
  members**. Every change was checked against only these transformations:
  (1) scalar collection declaration gains `WITH UNIQUE HASHED KEY by_k COMPONENTS k`;
  (2) scalar Map/Set reads gain `WITH KEY by_k COMPONENTS k`;
  (3) scalar Set deletion becomes `DELETE TABLE … WITH TABLE KEY by_k COMPONENTS k`.
  Reference-key collections, default regex runtime, names, interfaces, other
  classes, drivers and READMEs are byte-identical.
- Before anchor withdrawal: `ABAPITI_GRACECHECK=1 go test ./... -short`,
  `go vet ./...`, and the live OSG classification regression passed. No final
  post-withdrawal certificate is claimed: testing stopped after the wrong answer.
  Scratch outputs and temporary artifacts were removed; OSG stays clean at the
  new pin, with `tools/gogen/.out/go-cache` removed. No clone or push.

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
Lexer and statements have no affected hooks. Statements was attempted but
blocked by the host's native PCRE refusal; the remaining 15 shards were not run.
Next work is the pinned-host regex refusal boundary, Chai boxing/equality
emission, hook execution, remaining runtime ABIs, and complete per-shard impact
measurements—not weakening assertions.

## Reproduction

The current checkout is `f557e12f4217addcec7834b0607234aecf9b723a`
(OSG main CI green). The following invocation is the reproduction recipe,
**not a completed new-pin certificate**; resolve the blocker above before
reapplying the anchor migration and certifying both hosts. Historical M0
measurements remain bound to their original pin:

```sh
export GOCACHE=/mnt/ssd-510/osg/go-build TMPDIR=/mnt/ram/abapiti/tmp
export TRANSPILER=/mnt/ssd-510/osg/abapiti/transpiler-ee5c2cd1
export npm_config_cache=/mnt/ssd-510/osg/abapiti/npm-cache
export GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false GOMAXPROCS=4
# Full unpruned ABAP closure exceeds Node's default heap during Go emission.
export NODE_OPTIONS=--max-old-space-size=32768
export ABAPITI_OSG_HOME=/mnt/ssd-510/osg/abapiti/open-steamgate
ABAPITI_TESTS_TARGET=osg ABAPITI_TESTS_SHARDS=abap/lexer \
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
Set `ABAPITI_TESTS_SHARDS=abap/lexer,abap/statements` for the two-shard attempt.

## Gates and parity

Passed: `ABAPITI_GRACECHECK=1 go test ./... -short`, full Grace-enabled
`go test ./tsfront/... ./hir/... ./cmd/...`, `go vet ./...`, report/discovery
contracts, and the live OSG classification regression. Default `abaplint`
output versus **07714f1**, at the same output path, has **raw `diff -r` exit 0,
zero bytes, 6,312 files including the zip**. Go test emission remains on the
unchanged default path; new emission machinery is ABAP-target-only.

Generated closures, Go modules/binaries, raw reports, baseline output, and
invocation temporary artifacts are removed. OSG stays clean at the CI pin;
its ignored `tools/gogen/.out/go-cache` is removed. No push or clone.
