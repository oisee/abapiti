# LSP lookup, renamer guards, and downport receiver recovery

Branch `feat/fit-lsp-mix` starts at `origin/integ/yolo-5`,
`392aecaa1967cdaacec393a061154aadaafc9286`. The integration report records
10,255/11,060 passes and FAIL 0. This change measures only `lsp`,
`objects/rename`, and `rules`; the whole suite must be measured at train assembly.

Structural literal constructors retain a separate proof of property presence
and insertion order in HIR. Go activates that storage when own-key enumeration
is used, preserves initializer evaluation order, and updates the proof on native
and live-view writes. Enumeration sorts canonical array indices before other
keys, retains explicit undefined properties, and snapshots keys. Lookup's JSON
serialization reads current values through the same proof. Unknown spread,
duplicate-initializer, and prototype-setter layouts retain an explicit refusal.

The resolved `vscode-languageserver-types` namespaces lower `TextDocumentEdit.is`
and `RenameFile.is` to reviewed HIR operations, including aliases, nested native
records, arrays, nullable document versions, and validation of rename options.
Their contracts follow the pinned 3.18.0 implementation. Its integer predicate
checks a numeric range and admits fractions. Its rename predicate returns the
falsy candidate unchanged: conditions consume truthiness, while non-boolean
falsy results in value contexts explicitly refuse. ABAP reports the guard
operations as unsupported; own-key enumeration and generic JSON serialization
also retain explicit ABAP refusals.

Calls through a private producer annotated `any` recover a nominal receiver
only when every non-undefined return has the same checked reference type.
Nested functions are excluded. Private methods annotated `any` preserve implicit
fallthrough as a boxed undefined return, using the verifier's termination proof.
The producer is evaluated once, absent returns
are preserved, and the nominal conversion checks the runtime value. Public
virtual producers and unproven return domains retain their existing refusal.
This fixes `findConstructor(...)?.getParameters()` without a downport-specific
name match or override.

The final selected-shard report has FAIL 0 throughout:

| Shard | Pass before | Pass after | Trap before | Trap after | FAIL |
| --- | ---: | ---: | ---: | ---: | ---: |
| lsp | 97 | 129 | 91 | 59 | 0 |
| objects/rename | 0 | 0 | 31 | 31 | 0 |
| rules | 2,752 | 2,768 | 395 | 379 | 0 |
| Total | 2,849 | 2,897 | 517 | 469 | 0 |

The 32 lookup and 16 downport cases become passes. The original combined
renamer guard diagnostic disappears from all 20 cases, which now explicitly
trap at `in requires a record or a literal property on a lowered shape
(got classref<builtin.object>)`. The `in` operator is excluded from this task.
The other remaining traps are outside the assigned roots, including regexp,
callback lowering, numeric-domain refusals, and interface call mismatches.
The 17 not-run cases are unchanged: 15 Node pending tests and two unregistered
tests. No whole-suite result is inferred from this partial run.

The reports are [before](abaplint-tests-lsp-mix-base.json) and
[after](abaplint-tests-lsp-mix-after.json). The exact original messages, each
absent after this change, are:

- 32: `src/lsp/_lookup.ts: own-key enumeration requires original structural property storage`
- 20: `unsupported-lsp-call src/objects/rename/renamer.ts: vscode-languageserver-types TextDocumentEdit.is with 1 arguments is not lowered; unsupported-lsp-call src/objects/rename/renamer.ts: vscode-languageserver-types RenameFile.is with 1 arguments is not lowered`
- 16: `unsupported-call src/rules/downport.ts: method getParameters on dynamic is not lowered; unsupported-expr src/rules/downport.ts: identifier importing is not a lowered local or module value; unsupported-expr src/rules/downport.ts: identifier importing is not a lowered local or module value`

Measurements and gates use Go 1.26.0, Node 26.9.0, `GOMAXPROCS=2`, and
`GOFLAGS=-p=2`. The original upstream checkout is pinned at
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`, with its lockfile dependencies.
The base compiler was built before edits on the clean integration base.

```sh
work="${TMPDIR:-/tmp}"
export GOMAXPROCS=2 GOFLAGS='-p=2'
# Before: compiler captured from the clean 392aeca checkout.
go build -o "$work/fit-lsp-base" ./cmd/abapiti
ABAPITI_TESTS_SHARDS=lsp,objects/rename,rules \
  ABAPITI_TESTS_COMPILER="$work/fit-lsp-base" \
  ABAPITI_TESTS_COMPILER_COMMIT=392aeca \
  node tools/abaplint-tests-all.mjs "$work/fit-lsp-upstream" \
  "$work/fit-lsp-shards-base.json"
# After: compiler built from this branch's implementation.
go build -o "$work/fit-lsp-final-reviewed" ./cmd/abapiti
ABAPITI_TESTS_SHARDS=lsp,objects/rename,rules \
  ABAPITI_TESTS_COMPILER="$work/fit-lsp-final-reviewed" \
  ABAPITI_TESTS_COMPILER_COMMIT=392aeca \
  node tools/abaplint-tests-all.mjs "$work/fit-lsp-upstream" \
  "$work/fit-lsp-shards-reviewed.json"
```

The after measurement was made with an uncommitted implementation. The runner's
compiler revision field names its base; it must not be mistaken for a clean
392aeca compiler. The final three-shard measurement includes implicit undefined
returns, the conservative prototype-setter refusal, and single evaluation of
native write receivers. The commands above refer to that final build. Full-closure measurement depends only on HIR
lowering; the measured `fit-lsp-implicit-final` build has the final TS→HIR
sources, and the later backend-only receiver temporary does not change counts.

```sh
go build -o "$work/fit-lsp-implicit-final" ./cmd/abapiti
"$work/fit-lsp-base" abaplint --full -o "$work/fit-lsp-full-base"
"$work/fit-lsp-implicit-final" abaplint --full -o "$work/fit-lsp-full-implicit"
go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 \
  go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
./.github/ci/lint.sh gate origin/integ/yolo-5
```

The short gate includes `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`.
For the default artifact comparison, both compilers wrote to the same output
path, preserving README text and Go binary build paths. The base output was
moved aside before running the final compiler. The raw recursive diff is empty;
no files were filtered or normalized.

```sh
"$work/fit-lsp-base" abaplint -o "$work/fit-lsp-default-comparison" --target all,go
mv "$work/fit-lsp-default-comparison" "$work/fit-lsp-compare-base"
"$work/fit-lsp-final-reviewed" abaplint -o "$work/fit-lsp-default-comparison" --target all,go
diff -r "$work/fit-lsp-compare-base" "$work/fit-lsp-default-comparison"
```

The final full-closure measurement has 39 blocking diagnostics (28 root,
11 cascade), down from 44 (31 root, 13 cascade); HIR verification errors fall
from 103 to 94. The exact message counts are in `fit-lsp-mix-gap.json`.
Both namespace guard blockers (one each), `getParameters` (one), and the
unresolved `importing` cascades (two) disappear. Own-key enumeration is a runtime
trap, so its effect is measured by the LSP shard rather than this lowering-only
command. Remaining lowering blockers belong to other categories.

The Grace lexer golden was regenerated from the exported report. Its only
changes are catalogue facts for the two new operations: aliases, allocation,
raise, and write counts each increase by two; read facts increase by three
(two receiver reads and the rename guard's condition-context argument).
Method and expression facts are unchanged.
