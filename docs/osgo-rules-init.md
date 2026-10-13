# Rules initialization on TS-HA@OSGO

**73 Node-active registrations recovered; missing 85 → 12; initializer traps
9 → 3. Focused registrations 183/268 → 256/268. FAIL 0 on both runs;
credited passes remain 0.** All 256 registered bodies still trap. The full
3,149-active rules shard was **not run**; its final registration count must be
re-measured on the larger machine. [Compact evidence](evidence/osgo-rules-init.json)
retains counts, exact missing/recovered title multisets, native frames, Node
boundary answers, gates and default-Go hashes.

## Per initializer, same focused closure and host pin

Actions below have the prefix `z_member_test__`. Before is fresh `origin/main`
(`3da6606`), not an extrapolation from O1. After is `45a9790`, including the
inherited `9889515` fix and the clean main merge. Both use OSG `368baab28`.

| File under `test/rules/` | Action suffix | Before trap | After | Missing before → after | Recovered |
| --- | --- | --- | --- | ---: | ---: |
| `check_comments.ts` | `5641f70838341e` | `CX_SY_MOVE_CAST_ERROR in ?=` | registered | 16 → 0 | 16 |
| `fully_type_constants.ts` | `5641f70838341e` | `CX_SY_MOVE_CAST_ERROR in ?=` | registered | 16 → 0 | 16 |
| `keyword_case.ts` | `3607df2fd950eb` | `CX_SY_MOVE_CAST_ERROR in ?=` | registered | 5 → 0 | 5 |
| `keyword_case.ts` | `84533fc8724cf1` | `CX_SY_MOVE_CAST_ERROR in ?=` | registered | 5 → 0 | 5 |
| `line_length.ts` | `ccf00c576bdfc6` | `Casting failed, types not compatible` | `An exception was raised.` | 2 → 2 | 0 |
| `method_length.ts` | `56cb87476a9398` | `CX_SY_MOVE_CAST_ERROR in ?=` | registered | 16 → 0 | 16 |
| `newline_between_methods.ts` | `bba63f7978dcab` | `CX_SY_MOVE_CAST_ERROR in ?=` | registered | 15 → 0 | 15 |
| `no_dynamic_stuff.ts` | `0a497c851020f7` | `An exception was raised.` | same trap | 5 → 5 | 0 |
| `no_dynamic_stuff.ts` | `5641f70838341e` | `An exception was raised.` | same trap | 5 → 5 | 0 |
| **TOTAL** | | **9 trapped actions** | **3 trapped actions** | **85 → 12** | **73** |

"Registered" is registration recovery, **not a passing test body**. The
runner records initializer exceptions, not successful initializer records.
Title-multiset reconciliation independently verifies recovery, including the
repeated titles in `line_length.ts`; none were synthesized. Fresh original
Node passes **268/268**, with **0 pending**, for these seven files. Independent
per-file Node runs agree and have no cross-file title collisions.

| File | Node active | Registered before | Registered after |
| --- | ---: | ---: | ---: |
| `check_comments.ts` | 16 | 0 | 16 |
| `fully_type_constants.ts` | 16 | 0 | 16 |
| `keyword_case.ts` | 114 | 104 | 114 |
| `line_length.ts` | 4 | 2 | 2 |
| `method_length.ts` | 25 | 9 | 25 |
| `newline_between_methods.ts` | 15 | 0 | 15 |
| `no_dynamic_stuff.ts` | 78 | 68 | 68 |
| **TOTAL** | **268** | **183** | **256** |

The unchanged test bodies now report **188 `array.has` owning-body refusals
and 68 native runtime traps**. This is not a claim that those bodies executed
`array.has`, nor an attribution of all 68 native body traps. Initializer traps
and unregistered titles are separate from those 256 body traps. There are no
FAILs, unmatched titles, async-pending results, skipped titles or infrastructure
failures in either focused measurement.

## Scope, inherited repair and new harness support

`origin/main` merged cleanly in `101707c`. No upstream tests, shim, Go emitter,
OSG source, exemption, golden or assertion was changed.

The previous executor's `9889515` is retained and measured, not newly authored
here. It repairs two ABAP emission paths:

- Four erased-array initializer actions lose **63** registrations: both
  variable-config helpers, the method-length file helper, and newline tests.
  The helpers receive `tests: any`; reference-array storage holds original row
  objects, not pre-existing Dynamic tags. `hir/abap/emit.go` now boxes erased
  reads at `ForEach`, `IndexGet` and `array.get`, as Go's
  `hir/golang/live_class_view.go:arrayReference` does. Already tagged rows and
  class descriptors retain their respective representation; nominal typed reads
  retain checked casts. This is not array copying or removal of sparse guards.
- Two keyword-case actions lose **10** registrations: setters receive config
  records produced by object spread. `hir/abap/forwarders.go` now honors the
  frontend bridge's `dynamic.classView` argument, instead of substituting a
  nominal `as_ref`/checked cast. It delegates to the existing activated-layout
  view helper; it does not manufacture a config or invoke its constructor.

These storage/view changes remain dell-owned for further ABI review. This
continuation makes **no additional storage, class-view or numeric admission**.
The remaining owned roots are handed off below rather than patched here.

`ABAPITI_TESTS_FILES` was added to `tools/abaplint-tests-all.mjs`. It selects
whole original test files, validates unknown/duplicate/empty entries, and
marks the manifest partial with explicit exclusions. Shard/file composition
and partition tests are in `tools/abaplint-tests-all-test.mjs`. Dependency
materialization, the real Node oracle, borrowed-test ownership, initialization
order, title reconciliation and STOP-on-FAIL remain unchanged. The two runs
have only these seven test roots plus the shim and their required imports
(**1,548** closure files); no full rules closure was compiled.

`tools/abaplint-rules-init-oracle.mjs` is an additional regression driver using
freshly compiled, pinned original upstream constructors/setters on Node. It
asserts erased-row identity/mutation, object-spread config identity/mutation,
Number-valued assignment/config reads, and exact native boolean-key iteration.
Its checks are **not upstream suite pass credits**.

## Focused native throw-site evidence

An observational replay instruments `abap.Cast` failure and `abap.Raise` in
**copied** OSG Go-runtime files under the invocation output, never in OSG or
the upstream/compiler sources. `debug.Stack` is logged only while an original
module initialization action is on the stack. The unchanged emitted ABAP,
runner and assertions execute normally; the diagnostic JSON is value-identical
to the original report. Diagnostic executions earn no additional pass credits.

The before replay identifies all nine escaping roots:

- `check_comments`, `fully_type_constants` and `newline_between_methods`:
  row-to-Dynamic casts in `z_closure_2993_2986cc0d288930.clas.abap:71`.
- `method_length`: row-to-Dynamic cast in
  `z_closure_3146_748d47efa23833.clas.abap:57`.
- Both keyword actions: structural config-row to `KeywordCaseConf` casts in
  `z_src_rules_ke_2fc2aefaa44841.clas.abap:2535`.
- `line_length`: `CX_SY_MOVE_CAST_ERROR` raised in
  `z_runtime_dyna_a429568d5be89e.clas.abap:110` (`as_ref`), called by its slot
  forwarder at `z_src_rules_li_cf25ac0a655766.clas.abap:605`.
- Both `no_dynamic_stuff` actions: `view.refusal` raised by their shared config
  helper at `z_test_rules_n_ffd1487d6cc4ac.clas.abap:53` (`dynamic.keys`).

Four **caught** array-to-`view.storage` boxing probes are also logged before;
these are not extra initialization traps and are not blamed for the missing
registrations. This distinguishes the exact escaping casts from the probes
that the O1 report could not attribute. Neither the repaired constructor
factory nor `IRule` construction is the escaping root in these nine actions.
The after registration measurement is uninstrumented. Its new Number-refusal
branch below is identified from emitted source and the original Number input,
not claimed as a second traced-run certificate.

## Precise handoff to dell: 12 still unregistered

### 1. Primitive-backed class view: `line_length.ts:17` (2 titles)

Original expression: `testRule(tests, LineLength, new LineLengthConf().length = 400)`.
JavaScript assignment returns **the Number 400**, not the allocated config.
The original setter stores that Number unchanged; `getConfig()` returns 400.
Reading `length`, `maxIssuesPerFile` or `severity` on it returns **undefined**.
`10 > config.length` is false. Do **not** rewrite this input as a config object,
install constructor defaults, or pretend that its `length` property is 400.

Current generated chain:
`z_test_rules_l_16f58cfe9bfe0e=>z_member_test__ccf00c576bdfc6` boxes Number 400 →
`testRule` → `IRule.setConfig` →
`z_src_rules_li_cf25ac0a655766->z_member_setco_573ca8d60a84a3` → `dynamic.classView`.
The emitted slot accepts native identity or `tag_object`/structural-reference
storage; Number selects its explicit `z_view_refusal_6fcecbe45fbff9` raise
(generated ABAP line 615). Before the inherited forwarder repair, conversion
failed in `runtime.dynamic.as_ref`; the changed native message is a changed
refusal boundary, **not an assertion regression or recovery**.

**Ownership/action:** align `hir/abap/live_class_view.go:liveView` and the
field reader/dynamic getter with Go's certified **`PrimitiveReads`** class-view
boundary (`hir/golang/live_class_view.go:liveView`, `viewValue`). Go permits
primitive backing only for the activated data-only layout/ancestry carrying
that flag, retains the primitive backing value, and preserves missing-property
presence. Primitive writes must retain Go's checked/refusing behavior. Do not
broaden all nominal casts. Missing Number reads/comparisons must retain exact
undefined/NaN semantics or keep an explicit numeric refusal; ABAP `f` zero is
not a substitute. Recovery acceptance is these **two real registrations**,
with no fabricated object and FAIL 0; bodies may still hit later honest traps.

### 2. Native own-key enumeration: `no_dynamic_stuff.ts:108–117` (5 + 5 titles)

Both failed actions evaluate `conf("callMethod")` or `conf("internalTable")`
before calling `testRule`. The helper constructs `NoDynamicStuffConf`, calls
`Object.keys(config)`, turns only boolean values false and sets the requested
key true. Current emitted helper
`z_test_rules_n_ffd1487d6cc4ac=>z_member_conf_34f8c1bde4f9d5` boxes the compatible
native config, then **unconditionally raises** `z_view_refusal_6fcecbe45fbff9`
at generated ABAP line 53 for `dynamic.keys`. It never reaches the boolean
loop or registration. This is not a test-closure or constructor-factory fault.

**Ownership/action:** implement `dynamic.keys` on the same certified own-key
storage boundary as Go (`hir/golang/own_keys.go:dynamicKeys`, `ownKeyClass`),
including its native constructor/declaration-order proof and per-instance
structural metadata, with a presence-aware returned array. Native config
keys, in original insertion order, are `exclude`, `severity`, `callMethod`,
`callFunction`, `callDatabaseProcedure`, `callTransformation`, `callTransaction`,
`perform`, `submit`, `createObject`, `createData`, `getBadi`, `assign`,
`internalTable`, `exportImport`. The first two superclass-created fields are
own enumerable keys too; they are not boolean and must remain untouched. The
13 boolean fields must be mutable on the **original native reference** through
computed-key reads/writes. After enumeration is implemented, check the next
boundary too: nominal `view.storage` get/put methods currently refuse, so merely
returning the names cannot certify the helper's erased computed-key writes.
One named flag remains true; no defaults or snapshot
may be silently substituted. Do not enumerate ABAP implementation slots,
prototype methods or absent fields; preserve mutation and presence on the
record/view path as well. Keep unsupported prototype/key domains refusing.
Recovery acceptance: both original five-title suites register, Node's ordered
keys and nonboolean values match, FAIL 0. Subsequent runtime/body traps remain
separate work.

## Reproduction and gates

From this worktree, with caller-supplied upstream/host/cache paths and the
required environment (`GOTOOLCHAIN=go1.26.0`, `GOFLAGS=-buildvcs=false`, supplied
Go/npm caches, TMPDIR and transpiler), select exactly:

```sh
export ABAPITI_TESTS_TARGET=osg ABAPITI_TESTS_WORKERS=1
export ABAPITI_TESTS_FILES=test/rules/check_comments.ts,test/rules/fully_type_constants.ts,test/rules/keyword_case.ts,test/rules/line_length.ts,test/rules/method_length.ts,test/rules/newline_between_methods.ts,test/rules/no_dynamic_stuff.ts
# Check load < 12 and /proc/pressure/io some avg10 < 30 after acquiring lock.
flock /mnt/ram/abapiti/osg-heavy.lock \
  systemd-run --user --scope -p MemoryMax=48G nice -n 10 \
  node tools/abaplint-tests-all.mjs "$ABAPLINT_HOME" focused.json
node tools/abaplint-rules-init-oracle.mjs "$ABAPLINT_HOME"
```

Before/after host phase totals: **283.612 / 228.045 seconds** (frontend,
Go emission, Go build, execution). Shared machine load makes this **not a
controlled performance comparison**. Both emit **2,333 ABAP files**, maximum
line **255 bytes**, **0 overlong**, maximum object name **29 bytes**; lowering
has **0 blocking diagnostics / 0 HIR errors**. Native binaries exit 1 because
of honest traps; the reconciled orchestrator exits 0 while explicitly reporting
registration parity false. Neither exit is a full-suite certificate.

Local gates pass: `go vet ./...`; `go test -short -p 2 ./...`;
`ABAPITI_GRACECHECK=1 go test -count=1 -run TestLexerFactsReport ./tsfront/`;
`./.github/ci/lint.sh gate origin/main` (**0 new**, canary **5/5**, golangci-lint
2.13.2); harness-selection tests; fresh Node boundary oracles. No golden
regeneration. The final focused lexer measurement is **200/200, FAIL 0,
traps 0, unregistered 0**; host phases **286.837 seconds**, no envelope violation.
The inherited erased-row fixture also executes on OSGO: **1/1 ABAP Unit
SUCCESS**, testing loop/get/index boxing and preservation of an already tagged
string (`abcbcx`). This is a separate synthetic regression, outside the 268
upstream titles. Default `abaplint --target go` generated from `origin/main` and
the changed compiler at the **same output path** is byte-identical: **9 files,
0 changed**, executable included.

All completed OSG invocations were sequential under the shared lock, with
`MemoryMax=48G`, `nice -n 10`, `GOMAXPROCS=4` and a 32-GiB Node heap ceiling.
The resource guard was checked after acquiring the lock; attempts at excessive
load/I/O refused before invoking OSG. A redundant after-trace request was
cancelled while still waiting for another job's lock; it produced no run or
counts. Generated sources, archives, binaries, copied runtimes and owned
scratch/temporary files were removed after extracting evidence. OSG remains
clean at `368baab28111a072c3f5779a0e07d259305f836f`, with
`tools/gogen/.out/go-cache` absent. No clone or push.
