# Synthetic reference return bridges

Base: `40f90dd`, branched at `a0bee5c` with no-ff merges of `0de990c`,
`26bcfb6`, `29f352e`, and `569f496`, in that order. Merge repairs retain both
backend hooks/catalogues and both qualified-cast regression domains. The merged
Grace catalogue has one SHA-1 entry; its lexer golden records the union of the
catalogues. Historical measurements from both sides are retained separately.

The synthetic `buildEdits` forwarder returned a virtual call typed as
`optional<classref<builtin.object>>` into the narrower structural WorkspaceEdit
slot. The implementation body verified; the generated return did not. Nine
renamer classes shared this root: RenameGlobalInterface, RenameGlobalClass,
RenameDataElement, RenameDomain, RenameICFService, RenameMessageClass,
RenameProgram, RenameTable, and RenameTableType.

`completeReturnBridges` runs after the union ABI fixpoint and before async
bridge completion. It follows target chains, binds the final method name and
result, and inserts a checked reference narrowing when erasure prevents direct
assignment to the return slot. Ordinary assignable covariance retains its
existing HIR. An unrelated return remains a verifier mismatch. No new cast
relation, object snapshot, backend operation, external declaration override, or
void ABI is introduced. Optional reference absence and instance identity retain
the existing representations in Go and ABAP.

The fixpoint stamp includes class method signatures, so changes to a result or
parameter without adding a method are no longer invisible to convergence.
The oracles cover undefined and present WorkspaceEdit instances, optional
fields, return identity, live edit-array mutations through concrete and
ObjectRenamer calls, once-only effects, incompatible erased-instance refusal,
covariant interface/union dispatch, late target renaming and result widening,
bridge chains, idempotence, ABAP emission, and Grace equivalence. The erased
physical return fixture is injected independently of the external LSP package;
ordinary TS fixtures are also executed by original Node.

## Reproduction

All build/test commands use Go on PATH and package parallelism at most eight.
Pinned upstream is `577f875ebec44cfaf64841cfe71c8ab8dc32622e` with its own
root and core lockfiles installed. Generated sources, binaries, and raw shard
artifacts live under `/tmp/return-bridges`, outside committed files.

```sh
GOCACHE=/tmp/go-cache go build -p 8 -o /tmp/return-bridges/base ./cmd/abapiti
# Build the branch executable in its checkout with the same command and /after.
/tmp/return-bridges/base abaplint --full -o /tmp/return-bridges/full-base
/tmp/return-bridges/after abaplint --full -o /tmp/return-bridges/full-final
/tmp/return-bridges/base abaplint -o /tmp/return-bridges/default-base --target all,go
/tmp/return-bridges/after abaplint -o /tmp/return-bridges/default-final --target all,go
diff -r /tmp/return-bridges/default-base /tmp/return-bridges/default-final
go vet -p 8 ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -p 8 -timeout 30m ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 go test -p 8 ./tsfront ./hir/abap \
  -run 'TestLexerFactsReport|TestRuntimeOpsSupportedByEveryBackend' -count=1
./.github/ci/lint.sh gate 40f90dd
```

`--full` is present in origin/main and uses no reachability manifest. Blocking
message histograms and HIR message counts are kept as distinct measurements;
removing a synthetic HIR error does not claim removal of a source diagnostic.

The imported Chai oracles now locate the checkout via
`ABAPITI_TESTS_UPSTREAM` (default `~/dev/abaplint`), checking both root and core
node_modules. This removes the imported absolute home paths without changing
assertion semantics. The independent oracle command is:

```sh
ABAPITI_TESTS_UPSTREAM=/tmp/return-bridges/upstream go test -p 8 \
  ./tsfront ./hir/golang \
  -run 'TestChai(SmallNodeOracle|DeepNodeOracle|ThrowNodeOracle|DeepRuntimeNodeOracle)' \
  -count=1
```

The short gate's first attempt reached the default ten-minute package timeout
while running LSP fixture emission, with no preceding assertion mismatch. The
retry uses the same packages and `-p 8`, a thirty-minute package timeout, and the
pinned upstream environment above.

Default source output from separate directories agrees. The generated Go
binary and runner READMEs include their destination path, so the raw-byte gate
uses the **same destination** for both generation commands, preserving the
branch output in a snapshot before rebuilding the merged base there:

```sh
cp -a /tmp/return-bridges/default-final /tmp/return-bridges/default-final-snapshot
rm -rf /tmp/return-bridges/default-final
/tmp/return-bridges/base abaplint -o /tmp/return-bridges/default-final --target all,go
diff -r /tmp/return-bridges/default-final-snapshot /tmp/return-bridges/default-final
```

No output content is normalized or excluded from that comparison.

The shard commands use `tools/abaplint-tests-all.mjs` from the merged base with
`ABAPITI_TESTS_COMPILER` and `ABAPITI_TESTS_COMPILER_COMMIT` set to each built
executable and revision. Each `ABAPITI_TESTS_SHARDS` selection retains the
harness's original Node oracle, ownership subtraction, and immediate FAIL
check. A disposable runner copy changes only environment/cache handling,
package parallelism from two to eight, and successful scratch retention. It
makes no test-source, shim, title, or status changes. Up to eight baseline and four after-run shard workers select directories
independently, capped at twelve combined after observed compiler memory pressure. Their retained raw records
are combined with the exhaustive discovery manifest and a fresh whole-suite
Node oracle, then checked by `abaplint-tests-summary.mjs`.

The original grouped workers' completed `root`, `abap`, and `abap/expressions`
records are retained; their unused later work is stopped when the first report
exists. Remaining shards execute in individual workers. Aborted orchestration
attempts and any incomplete shard are excluded from the final aggregate.

An individual shard command is:

```sh
ABAPITI_TESTS_SHARDS=objects/rename \
ABAPITI_TESTS_COMPILER=/tmp/return-bridges/after \
ABAPITI_TESTS_COMPILER_COMMIT=a3ec015 \
ABAPITI_TESTS_SCRATCH=/tmp/return-bridges/runs-after/shard-objects-rename \
node tools/abaplint-tests-all.mjs /tmp/return-bridges/upstream \
  /tmp/return-bridges/runs-after/shard-objects-rename.json
```

The baseline executable predates the final test-only merge amendment: `d6564ae`
and the final merged base `40f90dd` differ only in the qualified-cast regression
file. Their compiler/runtime trees are identical. The final feature executable
is `a3ec015`; the subsequent portability commit changes only tests and a
historical documentation command.

## Full-closure and default-output results

| Measure | Merged base | Return-bridge fix |
| --- | ---: | ---: |
| Source blocking diagnostics | 48 | 48 |
| Root / cascade source diagnostics | 35 / 13 | 35 / 13 |
| HIR verification errors | 117 | 108 |
| Synthetic node-zero return assignment mismatches | 9 | 0 |
| All assignment mismatches | 15 | 6 |

Exact message counts and the nine attributed bridge roots are in
[evidence/return-bridges-gap.json](evidence/return-bridges-gap.json).
The source-blocker histogram is unchanged. This is a HIR return-boundary repair,
not nine source blockers removed or nine workloads estimated as passing.

The same-destination raw `diff -r` is empty, including generated Go binary,
ABAP targets, packages, source maps, and runner READMEs.

Commit author/committer headers were corrected to preserve the requested
`Alice V.` exactly. This changes commit IDs only; each corresponding file tree
is byte-identical. The original measured feature revision `7550127` corresponds
to corrected feature revision `a3ec015`, and merged base `13f9e58` corresponds to
`40f90dd`. Runtime/compiler files were not changed by this metadata correction.
