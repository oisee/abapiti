Allocation work on `proto/hir-golang-alloc`, starting at `8665338` (the O(1)
`splice1_view` fix). Changes are confined to the Go runtime/emitter and its tests;
HIR, lowering and abaplint sources are unchanged.

All heavy builds, guards and measurements take `flock /tmp/abapiti-heavy.lock`.
Environment: `GOCACHE=$HOME/.cache/abapiti-go-cache`,
`GOFLAGS=-buildvcs=false`, `TMPDIR=$HOME/.cache/hir-alloc/tmp`.
Go 1.26.0; default GOGC and GOMAXPROCS. Another builder shares the host.

Each row is the median of three complete invocations, each with CPU and memory
profiles. Check seconds is the CLI's parse+report interval (compilation and input
loading excluded); GNU time supplies peak RSS. GC CPU is the fraction of sampled
CPU stacks containing collection, worker, assist, sweep or scavenger frames,
counting a sample once. Allocation GiB and object counts are the sums of
`alloc_space` and `alloc_objects` in the full memory profile, including loading
and reporting. The CLI's exact `TotalAlloc` is recorded separately because Go's
sampled allocation profile can lag the latest GC epoch. No timing-only flags or
GC tuning are used.

Each invocation strips trailing horizontal whitespace and compares stdout byte
for byte against the supplied kit. SHA-256:

- clean: `9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`
- seeded: `0a08fa85133e89faa08a841a01572b228428191c50844b62e0c8bb9965145e50`

| Step | Input | Check s | GC CPU % | alloc GiB | Objects M | Peak MiB | Exact TotalAlloc GiB |
|---|---|---:|---:|---:|---:|---:|---:|
| Baseline `8665338` | clean | 9.751 | 41.88 | 3.220 | 75.354 | 632.0 | 3.421 |
| Baseline `8665338` | seeded | 9.752 | 42.04 | 3.249 | 76.144 | 625.9 | 3.427 |

| 1: small push + concat | clean | 9.691 | 39.99 | 2.356 | 75.157 | 579.7 | 2.652 |
| 1: small push + concat | seeded | 9.653 | 40.41 | 2.375 | 76.714 | 582.1 | 2.659 |

Raw per-run measurements: [baseline](2026-10-10-hir-golang-alloc/baseline.json).
Executables, generated sources, profiles and logs are preserved under
`$HOME/.cache/hir-alloc/{baseline,stepN}`. The measurement and sample parsing
scripts are `$HOME/.cache/hir-alloc/measure.py` and `recalc.py`.

The first fresh clean profile (individual samples, not medians) attributes
1.383 GiB / 20.79M objects to `array[any].push`, .288 GiB / 2.99M to `slice2`,
.248 GiB / 11.08M to `Sequence.run`, .157 GiB / 7.01M to `Expression.run`,
.121 GiB / .135M to `concat`, and .081 GiB / 2.72M to `ResultNode` construction.
Flat values do not overlap; cumulative call-site attributions must not be added
as independent allocations. Push bytes are led by `Sequence.run` (403 MB),
`Expression.run` (287 MB), and `OptionalPriority.run` (168 MB).
96% of `slice2` bytes arrive through `slice0`, largely concat in structures
`Star.run`; direct statement-parser slices account for about 10.5 MB.

Step 1 (this commit): push starts at one slot instead of four and retains
geometric growth. Concat allocates the independent result once and copies both
operands, with headroom based on the left operand. Self-concat and aliases keep
separate mutable storage. The guard passed `go test -v ./hir/... ./tsfront/...
-count=1`: lexer 44/44 and mutation rejection, array/iterator/sort/feature/config/
tagged XML Node oracles, 65 statements, 192 structures, 18 MemoryFile and 16 split
cases. Optional ABAP-only/environment-controlled emit gates keep their existing
skips; required Go oracle gates ran. All six full checks match the hashes above.

Exact allocation bytes fall 22.5%; sampled bytes fall about 27%. Object count and
check time are almost unchanged. Push falls from 1417 to 512 MB in the first
clean profile. Concat itself rises from 124 to 411 MB while its slice copy is
removed; headroom is too generous for this workload and will be revisited.
Remaining slice copies total 96 MB. Sequence/Expression still allocate 256/179 MB
of headers; those and small literal buffers are the next target.
Raw step data: [step 1](2026-10-10-hir-golang-alloc/step1.json).
