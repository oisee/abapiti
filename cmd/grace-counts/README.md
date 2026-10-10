# Singleton argument counts

`grace-counts` performs analysis only, before inlining, on the pinned 1,538-file
registry closure plus `RegistryRun`. It verifies source and npm-package hashes,
uses embedded reachability and fingerprinted registry overrides with assume-int,
and rejects blocking diagnostics or HIR verification errors. It runs Grace
`Analyze`; it never applies rewrites or generates backend code.

```sh
mkdir -p "$HOME/.cache/abapiti-go-cache" "$HOME/.cache/grace-tmp"
GOCACHE="$HOME/.cache/abapiti-go-cache" GOTMPDIR="$HOME/.cache/grace-tmp" \
  flock /tmp/abapiti-heavy.lock go run ./cmd/grace-counts \
  -output "$HOME/.cache/grace-counts"
```

Optional `-closure` (default `REGISTRY_CLOSURE`) uses an existing pinned closure
directory with its production tsconfig and npm inputs; the harness is refreshed.
Without it, sources, packages and tsconfig are materialized from embedded inputs
inside the output directory and removed after analysis.

Outputs: `sites.csv` (call/argument rows, receiver classes and corresponding
dispatch methods), `methods.csv` (each receiver method/parameter, yes/no and first
blocking HIR use, plus Grace escape/alias/flow evidence), `tables.txt` (also stdout).
Grace structural site paths distinguish calls on the same source line. Inherited
receiver implementations share method rows; receiver classes remain distinct in
each site row. Empty receiver sets and missing implementations never qualify.

S1 recognizes the frontend's empty array + exactly one push in the same block,
with source provenance confirming a nonempty nonspread array literal. It traces
declaration aliases using Grace binding identities. Intervening container uses,
additional pushes, and arbitrary assignments reject the singleton proof. Explicit
loop feedback `temp = callee(temp)` is `local-first` on its first iteration and
has a second `chained` row for subsequent iterations. Chained rows do not enter S1
counts or the coverage denominator. Other uncertain shapes are conservatively
excluded. Method/constructor calls are counted; adapter runtime operations have
no HIR parameter body to specialize and are excluded.

S2 allows only bare parameter reads directly as `ForEach.X`; unused parameters
qualify vacuously. Casts, aliases, indexing, length/other operations, stores,
returns, reassignment and forwarding calls block qualification. Lexical shadowing
is respected. Grace flow and escape facts include foreach element flow, so an
escaping element can mark its source parameter as escaping. Those coarse facts
are retained as evidence rather than overriding direct container-use checks.

S3 is `qualifies` for all resolved receivers, `partially` for some, and `none`
otherwise. Partial sites can use a default `m_one(e) = m([e])` for the rest. Hot
class marking follows profile #5: Expression, Sequence, Vers, Token, Word, Star,
Alternative, Optional, Plus, Permutation and WordSequence in statements/combi.ts.
Coverage is static sites in those caller classes, not execution-weighted savings.

The same run also reports R1/R2/R3 flow candidates and writes `flow-sites.csv`
(one row per ForEach, fresh-array producer or whole-container copy) and
`flow-methods.csv` (write/allocation/raising/divergence summaries). The stage
rows retain the singleton table's in-loop/outside/all/some/none shape; `some` is
zero because these guards have a single conservative verdict. R1 adds qualifying
value-row and object-reference columns; HIR has no separate native struct/record
row kind, so unresolved representations are blocked. Ref rows offer no kernel
copy saving. Combi callers retain per-class rows. Top-20 lists prioritize proven gains, reference-only qualifiers, then blocked
sites, sorting loop sites and stable HIR paths within each group. They include
first blockers rather than estimated execution frequency. Rankings
count qualifying static sites, counting only value rows for R1; ties are explicit.
A separate investigation order counts all screened sites without claiming savings.

R1 requires no transitive iterated-array/alias mutation and an unwritten,
non-escaping row. Unknown calls, any-write summaries and potentially aliasing
external writes block it; ensure_init writes inside the body are included.
R2 recognizes New arrays and the explicit fresh-container array-producing
runtime operations, requires no escape, one producer, exactly one fusable
consumer and no reads after that consumer or deeper-loop repeated consumption.
Declaration aliases/Seq yields are transparent and construction pushes are
allowed. Consumers are foreach, concat sources and join; current HIR lowers
spread/push-all to ordinary collection operations/loops and has no preserved
spread node. R3 screens complete array.slice0/set.copy operations, concat sources and
array.push value arguments. Reference sources require a last use, owned
non-escaping source, exactly one local reference binding and one syntactic
source read (excluding possible aliases in operand temporaries); reference appends
give no kernel value-copy gain. Primitive append sources require a declared
local with one definition and exactly one syntactic read at that last-use site,
excluding parameters and previous value copies. Primitives have value semantics,
so the coarse value-dependence escape relation is not an object-identity escape
proof for them. This says nothing about uniqueness of a runtime string's backing
storage. Bounded slices, arbitrary call arguments and emitter-level implicit
copies need separate contracts and are excluded. These are analysis
opportunities, not rewrite legality proofs or predicted seconds saved.

For reproducible production-pass timing only, set
`GRACE_COUNTS_MEASURE_INLINE=1`: after the same verified full-closure lowering,
the command times three existing `rewrite.Inline` runs on independent HIR copies
and skips all candidate analysis/output. It never mutates the lowered input.
This diagnostic mode separates loading/cloning/GC from the pass interval.

`flow-sites.csv` includes both row category and the exact HIR type string.
R3 site identities append `/copyN` to the runtime-op site to distinguish its
source operands; the source line still points to the copying operation.

`loop_depth` counts surrounding loops. An R1 ForEach at depth zero is a loop
outside any outer loop; an R1 in-loop row describes a nested loop. For R2/R3,
the same depth describes the producer/copy operation's surrounding loops.

`-typed-arrays` runs a separate analysis-only reference-array census. It scans
both the original lowering and a clone after the production ABAP emitter's
Singleton/Inline passes. The emitter and its settings are unchanged. Outputs
are `typed-array-{lowered,emitted}-{sites,types}.csv`, `typed-array-tables.txt`,
`names.json`, `profile-6-loops.csv`, the six profiled class sources, and the
shared object-array source. The profile IDs are resolved by their hash prefixes
in names.json; every hot-class HIR ForEach is cross-checked against its emitted
ABAP LOOP AT line and following cast, including string loops with zero casts.

The site CSV separates shared object tables from Optional reference tables,
which already have typed rows. Allocation sites are explicit New arrays and
fresh array-producing runtime operations (including slice/concat/splice,
map/set values/keys); reverse is an alias, not an allocation. Call sites returning
arrays are not counted as fresh allocations unless their runtime contract says
so. Reads are element-producing ForEach, IndexGet, get/pop/shift and the hidden
set.fromArray iteration. Index assignment is a write, not a read. Internal
indexOf/includes searches, reverse swaps, bulk table copying and runtime
implementation statements are not expanded into extra caller row-read sites;
they do not cast an object row to a declared class/interface. Loop depth counts
executing loops, so a ForEach read has depth at least one. Source expressions
retain surrounding depth; the source ForEach operation is included in its row's
depth. The report contains all element types, caller stages with combi per class,
and a top 20 sorted by profile class, removable cast, depth, and stable HIR path.
No execution counts or time savings are inferred from the profile class list.

The closed-world proof is a context-insensitive inclusion fixed point, distinct
from Grace's general value-dependence flow. It propagates array allocation
identities and row classes through lexical locals, assignments, casts,
conditionals, fields (merged by declaring owner), parameters, returns,
constructors, indexed stores, push/unshift/splice writes and slices/concat.
Class dispatch uses the nominal hierarchy; interface dispatch uses implementors
and compatible structural interface views. This overapproximates dispatch,
merges all invocations of a method and all instances' same declared field, and
is not a path-sensitive reachability proof. Unknown calls/native producers and
unknown mutations contribute explicit unknown rows. A missing non-null row
solution is reported as unresolved, never as an exact-type proof. Allocation
sites aggregate all their dynamic instances; an exact row class means every
resolved non-null row has that class, with no unknown producer in its flow.
Interface rows report implementing classes, rather than treating an interface
as an instantiated class. Coverage-trapped bodies and external inputs have the
same scope as production LowerRegistry; this is a proof about that lowered
program, not every possible future TypeScript workload.

`casts_removed_same_view` is potential saving if the read's declared element
view becomes the storage type. A narrower row type retains its cast. The
`casts_without_cross_view` column additionally requires resolved container
identities and a single element view across their aliases; it is a conservative
screen for covariance concerns, not a complete representation-change legality
proof. A subtype row still assigns directly to a typed base/interface reference,
so subtype contents alone do not require the current object-to-view cast.
Different element views must preserve shared array identity and writes.

The line estimate copies the measured shared array class's current full method
surface per nominal element type, retaining a generic fallback. The existing
builtin.object class is subtracted from added objects. It excludes wrapper or
bridge code, metadata, optional variants and build costs, and is not a minimal
specialized runtime design. "Hot pipeline stages" includes lexer, statements,
structures, syntax and rules; the six profile #6 classes are reported separately.
