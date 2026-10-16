# Step-4 storage, sorting, and completion lowering

Base: `origin/integ/selfhost-s7`,
`683d0e3036a2358946c4b84873ab89bf1d616b7e`. Branch: `dell/storage`.
OSG pilot input: `1638d1cbc2549003b5ad2c4384b749c34beb034b`.
Abaplint: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

## Changes and oracle evidence

Each construct started with a failing Node differential test, before its
implementation. `storage_destructure_test.go`, `storage_sort_test.go`, and
`storage_finally_test.go` run Node's stripped TypeScript, lower the same source,
execute `runGoHIR`, and collect ABAP through `emitABAPNamed`. The shared helper
attaches an ABAP Unit assertion against the Node result. The emitted ABAP is
also executed through OSG's `tools/gogen/unit.mjs`, with `--jobs 2`.

* Dynamic array binding declarations and for-of patterns publish a checked
  `dynamic.arrayView` once, then read their bindings from that storage. The
  ABAP erased reference-array adapter reads native scalar/reference arrays as
  well as parsed array graphs. Missing bindings retain undefined. The for-of
  prelude stays outside the loop; only pattern bindings enter its body.
  Defaults, rest and nested patterns on an erased array still receive a located
  blocking diagnostic. Runtime non-array storage refuses at the view boundary.
* Stored, module, and declared-method comparators use the existing callback
  adapters and stable insertion sort. The supplied pilot's four sort sites
  actually call `.sort()` without a comparator (six diagnostic occurrences).
  Default ordering compares string conversions by UTF-16 units, including
  surrogate pairs and trailing blanks. Certified keys are primitives, lowered
  primitive tuples, and checked flat primitive arrays, including boxed map-entry
  rows. Object/prototype coercions and nested dynamic arrays receive located
  refusals; required dense element storage and the existing consistent/pure
  comparator contract remain the admitted sorting subset.
* HIR Finally now preserves or replaces all ordinary completions. Go's existing
  return/loop transport handles the expanded HIR. ABAP transports escaping
  returns, breaks and continues through private exception classes outside the
  source throw hierarchy, runs cleanup once, then resumes the pending completion.
  A cleanup completion replaces an earlier one. For loops with a continue
  crossing a finalizer run their update at the next iteration's entry, after
  cleanup and only if cleanup did not replace the continue. Labelled jumps in a
  try/finally receive a located diagnostic until target-aware flag rollback exists.

The main oracles cover stable equal-key ordering, result/receiver identity,
stored and module comparators, non-ASCII default ordering, flat rows with null,
JSON/native array storage, missing elements, return-value snapshots, return
from a catch, nested finalizers, return/throw replacement, and break/continue.
A second finally oracle was red with Go `1` versus Node `01`: a for-loop update
ran before cleanup. Both Go and emitted ABAP now produce Node's `01`.
The existing A15 callback-capture oracle now requires successful ABAP emission
instead of its superseded generic-array-view refusal.

## Pilot

The measurement script temporarily creates a tsconfig in its OSG checkout.
To preserve read-only use of `/logs/pilotsetup`, the OSG tree was copied to
`/work/.local/osg`; the driver input's `/work/.ext/osg-1638...` path points to
that copy. The copied worktree's gitdir pointer was repaired to the original
read-only git metadata. The script and both library checkouts remain under
`/logs/pilotsetup`. Its hard-coded GNU time path is a local symlink to the
supplied tool. The final paired run invokes the supplied script first with
`/work/.local/abapiti-base`, then with `/work/.local/abapiti-final`.
All heavy runs use `ulimit -v 16777216`, and Go uses `-p 2` or `GOFLAGS=-p=2`.

Pilot output and complete diagnostic evidence are retained in
`/work/.local/m-base` and `/work/.local/m-final`.

| Requested source sites | Base | Final |
|---|---:|---:|
| Dynamic array destructuring | 3 | 0 |
| Sort | 4 | 0 |
| Finally | 3 | 0 |

Sort has six raw diagnostic occurrences at four source locations. None of the
old messages remains, and there are no replacement default-sort refusals.

| Step-4 pilot | Base | Final |
|---|---:|---:|
| Blocking diagnostics | 304 | 274 |
| Root / cascade | 206 / 98 | 185 / 89 |
| HIR errors | 1,691 | 1,677 |
| Files with errors | 37 | 37 |
| Lowering wall time | 294.69 s | 262.63 s |
| Peak RSS | 1,414,316 KB | 1,411,928 KB |

Both runs have 56 manifest files, 51 loaded files, 422 bodies, 226 executed
bodies and three unmatched coverage entries. Node reference output is
`cmp`-equal, SHA-256
`a13702d55b1eecdb200078205da5909961b4b49ce0f6c3fe3bdbf08e802d6227`.
The entire step-4 path remains blocked by other constructs; this change removes
30 diagnostics, including all ten requested source sites.

## Gates

* `go build ./...` and `go vet ./...`: pass.
* `go test -short -p 2 -timeout 30m ./hir/... ./tsfront/... ./cmd/abapiti/... ./internal/abapfixture/...`: pass (tsfront 597.410 s).
* `(cd extern/hostio && go test ./...)`: pass (0.303 s).
* Generated abaplint Go assignment-mismatch trap sum: base **2**, final **2**.
  Both full-core generations have zero blocking diagnostics and zero HIR errors.
  Root and `packages/core` dependencies were installed with `npm ci` in the
  local 577f875e checkout before generation.
* Downloaded `v0.5.2/zabapgit-check-kit.zip`: clean **0**, seeded **5** issues.
  Both binaries' outputs are mutually byte-identical and `cmp`-equal to both
  kit expected files. The release kit's own `check.sh` passes for each binary.
  Base kit wall time: 46.94 s (clean 22 s, seeded 25 s); final: 65.00 s
  (clean 30 s, seeded 35 s). Individual seconds are the kit's rounded readings.
* M1: pass, **80 issues**, byte-identical to pinned Node by `cmp` (50.43 s).
  The runnable procedure is recorded in `docs/evidence/gogen-a15.md` and
  `gogen-abaplint-traps.md`. The frozen kit and multifile driver are available
  under `/logs/m1wl/.tmp/selfhost`. Copying the driver into the newly generated
  final Go package, building it, and running the kit's `check.sh` verifies
  `--file files.txt --config abaplint.json --deps deps.txt -allow-read .`.
* Five emitted-ABAP Unit oracles: **SUCCESS** (destructuring, sorting,
  comparator throw, finally, finally/for-update ordering). Their Go executions
  also match their Node oracles.

Snapshot sorting changes `hir.go`, `hir_profile.go`, `literals.go` and
`site_profile.go`; source equality is not the behavior gate. Final core
emission took 127.276 s; the generator's Go build took 85.7 s. Baseline emission
took 72.421 s and the separate baseline build took 48.75 s. Concurrent gate
work affects these timings, so they are observations, not a speed claim.

Machine-readable summaries, source locations, ABAP verdicts, expected/output
hashes and kit timings are in [storage-lowering.json](storage-lowering.json).
Full logs and generated artifacts remain under `/work/.local/`.


The sort snapshot/writeback boundary follows
[SortIndexedProperties](https://tc39.es/ecma262/multipage/indexed-collections.html#sec-sortindexedproperties)
and [Array.prototype.sort](https://tc39.es/ecma262/multipage/indexed-collections.html#sec-array.prototype.sort).
The comparator-throw oracle was red with Go `1,2,3,99:compare` versus Node
`3,2,1,99:compare`. Sorting a snapshot fixes that failure and lets a read-only
comparator observe the original receiver until successful writeback.
The finalizer rule follows
[TryStatement evaluation](https://tc39.es/ecma262/multipage/ecmascript-language-statements-and-declarations.html#sec-try-statement-runtime-semantics-evaluation).
