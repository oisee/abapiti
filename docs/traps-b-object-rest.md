# Batch B: genuine syntax mismatch — object rest

The first merged-base full-suite run was **stopped**, not counted as a full
measurement: `abap/syntax` had one genuine assertion mismatch. The test
`syntax.ts, Check Variables inline character literal passed to string method
parameter is not compatible` returned zero issues, versus original Node's one.
No batch-B trap work had begun. The first six completed shards had zero FAIL.

## Root cause

`InlineData.stripDerivedFromConstant` uses
`const {derivedFromConstant, ...rest} = data` to remove literal-only assignment
relaxations from an inferred variable's type. `objectRest` reused the source's
nominal shape when omitted fields were optional, but copied **all** its fields
back into the new object. In addition, checker-to-HIR shape reuse can return the
source storage shape even before that widening branch. The inferred character
variable therefore retained `derivedFromConstant: true`, allowing a call that
Node correctly rejects. This is a shared frontend silent miscompile, not a
registry assignment, nested-optional, switch, or assertion-harness issue.

The frontend now obtains the remaining keys directly from the checker's rest
binding type. Excluded fields in reused optional storage receive absent values;
remaining fields are copied into fresh storage. An excluded field with no
absence representation refuses explicitly. Both Go and ABAP consume this HIR.

`TestGoObjectRestExcludesOptionalPropertiesNodeOracle` reproduces the old bug:

- Old Go: `true:changed:kept:true:kept:kept:true`
- Original Node / fixed Go: `undefined:changed:kept:true:kept:undefined:true`

It covers two exclusions, renamed binding, retained values and fresh-object
mutation without changing the source; ABAP emission is also checked. Full-suite
remeasurement follows the fix; no failing test is relabeled as trapped.
