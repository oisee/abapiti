# OSGO constructor-factory splitting

The current CI host pin is `98fa0675f47104ded037083c453ac0bb33b5ff57`
(OSG main, #721 PCRE hexadecimal escapes and #723 gogen CI). Host pins and
measurements below describe the historical feature run; final integration
measurements are recorded in [train-o3.md](train-o3.md).

The objects shard's constructor factory now has **246 methods, maximum 769
ABAP lines**, instead of a **29,071-line `new_args`** and a **3,487-line `new`**.
Line counts include `METHOD` and `ENDMETHOD`; the old `new_args` body alone is
29,069 lines. Method names are at most **19 characters**, and the factory's
maximum physical line length remains **253 bytes**.

Compact evidence: [osgo-splitargs.json](evidence/osgo-splitargs.json).

This changes compilation cost, not supported runtime-operation domains. No
catalogue entry was enabled, and no harness or runner was changed. This is not
an 11,060-test ABAP-host certificate.

## Implementation and semantics

Both public factory entry points normalize the emitted class name once and
call a balanced tree of private methods. Each routing method makes one string
comparison and calls one child. Sorted class names and preorder method numbering
make output deterministic. Leaves hold at most 64 classes and split further
when their combined constructor bodies exceed an 800-line budget.

Two individual wide field-copy constructors exceeded that budget even after
splitting the dispatcher: 317 and 58 argument slots. Wide constructors now use
a typed structure local to the factory invocation. Private helpers decode 24
slots at a time, in argument order. All decoding completes before lazy class
initialization and allocation. Further helpers copy fields in constructor-body
order after allocation. Inherited wide constructors use the same path. There
is no shared scratch state, including during nested factory calls.

The existing conversions, optional-presence guards, checked reference casts,
initialization calls, lexical-descriptor attachment and refusal domains remain.
Erased array reads still use `READ TABLE ... INTO` a raw `REF TO object` temp,
then `?=` to the Dynamic box. Missing slots remain undefined; surplus arguments
remain evaluated by the caller and ignored by the factory. The zero-argument
factory retains its existing dynamic `CREATE OBJECT` fallback.

Argument factories continue to refuse unknown/unbound descriptors, abstract
classes, variadics, unsupported parameter storage and behavioral constructors
above 40 parameters. Wide field-copy constructors retain their prior exception
to that limit. Wrong tags, missing required values, failed native reference
casts and absent optional primitives narrowed to required fields still raise;
helpers never replace them with default values.

## Compile-cost measurement

Baseline: `origin/main` at `3da6606`; implementation: `1c2b296`. Upstream source pin:
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. The unpruned objects closure has
**26 roots, 1,567 files, 2,276 HIR classes and 79 interfaces**, with zero blocking
diagnostics, HIR errors or lowering coverage traps. Manifest SHA-256:
`0cd89c2a4568fa6d57081759c51cea8999a421f1dc114b5b9a5cad99663b18df`.

OSG/gogen source was extracted at the repository's CI pin
`368baab28111a072c3f5779a0e07d259305f836f`. Native libraries were fetched into
isolated scratch checkouts at the exact commits in that pin's `libs.lock.json`,
including open-abap-core `d5aea88bdb3c8abfe3d57d35b686e933a2bdcfae`. The shared
OSG checkout and its libraries were not modified.

| Measurement | Before | After |
| --- | ---: | ---: |
| Factory methods | 2 | 246 |
| Maximum factory method lines | 29,071 | 769 |
| gogen frontend + Go emission | 30.446 s | 21.177 s |
| Complete benchmark-tool wall time | 35.32 s | 43.66 s |
| Benchmark-tool peak RSS | 844.50 MiB | 850.98 MiB |
| Fresh factory-package `go build` | 98.25 s | 43.24 s |
| Build peak RSS | 5.38 GiB | 2.16 GiB |

The measured Go build is **56% faster**, with **60% lower peak RSS**. These are
single samples on a shared machine. Tool wall time includes copying signatures
and the host Go module as well as the separately timed frontend/emitter phases;
it increased in this run, so no overall gogen wall-time improvement is claimed.

[The benchmark tool](../tools/class-factory-bench.mjs) preserves the factory
ABAP verbatim and removes other classes' method implementations, retaining
native signatures, layouts and inheritance as compilation scaffolding. Both
packages contain 2,442 compiled classes. This isolates factory compilation;
**the scaffold must not be executed** and does not establish suite semantics.
The factory must exist and have zero gogen refusal stubs; generated Go must
also build. Actual ABAP execution uses the complete fixtures described below.

Both builds use Go 1.26.0, `-p 2`, `GOMAXPROCS=2`, `GOMEMLIMIT=8GiB`, and a
24 GiB address-space limit. Dependencies were warm; the measured package was
forced to compile freshly by appending the same comment to each generated
source. `/usr/bin/time -v` measured wall time and peak RSS.

The **whole objects shard** maximum falls from 29,071 to **23,255 lines**. Its
new largest method is a pre-existing closure `invoke` in
`z_closure_3057_3824465ec85075`, outside the constructor-factory change. Other
large closure methods remain. The full shard Go binary was not built.

## Static refusals

The existing `MeasureABAPTestGap` / `--gap-only` counter from the M0 work is
already present on main and was used unchanged, with `ABAPITI_INLINE=classic`.
Runtime-operation occurrences are counted before ABAP passes; emitter refusals
are deduplicated after those passes. The objects-shard gap JSON is **byte
identical before and after**.

| Assigned factory item | Before refusal sites | After refusal sites |
| --- | ---: | ---: |
| `classvalue.newArgs` catalogue refusals | 0 | 0 |
| Constructor-factory emitter refusals | 0 | 0 |

The factory's dynamic refusal branches are deliberately preserved and are not
new source-site coverage. The objects closure still has **35 unsupported
runtime-operation occurrences** and **67 other emitter refusals**: 66 dynamic
Number `parseInt` sites and one erased structural `object.has` site. They are
unchanged and outside this speed change.

Supplementary audits of the original 625-root / 2,166-file closure were
attempted with the same M0 manifest
(`8bc69eca3e2735470d1f5aa2b955127eaf378f13cd0b781c750e2493185fa189`).
They were stopped after sustained memory growth without a returned gap report;
the unbounded baseline had approached the machine's memory ceiling. Even the
6 GiB Go heap-budget run exceeded 11 GiB RSS during emission. **Full-root
refusal counts are unmeasured here**, not inferred from the shard or prior
reports. The completed static comparison above is the objects closure.

## Validation

The ABAP backend exports and executes **273 ABAP Unit tests** through pinned
gogen/OSG with exact locked native libraries:

- 180 argument factories span multiple leaves, with distinct first/second slot
  values, missing optional arguments and evaluated surplus slots.
- 80 scoped zero-argument factories test both `new` and `new_args`, checked
  result types and lexical-descriptor identity.
- One refusal test checks missing required arguments, unknown names and an
  unbound descriptor.
- Six tests each for 41-slot and 317-slot wide factories check direct/inherited
  construction, distinct values across chunk boundaries, optional narrowing
  refusal and a wrongly tagged late argument.

Existing class-args, unsupported-storage, wide, class-value and fresh-descriptor
Go emitter tests pass. The complete implementations of the five exported ABI
fixture sets also compile through pinned gogen with zero emitted-class stubs,
and their generated Go packages build. The wider HIR/Node/Go semantic oracles
remain in the normal short gate.

Required gates:

```sh
export GOFLAGS='-p=2 -buildvcs=false' GOMAXPROCS=2

go vet -p 2 ./...
go test -short -p 2 ./...
ABAPITI_GRACECHECK=1 go test -p 2 -count=1 -run TestLexerFactsReport ./tsfront/
go test -short -p 2 -count=1 \
  -run TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend ./cmd/abapiti/
./.github/ci/lint.sh gate origin/main
```

Grace facts are unchanged; no golden regeneration was needed. Lint reports
zero new issues, canary 5/5, golangci-lint 2.13.2. Earlier gate attempts exhausted the package's cumulative 10-minute timeout
and picked up the lint gate's temporary canary package when lint and test
discovery overlapped. The final short gate runs after canary cleanup, with
child Go build parallelism bounded.

Default zabapgit Go output was generated from baseline and final executables at
the same output path: **`diff -r` exit 0, zero bytes, nine files**.

## Reproduction

Materialize the original objects roots using the existing closure tool and
runtime shim. Emit baseline and candidate ABAP with the unchanged test command;
a scratch no-op host bridge can be supplied to stop after emission. Obtain
static inventories without any host invocation using:

```sh
ABAPITI_INLINE=classic abapiti abaplint-tests "$CLOSURE" \
  --target osg --gap-only -o "$GAP_OUT"
```

Compile the emitted factory package without executing the signature scaffold:

```sh
NODE_OPTIONS=--max-old-space-size=8192 /usr/bin/time -v \
  prlimit --as=25769803776 node tools/class-factory-bench.mjs \
  "$ABAP" "$PINNED_OSG" "$BENCH_OUT"
GOMAXPROCS=2 GOMEMLIMIT=8GiB GOFLAGS=-buildvcs=false /usr/bin/time -v \
  prlimit --as=25769803776 go -C "$BENCH_OUT/go" build -p 2 ./generated/factory
```

Export complete executable fixtures:

```sh
ABAPITI_TEST_OUT="$FIXTURE_OUT" go test -p 2 -count=1 ./hir/abap \
  -run 'TestClassFactorySplitFixtures|TestClassArgs|TestFreshClass|TestClassValue'
OSG_HOME="$PINNED_OSG" GOFLAGS='-p=2 -buildvcs=false' GOMAXPROCS=2 \
  prlimit --as=25769803776 node "$PINNED_OSG/tools/gogen/unit.mjs" \
  --fixture "$FIXTURE_OUT/TestClassFactorySplitFixtures" --jobs 2 --no-cache
```

Repeat the last command for `TestClassArgsWideFieldConstructorABAP` and
`TestClassArgsHugeFieldConstructorABAP`; require every returned row to be
`SUCCESS`. No push was performed.
