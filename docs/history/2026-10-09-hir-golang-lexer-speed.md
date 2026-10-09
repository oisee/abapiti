# Go HIR lexer speed, round 3

Branch `proto/hir-golang`, starting at `648d4d0`. No push. Every implementation step was committed as Alice V. <ooisee@gmail.com> after `go test ./hir/... ./tsfront/...` passed. Full uncached suites also passed after the int32 conversion and int64 helper changes. This includes the 44-case Node lexer differential, the emitted-token mutation check, registry oracles, the bounded catalogue, and semantic edges.

## Benchmark and commits

`tools/lexer-go-timing.mjs 100`: 30 warmups and 100 measured runs **per file**, compilation/startup and token dump excluded. Each cell below is milliseconds/run in the order **wasm_compiler / bench_mem / abapgit**. GOGC is unset for default runs. These are the readings recorded in commit messages; small timings fluctuate, so close-to-2x intermediate results are not a robust margin.

| Step | Commit | Change | Default | GOGC=400 |
|---|---|---|---|---|
| Baseline | `648d4d0` → `4db7060` | Opt-in CPU/alloc profiling, no runtime optimization | 33.752 / 35.127 / 11.740 | 27.686 / 28.202 / 8.975 |
| 1 | `0763a4e` | Ordinary returns outside try/finally closures | 6.281 / 6.456 / 2.091 | 6.303 / 6.269 / 1.999 |
| 2 | `e85c25f` | Checked machine integer arithmetic | 3.179 / 2.937 / 0.927 | 2.597 / 2.578 / 0.833 |
| 3 | `dbac90d` | Typed receiver nil checks | 2.584 / 2.889 / 0.915 | 2.449 / 2.532 / 0.858 |
| 4 | `98e18cf` | Typed Map/Set indexes plus insertion-order slices | 2.179 / 2.133 / 0.639 | 2.052 / 2.107 / 0.644 |
| 5 | `ec20ef8` | Single-build UTF-16 uppercase, unchanged ASCII reuse | 1.659 / 1.604 / 0.490 | 1.513 / 1.541 / 0.464 |
| 6 | `a092eec` | Concrete pointers for classes without subclasses | 0.928 / 0.964 / 0.296 | 1.042 / 1.004 / 0.290 |
| 7 | `0536cd3` | Direct comparisons for small primitive Sets | 0.833 / 0.890 / 0.247 | 0.842 / 0.819 / 0.237 |
| 8 | `e53c1ae` | Convert source string literals once | 0.717 / 0.699 / 0.217 | 0.733 / 0.717 / 0.199 |
| 9 | `bc26c55` | Inline widened int32 arithmetic with checked narrowing | 0.786 / 0.661 / 0.200 | 0.704 / 0.749 / 0.205 |
| 10 | `38a6726` | Bitmap membership for byte-valued Number keys | 0.694 / 0.664 / 0.186 | 0.652 / 0.608 / 0.183 |
| 11 | `5dc6f55` | Exact-round-trip checked int32 conversion | 0.694 / 0.843 / 0.204 | 0.670 / 0.591 / 0.182 |
| 12 | `f22e9bc` | Inline checked int64 addition/subtraction | 0.649 / 0.676 / 0.191 | 0.650 / 0.581 / 0.185 |
| 13 | Final commit containing this report | Geometric array growth | 0.636 / 0.659 / 0.189 | 0.598 / 0.537 / 0.157 |

Steps 10 and 11 use isolated remeasurements after additional edge tests finished. Earlier step-10 readings while another test run was active were 0.905 / 0.838 / 0.429 by default and 0.758 / 0.731 / 0.221 at 400; initial step-11 readings were 0.779 / 0.735 / 0.222 and 0.669 / 0.617 / 0.198. These are not used to claim final speed.

Final same-tool Node comparisons:

| File | Go default | Node default-run | Ratio | Go at 400 | Node 400-run | Ratio |
|---|---:|---:|---:|---:|---:|---:|
| wasm_compiler | 0.636 | 0.373 | 1.70x | 0.598 | 0.321 | 1.86x |
| bench_mem | 0.659 | 0.370 | 1.78x | 0.537 | 0.347 | 1.55x |
| abapgit | 0.189 | 0.129 | 1.46x | 0.157 | 0.125 | 1.25x |

The measured target is met in both modes. Go is about 53x / 53x / 62x faster than this session's default baseline. It is still slower than Node.

## Profiles before each change

CPU and allocation profiles were captured separately for each implementation. Profiles include warmups; reported benchmark timings exclude them. Allocation profiles use Go's standard sampling rate, not an allocation-per-object trace. CPU percentages below use **total sampled CPU**, not wall time. GC share is the union of samples matching `gcBgMarkWorker|gcAssistAlloc|gcMarkTermination|gcStart`; it includes GC synchronization. Short profiles are noisy, so 1,000-run CPU profiles supplement later steps. Timing comparisons remain the required 100 runs.

`Lexer.process`, `Lexer.add`, `LexerBuffer.add`, and constructor names below decode the existing hashed Go symbols back to their HIR identities.

| Before step | CPU findings | Allocation findings | GC CPU share |
|---|---|---|---:|
| 1 | `pcvalue` 17.2% flat; `gopanic` 57.8% cumulative; `gorecover` 39.9% cumulative | `math/big.nat.make` 60.8%, `big.NewInt` 9.9%; total ~1199 MB | 1.7% |
| 2 | `integerArithmetic` 35.4% cumulative; `mallocgc` 24.6%; `nilRef` 9.5% | `math/big` 82.5%; total ~953 MB | 7.7% |
| 3 | `nilRef` 22.3% cumulative; reflection unpacking 12.8% flat | `upper` 34.4% cumulative; total ~161 MB | 3.2% |
| 4 | Number Set `has` 24.4% cumulative; `nilRef` 12.8% | `upper` 40.2% cumulative; total ~159 MB | 7.0% |
| 5 | `Lexer.process` 9.5% flat; stream helpers and string concatenation visible | `upper` 36.1% cumulative; total ~158 MB | 3.6% |
| 6 | `Lexer.process` 12.7% flat, `LexerBuffer.add` 9.1% flat; Set `has` 20.0% cumulative | Identifier 36.7%, array growth 23.1%, Position 16.6%; total ~100 MB | 1.8% |
| 7 | `mapaccess2` 25.7% cumulative; hash work and general arithmetic each 5.7% flat | Identifier 31.9%, array growth 25.0%; total ~99 MB | 5.7% |
| 8 | `str` 24.2% cumulative; Set `has` 9.1%, integer helper 6.1% flat | Identifier 29.6%, array growth 24.7%; total ~108 MB | 3.0% |
| 9 | Longer profile: Set `has` and `charCodeAt` each 7.1%, integer helper 6.6% flat | Identifier 30.7%, array growth 25.0%; total ~96 MB | 7.6% |
| 10 | Longer profile: Set `has` 7.8%, `LexerBuffer.add` 5.1% flat | Array growth 35.5%, Identifier 28.2%; total ~94 MB | 10.6% |
| 11 | Longer profile: `LexerBuffer.add` 9.2%, `Lexer.process` 6.8% flat | Identifier 35.1%, array growth 24.1%; total ~87 MB | 8.8% |
| 12 | Longer profile: integer helper 5.9%; `floatByte` 7.5%, buffer add 4.3% flat; caller stacks identify lookahead accessors | Identifier 32.4%, array growth 24.3%; total ~87 MB | 8.6% |
| 13 | Longer profile: `Lexer.add` 7.5%, initialization guard 4.6% flat; allocation/GC now substantial | Array growth 27.1%, Identifier 25.5%, Position 23.4%; total ~98 MB | 11.5% |

Final longer CPU profile: `floatByte` 8.4%, `Lexer.process` 6.1%, `Lexer.add` 5.0%, static initialization guard 3.9%, `charCodeAt` 2.8%, `AbstractToken.constructor` 2.8%, and byte equality 2.8% flat. `futex` is 7.8%. `Lexer.process` and `Lexer.add` are 83.2% and 44.7% cumulative, including their callees. General integer arithmetic, panic/recover returns, repeated source-literal conversion, and reflection no longer dominate.

Final allocation profile: Identifier construction 30.5%, array growth 29.1%, Position construction 19.9%, uppercase result storage 7.2%, Punctuation construction 5.5%; total ~90 MB across the three files and 130 invocations/file. The baseline was ~1199 MB. Sampling explains variation between adjacent profiles.

Final GC CPU share: **11.7% default** (210/1790 ms sampled CPU), **2.3% at GOGC=400** (30/1300 ms). Raising GOGC trades more live heap for fewer collections. GC's relative share rose after removing much larger non-GC costs; total lexer time fell sharply.

## Semantics and rejected work

UTF-16 storage and substring/charAt/slice views were already present at the starting commit. Profiles did not show an allocation per `charCodeAt`. The string issues found were uppercase construction and repeated literal conversion.

One attempted `nilRef` specialization added an interface type assertion to that generic helper. It made Number Set comparisons escape into boxed heap allocations: ~502 MB in Set membership, ~647 MB total, versus ~161 MB before. It measured 4.409 / 4.829 / 1.709 by default and 3.996 / 3.991 / 1.463 at 400. That runtime assertion was discarded before committing step 3; typed checks were retained only at generated method entry points.

Hoisting literals exposed a weakness in the mutation test: replacing only the first `str("Identifier")` changed the class descriptor but missed the hoisted token label. The test now mutates all emitted occurrences and again proves that changing token types is detected. The 44 corpus outputs themselves continued to match.

Additional semantic checks cover 4,500 arithmetic boundary combinations against math/big, typed nil and reference identity in indexed collections, insertion order after deletion/reinsertion, snapshot independence, NaN/negative zero/fractional keys, bitmap word boundaries, primitive optional keys, and exact checked numeric conversions.

Retained costs: returned tokens and Position objects keep distinct identities and can escape, so they are not pooled or reused across lexes. Polymorphic classes still dispatch through interfaces; root/dynamic reference equality still handles typed nil safely. Returns crossing generated try/finally closures retain unwinding so finally execution and return override behavior are preserved. UTF-16 slice views intentionally keep their backing input alive while tokens reference it. Primitive optionals were already `{Value, Has}` structs. These are constraints on further optimizations, not a claim that every remaining cost is irreducible.

## Reproduction

In this sandbox the writable Go cache and generated-module VCS stamping need:

```sh
export GOCACHE=/tmp/abapiti-go-cache
export GOFLAGS=-buildvcs=false
go test ./hir/... ./tsfront/...
node tools/lexer-go-timing.mjs 100
GOGC=400 node tools/lexer-go-timing.mjs 100
ABAPITI_LEXER_CPU_PROFILE=/tmp/lexer.cpu node tools/lexer-go-timing.mjs 100
ABAPITI_LEXER_ALLOC_PROFILE=/tmp/lexer.alloc node tools/lexer-go-timing.mjs 100
go tool pprof -top /tmp/lexer.cpu
go tool pprof -top -alloc_space /tmp/lexer.alloc
```

For longer CPU profiles, pass 1000 instead of 100. This session's baseline profiles are `/tmp/hir-before.cpu` and `/tmp/hir-before-sampled.alloc`; each step's profiles are `/tmp/hir-stepN.cpu` and `/tmp/hir-stepN.alloc` (step 3 uses `step3b`); final longer profiles are `/tmp/hir-step13-long.cpu` and `/tmp/hir-final-400.cpu`. Generated binaries were kept under `/tmp/hir-stepN` for inspecting symbols. Profiling instrumentation is opt-in and does not change default timing.

## Round 4 (starting at 5460973)

No fuzz harness, no emitter inliner, no push. Guard for every retained step:
`GOCACHE=/tmp/abapiti-go-cache GOFLAGS=-buildvcs=false go test ./hir/... ./tsfront/... -count=1`.
This runs the existing 44-case Node differential and mutation check, registry
oracles, catalogue and semantic edges. No skips in the baseline guard.
Timings use `tools/lexer-go-timing.mjs 100`, sequentially after the guards finish;
30 warmups/file, compilation/startup excluded. Triples are wasm_compiler /
bench_mem / abapgit, milliseconds/run. GOGC unset for default.

| Step | Default Go | GOGC=400 Go | Default Node | GOGC=400 Node |
|---|---|---|---|---|
| Fresh baseline, PGO off | .580 / .599 / .160 | .583 / .562 / .173 | .319 / .311 / .111 | .330 / .303 / .128 |
| R4.1 PGO baseline | .534 / .538 / .167 | .561 / .494 / .148 | .320 / .306 / .109 | .348 / .328 / .115 |

R4.1 stores the 1,000-run benchmark CPU profile as
`hir/golang/testdata/lexer/default.pgo`. Only the concrete lexer timing build
uses it by default; arbitrary HIR programs do not inherit lexer training.
`ABAPITI_GO_LEXER_PGO=off` disables it; an absolute profile path overrides it.
`ABAPITI_GO_LEXER_SOURCE=/tmp/round4-source` preserves generated source for
compiler diagnostics. Initial PGO timings overlapped the guard and were
rejected; the table is the isolated remeasurement.

Before emitter edits, compiled the saved module using
`go build -pgo=/tmp/round4-baseline.cpu -gcflags='-m=2 -d=ssa/check_bce'`.
PGO allows currentChar/nextChar/nextNextChar/charCodeAt and Number Set.has
(costs 390/342/342/319/252) to inline. Lexer.process and Lexer.add remain
non-inlineable (costs 14200/11145 > PGO budget 2000). Concrete leaf receivers
already dispatch directly; polymorphic token receivers retain interfaces.
Remaining slice checks occur at stream charCodeAt calls (generated lines
4978/5024/5087/5143/5199/5255), substring/charAt, and runtime casing loops.
No emitter inlining pass will be added. The future general `hir.Inline(p)` is
reserved for integration after rebasing onto Alice's main.

Baseline CPU: Lexer.process 8.11% flat, Lexer.add 7.03%, floatByte 4.86%,
charCodeAt 4.86%, initialization guard 3.78%, upper 2.16% flat / 8.11% cumulative.
Baseline allocations (1,030 invocations/file): Identifier 32.61%, array.push
29.76%, Position 18.29%, other token constructor 7.26%, casing storage 3.77%.
These confirm the ranked candidates, with allocation costs still substantial.
PGO and Go compiler indexing/loop changes are Go-only. Immutable membership
recognition and lifetime/capacity proofs could become general HIR rewrites and
benefit ABAP; this round will keep any backend representation changes in Go.
The report's previous round's 'target met' referred to its <2x target, not
beating Node; round 4's goal is Go/Node <1.

This installation lacks a prebuilt pprof tool; use
`GOCACHE=/tmp/abapiti-go-cache go run /usr/lib/go/src/cmd/pprof ...`.
Baseline profiles and compiler log: `/tmp/round4-baseline.cpu`,
`/tmp/round4-baseline.alloc`, `/tmp/round4-diag.log`.
