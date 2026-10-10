Allocation work on `proto/hir-golang-alloc`, starting at `8665338` (the O(1)
`splice1_view` fix). Changes are confined to the Go runtime/emitter and its tests;
HIR, lowering and abaplint sources are unchanged.

All heavy builds, guards and measurements take `flock /tmp/abapiti-heavy.lock`.
Environment: `GOCACHE=$HOME/.cache/abapiti-go-cache`,
`GOFLAGS=-buildvcs=false`, `TMPDIR=$HOME/.cache/hir-alloc/tmp`.
Subsequent runs also set `GOMODCACHE=$HOME/.cache/abapiti-go-mod-cache`.
Go 1.26.0; default GOGC and GOMAXPROCS. Another builder shares the host.

Each row is the median of three complete invocations, each with CPU and memory
profiles. Total seconds is GNU time wall duration (including profile finalization);
check seconds is the CLI's parse+report interval (compilation and input
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
| Step | Input | Total s | Check s | GC CPU % | alloc GiB | Objects M | Peak MiB | Exact TotalAlloc GiB |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| Baseline `8665338` | clean | 9.89 | 9.751 | 41.88 | 3.220 | 75.354 | 632.0 | 3.421 |
| Baseline `8665338` | seeded | 9.90 | 9.752 | 42.04 | 3.249 | 76.144 | 625.9 | 3.427 |
| 1: small push + concat | clean | 9.82 | 9.691 | 39.99 | 2.356 | 75.157 | 579.7 | 2.652 |
| 1: small push + concat | seeded | 9.78 | 9.653 | 40.41 | 2.375 | 76.714 | 582.1 | 2.659 |
| 2: small array coallocation | clean | 12.05 | 11.827 | 39.62 | 2.446 | 64.906 | 601.3 | 2.735 |
| 2: small array coallocation | seeded | 12.49 | 12.279 | 39.76 | 2.443 | 64.658 | 600.3 | 2.741 |
| 3: discard copies + capacity | clean | 13.22 | 13.030 | 38.68 | 2.372 | 67.134 | 578.3 | 2.557 |
| 3: discard copies + capacity | seeded | 12.10 | 11.968 | 38.33 | 2.365 | 66.846 | 582.5 | 2.563 |
| 4: unused class source slots | clean | 10.90 | 10.783 | 38.56 | 2.160 | 63.101 | 580.4 | 2.530 |
| 4: unused class source slots | seeded | 10.86 | 10.721 | 37.60 | 2.280 | 65.854 | 572.5 | 2.537 |

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

During queued validation the shared lock path was deleted while builders still
held its inode. A fresh locked shell was closed without doing work after that
was detected. Later batches acquire both the recreated `/tmp/abapiti-heavy.lock`
and an open `/proc/PID/fd/3` reference to the original queued lock inode. This
preserves serialization with the builders already waiting on the deleted file.
No heavy work in this task intentionally bypasses the shared lock.

Step 2 (this commit): fresh array declarations followed by one to four pushes
coallocate the slice header and fixed initial buffer. Length remains zero until
the original pushes execute; no element evaluation or publication moves. Both
block and sequence preludes use the plan. Other arrays keep ordinary storage.
The semantic probe checks an element reading the preceding length, distinct
instances, aliases, growth, slice independence and disjoint views.

The first probe initially called a non-variadic fixture helper with a statement
list; that test compilation error was corrected. The complete uncached guard
then passed, with the same required Go oracle counts as step 1. All six full
checks match the kit hashes. Object counts fall about 12–15% from baseline,
while exact bytes rise about 3% from step 1 due to small-object size-class rounding.
Coallocation is retained for its object reduction; typed element storage is the
planned next byte reduction. This intermediate step is not a byte-volume win
relative to step 1.

The shared lock path was removed again during this batch. Other builders using
new lock inodes ran alongside this step despite this task holding the original
and recreated-path locks. Compilation, tests and measurement are serial within
this task, but step 2 and later wall time/RSS/GC readings include this external
contention. They are observations, not a controlled speedup claim. Allocation
volume and output equality are independent of that scheduling contention.
Raw step data: [step 2](2026-10-10-hir-golang-alloc/step2.json).

Step 3 (this commit): concat uses 12.5% headroom instead of up to twice the left
length. Unshift grows through push and moves the elements within the receiver's
own capacity; capped splice prefixes still cannot overwrite their live tails.
For directly discarded slice results, the emitter evaluates receiver and all
arguments in order and keeps the nil receiver failure, without constructing a
copy. Discarded splice operations mutate in place, clear removed slots, and
construct no returned array. Returned slices/splices retain their previous ABI.

The new probes compare discarded splice mutations with ordinary splice over
negative/clamped indices, check independent view mutation after unshift/growth,
and assert receiver-before-argument evaluation and nil failure for discarded
slice. The evaluation probe initially used numeric HIR addition for a string;
using the catalogue's string.concat corrected the probe. Full uncached guard
then passed, with all required Go oracle counts unchanged. All six full checks
match the kit hashes. Exact allocated bytes are now about 25.3% below baseline;
small headers and pointer element storage remain the next targets.
Raw step data: [step 3](2026-10-10-hir-golang-alloc/step3.json).

Step 4 (this commit): build the actual recursive materializers before declaring
class storage. Only data shapes reached by those materializers retain their
source pointer. Other shapes keep normal distinct object identity without a
permanently nil source slot. ResultNode shrinks from 32 to 24 bytes on this
amd64 layout. Materialized root and nested objects still box back to their
original dynamic object, including unknown JSON fields; a dedicated probe checks
that and independent ordinary shape instances.

The complete uncached guard and all six full-check output hashes passed. Exact
allocated bytes fall a further ~27 MB, to about 26.1% below baseline. This does
not remove the required ResultNode instances or their distinct identities.
Raw step data: [step 4](2026-10-10-hir-golang-alloc/step4.json).
