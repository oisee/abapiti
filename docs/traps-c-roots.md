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
