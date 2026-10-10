# stmt-patterns

Deterministic, read-only measurement of the pinned abaplint ABAP output. It
resolves emitted class/member identities through `names.json`, counts actual
TypeScript AST statements and computations, measures lexical ABAP statements,
and records candidate folding sites with source lines and loop depth.

From the repository root:

```sh
mkdir -p .local/stmt-patterns/source
export TMPDIR="$PWD/.local/stmt-patterns"
export GOCACHE="$HOME/.cache/abapiti-go-cache"
export GOFLAGS=-buildvcs=false
tar -xzf tsfront/testdata/registrycorpus/abaplint-core-577f875e.tar.gz \
  -C .local/stmt-patterns/source
go build -o .local/stmt-patterns/abapiti ./cmd/abapiti
.local/stmt-patterns/abapiti abaplint -o out
go run ./cmd/stmt-patterns -hir
```

`-input` changes the emitted directory; `-source` points at the TypeScript core
root containing `src/`; `-output` selects the report directory; `-work` selects
workspace scratch for `-hir`. The complete closure's source fingerprints are
verified. With `-hir`, the tool runs the existing backend on a fresh lowering,
requires byte-for-byte equality of the selected classes with the input, and
counts the HIR after existing singleton/inlining transforms. It writes a small
HIR dump for the combinator run methods and Result.peek/remainingLength.

Outputs: `methods.csv`, `mix.csv`, `patterns.csv`, `occurrences.csv`,
`savings.csv`, `ngrams.csv`, `tables.md`, `manifest.json`, and (with `-hir`)
`hir-hot.txt`. Output order and ties are sorted; there are no timestamps or
absolute paths in report artifacts. The tool prints method/pattern tables and
the top 15 normalized two/three-statement n-grams. The interpretive report and
ranking are in `docs/history/2026-10-10-stmt-patterns/README.md`.

ABAP counts are lexical sites including declarations and control terminators,
not executed counts. The executable-site proxy excludes plain declarations and
control markers. Synthesized `_one` methods use their original method's TS body
as an explicitly labeled comparison denominator. Coverage traps remain visible
and are labeled unexecuted; generated interface forwarding wrappers are excluded.

All folding counts are candidates, not proofs that rewriting is legal. Def-use
counts ignore literal text and include string-template interpolations. Adjacent
assignment/argument candidates require a single subsequent token use; broader
CAST candidates can require movement and therefore need effect-order checks.
Initialization candidates stop at branches/loops/reads/explicit aliases.
N-grams can cross control boundaries and intentionally collapse temp/name
identities: `TMP = TMP` does not mean a self-assignment.

The per-call envelope `a + b*I` is a scenario where each outside-loop site runs
once and each loop site runs I times. It is branch-dependent; nested-loop visits
belong in I. Actual savings are the sum of each site's visits per call. Pattern
families overlap and must not be added. No profile call counts are invented.

Tests cover statement boundaries, literal/comment handling, nested loops,
single-use rejection, branch/read barriers, boolean forwarding, overwritten
CLEAR, normalized identities, and template-expression uses.

For the full-corpus, analysis-only peephole census (no TS checkout needed):

```sh
export TMPDIR="$PWD/.local/peephole/tmp"
export GOCACHE="$HOME/.cache/abapiti-go-cache"
export GOFLAGS=-buildvcs=false
mkdir -p "$TMPDIR"
go run ./cmd/stmt-patterns -peephole -input out -output .local/peephole/export
python3 tools/peephole-mine.py
```

The export contains all emitted class methods, concrete and normalized lexical
statements, original ABAP lines, loop depth, and names.json identities. Mining
writes the complete 2..4-window census, candidate ranking, occurrences, manifest,
top-20 report and pending ledger under docs/history. Counts are opportunities,
not proofs. The original selected-method analysis remains the default mode.
