# CDS, DDIC and Files differential

`fixtures.json` covers CDS expressions, SQL view names, entities, missing/empty/
broken sources, metadata-only DDLS, ordered mixed-case table components/keys,
VIEW aliases whose keys retain source field names, invalid dictionary types,
and missing/case-sensitive object lookups. `reference.jsonl` comes from original
published `@abaplint/core@2.120.73`, using `tools/library-data-reference.mjs`.

`stub.go.txt` and `unit_test.go.txt` compile with the real emitted Go runtime in
`TestRegistryGoLibrary`, including `-race`. They also exercise defined empty SQL
view names and empty StructureTypes: those are not constructible through the
pinned parser (the SQL annotation regex requires a nonempty name; StructureType
rejects zero components). A non-StructureType must return nil components; a
StructureType with zero components must return a non-nil empty slice. Tests check
TokenNode filtering, bounds, parse-before-lookup even for missing objects, TABL
versus VIEW key signatures, order/case, copied results and concurrent calls.

`api_test.go.txt` runs against a freshly emitted complete library through
`TestRegistryGoLibraryData`, enabled by `ABAPITI_LIBRARY_DATA_DIR`. It checks the
original Node fixture snapshots, pending edits, removal/recreation behavior,
metadata-only objects, lazy parse statistics and concurrent Checkers.

## Exact PR #839 scenarios

`scenarios.json` extracts all **41** mutations from open-steamgate PR #839 at
`bd29576752eb459dc56b8bf8cf845a8162d10405`:
`tools/step3/check-outline-diff.mjs`, SHA-256
`5431a72a665deaf8fe1bb8e0a0e3c5575097fb897acaeaf260f5fe3b9265a8cf`.
The extraction runs without the ADT/server/build dependencies:

```sh
node tools/library-data-scenarios.mjs OWN_PR839_CLONE > scenarios.json
cmp scenarios.json tsfront/testdata/library-data/scenarios.json
```

The inputs are the six class/include states, three inline overlays with explicit
restoration, 24 INTF/PROG/INCL/SRVD/DDLS variants, and eight dictionary/CDS casts.
Snapshots preserve the exact source strings and dictionary XML from the PR.
The standalone registry starts empty and removes prior shape files on transitions;
each state queries its target plus both dictionary objects for dictionary cases.
It yields 64 snapshots including missing-object baseline and restoration checks.
This exercises the data API, not OSG's CHECK/OUTLINE diagnostic policy.

## Three-way comparison

Emit a library from the base with `--target go --go-package abaplint`. In an own
scratch OSG clone fetched at PR #839, run the **unmodified** installer:

```sh
node OWN_PR839_CLONE/tools/step3/cds-inspect.mjs BASE_OUTPUT/go
```

The installer enforces the exact base `library.go` hash. Emit another library
from the branch with the same package name. For each consumer module, copy
`dump.go.txt` to `main.go` and either `adapter.go.txt` or `api.go.txt` to `api.go`.
Require `zabaplint v0.0.0` and replace it with the appropriate emitted `go`
directory. Build the consumers as `adapter-data` and `api-data`.

**All Go builds/tests must use the task's 16 GiB systemd scope and scratch
TMPDIR/GOCACHE.** For large full-core consumer builds, use `-p=1 -gcflags=-N`.
Keep at most two heavy commands active and enforce the task's load/disk limits.

Create `m1.json` with the frozen input validation in
`tools/library-tree-manifest.mjs`. The drivers accept this manifest and the
committed scenario/fixture manifests. For each manifest:

```sh
adapter-data MANIFEST > adapter.jsonl
api-data MANIFEST --legacy > api-legacy.jsonl
cmp adapter.jsonl api-legacy.jsonl
api-data MANIFEST > api.jsonl
node tools/library-data-reference.mjs ORIGINAL_CORE_DIR MANIFEST > node.jsonl
cmp api.jsonl node.jsonl
```

The exact adapter has no positions and collapses undefined SQL view names to
`""`. `--legacy` projects only those differences; the full API dump independently
compares every position and nil value against original Node. No component/key
sorting or case normalization occurs. Object query ordering alone is sorted for
M1. Files retain object order and raw source bytes. Go JSON escaping is matched
by the Node oracle.

M1 has 259 TABL and 36 VIEW objects and **no DDLS**. The scenarios provide 16
DDLS snapshots and 249 expression nodes; fixtures add entity and SQL-view CDS,
malformed/empty/metadata-only sources, and mixed-case DDIC coverage.
