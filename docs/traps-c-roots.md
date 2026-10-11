# Trap batch C roots

## Call receiver construction context

Downport's four arrays of ExpressionNode are flattened to ExpressionNode[].
The call result's array construction hint leaked into its receiver literal,
causing each inner array to be pushed as an ExpressionNode. This is a general
frontend context bug, not a runtime flat or argument ABI change. Library-call
receiver lowering now clears/restores the result hint, as property reads already
do. `TestGoArrayReceiverContextNodeOracle` checks nested literal flattening,
reverse, evaluation counts, fresh storage and references against Node; ABAP
emission passes. Grace-enabled small-builtins tests pass.

Focused upstream Downport: 1 passed, 297 trapped, 5 not-run, **FAIL 0**.
The 194-test argument mismatch is gone; newly reached cases chiefly trap at
crypto (157) or Dell-owned EditHelper (85). No pass gain is inferred from
removing an earlier trap. Generated module/closure deleted after execution.

## Conversion triage: ancestry operand versus parsed rule configs

The generic conversion message grouped different roots. Stack inspection of a
throwaway generated module identified checked nominal casts *before* ancestry
tests in obsolete_statement and avoid_use; Config/7bit_ascii instead encounter
parsed object-to-rule-config boundaries. These are not one shared conversion.

`instanceof` now tests the operand's natural storage type rather than applying
the checker's flow type first. Empty sibling classes are structurally identical
in TS but nominally distinct in JS; compound/repeated tests could otherwise cast
to the wrong sibling before returning false. The general Node/Go regression
covers A/B/C/base instances, repeated and short-circuit tests, and once-only
receiver evaluation; ABAP emission and Grace checks pass. Explicit user class
assertions still retain checked-view refusal behavior.

Focused obsolete_statement + avoid_use: **85 passed, 24 trapped, FAIL 0**.
Remaining traps are Dell-owned EditHelper (20) and registry assignment (4).
Generated builds and temporary instrumentation removed.
