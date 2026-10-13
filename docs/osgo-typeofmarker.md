# Constant-size ABAP callable typeof

The ABAP emitter replaces the closure-class census at each
`dynamic.typeofValue` site with one `IS INSTANCE OF` check against the hashed
`runtime.callable` marker interface. Each proven compiled closure implements
that interface. The marker uses the existing naming scheme and stays within
ABAP's 30-character object-name limit.

The predicate is identical to `hir/golang/typeof.go`: a `closure.*` class must
implement `closure.thunk` or `closure.function.*`. A class merely defining a
`call` method is insufficient. The dynamic reference-tag guard remains, as do
the existing primitive, null, undefined, array/object and class-descriptor
paths. Constructor descriptors already report `function`; they do not need
this marker. Box layout and call ABI are unchanged.

Native function values such as `Math.abs` and first-class `.bind(...)` values
still lack a proven frontend representation and retain explicit diagnostics.
The new Node-backed refusal tests verify this boundary. This change enables
no runtime operation and removes nothing from `unsupportedOps`.

## Verification

`TestTypeofCallableMarkerNodeOracle` compares Go with Node and attaches an
ABAP Unit assertion containing that same expected result through the
collecting `emitABAPNamed` helper. It covers captured arrows, function
expressions, void thunks, receiver-independent method values, static method
values, class constructor values, class instances, plain objects, arrays,
null, undefined, strings, numbers and booleans.

`TestCallableClassPredicate` covers both accepted closure interfaces and
non-callable lookalikes. `TestTypeofMarkerConstantSize` emits zero, one and 64
closure classes: no marker is emitted with zero closures; one and 64 produce
byte-identical probe classes with exactly one callable test.

Collected ABAP fixtures compile through the pinned gogen frontend, emitter
and Go compiler at OSG `368baab28111a072c3f5779a0e07d259305f836f`.
The oracle additionally executes as ABAP Unit on that host. Existing typeof
and closure regressions are included in the short suite. Native/bind refusal
oracles execute on Node.

## Measurement

Baseline is `origin/main` (`451603f`). Both emitters use the same materialized
objects test closure at abaplint `577f875e`, default Grace
inlining, and the same refusal/trap policy. Both revisions use the same mode. Counts include all
emitted `.abap` objects and runtime/runner sources. Method lengths count
`METHOD` through `ENDMETHOD`, inclusive.

| Objects closure metric | Before | After |
| --- | ---: | ---: |
| Emitted ABAP files | 2,571 | 2,572 |
| Total ABAP lines | 2,406,792 | 1,259,275 |
| Maximum method, including runtime | 29,295 | 29,295 |
| Maximum method in source classes | 27,646 | 7,497 |
| Maximum closure method | 27,646 | 3,226 |
| Callable-marker checks | 0 | 1,739 |

Total lines fall **47.7%**. The maximum closure method falls **88.3%**:
`z_closure_3057_3824465ec85075`'s invoke method replaces 8,177 callable-class
checks with 37 marker checks. There are 221 proven callable classes in this
closure. The overall maximum remains the unrelated `new_args` constructor
dispatcher in `z_runtime_clas_c216c6067abdc6`; this change does not optimize it.
The new interface contributes two lines, and each callable class contributes
one implementation declaration. Other closure-instance checks used for
non-typeof purposes remain intact.

The emission-only measurement calls `LowerAbaplintTests`,
`MeasureABAPTestGap` and `EmitAbaplintTestsOSG` on the same manifest for each
revision, writes the returned sources, then counts lines and trimmed
`METHOD`/`ENDMETHOD` boundaries. It does not execute or alter the test harness.
Compact size evidence is in [osgo-typeofmarker.json](evidence/osgo-typeofmarker.json).

Full-suite static refusals use the existing
`abapiti abaplint-tests CLOSURE --target osg --gap-only -o OUT` tool and its
`evidence/abap-gap.json`, as specified in [osgo-suite-gap.md](osgo-suite-gap.md).
Runtime counts are pre-pass HIR expression occurrences; emitter refusals are
deduplicated post-pass diagnostics. These are separate inventories.

The full **625-root / 2,166-file / 7,052-class / 85-interface** inventories
are identical as JSON before and after. Both lower with
zero blocking diagnostics and zero HIR errors. This run uses default Grace
inlining in both revisions; the inventory definitions and counting tool are
the same as the historical classic-inline measurement.

| Static inventory | Before sites | After sites |
| --- | ---: | ---: |
| `dynamic.typeofValue` refusals (this change) | 0 | 0 |
| Unsupported runtime operations, 15 present of 17 catalogue entries | 96 | 96 |
| Number `parseInt` without a proven finite safe-integer literal | 66 | 66 |
| Dynamic boxing of `i64` | 4 | 4 |
| Erased structural `object.has` | 2 | 2 |

The 72 other emitter refusals preserve the existing finite-number, boxing and
structural-layout boundaries. No nonfinite literal, line-envelope or name
refusal is present. The unsupported catalogue and each runtime operation's
site count are retained in the compact evidence. Their ABAP storage, dynamic
call, numeric, serialization or predicate implementations remain unproven;
this optimization supplies none of those missing semantics. No refusal is
removed or converted into a default value. Native/bind negative probes remain
explicit frontend refusals outside this successfully lowered static closure.

The post-change objects program also compiles through pinned gogen and a
single Go build with `-p 2 -gcflags=all=-dwarf=false -ldflags='-s -w'`.
Measured by GNU time: **420.59 seconds wall time**, **15,125,720 KiB peak RSS**
(14.43 GiB), exit **0**. `GOMAXPROCS=2` and `GOMEMLIMIT=18GiB` bound the build;
it ran alongside the full static inventories and tests. The generated Go
source is 114 MiB. No baseline build was timed, so this is a feasibility
measurement, not a build-speedup claim. The objects suite was not executed by
this measurement; explicit owning-body/runtime refusals remain present.

## Gates and output parity

Passed `go vet -p 2 ./...`, `go test -short -p 2 -timeout 30m ./...`,
`ABAPITI_GRACECHECK=1 go test -p 2 -count=1 -run TestLexerFactsReport ./tsfront/`,
`TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`, and
`./.github/ci/lint.sh gate origin/main`: **0 new issues, canary 5/5**.
The lexer facts golden is unchanged. The first short-suite attempt exceeded
Go's default 10-minute package timeout under concurrent measurement load;
the final complete run passed, with tsfront taking 543.901 seconds.

The pinned compile gate built 13 collected emitted fixtures (12 unique
programs), including existing typeof/closure regressions and the new marker
scaling fixtures. The refreshed oracle, including its additional void thunk,
also compiled and executed successfully through pinned gogen/ABAP Unit.

Default zabapgit Go generation used origin/main and the candidate binaries
sequentially at the same output path, retaining a baseline copy before the
candidate run. Raw `diff -r` exits **0** with **0 bytes**. No Go backend source
changed. Commits are local; nothing was pushed.

This is an emission-size change, not a whole-suite 11,060-test ABAP-host
certificate. Unrelated runtime and emitter refusals remain explicit.
