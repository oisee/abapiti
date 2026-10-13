# TS-HA@OSGO harness: speed and memory

OSG pin `368baab28`, Go 1.26.0, Node 26, 16-core host shared with other
sessions. Each run held the shared OSG flock inside a
`systemd-run --user --scope -p MemoryMax=48G` scope at `nice 10`, one heavy
step at a time. Times are wall-clock inside the lock (lock waiting is
excluded). Peak RSS is the largest single process in the tree (GNU time),
which is always either the Go compiler or the gogen frontend.

## Before / after

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

## Correctness gate

For lexer, root and statements the full multiset of `(title, status)` is
identical between the original harness and every after run (cold, rerun,
frontend-cache hit with a rebuilt binary, optimised build), and so are the
per-title messages. Lexer 200/200 pass; root 8 pass + 86 traps; statements
429 pass + 4,533 traps; FAIL 0 everywhere.

## What changed

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
  `cacheDir` (stoker's per-object cache, not in `368baab28`), the bridge
  passes `ABAPITI_OSG_CACHE/gogen-frontend`. Feature-detected; older pins are
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

## Not done: one shared abaplint core package

Building the shared core once needs gogen's layered emission (core / app /
unit packages, as `unit.mjs` does for OSG's own classes). `unit.mjs` turns
layering off for fixtures, and the core closure differs per shard, so a
shared package would have to be the union closure of all shards and emitted
byte-identically for `GOCACHE` to reuse it. That is a gogen change, not a
harness setting; the parse/type-check floor (~30 s for lexer, single-threaded)
is the part it would remove. The remaining fixed cost of an unchanged rerun
is abapiti's lowering (80–100 s), which could be cached separately.

## Reproduction

```sh
mkdir -p /mnt/ram/abapiti/tmp; touch /mnt/ram/abapiti/osg-heavy.lock
export GOCACHE=/mnt/ssd-510/osg/go-build TMPDIR=/mnt/ram/abapiti/tmp
export TRANSPILER=/mnt/ssd-510/osg/abapiti/transpiler-ee5c2cd1
export npm_config_cache=/mnt/ssd-510/osg/abapiti/npm-cache
export GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false
export ABAPITI_OSG_HOME=/mnt/ssd-510/osg/abapiti/open-steamgate
export ABAPITI_OSG_CACHE=/mnt/ram/abapiti/osg-cache
export ABAPITI_OSG_LOCK=/mnt/ram/abapiti/osg-heavy.lock ABAPITI_OSG_MEMORY_MAX=48G
ABAPITI_TESTS_TARGET=osg ABAPITI_TESTS_SHARDS=abap/lexer,root,abap/statements \
  node tools/abaplint-tests-all.mjs ../abaplint result.json
```

The before column used the bridge of `3da6606` with a nonce comment appended
to the generated Go so that `GOCACHE` could not serve the compile. Objects,
rules and syntax were not measured on this machine.
