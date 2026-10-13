# O3 statements: native exceptions and ABI repair

Base: `integ/o3`, `8275c0b`. Local branch: `fix/o3-statements`.
Original upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`, TypeScript
6.0.3. Clean pinned OSG: `368baab28111a072c3f5779a0e07d259305f836f`.
A compiler built from the actual checkout reproduces **433 passes, 0 FAIL,
4,529 traps, 0 async-pending, 0 not-run**, with all 4,962 original Node titles
registered and matched. Fresh original Node passes **4,962/4,962**, with no
pending cases. There are no initialization exceptions.

## Exact root-cause partition

The temporary host instrumentation prefixes `Exception.TextOf` with its native
class and records `abap.Raise` stacks in the copied output runtime. It changes
neither assertions nor status classification. These are complete-shard counts,
not estimates from a Config probe.

| Escaping exception | Tests | Actual cause | Ownership |
| --- | ---: | --- | --- |
| `Z_VIEW_REFUSAL_6FCECBE45FBFF9` | 4,528 | `runtime.dynamic.get` cannot cast the boxed native constructor map to `view.storage` | ABAP storage/emission ABI |
| `Z_EXCEPTION_CL_E4A48BC113B232`, carrying `FailCombinatorError` | 1 | A derived Error payload's wrapper does not inherit the wrapper caught for `Error` | ABAP exception/class ABI |
| **Total** | **4,529** | | |

Both inherited `get_text()` implementations return `An exception was raised.`.
The view refusal has no custom message. The second payload is constructed with
no message; its source `Error.message` is empty. Its generated payload class is
`Z_SRC_ABAP_2_S_E9C68C45B277EA`, from `src/abap/2_statements/combi.ts:814`.

There are also **4,528 internally handled** throws from `Registry.find()`:
`Z_EXCEPTION_CL_30EEC8E8805D60`, carrying the ordinary source Error
`find: object not found, <type> <name>`. `findOrCreate()` catches those to create
an object. They are not another group of trapped tests. The single
FailCombinatorError is logged again when its settled rejection is raised by
the runner; that rethrow is still one test.

Representative complete-shard title-family counts: APPEND 46 map traps,
SELECT 309, CALL FUNCTION 33, WRITE 82, and LOOP AT SCREEN two. The remaining
LOOP AT SCREEN case is exactly
`LOOP AT SCREEN statement version should fail, "LOOP AT SCREEN." should not be recognized`.
Title families can span files; these counts are illustrative subsets of the
complete exception partition above.

## Native frames and source attribution

Go's emitted `//line` directives give relative ABAP locations in native stacks.
Addresses and generated closure frames are omitted here.

```text
abap.Raise
  osg/gogen/abap/raise.go:93
Z_RUNTIME_DYNA_A429568D5BE89E.GET.func1.2
  z_runtime_dyna_a429568d5be89e.clas.abap:130
abap.Cast
  osg/gogen/abap/registry.go:56
Z_RUNTIME_DYNA_A429568D5BE89E.GET.func1
  z_runtime_dyna_a429568d5be89e.clas.abap:128
Z_SRC_ARTIFACT_F4D5693EF2F5FE_Z_MEMBER_NEWOB_2FC56538152049
  z_src_artifact_f4d5693ef2f5fe.clas.abap:521
Registry.findOrCreate -> Registry._addFiles -> Registry.addFile
test/abap/_utils.ts:parse -> run callback -> test runner
```

The raising ABAP is:

```abap
IF tag = tag_ref.
  TRY.
      adapter ?= oval.
    CATCH cx_sy_move_cast_error.
      RAISE EXCEPTION TYPE z_view_refusal_6fcecbe45fbff9.
  ENDTRY.
  result = adapter->get( p0 ).
ENDIF.
```

`src/artifacts_objects.ts:19` starts `buildObjectMap()`. Its reviewed collector
HIR in `tsfront/overrides/artifacts_objects.go` builds a native
`OrderedMap<string, ClassValue>`, boxes it into `objectMap`, and then populates
it through the original typed alias. The escaping read is
`this.objectMap[type]` at source line 13, after that population.
The emitted map is `Z_RUNTIME_MAP__A5FFC3D163A57B`. O3 boxes it as `tag_ref`
with `oval` pointing to that same map, but the map lacks `view.storage`.
The underlying `CX_SY_MOVE_CAST_ERROR` from `?=` is caught and replaced by the
named view refusal above. Namespace enumeration and constructor-map population
have already succeeded. The trap occurs before Chai's assertions.

The storage/as_ref Config probe exercises parsed graph objects and activated
class-field views. It does not exercise property reads on a boxed native
constructor map. Changing `as_ref` again would not repair this adapter cast.

The independent single case has this stack:

```text
FailCombinator.runInternal
  z_src_abap_2_s_732a01c9a5c05c.clas.abap:69
FailCombinator.run -> combinator sequence/alternatives -> Run.run
  z_src_abap_2_s_2c7826798d9e5a.clas.abap:314
statement parser -> test/abap/_utils.ts:getStatements -> negative-version callback
```

`FailCombinator.runInternal()` throws `new FailCombinatorError()` at
`src/abap/2_statements/combi.ts:833`. `Run.run()` catches it at line 1079 and
should return `undefined`. O3 emits a catch for
`Z_EXCEPTION_CL_30EEC8E8805D60` (the Error payload wrapper), while the thrown
subclass uses the unrelated `Z_EXCEPTION_CL_E4A48BC113B232` wrapper. The catch
body never runs. This is independent of both the constructor map and JSON5.

## Repair and regression checks

Native string-key maps implement storage **get/put** using their original
entries. Reads box the stored value with its actual tag, including `tag_class`
for constructor descriptors; missing keys remain undefined. Writes unbox
through the checked value ABI and update the same native map. Typed and erased
aliases, descriptor identity, and constructor argument evaluation are retained.
Map adapters retain refusals for structural class-view certification,
property-presence checks and deletion; unsupported write representations remain
explicitly refused. No parsed-record cast, prototype guard, Chai adapter, JSON
adapter, frontend override, Go emitter or OSG kernel is changed.

Class-payload exception wrappers now follow the payload's class ancestry and
share its root payload slot. A base Error catch receives derived Error
wrappers; a narrower catch uses a checked cast from that root slot. The source
object is preserved through catch and rethrow. Native runtime refusals remain
outside these typed catches.

`TestBoxedConstructorRecordABAPHost` compares Node, generated Go and one
SUCCESS row of gogen-compiled ABAP. It covers a native constructor record
boxed into two erased aliases, reads after typed mutation, dynamic
construction with arguments, erased writes visible through the typed alias,
missing keys, inherited constructor membership and identity.
`TestErrorSubclassCatchABAPHost` does the same for a two-level Error subclass,
base catches, rethrow, payload identity and inherited instanceof checks. Its
pre-fix native host result was an uncaught derived wrapper. Both use
`storageABAPProbe`, which calls the collecting `emitABAPNamed` helper. That
helper now inherits GOFLAGS rather than replacing the requested build flags.

Final focused ABI/storage/class-view/exception/settled regressions and
`go vet ./hir/abap ./tsfront` pass. The final
`./.github/ci/lint.sh gate origin/main` passes with **0 new issues** and
**canary 5/5**, using golangci-lint 2.13.2. Its temporary
`internal/lintcanary/` fixture was removed; the verdict is saved in
`.cache/o3/diagnosis/lint-final.log`. The first broader check included Node's
module warning in two existing oracle comparisons; rerunning with O3's
`--disable-warning=MODULE_TYPELESS_PACKAGE_JSON` setting passes.

## Full re-measurement

The repaired complete shard passes **4,962/4,962**, with **0 FAIL, 0 traps,
0 async-pending, 0 not-run**, all 4,962 registrations, exact original Node title
multiset parity, and no initialization exceptions. All 433 original passes
remain passing; all 4,529 original trapped titles now pass.

| Measurement | Pass | FAIL | Trap | Async-pending | Not-run | Registered |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Exact O3 baseline | 433 | 0 | 4,529 | 0 | 0 | 4,962 |
| Both ABI repairs | 4,962 | 0 | 0 | 0 | 0 | 4,962 |

The repaired trace contains the same 4,528 internally handled Registry Errors
and one **now handled** FailCombinatorError. It contains no view refusals and
no runner rethrow of the FailCombinatorError. Thus the negative Cloud case also
executes its original assertion successfully; it was not skipped or converted
to a pass by the harness. No non-ABI handoff remains for this shard.

Native host timings (milliseconds): frontend **239,165**, emission **18,269**,
Go build **234,233**, execution **282,691**; host total **774,358**. These exclude
abapiti lowering/emission and the Node oracle setup. The host exits 0. Its
2,619 ABAP files retain a 255-byte maximum line, zero overlong lines, and a
29-byte maximum object name.

The final static gap inventory still has zero blocking diagnostics, HIR errors,
coverage traps, nonfinite literals and occurring unsupported runtime operations.
The one unproved AbstractToken JSON serialization body in `lsp/help.ts` remains
refused and is not reached by these statements tests. Native frontend diagnostics
remain zero; the two unreached decimal-rendering stubs are unchanged.

## Reproduction and local diagnostic evidence

All builds are serial. GOFLAGS quotes the complete linker flag token, because
quoting only its value inside GOFLAGS is rejected by Go:

```sh
export GOFLAGS='-buildvcs=false -p=1 -gcflags=all=-dwarf=false "-ldflags=-s -w"'
export NODE_OPTIONS='--max-old-space-size=32768 --disable-warning=MODULE_TYPELESS_PACKAGE_JSON'
go test -p 1 -parallel 1 -count=1 \
  -run '^(TestBoxedConstructorRecordABAPHost|TestErrorSubclassCatchABAPHost)$' \
  -v ./tsfront
```

Set `ABAPITI_OSG_HOME` to a clean checkout at the pin above for host execution.
A focused closure can select original files through the existing
`materializeRegistryClosure` `roots` option and be passed to
`abapiti abaplint-tests ... --target osg`; no harness branch merge is needed.
A focused original append Node oracle passes 49/49. An early cached-compiler
attempt predating O3's final regex normalization hit a PCRE infrastructure
refusal; it is excluded from every root-cause count and certificate here.
All counted executions use compilers freshly built from this checkout.

The complete baseline raw report, native log, Node report and generated ABAP
are under `.cache/o3/diagnosis/`, alongside the final regression logs and
exported host fixtures. The saved final report is
`statements-fixed-results.json`, with native trace
`statements-fixed-native.log`. Comparing the saved baseline and final reports
confirms exact title multiset equality and that every final title passes; the
resume review did not rerun the measurement. `native-debug.patch` retains the temporary instrumentation
for replay; it patches only the output copy of the OSG runtime. The temporary
native diagnostic patch is reverted
before committing. The pinned upstream and OSG checkouts remain clean.
