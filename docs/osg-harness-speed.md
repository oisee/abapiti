# TS-HA@OSGO harness: speed and memory

## One build: five-shard measurement at `98fa0675`

**34m12s → 7m19s (4.7× faster); five lowerings/frontends/builds → one;
382/382 registrations unchanged. Build peak RSS: 9.48 → 13.10 GiB.**

| Path | In-lock wall (s) | Frontends / builds | Frontend total (s) | Go build total (s) | Build peak RSS (GiB) | Whole-run peak RSS (GiB) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Old per-shard (`ABAPITI_OSG_ONE_BUILD=0`) | 2052.09 | 5 / 5 | 690.11 | 357.10 | 9.48 | 9.48 |
| Default union, runtime selection | 439.22 | 1 / 1 | 173.86 | 93.73 | 13.10 | 13.10 |

Measured union implementation: `1537183`; the baseline uses the matching
single-shard emission and identical build flags. Both paths use clean OSGO
`98fa0675f47104ded037083c453ac0bb33b5ff57`, original abaplint
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`, Go 1.26.0 and Node 26.
Build mode is `fast`; harness snapshots and gogen's optional cache are disabled.
Go's shared `GOCACHE` is enabled on both paths. GNU `/usr/bin/time -v`
measures each generated-module build and each complete run. RSS is the largest
single process, not aggregate cgroup memory. Lock waiting is excluded; oracle,
materialization, lowering, resource admission and reporting are included.

**Timing caveat:** the old run recorded 406.44 s of initial frontend admission
waits, versus effectively zero for the union. Scheduler/build rechecks can also
wait. Removing just those recorded waits gives 1645.65 versus 439.22 s (3.7×);
this is not a controlled estimate of all admission-free work. The phase totals
still show the independent saving from doing five frontends/builds only once.

| Shard | Pass (both) | FAIL (both) | Trap (both) | Registered (both) | Unregistered (both) |
| --- | ---: | ---: | ---: | ---: | ---: |
| `abap/lexer` | 200 | 0 | 0 | 200 | 0 |
| `abap/expressions` | 22 | 0 | 85 | 107 | 0 |
| `utils` | 9 | 0 | 6 | 15 | 0 |
| `pretty_printer` | 2 | 0 | 29 | 31 | 0 |
| `abap/types` | 6 | 0 | 23 | 29 | 0 |
| **Total** | **239** | **0** | **143** | **382** | **0** |

Comparison checks the complete **ordered** `(title, status)` vectors, retaining
repeated-title multiplicity, plus counts, initialization diagnostics, Node-active
accounting and unregistered titles. Both runs have zero unmatched results,
infrastructure failures, async-pending, skipped and Node-pending tests. Generated
Go line numbers in native host diagnostics are not a message-identity guarantee.

The union contains 24 selected roots, 1,566 original TS closure files and 2,497
emitted ABAP class/interface files. It fits well below the ~40 GiB build budget;
**no grouping was needed**. This is not a memory measurement of all 17 shards.
Rules, objects, syntax and statements were **not run on u7**. This historical
five-shard union can be requested with `ABAPITI_OSG_SHARDS_PER_BUILD=5`;
the current default and broader build-failure fallback are described below.

## Historical flag/cache measurements (`368baab28`)

OSG pin `368baab28`, Go 1.26.0, Node 26, 16-core host shared with other
sessions. Each run held the shared OSG flock inside a
`systemd-run --user --scope -p MemoryMax=48G` scope at `nice 10`, one heavy
step at a time. Times are wall-clock inside the lock (lock waiting is
excluded). Peak RSS is the largest single process in the tree (GNU time),
which is always either the Go compiler or the gogen frontend.

### Before / after

| Shard | Run | In-lock wall (s) | Frontend (s) | Emit (s) | Go build (s) | Run (s) | Peak RSS (GiB) |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap/lexer` | before | 318 | 130 | 11 | 94 | 0.1 | 16.3 |
| `abap/lexer` | after, cold | 262 | 110 | 9 | 56 | 0.1 | 7.0 |
| `abap/lexer` | after, unchanged rerun | 83 | cached | cached | cached | 0.0 | 0.8 |
| `root` | before | 363 | 129 | 12 | 115 | 3.4 | 21.3 |
| `root` | after, cold | 313 | 124 | 11 | 66 | 4.3 | 9.0 |
| `root` | after, unchanged rerun | 109 | cached | cached | cached | 5.7 | 2.0 |
| `abap/statements` | before | 606 | 138 | 11 | 111 | 242 | 21.0 |
| `abap/statements` | after, cold | 607 | 126 | 10 | 64 | 303 | 8.9 |
| `abap/statements` | after, cold (repeat) | 634 | 132 | 10 | 2 (GOCACHE) | 389 | 7.5 |
| `abap/statements` | after, unchanged rerun | 409 | cached | cached | cached | 310 | 0.8 |
| `abap/statements` | optimised build (`ABAPITI_OSG_BUILD_MODE=default`) | 622 | 140 | 12 | 106 | 263 | 15.5 |

Peak memory falls by more than half on every shard (16–21 GiB to 7–9 GiB), so
two shards no longer approach the 32 GB compile that was OOM-killed. Cold wall
time falls 18 % (lexer) and 14 % (root). The Go build falls 40–43 %. An
unchanged rerun skips frontend, emission and build; what remains (80–110 s)
is abapiti's own TS→ABAP lowering of the closure plus the test run
(statements: 409 s, of which 310 s is the run).

Statements is run-bound: its 4,962 tests (4,533 traps) execute for 4–6
minutes, and the unoptimised `-N -l` binary runs slower (303–389 s against
242–263 s; run times on this shared host vary by ±25 %). There the build
saving and the run cost cancel; the memory saving remains. For a run-heavy
shard on a quiet host, `ABAPITI_OSG_BUILD_MODE=default` keeps optimisation and
still drops DWARF, symbols and line directives (106 s, 15.5 GiB).

### Historical correctness gate

For lexer, root and statements the full multiset of `(title, status)` is
identical between the original harness and every after run (cold, rerun,
frontend-cache hit with a rebuilt binary, optimised build), and so are the
per-title messages. Lexer 200/200 pass; root 8 pass + 86 traps; statements
429 pass + 4,533 traps; FAIL 0 everywhere.

### Build flags, snapshots and scheduling

- **Go build of the generated module** (`tools/abaplint-tests-osg.mjs`):
  `go build -p 1 -trimpath -ldflags='-s -w'
  -gcflags=osg/gogen/cmd/abapitests='-N -l -dwarf=false'`, `GOGEN_NOLINE=1`
  while gogen emits (no `//line` directives), `GOMAXPROCS=8`. The compiler
  profile of lexer (2.1 M lines, 107 MB of Go, 231,663 functions): parse and
  type-check take 27–30 s single-threaded regardless of flags; `-N -l` cuts
  the backend from 148 s to 32 s at the same concurrency. GOGC and GOMEMLIMIT
  stay at Go's defaults: stoker measured GOGC=50 at +38 % and a 12 GiB
  `GOMEMLIMIT` at 2.7× time on objects (thrashing). `ABAPITI_OSG_DEBUG=1`
  keeps DWARF, symbols and line directives; without it a panic points into
  generated Go rather than `.abap`. `ABAPITI_OSG_BUILD_MODE=default` drops
  `-N -l`.
- **Content-addressed snapshots** under the caller-owned `ABAPITI_OSG_CACHE`
  (`tools/osg-harness-cache.mjs`): the frontend snapshot (generated Go and
  refusal evidence) is keyed on the bytes of every registry-visible ABAP file
  and every skip decision, OSG `tools/` (gogen and its Go runtime),
  `node_modules`, the transpiler/core packages, lock files, `GOGEN_*`, the
  Node version and the bridge itself; the executable snapshot adds build
  flags, `GOFLAGS` and `go env`. Keys hash file contents, never mtimes or
  scratch paths (about 3 s per run). Entries are published atomically;
  corrupt or partial ones are misses. Verdicts are never cached: a hit
  restores code, which always runs in a fresh process. Go's own `GOCACHE`
  additionally reuses the compiled package when the generated Go is
  byte-identical (statements repeat: build 2 s).
- **gogen frontend cache**: when the pinned gogen `compileProgram` accepts
  `cacheDir` (available from `af66ed28`, not in the current `98fa0675` pin),
  the bridge passes `ABAPITI_OSG_CACHE/gogen-frontend`. Feature-detected; older pins are
  unaffected.
- **Memory release**: the frontend registry and IR are dropped and collected
  (`node --expose-gc`) before the compiler starts, so Node and the compiler
  do not peak together. The bridge's heap is capped at 12 GiB
  (`--max-old-space-size=12288`; stoker measured an ~11 GiB peak on objects)
  unless `NODE_OPTIONS` sets one.
- **Scheduler** (`tools/abaplint-tests-all.mjs`): with
  `ABAPITI_TESTS_TARGET=osg` shards run one at a time whatever
  `ABAPITI_TESTS_WORKERS` says, and each shard and each Go build is admitted
  only at load < 12, IO pressure avg10 < 30 and MemAvailable ≥ 20 GiB.
  `ABAPITI_OSG_LOCK=<file>` wraps each shard in `flock`,
  `ABAPITI_OSG_MEMORY_MAX=48G` in a systemd user scope at `nice 10`. Do not
  set `ABAPITI_OSG_LOCK` when the caller already holds the same lock.

## Runtime-selected union (no OSG changes)

`tools/abaplint-tests-all.mjs` defaults to one OSG build per group of up to
**three contiguous selected shards** (`ABAPITI_OSG_SHARDS_PER_BUILD`, integer
1–17; set 17 to attempt the entire pinned suite in one union).
`ABAPITI_TESTS_SHARDS`, `ABAPITI_TESTS_FILES` and `ABAPITI_TESTS_FIRST` retain their existing selection/ownership/order behavior.
The Go target remains per-shard. `ABAPITI_OSG_ONE_BUILD=0` selects the old OSG
path for comparison or troubleshooting.

### Default grouping and automatic build-crash fallback

The pinned closure plan (`577f875e`) has 17 shards. Inspecting the manifests
produced by `materializeRegistryClosure` gives the following sizes. Test bytes
include borrowed test sources, but exclude the largely shared core/runtime.

| Large shard | Roots | TS closure files | Test closure files | Test source bytes |
| --- | ---: | ---: | ---: | ---: |
| `abap/statements` | 286 | 1,828 | 288 | 576,481 |
| `abap/syntax` | 8 | 1,550 | 10 | 569,633 |
| `objects` | 26 | 1,567 | 27 | 178,332 |
| `rules` | 192 | 1,732 | 192 | 967,294 |

The default of **3**, rather than all 17, makes six builds in the canonical
plan order:

1. `abap`, `abap/expressions`, `abap/lexer`
2. `abap/nodes`, `abap/statements`, `abap/structures`
3. `abap/syntax`, `abap/types`, `root`
4. `cds`, `ddl`, `lsp`
5. `objects`, `objects/rename`, `pretty_printer`
6. `rules`, `utils`

Thus the four large shards never share a default full-suite build, while the
smaller shards still amortize the common ~1,540-file core closure. Syntax's
size illustrates why root count alone is not a useful size estimate. This is
a conservative planning default, **not a measured memory guarantee**. Selection
and `ABAPITI_TESTS_FIRST` preserve their requested order and can change group
composition; the explicit size override always chunks that order unchanged.

**Any failure in the union build-only phase**—abapiti lowering, gogen
frontend/emission, or Go build—now halves the group into contiguous halves
(ceil/floor), then retries recursively before running any tests. Non-zero
exits, signals, stack overflow/`RangeError`, OOM, and errors without an OOM
signature all take this path. A successful build measured above 40 GiB still
splits too. Each split logs the shard names, child sizes, and first error line
(prefer stderr when available; otherwise the exception message). This handles
the observed all-17 gogen `lines.push(...stmt())` stack overflow without
modifying OSG or masking the upstream bug.

A singleton build failure is not retried: it reaches the existing per-shard
infrastructure-failure path, with no registrations and every oracle case
not-run. Other shards continue. There are no fabricated passes, omitted
shards, or verdict caches. Runtime failures never trigger splitting, and a
genuine FAIL still stops scheduling after draining in-flight work. Failed
parent build artifacts are disposed before smaller builds are materialized.

`tools/abaplint-tests-autosplit-test.mjs` stubs only the builder (no OSG):
4 → 2+2 → 1+1 checks exact per-shard verdict equality, lowering/frontend/Go
errors, signals, OOM, first-line split logs, odd-sized halving, the 40 GiB
fallback, persistent singleton failure through the real report/summary tools,
default groups, and runtime FAIL isolation.

Autosplit verification: `go vet ./...`; `GOFLAGS=-buildvcs=false go test -short
-p 2 ./tsfront/ ./cmd/... -count=1`; all 15 `tools/*-test.mjs` files run
individually with `GOTOOLCHAIN=go1.26.0` (including the 9 new autosplit tests);
lint against `origin/main`: **0 new issues**, canary **5/5**, golangci-lint
**2.13.2**. No OSG run was performed for this change, and no OSG files were
modified.

`tools/abaplint-tests-sharded-closure.mjs` materializes the union of the original,
unmodified static closures, but stores each shard's original dependency-first
initialization order separately in `closure.json`. Shared sources are lowered
once. The ABAP runner's `CASE shard` executes only the selected initialization
plan, including borrowed test modules and the forced runtime initializer. It
never registers all tests and then filters titles. No shared-core package,
gogen layering or OSG source edits are needed.

The bridge's internal `ABAPITI_OSG_BUILD_ONLY=1` phase runs gogen and Go once.
`tools/osg-runner-execute.mjs` then invokes that same binary in a fresh process
for each shard, with `ABAPITI_OSG_SHARD` passed by generated main into the ABAP
runner. Checkpoint/resume state is freshly allocated per shard; restarts retain
that selector. Local initialization/test indices, stable registration vectors,
FAIL handling, borrowed-registration ownership and the legacy JSON report keys
are unchanged. An unknown selector fails closed without a plan or results.

Each report retains `osgEvidence`: its build/front-end timings and refusal data
refer to the shared union, while `timingMs.run` and exit status refer to that
shard. `host.sharedBuild` identifies shared evidence; do not sum the repeated
build timings across those rows. Verdicts are never cached.

`TestAbaplintTestShardsOSGEmission` uses the fixture-collecting emitter helper.
With `ABAPITI_OSG_HOME` it builds once and alternates a/b/a/b selections, checking
borrowed registrations, duplicate titles, shard-local initialization and body
host-dump recovery, an earlier genuine FAIL, a following pass and rejection of
unknown selectors. Both real OSG fixtures pass at `98fa0675` (6.1 s each).
On each a invocation the merged report retains 3 passes, 1 genuine FAIL,
1 host test trap and 1 host initialization trap, with all 5 registrations;
b retains 2 passes and both registrations without contamination. The alternating
selection executes 8 subprocesses plus one rejected unknown-selector invocation,
all using one binary. The original fixture separately retains its 4 real host
dumps and all 12 registrations.

Final gates: `go vet ./...`; `GOFLAGS=-buildvcs=false go test -short -p 2
./tsfront/ ./cmd/... -count=1`; all **38 Node tests**; lint against `origin/main`
with **0 new issues**, canary **5/5**. OSG remains clean; the gogen Go cache and
caller-owned scratch are removed after verification.

## Reproduction

```sh
mkdir -p /mnt/ram/abapiti/tmp; touch /mnt/ram/abapiti/osg-heavy.lock
export GOCACHE=/mnt/ssd-510/osg/go-build TMPDIR=/mnt/ram/abapiti/tmp
export TRANSPILER=/mnt/ssd-510/osg/abapiti/transpiler-ee5c2cd1
export npm_config_cache=/mnt/ssd-510/osg/abapiti/npm-cache
export GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false
export ABAPITI_OSG_HOME="${OSG_HOME:?clean checkout matching .github/ci/osgo.ref}"
export ABAPITI_TESTS_TARGET=osg
export ABAPITI_TESTS_SHARDS=abap/lexer,abap/expressions,utils,pretty_printer,abap/types
# Start only at load <12. Do not also set ABAPITI_OSG_LOCK inside this flock.
flock /mnt/ram/abapiti/osg-heavy.lock \
  systemd-run --user --scope -p MemoryMax=48G nice -n 10 \
  /usr/bin/time -v node tools/abaplint-tests-all.mjs "${ABAPLINT_HOME:?}" result.json
# Repeat with ABAPITI_OSG_ONE_BUILD=0 for the old per-shard path.
# Optional snapshots / newer-pin gogen cache:
# export ABAPITI_OSG_CACHE=/mnt/ram/abapiti/osg-cache
# Default grouping is 3; reproduce the historical five-shard union explicitly:
# export ABAPITI_OSG_SHARDS_PER_BUILD=5
# To attempt all 17 at once (build crashes still split automatically):
# export ABAPITI_OSG_SHARDS_PER_BUILD=17
```

The historical before column used the bridge of `3da6606` with a nonce comment
appended to the generated Go so that `GOCACHE` could not serve the compile. Objects,
rules and syntax were not measured on this machine.
