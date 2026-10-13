# OSGO Chai: unit and lexer pass; statements still blocked

**43 generated ABAP assertions execute successfully on pinned OSGO;
lexer 200/200, FAIL 0, traps 0, unregistered 0. Statements 429/4,962,
FAIL 0, 4,533 traps, unregistered 0. This is a two-shard measurement,
not full-suite certification or permission to merge O1.**

| Shard | O1 baseline pass / active | After Chai pass / active | FAIL | Traps | Active unregistered | Host phases (s) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap/lexer` | 85 / 200 | **200 / 200** | 0 | 0 | 0 | 245.701 |
| `abap/statements` | 433 / 4,962 | **429 / 4,962** | 0 | 4,533 | 0 | 451.846 |
| **TOTAL** | **518 / 5,162** | **629 / 5,162** | **0** | **4,533** | **0** | **697.547** |

Measured 12 Oct 2026 with compiler `93cb475`, OSG
`f557e12f4217addcec7834b0607234aecf9b723a`, unchanged original Node
oracle `577f875ebec44cfaf64841cfe71c8ab8dc32622e`. Both shards have
complete registration parity and no Node-pending titles.
[Compact host evidence](evidence/osgo-chai-host.json).

Lexer gains all 115 previously trapped cases. In statements, **4,528** former
Chai lowering traps now reach **`json5.parse`**, which still has no ABAP
emitter (`src/config.ts:99:31`). Removing the Chai refusals did not turn those
cases into passes. Five more cases trap with the native message
`An exception was raised.`; O1 had one such case, so four former passes are
lost. Two unmodified statements runs give the same counts. A focused diagnostic
replay attributes the four extra traps; no trap is credited as a pass.

`Result.getNodes()` (`src/abap/2_statements/result.ts:78`) allocates
`new Array(this.nodeCount)` and fills it backwards. The Chai-enabled emitter
refuses nonempty preallocation before padding, so this pattern is outside its
current sparse-array ABI even when later fully filled. A focused
replay of the five exact affected titles, with observational logging around
`abap.Raise` in a temporary generated-Go copy, confirms **four**
`Z_EXCEPTION_CH_7F15176A28F44D` raises from `Result.getNodes()` at generated
ABAP line 348. The RECEIVE version-failure case and three WITH version-failure
cases are the four regressions. The fifth, LOOP AT SCREEN, is the pre-existing
native trap. The replay is diagnostic evidence, not a changed certification
suite; exact titles and frames are retained in the compact evidence. The
existing sparse safeguards remain intact. Fixing preallocation requires the
array storage ABI to preserve missing slots while permitting later filling;
this branch is not merge-ready while the four coverage regressions remain.

## Host compatibility repairs and completed gates

The real host run caught two OSG frontend limits that source syntax checks
had not caught. Plain `FIND FIRST OCCURRENCE OF ... IN ...` preserves ABAP's
case-sensitive substring default and avoids unsupported `SUBSTRING` and
`RESPECTING CASE` forms. The real-Chai `BOOM`/`boom` answers remain unchanged.
Chai catches translated payload exceptions explicitly, rather than passing
OSG's runtime `exc` representation from `CATCH cx_root` into a reference
parameter. Native faults and coverage refusals propagate directly.

The generated unit executes all **43** assertions with **SUCCESS**, including
cycles, duplicates, signed-zero boundaries, mutation, translated string/Error
throws, settled rejection draining and refusal propagation. The synthetic
host classifier also executes successfully: **1 pass / 2 intentional assertion
failures / 1 explicit trap**, with the expected failure message. These intentional
fixture failures are outside the upstream suite's FAIL count.

`go vet ./...`, `go test -short -p 2 ./...`, uncached Grace lexer facts and
lint against `origin/main` pass; lint reports **0 new issues, canary 5/5**.
Default Go output from `e29f167` and the changed compiler, generated at the same
path, is byte-identical: **0 changed bytes**, executable included.

## Implemented observation boundary

`hir/abap/chai.go` implements `chai.box`, `chai.deepEqual`, `chai.deepInclude`,
`chai.arrayInclude`, `chai.members` and `chai.throwMatch`. The catalogue routes
all six to checked emission instead of refusing their entire owning bodies.
The unsupported catalogue is **24 → 18** versus the actual `bef1975` catalogue.
The old synthetic inventory test's 26 was already stale at that base; its new
exact count retains the independent synthetic-occurrence/nonfinite checks.

- Boxing preserves references, undefined and null. No native object/array is
  snapshotted merely to assert. Parsed/native snapshots and backed live views
  refuse at the boxing boundary.
- Deep equality uses SameValue primitive comparisons, positional dense arrays,
  exact own enumerable key sets and pair memoization for cyclic graphs.
  Graph records, source-certified typed records (`OwnKeys` on construction),
  and the Go backend's required-field data shapes have enumeration adapters.
  Each adapter reads original storage on every invocation; mutations and
  aliases remain observable.
- Integral boxes, numeric zero literals (including unary signs) and JSON number
  tokens retain zero-sign metadata. Proven `0` and `-0` compare differently in
  deep equality and equally in strict equality. Unproven runtime zero/zero
  comparisons explicitly refuse. Nonfinite literals/producing operations retain
  the existing ABAP refusals; no NaN/Infinity substitute is introduced.
- Shallow array inclusion uses strict equality. Members consume one matching
  occurrence at a time, with either strict or deep comparison. Deep object
  inclusion requires property presence as well as value equality, preserves
  the empty-subset no-assertion result `2`, and refuses unmodeled inherited
  properties. String matching is case-sensitive `FIND ...`, **not**
  ABAP's case-insensitive `CS`.
- Throw matching invokes an action once and ignores its return value. Certified
  typed thunks drain settled rejections; erased action dispatch accepts only
  compiled parameterless closures. Only translated string/native Error
  payloads supply a substring message. Coverage refusals, unsupported payloads
  and native host faults propagate, never converted to a successful match.

### Explicit remaining representation limits

This is **not unrestricted Chai/deep-eql support**. Distinct native Map/Set
instances and nominal classes with unproven enumerable storage refuse, as in
Go's existing Chai adapters; identical references retain identity equality.
Sparse arrays remain outside the ABAP storage ABI: sparse writes and nonempty
`new Array(n)` refuse before dense default-value padding can fabricate slots.
RegExp arrays with missing-capture metadata also refuse observation. Dense
present-undefined slots are not treated as holes. No generic prototype, symbol,
custom deep-eql hook, asynchronous action or erased callable ABI is added.

The ABAP runtime additions are usage-gated. Non-Chai programs acquire neither
the helper nor its zero-sign/record metadata. No Go backend or frontend
production code changed.

## Tests and gates completed

- `TestChaiEmission` always runs, including without upstream npm dependencies;
  checks all six emissions, cycle/case/refusal boundaries, line envelope and
  usage gating. `TestChaiSparseRefusalEmission` requires refusal **before**
  sparse padding.
- `TestChaiFixtures` uses actual Chai **4** on Node, not the shim or a second
  deep-eql implementation. Its 32 checks cover primitive/tag distinctions,
  property presence, positional arrays, cycles, strict identity, duplicate
  members, deep inclusion, case-sensitive string/throw matching, string/native
  Error throws, signed zero and data shapes. Those answers generate a unit
  fixture with **43 assertions**, also covering typed-array mutation, typed
  thunk rejection draining and explicit refusal propagation. Node checks and
  emission passed; independent ABAP syntax lint: **30 files, 0 issues**.
  The generated ABAP assertions now execute successfully on OSGO, as measured above.
- The existing frontend real-Chai/Go differential remains intact and green.
  It uses a fresh lowering for its new ABAP emission check, so ABAP passes
  cannot mutate the Go oracle's HIR.
- The synthetic live-host classifier expectation is strengthened from
  `1 pass / 1 intentional failure / 2 traps` to
  **`1 pass / 2 intentional failures / 1 trap`**: the former equality coverage
  trap must now be an assertion failure with the exact message
  `assertion failed: expected 1 to equal 2`. Its explicit Date refusal and
  `expect.fail` checks remain. This host-only portion now passes on OSGO.
- Passed `go vet ./...`; `go test -short -p 2 ./...` (**227.35 s**, then final
  cache-confirming run **2.50 s**); Grace-enabled, uncached
  `TestLexerFactsReport` (**5.77 s**); lint against `origin/main`: **0 new issues,
  canary 5/5**, golangci-lint 2.13.2 (**5.18 s**). No golden was changed.
- Default Go output generated at the same output path from `git archive
  bef1975` and the changed compiler: raw `diff -r` **exit 0, 0 bytes**, all
  **9 files including `zabaplint-go`** identical. No clone or push.

Implementation commit: `17e1ff4`.

## Earlier blocked attempt and continuation

The initial builder stopped before OSG execution because its sandbox could not
see the concurrent O1 job's exit marker. O1 subsequently finished; the current
coordinator read `pi o1 exit=0` and merged its repaired head `63b91b7` before
host validation. The original factory defect is repaired in `3ab143c`.

O1 itself remains blocked by 85 unregistered rules tests; see
[train-o1.md](train-o1.md). Chai's lexer result does not certify the classview
operations or those initializers. Next work is the JSON5 emitter, the array storage ABI
for the four preallocation refusals, and the pre-existing LOOP AT SCREEN trap. Preserve FAIL 0
and registration accounting; do not remove sparse refusals merely to regain a
passing count.
