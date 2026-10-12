# Heterogeneous collection element storage

Branch `feat/fit-hetero-elements`, based on the requested train-6 + H commit
`933e68fc1ebf8d73a73d29c227631b7d6525fd70`. Semantic commit:
`57c003fed6bab2407e748775efae3ab830bd3d55`.

## Repair and ownership

Unpruned TS→HIR collection inference examines every class element and each
fresh row/tuple slot. Explicit annotations and array-compatible checker contexts
retain their declared storage. Otherwise sibling references use their nearest
common superclass or shared interface. Undefined contributes absence to an
optional reference slot. The old first-element fallback and unused statement
array builder are removed.

Joined storage remains a reference to the original instance. Inferred reads and
callback parameters retain that storage instead of downcasting to the checker's
structurally collapsed first class. Class-field inference runs after all class
ancestry/signatures are registered, including forward declarations. Existing
field overrides are honored. Positional tuple reads use declared literal-index
slots; unsupported tuple indices still refuse.

The concrete matcher-table root is
`test/abap/expressions/reuse.ts:10`: its `r` slot now stores the common
`Expression` reference, so `FieldLength` and the other concrete expressions fit.
The consequential readiness refusal at line 70 disappears. Constructors,
virtual methods, matcher code, titles, registration loops and assertions execute
from the original pinned source. No upstream test source or harness/reporting
code is changed.

The pinned default/reachability ABI remains frozen. No runtime operation,
string/regex/Math implementation, typeof lowering, LSP type, small builtin,
object-literal construction behavior or void ABI is changed. The general union
mapper is unchanged; the nominal join is confined to collection inference.

## Fresh measured results

Only the touched `abap/expressions` shard was measured: 11 original test files,
107 active titles. Original pinned Node passes all 107 in both measurements.
Duplicate titles reconcile as a multiset.

| Measurement | Passed | FAIL | Trapped | Unregistered | Registered |
| --- | ---: | ---: | ---: | ---: | ---: |
| Base 933e68f | 42 | 0 | 8 | 57 | 50 |
| Semantic 57c003f | 105 | 0 | 2 | 0 | 107 |

**All 57 missing matcher titles register and pass**, verified against retained
per-title records, including both occurrences of the duplicate TypeTable title.
The matcher initializer has one nominal storage failure plus one consequential
readiness failure before the repair, and zero afterward.

Six already-registered SQL-set-op bodies also pass after array literals use their
checker-supplied contextual recursive tuple type. Their two diagnostic groups
(5 + 1 cases) disappear. The remaining two traps are the two SELECT privileged
access-level version cases at `src/config.ts:11:3`, which retain the existing
numeric/conversion-domain refusal. There are no assertion mismatches, unmatched
titles, async-pending cases, exclusions within the selected shard or fabricated
registrations. No whole-suite pass total is extrapolated.

The production registry closure was measured with `abaplint --full`, without the
reachability manifest: **36 blocking diagnostics (24 root + 12 cascade) and
100 HIR verification errors before and after**, 136 combined. The matcher test
initializer is outside that production closure; its category has zero production
occurrences in both runs. The complete blocking and HIR histograms by message,
and the exact touched-shard initializer diagnostics, are retained in the
[evidence](evidence/hetero-elements.json).

## Gates and reproduction

All final gates passed on the semantic source:

- `go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...`
- `go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/... -timeout 40m`
- `ABAPITI_GRACECHECK=1` with `TestLexerFactsReport`: exported facts are
  byte-identical to the committed golden, SHA-256
  `7281eb9f30846711b8f9d7aaa59244a9fef1222d144db71ced79ef79125065dd`.
- `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`, explicitly rerun.
- `./.github/ci/lint.sh gate origin/feat/fit-traps-h`: zero new issues,
  canary 5/5, checksum-pinned golangci-lint 2.13.2. This final lint ran after
  committing the source; its temporary canary did not overlap final vet/tests.
- The base and final compilers emitted `abaplint --target all,go` at the same
  absolute scratch output path; **`diff -r` was empty**, including the Go binary
  and default zabapgit artifacts.

`TestHeterogeneousCollectionElementsNodeOracle` compares original Node with
verified emitted Go for sibling and reversed arrays, a third subclass pushed
through base storage, row identity, explicit base/interface contexts, an inferred
shared interface, tuple slots, spreads, undefined, instance fields and
forward-declared element classes. Existing computed-name refusals, generator
consumers and numeric destructuring tests pass in the complete short gate.

The baseline compiler was built from `git archive 933e68f`. Both measurements
use upstream `577f875ebec44cfaf64841cfe71c8ab8dc32622e` with original lockfile
dependencies and fresh oracle builds. Set `GOMAXPROCS=2`, `GOFLAGS=-p=2`, and one
shard worker. With `scratch` holding the compilers and a pristine pinned upstream:

```sh
ABAPITI_TESTS_SHARDS=abap/expressions ABAPITI_TESTS_WORKERS=1 \
  ABAPITI_TESTS_COMPILER="$scratch/base" \
  ABAPITI_TESTS_COMPILER_COMMIT=933e68f \
  node tools/abaplint-tests-all.mjs "$scratch/upstream" "$scratch/before.json"
ABAPITI_TESTS_SHARDS=abap/expressions ABAPITI_TESTS_WORKERS=1 \
  ABAPITI_TESTS_COMPILER="$scratch/after" \
  ABAPITI_TESTS_COMPILER_COMMIT=57c003fed6bab2407e748775efae3ab830bd3d55 \
  node tools/abaplint-tests-all.mjs "$scratch/upstream" "$scratch/after.json"
"$scratch/base" abaplint --full -o "$scratch/gap-before"
"$scratch/after" abaplint --full -o "$scratch/gap-after"
```

The evidence records the default-output commands and every gate command.
A read-only filesystem observer retained the original runner's Go/Node records
before its normal cleanup, allowing exact 57-title status accounting. Compact
JSON/Markdown alone are committed.

An exploratory broad union change produced unrelated interface verification
errors and was reverted. Focused tuple-index, computed-key, generator-context
and inferred destructuring regressions were repaired and the full short gate
rerun. A concurrent lint-canary experiment caused a temporary gate failure;
final certification ran the canary separately. No original abaplint FAIL was
encountered or relabeled.

## Related ABI work remaining

The optional three related cases remain unchanged and were not measured here:

- `test/abap/structures/_combi.ts:276` (one active case): block-local `Normal`
  requires per-invocation constructor identity/captures. Hoisting changes the
  program; this needs a separate local-class ABI repair.
- `test/rules/line_length.ts:17` (two active cases): the config assignment returns
  primitive `400`. Primitive property reads and comparison domains need erased
  configuration storage; substituting `{length:400}` changes the program.

The branch has local commits only, following the task-specific no-credentials
instruction. No push to main or PR was made.
