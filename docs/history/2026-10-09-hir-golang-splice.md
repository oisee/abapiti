The Go structure matcher used the existing `array.splice1_view` operation,
but its runtime implementation called copying `splice1`/`splice2`/`slice2`.
The first fresh clean allocation profile attributed 57.75 GiB to
`src/abap/3_structures/structures/_combi.ts.SubStatement.run` at
`statements.splice(1)`. Copying tails accounted for roughly 95% of allocated
bytes. `unshift` and ordinary front removal were not material allocating
operations in this workload; `shift` already advances a Go slice in O(1).

This step changes only the Go runtime implementation and adds semantic tests.
It splits the existing storage into a returned tail and a retained prefix.
The prefix uses a full slice expression to cap its capacity at its length,
so subsequent growth allocates separate storage and cannot overwrite the
tail. Existing aliases of the input still observe its truncation; the result
is a distinct array. Writes, reverse, shift, pop and splice operate within
disjoint ranges. Empty ranges release their backing reference. No HIR,
frontend, emitter, `hir.Inline`, or benchmark-runner changes were needed.
The complete freshly emitted class bodies and CLI have exactly the same
SHA-256 hashes as the baseline materialization.

Validation passed with `GOFLAGS=-buildvcs=false go test ./hir/... ./tsfront/...`:
lexer 44/44 including the token mutation check, registry array (17 methods),
iterator (1), sort (3), feature, config and tagged XML oracles, 65 statement
dumps, 192 structure dumps, 18 MemoryFile cases and 16 split cases.
The new generated-Go semantic probe covers negative and clamped indices,
input identity and truncation, prefix/tail mutations and growth, and views
of views. All full-check clean and seeded stdout files match the kit:

- clean: `9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`
- seeded: `0a08fa85133e89faa08a841a01572b228428191c50844b62e0c8bb9965145e50`

Measurements use the unchanged `tools/hir-go-check.mjs` runner and pinned
fresh Node oracle. The order is baseline 1, splice view 1, baseline 2,
splice view 2, baseline 3, splice view 3. Every invocation checks both clean
and seeded input, with CPU and allocation profiles. Compilation is excluded;
no builds or tests from this task overlap these alternating measurements.
The earlier profiling run overlapped compilation and is excluded from the
medians. Each table column is the median of three observations. Peak RSS is
GNU time maximum RSS; GC share is the runner's sampled GC CPU stack share;
alloc_space is the sum of sampled allocation bytes from the full-check
memory profile, including loading and reporting, rather than only structures.
GOGC and GOMAXPROCS retain their defaults. The machine uses Go 1.26.0 and
Node 26.9.0. Another builder was running a full registry closure test at about
200% CPU and 1–3 GiB RSS; it was left running. Timings include that contention.

Shared `/tmp` is a small tmpfs and filled during initial builds. Subsequent
build, test and Node oracle scratch files used
`/home/alice/.cache/hir-splice-tmp` on the disk filesystem. The installed Go
distribution omitted the `pprof` executable: `go build -o /tmp/hir-pprof
cmd/pprof` built it from the installed Go sources, and a temporary PATH wrapper
routed `go tool pprof` to it without modifying the runner. Both variants used
the same wrapper and environment.

The baseline executable is `/tmp/hir-full-final/zabaplint`; the new executable
is `/tmp/hir-splice-view/zabaplint`. Raw profiles, output files, runner JSON and
logs are under `/tmp/hir-splice-bench/alternating`. The committed JSON report
records per-run resources, allocation totals, output hashes and medians.

The median full-check measurements for the one green implementation step are:

| Revision | Input | Structures s | Check s | Peak MiB | GC CPU | alloc_space GiB |
|---|---|---:|---:|---:|---:|---:|
| 216212c baseline | clean | 49.220 | 55.678 | 717.1 | 83.16% | 61.128 |
| 216212c baseline | seeded | 38.678 | 47.454 | 674.7 | 83.17% | 61.150 |
| O(1) splice view (this commit) | clean | 0.772 | 7.773 | 546.1 | 53.54% | 3.205 |
| O(1) splice view (this commit) | seeded | 0.728 | 8.537 | 536.1 | 52.73% | 3.257 |

Flat allocations from the first clean profile in each alternating series
(individual samples, rather than medians; rows must not be added to cumulative
call-stack attribution):

| Allocator / TS method | Before GiB | After GiB | Before objects M | After objects M |
|---|---:|---:|---:|---:|
| `array[any].slice2` | 58.169 | 0.300 | 3.40 | 3.76 |
| `array[any].push` | 1.400 | 1.363 | 21.10 | 20.40 |
| `statements/combi.ts.Sequence.run` | 0.246 | 0.249 | 10.99 | 11.14 |
| `statements/combi.ts.Expression.run` | 0.170 | 0.163 | 7.62 | 7.27 |
| `array[any].concat` | 0.113 | 0.113 | 0.13 | 0.10 |
| `statements/result.ts.ResultNode constructor` | 0.077 | 0.084 | 2.59 | 2.83 |

Before, `SubStatement.run` drives `splice1_view → splice1 → splice2 → slice2`.
After, `push` is driven mainly by statement `Sequence.run`, `Expression.run`,
`OptionalPriority.run`, and `Result.wrapConsumed`. Remaining `slice2` is
mostly `slice0`, including `concat` from structures `Star.run`;
`StatementParser.removePragma` accounts for most of its direct calls.
`Token.run`, `Word.run`, `Vers.run`, and `ResultNode` construction also rank
high by object count. Total sampled object counts in these clean profiles
are approximately 77.1 million before and 76.1 million after: the major
improvement removes copied bytes and scanned pointer storage, rather than
most small object allocations.

[Raw per-run measurements and allocator identities](2026-10-09-hir-golang-splice.json)

Remaining allocation work is concentrated in statement combinators: growing
small result arrays, `Sequence.run`, `Expression.run`, `OptionalPriority.run`,
`AlternativePriority.run`, and `ResultNode` objects. Ordinary copying slices,
concat and result-discarded splice/slice calls retain their previous behavior.
General front-removing `splice2` still moves the remaining elements;
unshift still reallocates, and pop/shift do not clear vacated slots. Shared
disjoint ranges can retain a backing allocation until its last range dies;
this patch does not promise arbitrary slices can become shared views. Any
further optimization should start from the new profile and preserve the same
aliasing/identity contracts. Nothing was pushed.
