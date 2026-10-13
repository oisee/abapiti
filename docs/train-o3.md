# Train O3 and O4

## O3+O4 resumed assembly (2026-10-13)

The resumed checkout already contained the three `--no-ff` merges below and
helper repair `0082223`. They were preserved; no completed merge was repeated.
After fetching, `origin/main` remained at `08dd0e2` (#136). PR #137 was not
included in that main ref, so its separate merge is required and retained.
The measured compiler is `9f05b84`. Its commit message was subsequently amended
to group both requested trailers, producing `a859619` with the identical
source tree. The documentation commit changes no executable code. Upstream
and OSG pins remain unchanged.

| Input | Input commit | Merge |
| --- | --- | --- |
| `origin/main` (#136 collecting-helper guard and fixture gate) | `08dd0e2` | `10f31eb` |
| `origin/osgo-splitargs` (bounded constructor factories) | `371ca32` | `5c2220c` |
| `origin/perf/abap-template-literals` (#137 control-character escapes) | `ebd8e34` | `d84d1f3` |

The factory merge retains storage's `p1->load( )` refresh before dispatching
constructor arguments, while using the bounded routing tree and wide-copy
helpers. There is one factory implementation. The template-literal merge is
clean. The train's proved operation-map union and all tests are retained.
`0082223` changes the split-factory determinism check from direct `EmitNamed`
to `emitABAPNamed`, preserving its assertions. The template-literal regression
already uses its collecting helper. The collecting-helper guard passes.

The exact CI fixture wrapper initially exposed a pre-existing invalid legacy
`ts/TestTranspileLexer` emission once the TypeScript toolchain was available.
Commit `a859619` (measured as `9f05b84`) repairs named table return types, parameter and field bindings,
constructor argument names, distinct locals for sibling TS blocks, and escaped
control characters in string templates. The test and collector remain intact;
no fixture is excluded. Its focused pinned-gogen compile passed, followed by
a successful rerun of the complete CI wrapper.

### Final gates

| Gate | Result |
| --- | --- |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test -short -p 4 -count=1 ./...` | PASS; `tsfront` 261.067 s |
| `TestABAPEmitterTestsUseCollectingHelpers`, uncached | PASS; also executed in the short and full collection gates |
| Grace `TestLexerFactsReport`, `ABAPITI_GRACECHECK=1`, uncached | PASS; golden unchanged, no regeneration |
| `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend` | PASS; explicit run and final short/full gates |
| `.github/ci/lint.sh gate origin/main` | PASS; 0 new issues, canary 5/5, golangci-lint 2.13.2 |
| Exact `.github/ci/abap-fixture-compile.sh` CI wrapper | PASS; 509 collected emissions / 403 unique programs on pinned gogen, empty Go build log |
| Default zabapgit Go output vs `origin/main` | PASS; entire output directory `diff -r` exit 0, 0 bytes, 9 files including executable and `sites.json` |

An initial concurrent lint run exposed its intentionally invalid temporary
canary to vet. Vet and the uncached short gate were rerun after canary cleanup;
the final gate results above are from the clean repaired tree. Lint is sequenced
before those reruns. The default output comparison generates both versions at
the same destination and includes the built executable. No main shard is run.

### TS-HA@OSGO: lexer and added shards

All five shards use `9f05b84`, the unchanged pinned host, one worker, a fresh
original Node oracle per invocation, Go 1.26.0, `GOMAXPROCS=4`, `GOGC=50`, and
`GOMEMLIMIT=32GiB`. They run separately in the order shown. Go builds use
`-gcflags=all=-dwarf=false` and `-ldflags='-s -w'`. No assertion, registration,
ownership reconciliation, status classifier or host pin is changed.

| Shard | Node-active | Credited pass | FAIL | Trap | Unregistered / no-report | Registered |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap/lexer` | 200 | 200 | 0 | 0 | 0 | 200 |
| `abap` | 216 | 0 | unmeasured | unmeasured | 216 (no report) | unmeasured |
| `abap/expressions` | 107 | 98 | 0 | 9 | 0 | 107 |
| `pretty_printer` | 31 | 0 | 0 | 31 | 0 | 31 |
| `utils` | 15 | 7 | 0 | 8 | 0 | 15 |

**Lexer remains 200/200, FAIL 0.** The other three completed shards register
all 153 active titles: 105 passes, 48 traps, FAIL 0, unregistered 0. Their traps
all report `An exception was raised.`; their exception classes are not
attributed by this runner. They remain traps. These completed measurements,
including lexer, total 305 passes and 48 traps across 353 active titles, with
zero FAILs, unmatched titles, initialization records or async-pending cases.

**The expected aggregate unregistered 288 → 0 is not certified.** The `abap`
shard compiles, then aborts without a trustworthy runner report:

```text
NOT_COMPILED in FIND PCRE: a text with line ends: PCRE's ^ $ and . around them are not measured
```

This is the pinned native kernel's explicit multiline-PCRE domain refusal
(`tools/gogen/go/abap/find.go:335`), escaping ABAP `CATCH`. The bridge exits 2;
the accounting command exits 1. All 216 Node-active cases remain not-run/no-report,
and another four Node-pending titles remain outside the active denominator.
The harness's empty-record zero FAIL/trap counters are not an execution
certificate. Actual registrations, assertion outcomes and traps are unmeasured;
no results are invented and no host refusal is converted into a pass.
The static closure has zero unsupported runtime-operation sites, including
zero `array.has` refusals. Storage has removed that earlier catalogue blocker,
but the multiline-PCRE infrastructure blocker prevents this shard's final
registration measurement. Follow-up must address that host/runtime boundary
and rerun `abap`; this train does not claim the addition's aggregate target met.

| Shard | Frontend (s) | Go emission (s) | Go build (s) | Native run/abort (s) |
| --- | ---: | ---: | ---: | ---: |
| `abap/lexer` | 146.161 | 15.023 | 170.392 | 0.065 |
| `abap` | 152.709 | 14.354 | 179.826 | 0.158 |
| `abap/expressions` | 137.453 | 12.035 | 146.276 | 6.377 |
| `pretty_printer` | 150.264 | 12.619 | 210.679 | 1.715 |
| `utils` | 142.354 | 12.059 | 157.309 | 0.495 |

These host phases exclude abapiti lowering/ABAP emission and Node setup.
All five emitted closures have maximum line length 255 bytes, zero overlong
lines, and maximum object-name length 29 bytes. Each has zero blocking
lowering diagnostics, HIR errors, lowering coverage traps, unsupported runtime
sites and nonfinite literals. Their one static emitter refusal remains the
unproved `AbstractToken` own-property order/toJSON in `lsp/help.ts`; it is
separate from the native multiline-PCRE abort and measured runtime traps.

Compact path-free evidence is retained in
[evidence/train-o3-o4.json](evidence/train-o3-o4.json).
Local logs, raw summaries and prepared checkouts are under `.tmp/o3/`.
The local environment file records the actual pinned setup and compiler.
With that environment loaded, reproduce serially:

```sh
. .tmp/o3/env.sh
ABAPITI_TESTS_SHARDS=abap/lexer node tools/abaplint-tests-all.mjs .tmp/o3/abaplint .tmp/o3/lexer.json
ABAPITI_TESTS_SHARDS=abap node tools/abaplint-tests-all.mjs .tmp/o3/abaplint .tmp/o3/abap.json
ABAPITI_TESTS_SHARDS=abap/expressions node tools/abaplint-tests-all.mjs .tmp/o3/abaplint .tmp/o3/expressions.json
ABAPITI_TESTS_SHARDS=pretty_printer node tools/abaplint-tests-all.mjs .tmp/o3/abaplint .tmp/o3/pretty_printer.json
ABAPITI_TESTS_SHARDS=utils node tools/abaplint-tests-all.mjs .tmp/o3/abaplint .tmp/o3/utils.json
```

The `abap` invocation is expected to return the documented infrastructure
failure on this pin; the other accounting commands exit 0. Git identity is
configured as `Alice V.` / `ooisee@gmail.com`. Commits retain the requested
co-author and session trailers. No push is performed.

## Historical O3 assembly

Base: `origin/main` at `3da6606` (O2). Local integration branch: `integ/o3`.
Upstream abaplint: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.
OSG remains pinned to `368baab28111a072c3f5779a0e07d259305f836f`.

## Merges and conflict decisions

All feature merges use `--no-ff`, in this order:

| Feature | Input | Merge |
| --- | --- | --- |
| Numeric: guarded Number parseInt, ceil, safe i64 boxing | `bf2669b` | `5a81cda` |
| Storage: sparse presence, live arrays, record deletion, native calls, LSP guards | `0a61c21` | `4c09f43` |
| JSON: stringifySubset and indent | `42b828f` | `dfce410` |
| String/XML: padStart, parseFloatNumber, SHA1, splitRegex, parseWithErrors | `ad7c56a` | `4243e7c` |
| Parsed nested Config view regression | `3e61046` | `ec87fb7` |

The existing PR #136 fixture compile gate was integrated by `bcc40e8`;
`604f77c` adds the sparse parsed-array Chai observation refusal.

O2 and storage share one nested-class implementation in `live_class_view.go`.
Storage's live array adapter refreshes typed read buffers from the original graph
and publishes admitted writes to that graph. Nested activation, unknown keys,
replacement and old aliases retain O2 behavior. Chai and JSON5 remain enabled.
Chai refuses sparse arrays at its observation boundary, including parsed arrays.

The JSON merge retains storage's native writes, deletion-mask clearing, own-key
presence tracking and subclass refusal. JSON key-order tracking is initialized
alongside storage presence metadata and updated by direct structural writes and
native adapter writes. The existing JSON writer gains the subset/indent paths;
there is one implementation of each operation. HIR CI fixture lists are unions,
including numeric and JSON cases. Existing refusal assertions were updated to
require emission for newly implemented operations; their Node/Go oracles remain.

Operation maps are the union of implemented features. The synthetic catalogue
regression now requires **one** unsupported operation: `string.parseFloat`.
Value-domain, sparse-array and unproved-layout refusals still apply.
The asref regression merged cleanly onto storage; it adds no duplicate runtime.

`9f012c8` routes merged SHA1/split emission checks through the train's collecting
helpers. `2b54ebe` does the same for JSON/string HIR tests and retains their ABAP
Unit files in the compile fixture directory. These repairs preserve every test
and assertion. The first integration check found missing helper references;
the first collector audit found direct emission calls, both corrected locally.
`5ce3f44` updates concat/JSON emission expectations and keeps the synthetic
refusal-ownership test on unsupported `string.parseFloat`.

## Gates

| Gate | Final result |
| --- | --- |
| `go vet ./...` | PASS |
| `go test -short -p 4 -count=1 ./...` | PASS; `tsfront` 191.203 s |
| Grace `TestLexerFactsReport`, `ABAPITI_GRACECHECK=1`, uncached | PASS; golden unchanged |
| `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`, uncached | PASS; executed |
| Lint gate against `origin/main` | PASS; 0 new issues, canary 5/5, golangci-lint 2.13.2 |
| Legacy `TestOSD_EmitUnitClasses` collector | PASS |
| Emitted ABAP fixtures compiled through pinned gogen | PASS; 502 emissions, 396 unique programs; Go build log empty |
| Default zabapgit Go output vs `origin/main` | PASS; raw `diff -r` exit 0, 0 bytes, 9 files including executable |

The initial integration checks exposed stale unsupported-operation expectations
and missing compile collectors. After resolving those, the complete short gate
passed on `5ce3f44` (`tsfront` 306.891 s), and the compile gate passed with 502 emissions / 396
unique programs. Its first statements attempt then exposed the host
initialization refusal described below. Required gates passed on the final
repair commit `a8e56e8`; final logs and artifacts are under
`.cache/o3/validated/`. Earlier integration diagnostics are in
`.cache/o3/final/` and `.cache/o3/final-tree/`. The fixture gate consumes the
fresh short-test exports plus legacy exports, using the unchanged compile driver; it avoids repeating the
whole test suite inside the shell wrapper.

The default Go comparison reuses the saved main-generated output and generates
O3 at the same destination. It does not run main's test shards.

Caches reuse `.cache/go-build`, `.cache/go-mod`, `.cache/abaplint`, and the pinned
OSG checkout. Go uses `go1.26.0`, `GOMAXPROCS=4`, and `GOFLAGS=-buildvcs=false`.
Node uses a 32 GiB heap and suppresses only
`MODULE_TYPELESS_PACKAGE_JSON`, whose diagnostics otherwise pollute combined
oracle output with repository-scoped temporary TypeScript files.

The **full static gap census is skipped** for this final train. Earlier,
pre-JSON integration censuses do not describe this tree and are not reported
as final results.

## TS-HA@OSGO measurement

Only O3 is measured, with `abap/statements` first and then `abap/lexer`.
No main shard run is performed; main comparison figures belong to the separate
full baseline run. Default Go output comparison is a byte-diff gate, not a main
suite measurement.

The first statements attempt on `5ce3f44` compiled but refused during host
initialization: `NOT_COMPILED in FIND PCRE: ... \x23`. All 4,962 Node-active
cases were unregistered/not-run; FAIL was 0. This infrastructure result is
not a successful measurement, and lexer did not run in that attempt.

Commit `a8e56e8` repairs the existing PCRE normalizer: absolute assertions use
`\A` / `\z`, literal spaces/hash use character classes, and reviewed hexadecimal
code points become quoted literal units. This preserves extended-mode literal
semantics and uses syntax accepted by the unchanged pinned OSG kernel. Dynamic
pattern validation and the emitted anchor converter use the same absolute
anchors. The reviewed DEFINE word boundary remains ASCII `\b`.
Focused regex regressions pass. A pinned-host preflight checked all 138 reviewed
patterns and 3,174 match/capture comparisons against the previous normalized
patterns in Node, with zero mismatches or host refusals. The same 3,174
comparisons also pass through the pinned native OSG Go kernel. The temporary
preflight artifacts are in `.cache/o3/final-tree/pcre-preflight.*`.

Measurements use the cached compiler at `a8e56e8`, one shard worker,
`GOGC=50`, and `GOMEMLIMIT=32GiB`. Each shard uses a fresh original Node oracle.
The unchanged harness preserves borrowed-test FAIL checks and reconciles owned
titles; no assertion, registration or status classifier was changed.

| Shard | Node-active | Pass | FAIL | Trap | Async-pending | Not-run | Registered |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap/statements` | 4,962 | 433 | 0 | 4,529 | 0 | 0 | 4,962 |
| `abap/lexer` | 200 | 200 | 0 | 0 | 0 | 0 | 200 |

Statements returned no initialization failures, no unmatched titles, and no
Node-pending cases. All 4,529 traps report `An exception was raised.`; they are
runtime exceptions and remain traps, not passes or assertion FAILs. The host
executable's exit 1 reflects these traps; the shard accounting command completed
successfully with zero genuine FAILs.

Statements host timings: frontend 161.236 s, emission 16.000 s, Go build
257.075 s, native execution 310.743 s; host total 745.054 s. These exclude the
abapiti lowering/emission stage and Node oracle setup. The 2,619 emitted ABAP
files satisfy the envelope: maximum line 255 bytes, zero overlong lines, maximum
object name 29 bytes.

The per-shard statements closure has zero blocking diagnostics, HIR errors,
coverage traps and nonfinite literals. Its gap inventory has no occurring
unsupported runtime operations and one static subset-JSON refusal in
`lsp/help.ts` for unproved `AbstractToken` own-property order/toJSON. No measured
statements test reaches that refused body; this inventory is separate from the
4,529 runtime exceptions and is not a full static census.

Lexer also has no initialization failures, unmatched titles or Node-pending
cases. Its host timings are frontend 178.608 s, emission 15.705 s, Go build
233.919 s, native execution 0.070 s; host total 428.302 s. Its 2,361 emitted ABAP
files satisfy the same 255-byte line / 29-byte object-name envelope, with zero
overlong lines. The lexer closure also has zero blocking diagnostics, HIR errors,
coverage traps and nonfinite literals, and the same unreached static
`AbstractToken` JSON refusal.

Both accounting commands exited 0. The combined selected scope is **633 passes,
0 FAIL, 4,529 traps, 0 async-pending and 0 not-run**, across 5,162 Node-active
cases. This certifies only these two shards. The 4,529 traps remain limitations;
zero FAIL does not imply all tests pass.

Compact reproducible counts, gates, timings, envelopes and per-shard inventories
are retained in [evidence/train-o3.json](evidence/train-o3.json). Raw local summary
reports are `.cache/o3/validated/statements.json` and
`.cache/o3/validated/lexer.json`.

Reproduce the selected measurements after the gates with `. .cache/o3/env.sh`,
`NODE_OPTIONS="$NODE_OPTIONS --disable-warning=MODULE_TYPELESS_PACKAGE_JSON"`,
`ABAPITI_TESTS_TARGET=osg`, `ABAPITI_TESTS_WORKERS=1`, `GOGC=50`, and
`GOMEMLIMIT=32GiB`. Set `ABAPITI_TESTS_COMPILER` to the validated cached compiler
and `ABAPITI_TESTS_COMPILER_COMMIT=a8e56e8`, then run separately in this order:

```sh
ABAPITI_TESTS_SHARDS=abap/statements node tools/abaplint-tests-all.mjs \
  .cache/abaplint .cache/o3/validated/statements.json &&
ABAPITI_TESTS_SHARDS=abap/lexer node tools/abaplint-tests-all.mjs \
  .cache/abaplint .cache/o3/validated/lexer.json
```

All commits retain the existing `Co-Authored-By` and `Claude-Session` trailers.
No push is performed.
