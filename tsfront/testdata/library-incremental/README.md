# Incremental library differential and timings

First generate `m1.json` with `tools/library-tree-manifest.mjs` and the frozen M1
kit. `tools/library-incremental-manifest.mjs m1.json > sequences.json` adds four
edit/check/restore sequences: undefined variable in the **first** URI method,
unknown statement, empty source, and a comment. Each step checks the edited class
and its 13 direct readers. Both broken and restored snapshots retain all issue
ordering, coordinates, filenames, structures, statements, tokens and nil values.

Build `dump.go.txt` as a consumer of the freshly emitted library. Compare its
output byte for byte with:

```
node tools/library-incremental-reference.mjs ORIGINAL_CORE_DIR sequences.json
```

The independent original Node 2.120.73 oracle invalidates direct raw-source
lexical readers as the host does; `--native` isolates Registry.updateFile without
that host invalidation. Bare Registry.updateFile only dirties the owner; on the
empty-source step its cached dependent diagnostics differ for 13 readers. The
edited owner's snapshots agree with native Node for all four edit/restore pairs.
The default differential compares the same direct-reader policy on both engines.
It reports dirty ABAP objects and direct reader identities
to stderr. Run the same drivers on `../library-data/scenarios.json` for all 41
PR #839 scenarios, including the three explicitly restored inline overlays.
The existing tree and data dump/oracle pairs remain additional gates.

For timings, build `bench.go.txt` with `outline-before.go.txt` against base, and
with `outline-after.go.txt` against the branch. Build with the same flags and use
the required 16 GiB systemd scopes, scratch TMPDIR/GOCACHE, and load/disk guards.
Then run:

```
python3 tools/library-incremental-bench.py BEFORE AFTER m1.json timings.jsonl
python3 tools/library-incremental-bench.py BEFORE AFTER m1.json gc.jsonl --gc
```

Each server primes M1 once and completes the preceding lazy restore before the
next timed operation. Two warmups precede 20 alternating, balanced A/B pairs.
Broken timings include overlay Update, CheckObject, restore Update. `probe`
separates update, registry parse (Files of a missing object), check, restore;
it is a diagnostic measurement, not a different API implementation. `tree` uses
the persistent full Checker; `snapshot` primes an isolated Checker with the URI
object's files then calls Tree; `outline` compares that isolated Checker with
TreeOfWithConfig. CPU profiles are collected separately after timing. `--gc`
compares GOGC=100/200 for the branch; the library never changes process GC policy.

TreeOf skips global type-definition discovery, retaining the pinned registry's
per-object parsing and cross-include macro pass. TreeOf uses default core configuration. TreeOfWithConfig matches Prime's syntax
release and global-macro settings. External INCLUDE/macro definitions are absent
unless their files are supplied. Existing Checker.Tree retains full-registry
expansion. Both APIs return copied data under libraryMu. Non-ABAP objects return
a found, non-nil empty slice; missing objects return nil, false.

Stats.ParseObjects counts pending invalidations submitted after Update. It does
not count untouched permanently dirty no-op parser objects, initial Prime, or
extra internal macro passes. DirtiedObjects counts unique owners per Update;
several Updates before a parse accumulate a union in ParseObjects. No-op Updates
retain pending invalidations. ReaderVisits counts candidates visited across the reverse lexical closure.
DirtiedObjects/ParseObjects count reparses, excluding syntax-only cache drops.

The opt-in Go test accepts `ABAPITI_LIBRARY_INCREMENTAL_DIR` (emitted core),
absolute `ABAPITI_LIBRARY_INCREMENTAL_MANIFEST` and `ABAPITI_LIBRARY_INCREMENTAL_REFERENCE`
(the original Node JSONL), and runs with
`go test -short ./tsfront -run TestRegistryGoLibraryIncremental -v` inside the
required systemd scope. Run once for the M1 sequences and once for scenarios.

The u7 measurements are committed in `docs/evidence/library-incremental-u7.json`.
With default GC, broken checks improved from 1,183 to 655 ms and allocated 472
instead of 209 MB. An isolated Checker + Tree took 16.8 ms; TreeOf took 16.0 ms
and allocated 7.44 instead of 7.90 MB. Parsing dominates the outline, so its
latency improvement is modest. The GC=100/200 comparison measured 668/512 ms for
broken checks and 1.98/2.70 GiB peak RSS. All medians use 20 alternating pairs.

## Transitive correctness (2026-10-22)

Update now walks the reverse closure of the raw lexical-reader index, clearing
`ABAPObject.syntaxResult` through the emitted base accessor. This is a single
field write per ABAP consumer; parsed files, OO definitions, XML/text caches,
and the parse dirty flag survive. Only the edited owner and direct readers
reparse, with one macro exception: a transitive consumer containing `MacroCall`
or `Unknown` reparses as well. ExpandMacros stores expanded statements inside
ABAPFile; clearing syntax cannot update them. Unknown may become a macro call
when a previously missing definition is introduced. The core's macro references
do not completely describe nested expansions, so these potential macro consumers
are conservatively rebuilt within the lexical closure. All operations remain
under libraryMu; only the edited owner is unindexed/reindexed.

**Node stale:** original native Node 2.120.73 retains obsolete syntax results in
all five critic scenarios (inheritance, interface alias, INCLUDE chain, macro
chain, type pool). Its direct-reader host policy is also stale for these
transitive cases. `transitive.json` preserves those minimal inputs; the opt-in
`TestRegistryGoLibraryTransitive` uses `ABAPITI_LIBRARY_TRANSITIVE_DIR` and compares
incremental issues and trees against fresh Checkers, with and without a prior
syntax check, and after restoration. It asserts the semantic error and that
only two owners reparse (three for the stored macro expansion).

Keep the original direct-reader Node differential as a separate compatibility
gate. Add `--transitive` to library-incremental-manifest.mjs for the full M1
consumer closure (48 queries, versus 14 direct queries). Run the emitted
`dump.go.txt` consumer with `MANIFEST --fresh` to compare every baseline, edited,
and restored query's complete issues and tree with a Checker freshly primed on
that exact state. It exits on any mismatch and otherwise emits the usual JSONL.
Fresh-state equality is the correctness oracle when native or host-policy Node
retains stale caches. The same driver also supports the 41 #839 scenarios.

The opt-in Go differential also accepts `ABAPITI_LIBRARY_INCREMENTAL_FRESH=1`
in place of `_REFERENCE`, keeping the original Node comparison as the default.

For a container without systemd, use its memory cgroup and limit heavy Go
processes explicitly. The i10 run used GOMAXPROCS=2, scratch TMPDIR/GOCACHE,
and GOMEMLIMIT=12GiB for the short suite. If scratch temporary directories sit
inside this Git checkout, set `GIT_CEILING_DIRECTORIES` to that temporary root
so isolated host-I/O probes do not discover the parent repository. With Node 26,
`NODE_OPTIONS=--disable-warning=MODULE_TYPELESS_PACKAGE_JSON` keeps module-type
warnings out of CombinedOutput-based oracle comparisons. The benchmark disk
guard now checks its output filesystem instead of a host-specific mount.

The load guard defaults to 12. On an already busy shared host, an explicit
`ABAPITI_BENCH_MAX_LOAD` can raise that limit; preserve the raw sampled Load
values and qualify the resulting medians as shared-host measurements.
