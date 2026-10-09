The complete translated abaplint closure now runs directly through `hir/golang`
and checks the v0.1.1 zabapgit kit byte-identically with fresh Node abaplint
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. Work is on `proto/hir-golang`,
based on `e08ea9b`; nothing was pushed.

The CLI Go target uses exactly the existing source materialization, pinned
closure, overrides, reachability and assume-int default. It lowers 1,538
TypeScript files plus the harness into 1,927 classes and 73 interfaces. Every
class compiles into the Go executable. It does not call `hir.Inline`. The 1,995
bodies pruned by the recorded zabapgit workload remain explicit traps.

Green steps:

- `da804e8`: full closure compiles; reviewed regex Node oracle (117 patterns
  × 1,033 inputs) and rest/super/try-control/finally oracle pass.
- `ad474ec`: all 65 statement/regression dumps, 192 structure dumps,
  18 MemoryFile cases and 16 split cases match the Node corpora. Statement,
  structure and MemoryFile oracles were also regenerated from a fresh clean
  checkout of 577f875e and match the stored dumps.
- `9e6ac8c`: native Go check equals fresh Node and the kit outputs on clean
  (0 issues) and seeded (5 issues); adds the shared benchmark runner.
- This report records the completed timing/profile step and corrects GC
  attribution to use sampled CPU stacks for the direct Go backend.

`go test ./hir/... ./tsfront/... ./cmd/abapiti` passes. The opt-in
`ABAPITI_GO_FULL_TEST=1 go test ./cmd/abapiti -run TestGoFullClosure` also
passes. Existing lexer (44 cases), registry feature and runtime catalogue
checks remain green. The release ABAP→Go binary also matches the clean kit
output.

The stdout files include the terminal newline. Clean SHA-256 is
`9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`;
seeded SHA-256 is
`0a08fa85133e89faa08a841a01572b228428191c50844b62e0c8bb9965145e50`.
Go, freshly rebuilt Node and the expected kit files match byte for byte.

Emission gained trailing packed-array parameters, cross-method super lookup,
loop control across try closures, optional reference narrowing, void sequence
expressions, and a checked repacking of fresh unaliased optional arrays. The
lexer capacity proof now tolerates empty statement entries. The telemetry
clock returns integer milliseconds for assume-int. ClassValue descriptors and
factories were already implemented before this work.

Remaining refusals are deliberate: unreviewed regex languages/flags and dollar
replacement substitutions; escaping or aliased primitive array covariance;
try/finally whose try block exits, and try/catch/finally (frontend/HIR contract);
ClassValue factories requiring arguments or abstract classes; the documented
finite-number/divisor/remainder limits, restricted string ordering, strict JSON,
and reviewed XML/materialization shapes. Their contracts have not been proven
general JavaScript-equivalent. Reached pruned bodies still report their original
TypeScript source location. See [the backend contract](../../hir/golang/README.md).

Measurements used one runner, `tools/hir-go-check.mjs`, with sequential fresh
processes on the same Linux amd64 machine: AMD Ryzen AI 7 PRO 350, Go 1.26.0
(both Go binaries), Node 26.9.0, and TypeScript 6.0.3 for the freshly compiled
original oracle. GOGC uses its default. Compilation is excluded. Direct Go
checks include CPU profiling; Node includes the GC observer; the release
binary includes gctrace. These are single observations, not medians. The
release total is command wall time, including input loading; Go-HIR and Node
check totals time the harness parse/report. Stage columns are seconds.

| Host | Input | Check s | Lexer | Statements | Structures | Syntax | Rules | Peak MiB | GC share |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Node | clean | 11.315 | 0.272 | 1.250 | 8.358 | 1.115 | 0.145 | 1211.5 | 18.98% |
| Go-HIR | clean | 32.360 | 0.185 | 3.942 | 26.568 | 1.253 | 0.237 | 651.9 | 84.74% |
| Node | seeded | 17.436 | 0.326 | 1.418 | 13.694 | 1.590 | 0.190 | 1290.3 | 18.33% |
| Go-HIR | seeded | 48.569 | 0.280 | 5.277 | 40.953 | 1.637 | 0.205 | 660.5 | 83.57% |
| Go-ABAP | clean | 128.400 | 25.154 | 23.304 | 57.490 | 12.661 | 7.725 | 877.3 | 85.58% |

Peak memory is GNU time maximum RSS, in MiB. Go-HIR GC share is the fraction
of CPU-profile samples with GC worker/assist/collection/sweep/scavenge frames.
Node GC share is observed GC duration divided by check wall time. Go-ABAP GC
share estimates traced GC CPU time divided by GNU time process CPU time.
These definitions differ and should not be compared as identical metrics.
The raw Go runtime CPU estimator exceeded actual process CPU on the seeded
run under scheduling contention; the reported HIR GC shares use profile
samples instead. Raw estimates remain in the JSON evidence for transparency.

Top ten CPU entries for the clean full Go-HIR check (flat seconds and percent;
total sampled CPU exceeds wall time because GC uses multiple threads):

```text
File: zabaplint
Build ID: 7b664ec7064d9f726c22656498f716808238be2d
Type: cpu
Time: 2026-10-09 23:04:54 IST
Duration: 32.39s, Total samples = 174.28s (538.07%)
Showing nodes accounting for 134.53s, 77.19% of 174.28s total
Dropped 986 nodes (cum <= 0.87s)
Showing top 10 nodes out of 107
      flat  flat%   sum%        cum   cum%
    28.18s 16.17% 16.17%     60.21s 34.55%  runtime.scanObjectsSmall
    26.24s 15.06% 31.23%     58.25s 33.42%  runtime.tryDeferToSpanScan
    24.15s 13.86% 45.08%     39.67s 22.76%  runtime.scanObjectSmall
    21.40s 12.28% 57.36%     21.40s 12.28%  runtime.spanClass.sizeclass (inline)
     8.68s  4.98% 62.34%    130.60s 74.94%  runtime.scanSpan
     7.33s  4.21% 66.55%      7.33s  4.21%  runtime.extractHeapBitsSmall
     6.42s  3.68% 70.23%      7.12s  4.09%  runtime.spanSetScans
     4.46s  2.56% 72.79%      4.46s  2.56%  runtime.(*spanScanOwnership).or (inline)
     4.23s  2.43% 75.22%      4.23s  2.43%  runtime.memmove
     3.44s  1.97% 77.19%      3.48s  2.00%  runtime.(*mspan).moveInlineMarks
```

The clean allocation profile attributes 95.21% of allocated bytes to
`array[any].slice2`: 58.19 GiB out of 61.11 GiB sampled total. Copied structure
matching slices are the next performance bottleneck; this work retained their
observable mutation/identity behavior. It does not claim an optimized full
checker.

Reproduce from this clone after unpacking the release check kit:

```sh
GOCACHE=/tmp/abapiti-go-cache GOFLAGS=-buildvcs=false \
  go run ./cmd/abapiti abaplint --target go -o /tmp/abaplint-go
(cd /tmp/abaplint-go/go && GOFLAGS=-buildvcs=false go build -o ../zabaplint .)
GOCACHE=/tmp/abapiti-go-cache GOFLAGS=-buildvcs=false \
  node tools/hir-go-check.mjs /path/to/zabapgit-check-kit \
  /tmp/abaplint-go/zabaplint /tmp/hir-check-measurements \
  /path/to/zabaplint-linux-amd64 /path/to/clean-abaplint-577f875e
```

The runner writes both stdout comparisons, stage/resource JSON, CPU and
allocation profiles, and `top10.txt`. The reporting step can be repeated
without rerunning any checks:

```sh
node tools/hir-go-check.mjs --summarize \
  /tmp/hir-check-measurements /tmp/abaplint-go/zabaplint
```

[Raw timing/resource observations](2026-10-09-hir-golang-full-check.json)
are committed beside this report. The session binaries and profiles are in
`/tmp/hir-full-final` and `/tmp/hir-full/benchmark-final`.

Input provenance (SHA-256):

| Asset | SHA-256 |
|---|---|
| v0.1.1 check kit zip | `2e3311517885ed72e651c97004d6cba064fa5093873c75f1104955891e965d7a` |
| clean standalone | `aa57853bb7839dd06c946ece94c22c8da1f16cef36e546e3c430be0ce47a2124` |
| seeded standalone | `b795e93348b396a0e966e2ea08f9b47f992099288a88671ddd0b62d238a63ae9` |
| abaplint.json | `057278ffa311716f90277e10a9a0f2e1e3496591b264a6dcd60a5d1bdc930b32` |
| release zabaplint-linux-amd64 | `e9f9666e3a2b66c762b4731528425f6379fb552834897faeb2555e42cc6a6ac7` |

The kit and native binary hashes match the release SHA256SUMS. The kit's
`deps.txt` supplies all 360 dependency files in the same order to every host.
