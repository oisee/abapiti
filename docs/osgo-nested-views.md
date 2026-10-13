# Nested structural fields on TS-HA@OSGO

After JSON5 normalization, reading a nested parsed object through an ABAP
class view raised CX_SY_MOVE_CAST_ERROR: the field reader called as_ref
instead of creating a view of the child record.

Nested fields now use the same activated-layout boundary as the Go emitter.
A view retains the original child graph, including identity, mutations and
unknown keys. Fields whose layouts are not activated retain their checked
nominal-reference boundary. This does not invent a prototype on a record.

[Executed unit evidence](evidence/osgo-nested-views-unit.json) covers writes
in both directions, repeated-read identity, replacement of a nested field,
old-alias independence after replacement, unknown-key preservation, skipped
constructors, missing optional children, compatible native identity and
behavior, nominal mismatch rejection, and invalid-source refusals. Both the
synthetic structural layout and an explicitly activated data-only class layout
pass on the pinned OSG Go host. Node supplies the alias/identity oracle.

The [first diagnostic full root run](evidence/osgo-nested-views-initial-root.json)
retains 8/94 passes, FAIL 0, and zero unregistered cases. The previous 62
incompatible-cast traps disappear; there are now 65 generic runtime refusals,
19 stringifySubset refusals, one delete refusal and one stringify/indent
refusal. That run preceded the final alignment with Go's explicitly activated
layouts; it is not a final-commit certification. The
[exact-commit root run](evidence/osgo-nested-views-final-root.json) at db9fbd3
also gives 8/94, FAIL 0, 86 traps and zero unregistered or async-pending cases.
All eight baseline passing titles remain passes. The trap groups are unchanged
from the first diagnostic run. This certifies only the root-shard measurement,
not the whole integration or suite.

Parsed arrays are still refused by the ABAP live-field reader. Their storage
must preserve shared mutation and presence, including the four known sparse
array regressions. JSON serialization remains incomplete. No whole-suite
pass claim or merge is made here; the goal remains 11,060/11,060 on OSGO,
then the same suite on A4H.
