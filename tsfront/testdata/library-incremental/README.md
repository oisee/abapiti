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
retain pending invalidations. ReaderVisits counts direct index candidates.

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
