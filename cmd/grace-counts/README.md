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

Value-object screening is a separate analysis mode:

```sh
GOCACHE="$HOME/.cache/abapiti-go-cache" GOFLAGS=-buildvcs=false \
GOTMPDIR="$HOME/.cache/grace-tmp" TMPDIR="$HOME/.cache/grace-tmp" \
  go run ./cmd/grace-counts -value-objects -output "$HOME/.cache/value-objects"
```

It uses the same verified 1,538-file closure and Grace flow/write summaries.
The separate `value_objects.grace` rules do not participate in optimization.
Outputs are `value-objects.csv` (every non-module class),
`value-allocations.csv` (each static class New with caller stage, including combi
class, provenance and loop depth), `value-facts.json` (first witnesses per blocker
category and nullable type, every allocation), `value-names.json` (stable class
identities from the same `hir.Names` algorithm as ABAP `names.json`), and
`value-tables.txt` (requested candidates and any qualifying parser opportunities).
No ABAP or other backend is invoked. Generated record classes are included in
CSV/facts so nested object allocations are visible.

`fail:unproven` is a conservative proof failure, not a witnessed semantic
violation. The `cN_first_definite` columns distinguish witnessed blockers from
potential dynamic/supertype uses and unresolved effects. Every such uncertainty
prevents qualification. C4 is informational: an Optional reference requires an
explicit absent flag and does not disqualify a representation supporting it.
C5's declared heuristic is at most 64 inline bytes and four reference slots;
these are handle/descriptor estimates excluding referenced heap payloads and
are not measured ABAP row layouts. Strings use a 16-byte descriptor estimate;
object/container fields remain references and their allocations are not removed.

Ranking counts static sites in lexer, statements and structures. `parser_hot_sites`
is the conditional opportunity if a class qualified; `qualifying_hot_sites_removed`
is zero for every rejected/uncertain class. Ties in the latter are ordered by
conditional opportunity, then class identity. Neither column is an execution
count or a prediction of the reported 22–32 seconds. Dynamic constructor
operations are reported separately as `dynamic_new_potential` and are excluded
from savings because the concrete class is not a static `new C` proof.

The opt-in full reference gate compares these rules against the independent
reference evaluator on full-closure facts:

```sh
GOCACHE="$HOME/.cache/abapiti-go-cache" GOFLAGS=-buildvcs=false \
GOTMPDIR="$HOME/.cache/grace-tmp" TMPDIR="$HOME/.cache/grace-tmp" \
ABAPITI_VALUE_OBJECTS_FULL=1 go test ./cmd/grace-counts \
  -run '^TestValueObjectsFullClosure$' -count=1 -v -timeout=30m
```
