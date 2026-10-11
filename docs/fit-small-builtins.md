# Small fit-gap builtins

Implemented lowering uses existing structured HIR wherever possible: depth-one
`flat` is fresh-array iteration/push, updates capture a reference once and use
ordinary assignment, labels propagate completion flags between nested loops,
and non-nullish `??` retains only the left operand. Both emitters therefore use
those existing HIR constructs. `trimStart`, `trimEnd`, and `map.delete` have new
verified runtime signatures and effect summaries.

`toLocaleLowerCase()` / `toLocaleUpperCase()` intentionally use the existing,
locale-independent Unicode lower/upper mapping. Explicit locale arguments are
refused, not silently ignored. This matches core's no-argument uses in practice.
Trimming uses ECMAScript whitespace, including BOM, rather than Go whitespace.
Map deletion returns whether the key existed; record property deletion keeps
its existing always-true JavaScript semantics.

Supported `flat` depth is one, omitted or literal `1`, on dense homogeneous
arrays (including arrays of references); heterogeneous/presence-tagged outer
elements are refused. `in` supports index-signature records and literal
properties on lowered fixed shapes, including inherited members. It is not
implemented as Map-entry membership for actual JavaScript Maps. Optional shape
properties need explicit property-presence metadata and are refused.

## Validation

Environment: `GOCACHE=$HOME/.cache/go-build GOTOOLCHAIN=go1.26.0
GOFLAGS=-buildvcs=false`.

- `go test ./tsfront/... ./hir/... ./cmd/...` passes.
- `go vet ./...` passes.
- `TestSmallBuiltinsNodeOracle` lowers/verifies each feature, emits ABAP, and
  executes generated Go against Node, including nested labeled jumps, updates
  with effectful receivers, falsy `??`, zero-valued record membership, and
  fresh flat arrays with shared object elements.
- `TestEmitSyntaxFeatures` now emits ABAP Unit oracle observations covering all
  implemented features; `node tools/syntax-features-oracle.mjs` regenerated the
  five expected cases. ABAP runtime execution was not performed locally.
- HIR signature, ABAP one-sided trim/deletion emission, and Go UTF-16/whitespace
  and ordered-deletion edge tests pass.
- Default `abaplint -o X --target a4h --quiet`: `diff -r` is empty against the
  before-change build at `48070b3fffa6f7c478b3d1379d36ef45534a9d40`, which is
  also `origin/main`. No changed generated files.

Unpruned lowering used a temporary, **uncommitted and removed** environment
hook setting coverage to nil, with `--assume-int=false --evidence`. The default
integer rewrite already panics at `tsfront/assume_integer.go:298` when run
unpruned before these changes. No reachability pruning was used for these
counts. This checkout's measured baseline is 644 rather than the approximate
625–635 in the task; after is 632. HIR errors change from 392 to 391.

| Reported root blocker | Before | After |
| --- | ---: | ---: |
| trimEnd | 2 | 0 |
| trimStart (not used in core) | 0 | 0 |
| toLocaleLowerCase / toLocaleUpperCase | 3 | 0 |
| flat | 1 | 0 |
| Map.delete | 1 | 0 |
| prefix ++ | 1 | 0 |
| labeled statement | 1 | 0 |
| non-optional ?? | 2 | 0 |
| in | 1 | 1 |
| purported Map.is | 2 | 2 |

The `in` site at `objects/rename/renamer.ts:132` receives
`classref<builtin.object>`, not a record or a lowered shape: imported LSP edit
union values have no runtime property metadata. It remains an explicit
refusal, with the receiver type in the diagnostic. Implemented record/fixed
shape cases are covered by Node comparisons.

The two `.is` calls at lines 104/107 are **not Map methods**. They are imported
`vscode-languageserver-types` namespace guards. Reading that package's JS shows
`TextDocumentEdit.is` validates a versioned text-document identifier and an
edits array; `RenameFile.is` checks the rename discriminant, URIs, options and
annotation. The materialized closure contains their `.d.ts` namespace
declarations, not their executable bodies. Generic Map lowering cannot
implement those guards soundly; declaration/namespace dependency lowering is
still required. The unrelated `downport.ts:741` cascading `candidates`
diagnostic also disappears, accounting for the total reduction of 12.
