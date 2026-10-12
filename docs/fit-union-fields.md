# Shared union field reads

Base: `origin/integ/train-8` at `4b6a465c5663360a5f08cb249ef7cb936dfe36df`.
Branch: `feat/fit-union-fields`. Semantic commit:
`e71aa824e7eceb40fc75539646a5e8f48f7e5f6b`.
The J notes at `dda5a14:docs/traps-j-roots.md` identify the edit union's
`range` access at `renamer.ts:135`. Train 8 reaches **29** tests at this
same root, rather than the older notes' 23.

The TS→HIR frontend proves that every constituent of the checker's **flow
union** owns the requested public data field. A compatible field type must
accept every constituent's slot type; candidate selection is independent of
union ordering. Structural data unions use the existing live data-class view,
retaining original storage, identity, extra properties and nested aliases.
Native class unions use a synthesized getter interface: each constituent reads
its real field through a forwarder. There are no source-specific field tables,
source overrides, fake constituent instances, new runtime operations, or
backend changes.

Missing fields explicitly require narrowing. Incompatible fields, mixed native
and structural storage, and writes through a union field view also refuse.
Container element types remain invariant; this change does not widen mutable
row or array storage. Fresh object-literal unions retain their existing collection-row proof and
physical slot layout. The reachability-pruned default path retains its
original behavior.

Node/Go differential regressions cover both storage paths, single receiver
evaluation, required and optional primitives, compatible nested structural
layouts, nested identity and mutation through a declared shared type, and
successful nominal narrowing. Negative regressions preserve explicit refusals
for missing fields, incompatible types and union writes. Both shared read paths
also pass HIR verification and ABAP emission. No upstream source, tests,
registrations, exclusions, LSP type overrides or runtime goldens are changed.

Measurements and gates are recorded in
[evidence/fit-union-fields.json](evidence/fit-union-fields.json). Only the
`objects/rename` and `lsp` test shards are measured; these results are not a
whole-suite claim. Full-closure evidence uses the existing `abaplint --full`
command, also present in `origin/main`, rather than a reachability manifest.

The first broad short gate found two existing registration regressions: a
computed-key initializer became executable without its row proof, and a
normalized optional row slot was replaced with checker-constituent types.
The repair confines shared union views to declared layouts and leaves fresh
literal rows on the original collection path. Both original regression tests
and every union-field oracle pass without changing their assertions or fixtures.

| Selected shard | Passed before → after | Traps before → after | FAIL before → after |
| --- | ---: | ---: | ---: |
| `objects/rename` | 0 → 29 | 31 → 2 | 0 → 0 |
| `lsp` | 178 → 178 | 10 → 10 | 0 → 0 |
| Total | 178 → 207 | 41 → 12 | 0 → 0 |

The selected original pinned Node oracle has 219 active tests and five pending;
Go registrations, missing-test accounting and title ownership reconcile with
zero unmatched executions or initialization failures. Five tests remain not-run
with unchanged skipped/focused accounting. The recovered **29** tests all leave
the `renamer.ts` common `range` root; no runtime failures are reclassified.

Full closure: **29 → 29 blocking lowering diagnostics** (19 root, 10 cascade),
with an identical message histogram. **77 → 70 HIR errors**: exactly six
`unresolved field range` and one `unresolved field newText` disappear, with every
other verifier message count unchanged. Combined findings are 106 → 99; this
sum is a findings count, not a count of unique semantic roots. Exact commands
and both message histograms are in the JSON.

Remaining selected traps are outside the shared-field scope: two DDIC native
conversion refusals in `RenamerHelper.renameDDICTTYPReferences`; three LSP rename
factory argument mismatches for nullable version storage; two CreateFile/DeleteFile
factory-test refusals; and one each in class-data conversion, help conversion,
semantic class-value narrowing, dump-scope ToPrimitive, and help delete/stringify.
The new view does not widen those storage or builtin boundaries. The full
closure's other 70 HIR errors and 29 lowering diagnostics also remain unresolved.

Final gates on the frozen semantic commit:

- `go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...`: pass, with `-p=2`
  through GOFLAGS and `GOMAXPROCS=2`.
- `go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/... -timeout 40m`:
  pass, including `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`.
- `ABAPITI_GRACECHECK=1` lexer facts/Grace check: pass; regenerated facts compare
  byte-identically with the existing golden. No golden update.
- `./.github/ci/lint.sh gate origin/integ/train-8`: zero new issues, canary 5/5,
  pinned golangci-lint v2.13.2.
- Base and branch each run `abapiti abaplint -o "$scratch/default" --target all,go`
  at the same absolute output path. Raw `diff -r` is empty across **6,321 files**,
  including the Go executable and ABAP packages. Nothing is normalized.

Go gates/compiler use Go 1.26.9; the unmodified shard runner pins Go 1.26.0.
Node is v26.9.0. Final shards use one worker to reduce memory pressure; every
Go build/test uses at most `-p 2`. No PR or push; local branch commits only.
