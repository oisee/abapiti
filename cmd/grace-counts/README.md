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
