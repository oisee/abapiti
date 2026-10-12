# OSGO Chai: emitter implemented; host certification blocked

**6 Chai operations emitted; 30 ABAP fixture files with 0 syntax issues;
43 generated ABAP unit assertions; 32 direct real-Chai oracle checks;
0 changed bytes in default Go output versus `bef1975` (9 files, executable included).
New OSGO passes, FAIL, traps and execution times are unmeasured.**

| Shard | Historical baseline pass / Node active | Baseline FAIL | Baseline traps | Baseline OSG phases (s) | After this change |
| --- | ---: | ---: | ---: | ---: | --- |
| `abap/lexer` | 85 / 200 | 0 | 115 | 266.517 | Not run: O1 exit guard blocked |
| `abap/statements` | 433 / 4,962 | 0 | 4,529 | 348.681 | Not run: O1 exit guard blocked |

Baseline numbers are from [osgo-suite-gap.md](osgo-suite-gap.md), **not** a fresh
measurement or a certificate for this branch. There is no claim of a pass gain
or post-change FAIL 0. The expected ABAP answers below have not yet been compared
with actual OSGO execution.

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
  properties. String matching is case-sensitive `FIND ... SUBSTRING`, **not**
  ABAP's case-insensitive `CS`.
- Throw matching invokes an action once and ignores its return value. Certified
  typed thunks drain settled rejections; erased action dispatch accepts only
  compiled parameterless closures. Only translated string/native Error
  payloads supply a substring message. Coverage refusals, unsupported payloads
  and native host faults are re-raised, never converted to a successful match.

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
  **The generated ABAP assertions have not been executed on a host.**
- The existing frontend real-Chai/Go differential remains intact and green.
  It uses a fresh lowering for its new ABAP emission check, so ABAP passes
  cannot mutate the Go oracle's HIR.
- The synthetic live-host classifier expectation is strengthened from
  `1 pass / 1 intentional failure / 2 traps` to
  **`1 pass / 2 intentional failures / 1 trap`**: the former equality coverage
  trap must now be an assertion failure with the exact message
  `assertion failed: expected 1 to equal 2`. Its explicit Date refusal and
  `expect.fail` checks remain. This host-only portion is **pending**, not green.
- Passed `go vet ./...`; `go test -short -p 2 ./...` (**227.35 s**, then final
  cache-confirming run **2.50 s**); Grace-enabled, uncached
  `TestLexerFactsReport` (**5.77 s**); lint against `origin/main`: **0 new issues,
  canary 5/5**, golangci-lint 2.13.2 (**5.18 s**). No golden was changed.
- Default Go output generated at the same output path from `git archive
  bef1975` and the changed compiler: raw `diff -r` **exit 0, 0 bytes**, all
  **9 files including `zabaplint-go`** identical. No clone or push.

Implementation commit: `17e1ff4`.

## O1 guard and required continuation

The required guard is:

```
/tmp/claude-1000/-home-alice-dev-abapiti/3f37988e-1ca8-4480-a1b0-02daa31f268f/scratchpad/o1/pi.log
```

It must contain `pi o1 exit=` before **any OSG/OSGO invocation**. In this
agent's sandbox only its own scratch directory is mounted under the temporary
namespace; the O1 log is absent. O1 was also observed still running. No host
invocation was attempted, no marker was fabricated, and another process's exit
or output was not substituted for that guard. Independent `@abaplint/core`
syntax lint reads dependency sources only; it does not execute OSG.

After the actual guard becomes readable and passes, regenerate and execute:

```sh
export GOCACHE=/mnt/ssd-510/osg/go-build TMPDIR=/mnt/ram/abapiti/tmp
export TRANSPILER=/mnt/ssd-510/osg/abapiti/transpiler-ee5c2cd1
export npm_config_cache=/mnt/ssd-510/osg/abapiti/npm-cache
export GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false GOMAXPROCS=4
export NODE_OPTIONS=--max-old-space-size=32768
export ABAPITI_OSG_HOME=/mnt/ssd-510/osg/abapiti/open-steamgate
work=$(mktemp -d "$TMPDIR/osgo-chai.XXXXXX")
ABAPITI_TEST_OUT="$work/fixtures" nice -n 10 go test -count=1 \
  ./hir/abap -run '^TestChaiFixtures$'
nice -n 10 node "$ABAPITI_OSG_HOME/tools/gogen/unit.mjs" \
  --input "$work/fixtures/TestChaiFixtures" \
  --class Z_CHAIPROBE_F9474CFB5E78BB --jobs 1 --no-cache \
  --out "$work/unit" > "$work/unit.json"
# Synthetic classifier: two expected assertion failures, one explicit trap.
nice -n 10 go test -count=1 ./tsfront \
  -run '^TestAbaplintTestHostOSGEmission$'
ABAPITI_TESTS_TARGET=osg ABAPITI_TESTS_SHARDS=abap/lexer,abap/statements \
  ABAPITI_TESTS_SCRATCH="$work/shards" nice -n 10 node \
  tools/abaplint-tests-all.mjs /home/alice/dev/abaplint "$work/shards.json"
```

Require the unit to pass first. A genuine answer mismatch must stop work, be
root-caused and receive a regression before running the upstream shards.
Record each shard's N, FAIL, traps, registration parity and host phase times;
FAIL must remain zero. Update this document with measured numbers, not an
estimate based on the removed catalogue entries.

Own generation/archive/output scratch was removed. OSG was verified clean at
`f557e12f4217addcec7834b0607234aecf9b723a` and was not modified. No OSG invocation
from this job created a cache; the concurrent O1 job's cache was deliberately
left untouched. After host continuation, remove its invocation scratch and
`tools/gogen/.out/go-cache` only once the clone is free.
