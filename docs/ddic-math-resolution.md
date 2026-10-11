# DDIC computed packed lengths and builtin provenance

The specified merged base already contains the original DDIC fixes. Batch E
recognizes the checked global `Math.ceil` call before attempting to lower `Math`
as an ordinary receiver, and DDIC-locals preserves one HIR name per checker
binding when a switch block is copied for several labels. The new production
change checks every declaration of the global binding through the existing
`librarySymbol` helper. A user declaration merged into `Math` now refuses rather
than acquiring builtin numeric semantics from its library value declaration.

The branch starts at `a0bee5c` and merges `0de990c`, `26bcfb6`, `29f352e`, and
`569f496` in that order with merge commits. The frozen merged base is
`f00f7a4bed8828700ae034171497c9507d6547be`. The measured after compiler corresponds
to production source at `db97c3f51d8faed838bfa36eeb839d69814d7f3d`; subsequent
changes reconcile test fixtures and the Grace golden. Historical measurements
from conflicting branches are retained separately in `fit-*-abaplint-tests*`.

The combined packed-length regression executes every grouped switch branch
against original Node, including odd lengths, negative values, decimal prefixes,
and nested shadowed bindings. It verifies HIR, checks ABAP's explicit unsupported
binary64-ceil refusal, and requires an integer-proof diagnostic instead of
truncating the division. With the merged-base lowering restored through a Go
overlay, this regression passes, while the user-global provenance regression
fails. Both pass with the production fix.

## Full closure

Both commands lower the full 1,538-file core closure without a reachability
manifest. Exact message histograms and source diagnostics are retained in the
JSON evidence.

```sh
"$task_compiler" abaplint "$task_upstream" --full --assume-int=false -o "$task_evidence/full-float"
"$task_compiler" abaplint "$task_upstream" --full -o "$task_evidence/full-int"
```

| Measure | Merged base | After |
| --- | ---: | ---: |
| Binary64 lowering blockers | 30 | 30 |
| Binary64 HIR errors | 74 | 74 |
| Binary64 DDIC diagnostics / HIR errors | 0 / 0 | 0 / 0 |
| Integer lowering blockers | 48 | 48 |
| Integer HIR errors | 117 | 117 |
| Integer DDIC integral-proof refusals | 9 | 9 |
| Integer DDIC marked packedLength cascades | 9 | 9 |

Integer mode still requires evidence for
`Math.ceil((parseInt(input.length, 10) + 1) / 2)`. Its nine switch-label copies
produce nine explicit refusals and nine marked cascades; these are distinct from
an unresolved builtin in binary64 mode. ABAP's general binary64 `number.ceil`
implementation remains outside this frontend task. Neither integer truncation
nor a permissive fallback was added.

## Measurements and gates

Suite results and final gate receipts will be filled from completed executions.
