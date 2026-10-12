# Train O1: TS-HA@OSGO certification remains blocked

## Numbers and verdict

**518/8,594 Node-active tests pass; FAIL 0; 7,991 traps; 85 active titles
unregistered. All five selected shards compile and execute. Full certification
is still BLOCKED by rules registration parity, not by the repaired constructor
factory. No measured classview pass gain is credited.**

| OSGO harness shard | Pass / Node active | FAIL | Traps | Active not-run (unregistered) | Node pending outside denominator |
| --- | ---: | ---: | ---: | ---: | ---: |
| `abap/lexer` | 85 / 200 | 0 | 115 | 0 | 0 |
| `abap/statements` | 433 / 4,962 | 0 | 4,529 | 0 | 0 |
| `objects` | 0 / 144 | 0 | 144 | 0 | 1 |
| `rules` | 0 / 3,149 | 0 | 3,064 | 85 | 10 |
| `abap/structures` | 0 / 139 | 0 | 139 | 0 | 1 |
| **TOTAL** | **518 / 8,594** | **0** | **7,991** | **85** | **12** |

Fresh original Node passes **8,594/8,594 active tests**, with 12 pending.
OSGO registers **8,521/8,606** titles including those pending titles; all 12
pending titles are skipped, not credited as passes. Total not-run is therefore
**97 = 85 active unregistered + 12 Node-pending skipped**. There are **0**
unmatched titles, assertion failures, failed initializers, async-pending cases,
or host infrastructure failures. The unchanged all-shard orchestrator exits
**0**, but its summary explicitly reports **registrationParity=false**. That
exit is not a complete-suite certificate. Each individual host exits **1**
because of traps. No wrong answer was observed; absent registrations are not
relabeled as traps or passes.

| Shard | OSG frontend s | Go emission s | Go build s | Run s | Host phase total s |
| --- | ---: | ---: | ---: | ---: | ---: |
| `abap/lexer` | 112.993 | 9.733 | 105.448 | 0.043 | 228.217 |
| `abap/statements` | 130.676 | 11.212 | 152.037 | 4.733 | 298.658 |
| `objects` | 109.799 | 10.227 | 113.679 | 0.814 | 234.519 |
| `rules` | 184.569 | 14.830 | 313.955 | 2.897 | 516.251 |
| `abap/structures` | 103.639 | 9.798 | 106.952 | 0.097 | 220.486 |
| **TOTAL** | | | | | **1,498.131** |

These are measured host phases, excluding TypeScript/HIR lowering and ABAP
emission. Shards ran sequentially; lexer overlapped local compiler gates.
Every shard has **0 blocking lowering diagnostics, 0 HIR errors, 0 overlong
ABAP lines**, maximum line length **255 bytes**, and maximum object name
**29 bytes**. All generated Go builds succeed, including the large rules closure.

Pre-classview at the **same OSG pin**: lexer **85/200, FAIL 0, 115 traps,
266.517 s**; statements **433/4,962, FAIL 0, 4,529 traps, 348.681 s**
([prior evidence](osgo-suite-gap.md)). Pass/trap counts are **unchanged**.
The new times are not a controlled speed comparison. No pre-classview OSGO
objects/rules/structures measurements are available in the assigned evidence.

Other requested results: **4,077/4,077 CI OSG unit tests pass, FAIL 0**;
zabapgit clean (**0 issues**) and seeded (**5 issues**) match fresh Node on
OSGO. All requested compiler gates pass on the repaired head. Default output
against `4fe2486` is **identical: 6,315 files, 0 changed paths**.

Final compact evidence: [train-o1-resume.json](evidence/train-o1-resume.json).
The [initial blocked-run evidence](evidence/train-o1.json) remains immutable
historical evidence; its null runtime counts are superseded by this retry.

## Scope and repaired factory blocker

Validated compiler head: **`3ab143ca63fcab0081c20c6e078bedde5beb2e3c`**,
branch `integ/o1`; original merge head `bef1975`; pre-repair report `4fe2486`.
Baseline `origin/main`: **`a15e063be697ba4709d68c93377c7ca9fa6bcd01`**;
classview tip `9016fba`. OSG remains clean at
**`f557e12f4217addcec7834b0607234aecf9b723a`**; OSG-JS transpiler/runtime:
**`ee5c2cd1a45aaac6be503a46fc903ce95a946877`**. Original abaplint pin:
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

The initial run stopped on an inherited classview factory compile defect:
`READ TABLE p1->items ... INTO d` read an erased `REF TO object` table entry
into a narrower Dynamic reference. The standalone `TestClassArgsFactoryABAP`
fixture reproduced the same OSG-generated Go error as all five shards. That
source was then byte-identical to `origin/osgo-classview`, not merge fallout.

The coordinator repaired it in **`3ab143c`**: read into a raw `REF TO object`
temporary, then use **`?=`** to retain the checked, identity-preserving cast.
The unchanged standalone test was exported and compiled using the pinned OSG
frontend/emitter and copied Go runtime, with an empty main:

- **20 ABAP files**, 0 overlong lines; maximum line **239 bytes**, object name
  **29 bytes**.
- Frontend **2.835 s**, Go emission **0.065 s**, build **1.278 s**, empty-main
  execution **0.005 s**, **exit 0**.
- This is a **compilation regression check**, not an argument-factory semantic
  execution certificate. All five subsequent real harness shards also compile.

No additional compiler, runtime, test, harness, OSG source, or exemption changes
were made during the resume.

## Remaining blocker: rules initialization and registration

Rules records **nine trapped initialization actions**, leaving **85** original
Node-active titles unregistered. These are separate from the **3,064** trapped
registered test bodies. Exact actions/messages are retained in the final JSON.

| Upstream file under `test/rules/` | Initialization action suffix (after `z_member_test__`) | Native message | Active titles missing |
| --- | --- | --- | ---: |
| `check_comments.ts` | `5641f70838341e` | `CX_SY_MOVE_CAST_ERROR in ?=` | 16 |
| `fully_type_constants.ts` | `5641f70838341e` | `CX_SY_MOVE_CAST_ERROR in ?=` | 16 |
| `keyword_case.ts` | `3607df2fd950eb`, `84533fc8724cf1` | `CX_SY_MOVE_CAST_ERROR in ?=` | 10 |
| `line_length.ts` | `ccf00c576bdfc6` | `Casting failed, types not compatible` | 2 |
| `method_length.ts` | `56cb87476a9398` | `CX_SY_MOVE_CAST_ERROR in ?=` | 16 |
| `newline_between_methods.ts` | `bba63f7978dcab` | `CX_SY_MOVE_CAST_ERROR in ?=` | 15 |
| `no_dynamic_stuff.ts` | `0a497c851020f7`, `5641f70838341e` | `An exception was raised.` | 10 |

An **emission-only diagnostic replay of the unchanged 192-root rules closure**
confirms the initialization actions call `testRule`,
`testRuleWithVariableConfig`, or `testRulesWithFile` before registration.
The shared helpers construct the descriptor-selected rule and checked-cast it
to `IRule`; config dispatch also contains checked casts. The boxing probes
against `view.storage` catch their own cast failures. The native report has no
throw-site backtrace, so it does **not** identify which downstream cast escaped.
Do not attribute these traps to the coordinator's repaired `newArgs` read, or
claim they are proven merge fallout, without a focused host trace.

A specific boundary visible in source: `line_length.ts:17` passes
`new LineLengthConf().length = 400` as the config argument—**a Number**, not a
native config reference. Emission boxes that Number before `setConfig`; the
native-reference conversion refuses it. That is an unsupported input domain,
not evidence of an assertion mismatch. Remaining initialization attribution
and any classview repair belong to the implementation owner; no casts were
removed, structural objects invented, or registrations synthesized here.

Of the **7,991 registered-body traps**, **7,990** are explicit lowering
refusals (primarily Chai boxing/equality, plus rules' `array.has` and JSON
operations). The remaining **one** is the previously recorded statements
native exception in `LOOP AT SCREEN statement version should fail, "LOOP AT
SCREEN." should not be recognized`: `An exception was raised.` It is still
unattributed, not a pass or a newly observed classview regression.

### Why these shards

Original 17-shard manifests were materialized read-only, without changing roots
or mocha/chai. **16/17 closures** contain `src/artifacts_objects.ts` (the two
`newArgs` sites), `src/ddl/ddl_parser.ts` (root-object `classOf`), and
`src/lsp/help.ts` (the other two root-object sites); only `abap/nodes` does not.
The selected lexer/statements/objects/rules/structures closures all contain
those shared sources. `objects` additionally owns checked class-view sites in
`test/objects/class.ts`; `abap/types` owns the documented examples in
`test/abap/types/types.ts`. `abap/structures` uniquely contains the fresh
lexical-descriptor site in `test/abap/structures/_combi.ts`, so it was included
in addition to requested objects/rules coverage. Membership is not operation
reachability or a passing-test count. Full membership is retained in the
initial JSON. Objects and structures compile, but **all their active tests
trap**; this is not a runtime certificate for the four admitted classview groups.

## Merge fallout fixed and gates

The M0 synthetic gap test asserted **26** unsupported catalogue entries.
Classview intentionally admits `object.checkedClassView` and
`classvalue.newArgs`, making the merged catalogue **24**. The prior merge-only
correction in `tsfront/abaplint_tests_osg_test.go` retains the exact size check
at 24 and explicitly asserts both admitted names are absent. The synthetic
unsupported occurrences, nonfinite literal, and emitter-refusal assertions
remain unchanged. No oracle or semantic assertion was weakened.

| Gate | Initial exit / wall s | Repaired-head exit |
| --- | ---: | ---: |
| `go vet ./...` | 0 / 0.65 | 0 |
| `go test -short -p 2 ./...` | 0 / 238.65 | 0 |
| `ABAPITI_GRACECHECK=1 go test -count=1 -run TestLexerFactsReport ./tsfront/` | 0 / 6.97 | 0 |
| `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend` (uncached) | 0 / 20.68 | 0 |
| `./.github/ci/lint.sh gate origin/main` | 0 / 3.27 | 0 |

Lint: **0 new**, canary **5/5**, golangci-lint **2.13.2**. The resume initially
lacked the linter on PATH; rerunning with `/home/alice/go/bin` passes. Lexer
facts match the unchanged golden and `origin/main`, SHA-256
`c173befedf56bd12e57aa6af1ce51e3354f16d569f5d7cef60fed72af9923940`.
**No golden diff or regeneration.**

## CI OSG checks and zabapgit parity

The unchanged `.github/ci/osgo-unit.sh` and `hir-unit.sh` orchestration is wired
in **`osd-m1.yml`**, not `ci.yml`. Initial checks reused the pinned checkout and
installed dependencies. These results are retained, **not rerun after the
factory fix**, as directed: fresh default output against `4fe2486` is identical
and contains **no `METHOD new_args.`** in any default ABAP file.

- OSG unit: **4,077/4,077 SUCCESS**, FAIL/error/not-compiled/skipped **0**;
  **120.65 s** wall.
- HIR main fixtures: OSGO **490/491 SUCCESS**, FAIL **0**, **1** unchanged
  negative-lookbehind `NOT_COMPILED` exemption in `TestEmitRegExpExec`;
  OSG-JS **491/491 SUCCESS**, FAIL **0**. That refusal is not a pass.
- ABAP syntax and verifier rejection checks pass. Both hosts: lexer baseline
  **44/44 cases, 4,663 tokens**; all **17** critic probe sets and Number Fix 2
  pass. Each host rejects all **3** intentional early-return, skipped-case and
  wrong-token-count mutations with expected FAIL.
- `hir-unit.sh`, both hosts and probes: **exit 0, 1,503.12 s**.

Fresh original Node matches the existing `stoker/prof-kit`'s exact bytes:
clean **0 issues, 20.013 s**, seeded **5 issues, 17.573 s**. Both embedded OSGO
registry units are **SUCCESS** with unchanged SHA assertions:

| Input | OSG frontend s | Go emit s | Go build s | Unit run s | Unit wall s |
| --- | ---: | ---: | ---: | ---: | ---: |
| clean | 90.546 | 14.161 | 65.882 | 107.248 | 286.14 |
| seeded | 121.203 | 17.627 | 79.497 | 114.544 | 342.41 |

Separate successful report invocations printed:

```text
REGISTRY ok=X want_issues=0 load_us=90949759 run_us=18119462 sha256=5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9
REGISTRY ok=X want_issues=0 load_us=92375951 run_us=18666969 sha256=7b9efbf9bf52830aa9868b079f473473f7c4f03d90d3339aed6e94b0081595b7
```

The helper repeats the compiled driver after verifying the successful unit;
walls **111.58/113.60 s**, not timings from the first invocation. `want_issues=0`
is legacy CLI metadata, **not** a seeded issue-count assertion. Seeded acceptance
is Node's five-issue dump hash. These hashes exclude the terminal newline;
exact-byte hashes including it and full live report lines are in the initial JSON.

## Default-output diff: attribution, not unexplained churn

Initial default output was generated at the **same output path** using
`origin/main`, **`ad3a671`** (pin bump + main, before classview), and the merged
train. Main/pin have **6,312 files**; train has **6,315**. There are **5,943
changed or added paths**, **3 additions**, **0 removals**. Only the a4h zip
overlaps the two change sets. Fresh output from `3ab143c` against `4fe2486` is
byte-identical at all **6,315 paths**, so this attribution remains valid:

| File group | Pin hashed keys | Classview | Union |
| --- | ---: | ---: | ---: |
| `classes/*.abap` | 50 | 1,930 | 1,980 |
| `osg/*.abap` | 50 | 1,930 | 1,980 |
| `native/lib/*.abap` | 50 | 1,930 | 1,980 |
| `names.json` | 0 | 1 | 1 |
| `a4h/README.md` | 0 | 1 | 1 |
| `a4h/abaplint-577f875e-a4h.zip` | 1 | 1 | 1 |
| **TOTAL** | **151** | **5,793** | **5,943** |

- **Pin:** 48 scalar-key Map and 2 Set identities in each of three copies.
  Every textual diff is the hashed `by_k` declaration, named-key reads, or
  scalar Set `DELETE TABLE ... WITH TABLE KEY by_k COMPONENTS k`. Reference-key
  collections are unchanged. Exactly **50 zip members** change.
- **Classview:** native-constructor methods/interface dispatch on **1,927**
  emitted program classes (including builtin Error), the `runtime.described`
  interface, `runtime.classvalue` lexical flag, and new `class.refusal` class:
  **1,930 ABAP paths per copy**. The three additions are refusal-class copies.
  `names.json` adds its identity and `builtin.nativeConstructor`; README object
  count **2,105 → 2,106**. Exactly **1,931 zip members** change, including
  refusal-class metadata.
- No other group differs. The intermediate executable directly separates the
  causes; changed-path inventory hashes remain in the initial JSON. The final
  default-output content-inventory hash is in the resume JSON.

## Reproduction and cleanup

Substantive invocations used `nice -n 10`, Go **1.26.0**, Node **26.9.0**,
`GOFLAGS=-buildvcs=false`, `GOMAXPROCS=4`, supplied SSD Go/npm caches,
`TMPDIR=/mnt/ram/abapiti/tmp`, supplied transpiler, and
`NODE_OPTIONS=--max-old-space-size=32768`. The unchanged harness was run with:

```sh
ABAPITI_OSG_HOME=/mnt/ssd-510/osg/abapiti/open-steamgate \
ABAPITI_TESTS_TARGET=osg \
ABAPITI_TESTS_SHARDS=abap/lexer,abap/statements,objects,rules,abap/structures \
  nice -n 10 node tools/abaplint-tests-all.mjs /home/alice/dev/abaplint RESULT.json
```

The initial standalone reproduction and final compilation probe use the same
read-only host bridge with an **empty main** instead of the test-runner entry;
neither is a semantic run. The rules emission-only probe executes no tests and
changes no harness or oracle. Generated closures, archives, binaries, fixtures,
default trees, raw reports and owned RAM temporary files were removed after
extracting evidence; OSG `tools/gogen/.out/go-cache` was deleted. OSG remains
clean at the supplied pin. The resume commits only documentation/evidence;
coordinator's factory fix is retained unchanged. **No push.**
