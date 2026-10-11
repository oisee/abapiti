# Integration train 1

Branch: `integ/train-1`, based on `origin/main` **7d6e5d903468a9b7e0c2b48e4781bd57b1ec22ac**.
Measured code revision: **e122226**. All work was performed in
`/home/alice/dev/abapiti/.local/worktrees/train`. No push, rebase, or transpiler
clone was performed. The broader ABAP runtime CI attempt has an environmental
blocker; it is **not** reported as passing.

## Merges

After `git fetch origin`, each branch was merged with `git merge --no-ff`.
The dependency-aware order in the task's detailed ordering was used: forEach
before config, rather than the earlier bullet list's placement at the end.

| Order | PR / branch | Source SHA | Merge commit |
| ---: | --- | --- | --- |
| 1 | #107 `feat/abaplint-tests-harness` | `43bb2ba78b53d876765ac7b2a7c15619904282a1` | `8546788` |
| 2 | #93 `feat/string-repeat-match` | `6b24038f12f1f4456f166abbd94323f40dd9e109` | `f364364` |
| 3 | #103 `feat/regexp-exec` | `450014b08ad6bac05fccde4f5b4450806b7c772b` | `142393e` |
| 4 | `feat/fit-tests-foreach` | `681913c3913a628a17c233c533f9ef106c35501a` | `c09e519` |
| 5 | #108 `feat/fit-config-json` | `fe22375062771deadd9f3d353b8913d7bec9c23e` | `aa2484e` |
| 6 | #104 `feat/fit-rules-2` | `5e4137b5538bf84558c58f51a88cc44e44b9dc75` | `bee655c` |
| 7 | #106 `feat/fit-abap-debt` | `7b0bcdbc0090e48c4d6e34d08746dcbe0bfda4f7` | `e77df7f` |

### Conflict resolution

- **Harness:** merged without conflicts. Retained initialization recovery,
  readiness guards, typed registration rows, strict assertions, source-attributed
  traps, and nil-coverage test lowering.
- **Repeat/match:** reconciled fixture linting and suite registration in
  `.github/ci/hir-lint.mjs` and `hir-unit.sh`; retained DDIC dependencies and
  exact `osgo-known-gaps.txt` handling. Kept match signatures in `hir/runtime.go`.
  Automatic merging also duplicated repeat catalogue/effect/switch entries and
  the emitted Go method; consolidated these to one implementation. The frontend
  retains the harness's omitted-count behavior and #93's numeric coercion.
  Updated the old Go-only-repeat rejection test to assert the newly supported
  ABAP behavior. Initial failed compilation was corrected and the merge amended
  before proceeding to #103.
- **RegExp exec (older #93 stack):** retained current repeat/match, trim,
  generator, Promise, and locale-case behavior, adding exec/lastIndex operations,
  reviewed shared patterns, routing, and effects. PCRE is confined to the gated
  JS-regex subclass. Retained slash escaping, inline-free DDIC RESULTS types
  (`match_result_tab`, `match_result`, `submatch_result_tab`, `submatch_result`),
  FIRST into a structure followed by APPEND, and the groups copy before looping.
  Removed the older stack's transformations back to incompatible RESULTS table
  operands / inline declarations. Adjusted exec/replacement typing and tests to
  that contract. Added all three RegExp operations to the ABAP catalogue.
- **forEach (older harness stack):** kept current local-function naming,
  body-trap boundaries, generator/Promise state, and repeat support. Added
  presence-sensitive forEach/every, typed catches, boxed field adapters,
  registration fixes, branching callback returns, and CRLF lowering. Preserved
  both generator ABI and erased-call narrowing guards. Historical harness docs
  take the newer forEach branch version; this document supersedes their numbers
  for the integrated train.
- **Config JSON:** combined the usage-gated JSON writer with sparse-array,
  typed-catch, and Promise emission; retained both effect/catalogue additions.
- **Rules 2:** combined usage-gated XML validation and property lookup with the
  existing runtime features; retained config verification alongside property
  verification and the expanded source-pin tests / gap summarizer. Fixed a
  cross-branch interaction: generic boxed field reads must not preempt the
  rule-config lookup's actual-derived-class dispatch. Indexed `__proto__`
  assignment remains explicitly refused. Both existing regressions pass.
- **ABAP debt (older rules stack):** combined operation catalogues, retaining
  repeat/match/RegExp and adding property/XML support plus loud backend checking.
  CI contains the union of six suites; feature lint uses `--fixture`. Both exact
  known-gap handling and the narrowly scoped pinned PCRE-lookbehind allowance
  are retained. An EXIT trap removes generated HIR builds even on failure.
- **Integration follow-up `e122226`:** sparse-runtime selection preserves regex
  `Missing` metadata separately from holes: unmatched captures are present
  undefined values, not deleted array slots. Added a Node/Go differential that
  combines RegExp exec/capture absence with sparse forEach. Existing sparse-array
  tests and the new integration regression pass.

Every emitted operation is in the backend catalogue. ABAP explicitly refuses
`array.has`, `array.delete`, and `json.stringifyConfig`; these features were
Go-only on their source branches. The guard checks this exact reviewed unsupported
set rather than falsely claiming implementations. Default-workload coverage and
its frozen operation allowlist also pass.

`tsfront/testdata/syntaxfeatures/probe.ts` retains main's generator probes and
repeat/match probes. Regenerated oracles with:

```sh
node tools/syntax-features-oracle.mjs
node tools/registry-features-oracle.mjs
node tools/string-repeat-match-oracle.mjs
node tools/regexp-exec-oracle.mjs
```

The final regeneration produces no working-tree differences. Lexer facts were
exported with an **absolute** `GRACE_FACTS_OUT`, inspected, and updated only for
runtime-operation catalogue counts. Against main: aliases/allocates/may-raise/
writes 96 → 109, reads 141 → 164, conservative 9 → 10; all lexer method and
call-site facts remain unchanged. Final export matches the golden exactly.

## Gates

Environment throughout Go tests:
`GOCACHE=$HOME/.cache/go-build GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false
ABAPITI_GRACECHECK=1`. Final runs used `GOMAXPROCS=4`; generated harness builds
also used `-p=4` to bound resource use.

After **each** merge, `go build ./...` and
`go test ./tsfront/... ./hir/... ./cmd/... -short` passed after conflict fixes.
Final code was then rebuilt and remeasured:

| Gate | Result |
| --- | --- |
| `go build ./...` | PASS |
| `go test ./... -short` | PASS |
| `ABAPITI_GRACECHECK=1 go test ./tsfront/... ./hir/... ./cmd/...` | PASS |
| `go vet ./...` | PASS |
| Node harness report/summary contract | PASS |
| Node harness runtime contract | PASS, 6 scenarios |
| HIR lint regression | PASS |
| Final lexer facts export / comparison | PASS |

### Generated and linted train suites

Each suite was freshly generated and checked using
`node .github/ci/hir-lint.mjs <dir> /mnt/ssd-510/osg/abapiti/open-steamgate --fixture`.
Lint selects v755 only for suites containing PCRE statements; other suites use
v750. The following is the complete union of the merged CI suites:

| Suite | ABAP files | Version | Issues |
| --- | ---: | --- | ---: |
| `TestFixtures` | 43 | v750 | 0 |
| `Test750Semantics` | 7 | v750 | 0 |
| `TestEmitSyntaxFeatures` | 66 | v750 | 0 |
| `TestEmitStringRepeatMatch` | 6 | v750 | 0 |
| `TestEmitRegExpExec` | 9 | v755 | 0 |
| `TestRuntimeDebtFixtures` | 40 | v750 | 0 |

Supplemental existing feature suites also linted with **0 issues**:
RegistryArrays (26 files), RegistryFeatures (22), RegistrySorts (10),
RegistrySingletons (16), RegistryIterators (14), ArrayViews (4), and
MemoryFileDifferential (11, with `MEMORY_FILE_EMIT=1`). The pre-existing
input-dependent RegistryJSON/XML export tests skip without their external input
variables; they are not members of the merged six-suite CI train.

One full `.github/ci/hir-unit.sh` attempt used the already installed pinned
transpiler/runtime **ee5c2cd1a45aaac6be503a46fc903ce95a946877**, with `TRANSPILER`
pointing at the existing build (no clone). Generation, six-suite linting, and
transpiler verification passed. **Runtime execution was blocked**: osgo and
OSG-JS cannot `mkdtemp` under the supplied read-only OSG checkout:
`EROFS ... open-steamgate/.local/osgo-unit-XXXXXX` / `osgjs-unit-XXXXXX`.
The runners consequently report zero tests and a harness ERROR; lexer runtime
CI is blocked for the same reason. These are not successes or known semantic
gaps. No writable external checkout was created or modified.

### Default output

Fresh default OSG generation exited 0. Scanned **4,222 ABAP files**:
**no FIND/REPLACE … PCRE statements and no `create_pcre`**.

Built the main reference executable from `git archive origin/main` inside this
worktree, then removed the extracted sources. Main and train each ran
`abaplint -o .local/train-default --target a4h --quiet`, with identical output
spelling. Both exited 0 and produced **2,114 files**. All differences:

| File | Reason |
| --- | --- |
| `classes/z_closure_1063_d771406cea51bd.clas.abap` removed; `classes/z_closure_1063_659efc9f101dad.clas.abap` added | Expected #93 closure serial shift, `106339` → `106334`, at `src/abap/5_syntax/statements/perform.ts:79:25`. Byte-identical after normalizing only the serial and emitted name. |
| `classes/z_runtime_desc_61ac7bd03e15db.clas.abap` | Descriptor registration serial/name for that closure only. |
| `classes/z_src_abap_5_s_7035f4eed88c9b.clas.abap` | Closure reference declaration and constructor name only. |
| `names.json` | That closure's ID/name and member source-origin references only. |
| `a4h/abaplint-577f875e-a4h.zip` | Packages the same closure rename, its `.clas.xml` rename, and the two reference changes. Decoded member hashes show no other changes. |

### Full closure

Fresh reference and train `abapiti abaplint --full` runs both exited 0 (measurement
mode, **not** successful emission of the full closure):

| Measurement | Main | Train | Delta |
| --- | ---: | ---: | ---: |
| Blocking | 347 | 35 | -312 |
| Root blocking | 136 | 29 | -107 |
| Cascade blocking | 211 | 6 | -205 |
| HIR verification errors | 284 | 173 | -111 |

## Headline: original abaplint tests on generated Go

Fresh original-Mocha Node oracles and generated-Go executions for **all four
shards**, all selected files sorted, pinned upstream
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. Final measurement was repeated after
the integration regression fix; the N table is unchanged. No smoke subset is
counted twice; these 8,461 tests are not the whole upstream test suite.

| Shard | Files | Passed | Failed | Trapped | Not-run | Registered | Node total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Rules | 192 | 10 | 0 | 1,669 | 1,480 | 3,147 | 3,159 |
| Lexer | 3 | 200 | 0 | 0 | 0 | 200 | 200 |
| Statements | 286 | 434 | 20 | 4,508 | 0 | 4,962 | 4,962 |
| Structures | 30 | 24 | 0 | 114 | 2 | 139 | 140 |
| **Unique total** | **511** | **668** | **20** | **6,291** | **1,482** | **8,448** | **8,461** |

Node: rules 3,149 passed / 10 pending; lexer 200 passed; statements 4,962 passed;
structures 139 passed / 1 pending. **Zero Node failures, zero unmatched Go
multiset titles, zero async initialization failures.** Go not-run comprises
1,458 async/done callbacks, 11 skipped tests, and 13 unregistered tests.
Registration parity holds for lexer and statements, not rules and structures.
The 20 ordinary failures remain explicit Config conversion-domain refusals in
statement tests, not counted as lowering traps or passes.

Remaining lost registrations: `keyword_case.ts:289` and `:304` (5 each),
`line_length.ts:17` (2), and structures `_combi.ts:276` (1, block-local class).

### Top 10 reached traps

Only **three distinct reached trap groups** exist; ranks 4–10 are empty, not
invented from compile-time diagnostics. Counts exclude initialization failures.

| Rank | Reached tests | Trap / source | Shards |
| ---: | ---: | --- | --- |
| 1 | 6,258 | `find_global_definitions.ts:24:3` body trap: HIR call arity mismatch at `src/abap/5_syntax/global_definitions/find_global_definitions.ts:39:9` (`o.parseType(this.reg)`) | rules 1,636; statements 4,508; structures 114 |
| 2 | 18 | `test/rules/_utils.ts:13:21`: `identifier JSON is not a lowered local or module value` | rules 18 |
| 3 | 15 | `src/artifacts_objects.ts:8:3`: pinned artifacts-object trap | rules 15 |

The Config round-trip and forEach barriers no longer dominate reached traps;
execution reaches the existing global-definition HIR failure instead. The
remaining generic `JSON.stringify` helper is outside the pinned Config writer.
No trap, unsupported assertion, or lost registration was bypassed to inflate N.

## Evidence and cleanup

Compact logs and JSON summaries are in the requested scratchpad `train/` directory
under `<scratch>`.
Generated Go/ABAP builds, copied test closures, original oracle builds, runtime
outputs, baseline source extraction, and executables were deleted after runs.
Only compact evidence and this committed report remain. No push was performed.
