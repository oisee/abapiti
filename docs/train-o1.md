# Train O1: blocked TS-HA@OSGO certification

## Numbers and verdict

**4,077/4,077 CI OSG unit tests pass, FAIL 0. Zabapgit clean (0 issues) and
seeded (5 issues) match fresh Node on OSGO. All requested compiler gates pass
after one merge-only test correction. Certification is nevertheless BLOCKED:**
all five selected unpruned harness shards fail Go compilation before execution.
There is **no new harness FAIL-0 certificate** and no credited classview gain.

| OSGO harness shard | Credited passes / Node active | Runtime FAIL | Runtime traps | Active not-run | Pending outside denominator |
| --- | ---: | --- | --- | ---: | ---: |
| `abap/lexer` | 0 / 200 | unmeasured | unmeasured | 200 | 0 |
| `abap/statements` | 0 / 4,962 | unmeasured | unmeasured | 4,962 | 0 |
| `objects` | 0 / 144 | unmeasured | unmeasured | 144 | 1 |
| `rules` | 0 / 3,149 | unmeasured | unmeasured | 3,149 | 10 |
| `abap/structures` | 0 / 139 | unmeasured | unmeasured | 139 | 1 |
| **TOTAL** | **0 / 8,594** | **unmeasured** | **unmeasured** | **8,594** | **12** |

Original Node passes **8,594/8,594 active tests**, with 12 pending. OSGO returns
no runner JSON and registers no titles. The reporter's empty-result FAIL/trap
zeros are **not measurements**; every active title is not-run, not trapped.
The reconciled four-shard retry exits **1** for infrastructure failures; its
host compiler exits **2**. No genuine runtime wrong answer was observed.

| Shard | OSG frontend s | Go emission s | Failed Go build s | Run s | Host phase total s |
| --- | ---: | ---: | ---: | ---: | ---: |
| `abap/lexer` | 144.544 | 13.078 | 3.145 | 0 | 160.767 |
| `abap/statements` | 207.438 | 15.722 | 3.908 | 0 | 227.068 |
| `objects` | 114.987 | 10.845 | 3.048 | 0 | 128.880 |
| `rules` | 194.846 | 14.919 | 5.414 | 0 | 215.179 |
| `abap/structures` | 148.982 | 10.309 | 2.848 | 0 | 162.139 |

These are failed-build phase totals, not execution or performance comparisons.
TypeScript/HIR lowering and ABAP emission are outside them. Runs overlapped
local gates and CI checks. Each shard has **0 blocking lowering diagnostics,
0 HIR errors, 0 overlong ABAP lines**, maximum line length **255 bytes**, and
maximum object name **29 bytes**.

Pre-classview at the **same OSG pin**: lexer **85/200, FAIL 0, 115 traps,
266.517 s**; statements **433/4,962, FAIL 0, 4,529 traps, 348.681 s**
([prior evidence](osgo-suite-gap.md)). Those completed executions must not be
replaced by this train's unmeasured runtime counts. No pre-classview OSGO
objects/rules/structures measurements are available in the assigned evidence.

Compact evidence: [train-o1.json](evidence/train-o1.json).

## Scope and first blocker

Compiler merge head: **`bef1975`**, branch `integ/o1`; baseline `origin/main`:
**`a15e063be697ba4709d68c93377c7ca9fa6bcd01`**. Classview tip: `9016fba`.
OSG remains clean at **`f557e12f4217addcec7834b0607234aecf9b723a`**;
OSG-JS transpiler/runtime is **`ee5c2cd1a45aaac6be503a46fc903ce95a946877`**.
Original abaplint pin: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

The lexer first exposed the shared `classvalue.newArgs` factory compilation
failure. Remaining harness work was paused for attribution, then the requested
high-risk closures were attempted unchanged. All reproduce the same blocker:

```text
z_runtime_clas_c216c6067abdc6.clas.abap:3183:
cannot use p1.items[idx0 - 1] (variable of interface type any)
as *Z_RUNTIME_DYNA_A429568D5BE89E value in assignment: need type assertion
```

`hir/abap/class_abi.go:111` emits `READ TABLE p1->items INDEX ... INTO d`,
where `d` is a Dynamic reference. The established `arrayStorage` ABI erases
reference arrays to `STANDARD TABLE OF REF TO object`; OSG consequently stores
the entries as Go `any`. The factory does not insert a checked cast when reading
an erased entry into the narrower reference. The standalone emitted
`TestClassArgsFactoryABAP` fixture reproduces those same Go compilation errors,
without the full test closure (compile-only, with an empty Go main).

**This is an inherited classview implementation defect, not merge fallout.**
The factory source is byte-identical to `origin/osgo-classview`, SHA-256
`0c1dafe8a8404c65b7ea5882b966a5e9b8bb15ba730d02f686e449bb706dff15`.
The same reference-array erasure exists on that branch and predates classview
(`5515481a`). Its documentation explicitly had no OSGO execution validation.
The factory is activated by the unpruned shared artifacts closure, even on the
lexer shard; the pruned zabapgit workload does not activate this argument
factory. No factory, runtime, harness, OSG source or exemption was changed here.

**Follow-up outside this task's merge-only scope:** repair the erased argument
read using an explicit identity-preserving checked conversion, exercise valid,
absent and wrongly typed arguments on the host, and rerun all five shards.
Do not re-refuse `newArgs`, drop registrations, or treat a missing runner report
as FAIL 0 to make the train appear certified.

### Why these shards

Original 17-shard manifests were materialized read-only, without changing roots
or mocha/chai. **16/17 closures** contain `src/artifacts_objects.ts` (the two
`newArgs` sites), `src/ddl/ddl_parser.ts` (root-object `classOf`), and
`src/lsp/help.ts` (the other two root-object sites); only `abap/nodes` does not.
The selected lexer/statements/objects/rules/structures closures all contain
those shared sources. `objects` additionally owns the checked class-view sites
in `test/objects/class.ts`; `abap/types` owns the documented examples in
`test/abap/types/types.ts`. `abap/structures` uniquely contains the fresh
lexical-descriptor site in `test/abap/structures/_combi.ts`, so it was included
in addition to the requested objects/rules coverage. Source membership is not
an operation-reachability or passing-test count. Full membership evidence is
retained in the JSON.

## Merge fallout fixed and gates

The M0 synthetic gap test asserted **26** unsupported catalogue entries.
Classview intentionally admits `object.checkedClassView` and
`classvalue.newArgs`, making the merged catalogue **24**. Only
`tsfront/abaplint_tests_osg_test.go` changed: retain the exact catalogue-size
check at 24 and explicitly assert both newly admitted names are absent.
The two synthetic unsupported occurrences, one nonfinite literal, and the
nonfinite emitter-refusal assertions remain unchanged. No semantic assertion
or oracle was weakened; the host compilation defect remains visible.

| Final gate | Exit | Wall s |
| --- | ---: | ---: |
| `go vet ./...` | 0 | 0.65 |
| `go test -short -p 2 ./...` | 0 | 238.65 |
| `ABAPITI_GRACECHECK=1 go test -count=1 -run TestLexerFactsReport ./tsfront/` | 0 | 6.97 |
| `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend` (uncached) | 0 | 20.68 |
| `./.github/ci/lint.sh gate origin/main` | 0 | 3.27 |

Lint: **0 new**, canary **5/5**, golangci-lint **2.13.2**. Lexer facts were
exported separately and match the read-only golden and `origin/main`
byte-for-byte: SHA-256
`c173befedf56bd12e57aa6af1ce51e3354f16d569f5d7cef60fed72af9923940`.
**No golden diff or regeneration.**

## CI OSG checks and zabapgit parity

The unchanged `.github/ci/osgo-unit.sh` and `hir-unit.sh` orchestration is
currently wired in **`osd-m1.yml`**, not `ci.yml`. The existing pinned checkout
and installed dependencies were reused; no setup reinstall was needed.

- OSG unit: **4,077/4,077 SUCCESS**, FAIL/error/not-compiled/skipped **0**;
  **120.65 s** wall.
- HIR main fixtures: OSGO **490/491 SUCCESS**, FAIL **0**, **1** unchanged
  negative-lookbehind `NOT_COMPILED` exemption in `TestEmitRegExpExec`;
  OSG-JS **491/491 SUCCESS**, FAIL **0**. That one refusal is not a pass.
- ABAP syntax and verifier rejection checks pass. On **both** hosts, lexer
  baseline is **44/44 cases, 4,663 tokens**; all **17** critic probe sets and
  Number Fix 2 pass. Each host rejects all **3** intentional early-return,
  skipped-case and wrong-token-count mutations with the expected FAIL.
- `hir-unit.sh`, including both hosts and all probes: **exit 0, 1,503.12 s**.

Fresh original Node on the existing `stoker/prof-kit` matches the kit's exact
output bytes: clean **0 issues, 20.013 s**, seeded **5 issues, 17.573 s**.
Both embedded OSGO registry units are **SUCCESS** with unchanged SHA assertions:

| Input | OSG frontend s | Go emit s | Go build s | Unit run s | Unit wall s |
| --- | ---: | ---: | ---: | ---: | ---: |
| clean | 90.546 | 14.161 | 65.882 | 107.248 | 286.14 |
| seeded | 121.203 | 17.627 | 79.497 | 114.544 | 342.41 |

Separate successful report invocations printed:

```text
REGISTRY ok=X want_issues=0 load_us=90949759 run_us=18119462 sha256=5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9
REGISTRY ok=X want_issues=0 load_us=92375951 run_us=18666969 sha256=7b9efbf9bf52830aa9868b079f473473f7c4f03d90d3339aed6e94b0081595b7
```

The helper repeats the same compiled driver after verifying the successful unit
result; walls **111.58/113.60 s**, not timings from the first unit invocation.
`want_issues=0` is legacy CLI metadata, **not** a seeded issue-count assertion.
The seeded acceptance is Node's five-issue dump hash. Hashes above exclude the
CLI terminal newline; fresh exact-byte Node/kit hashes including it are retained
in the train JSON. Full live report lines with stage timings are in the
train JSON.

## Default-output diff: attribution, not unexplained churn

Fresh default `abaplint` output was generated at **the same output path** from
three executables: `origin/main`, **`ad3a671`** (pin bump + main, before
classview), and this train. Main/pin have **6,312 files**; train has **6,315**.
There are **5,943 changed or added paths**, **3 additions**, **0 removals**.
The only overlap between the two change sets is the a4h zip.

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
  interface, `runtime.classvalue` lexical
  flag, and new `class.refusal` class: **1,930 ABAP paths per copy**. The three
  additions are that refusal class's copies. `names.json` adds its identity
  and `builtin.nativeConstructor`; README object count is **2,105 → 2,106**.
  Exactly **1,931 zip members** change, including refusal-class metadata.
- No other file group differs. The intermediate executable separates the
  causes directly; changed-path inventory hashes are retained in the JSON.

## Reproduction and cleanup

All substantive invocations used `nice -n 10`, Go **1.26.0**, Node **26.9.0**,
`GOFLAGS=-buildvcs=false`, `GOMAXPROCS=4`, SSD Go/npm caches, RAM temporary
storage, the supplied transpiler, and `NODE_OPTIONS=--max-old-space-size=32768`.
The original all-shard harness was used without registration/oracle changes:

```sh
ABAPITI_TESTS_TARGET=osg ABAPITI_TESTS_SHARDS=abap/lexer,abap/statements,objects,rules \
  nice -n 10 node tools/abaplint-tests-all.mjs /home/alice/dev/abaplint RESULT.json
# Initial work paused on the blocker; completed unchanged retry:
ABAPITI_TESTS_TARGET=osg ABAPITI_TESTS_SHARDS=abap/statements,objects,rules,abap/structures \
  nice -n 10 node tools/abaplint-tests-all.mjs /home/alice/dev/abaplint RESULT.json
```

Generated closures, archives, binaries, fixtures, default-output trees, raw
reports and owned RAM temporary files were removed after extracting evidence;
OSG `tools/gogen/.out/go-cache` was deleted. The OSG checkout remains clean at
the supplied pin. Only the merge-only test correction and this report/evidence
are committed. **No push.**
