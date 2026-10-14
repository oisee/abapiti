# Tail batch 4: Error metadata, JSON extras, and checked NaN storage

Runtime pin: `f83dc5c3`; upstream: `577f875e`.

## Downport: the emitted class was rejected, not omitted

`src/rules/downport.ts.SkipToNextFile` was emitted and selected by the host
bridge. Its constructor contained a write to the inherited record-deletion
mask. The ordinary class emitter declares that mask on roots, but
`builtin.Error` is subsequently replaced by the special runtime emitter,
which did not declare it. Gogen's syntax diagnostics rejected the subclass
and removed it from the compiled set; `NEW` then refused it.

The repair is in `hir/abap/runtime.go`: retain that root metadata on the
runtime Error class, including when class-value descriptors are enabled.
`TestErrorSubclassRecordMask` exercises construction, deletion, reassignment,
and own-property presence on an Error subclass. No closure or OSG changes
are needed.

## Old config: added own properties, not malformed JSON

The exact tests are in `test/config.ts`. They add the legacy
`global.applyUnspecifiedRules` property, then stringify a native structural
object. Its layout supports live views but this instance has no graph source.
The subset writer refused unknown own keys precisely for such live layouts,
even though their extras table retained the values.

`hir/abap/config_json.go` now reads those extras just as it does for other
native structural layouts. Own-key order, undefined omission, cycle checks,
and unsupported prototype/toJSON checks remain in force. This is separate
from JSON parsing and the earlier SyntaxError payload repair.
`TestJSONLiveNativeExtras` compares both boolean values with Node.

## Single dot: retain NaN rather than substitute zero

The line-only-punctuation rule builds a temporary edit using
`rows[i - 1]?.length + 1 + offset`. For a single dot on the first row, Node
computes NaN and constructs a Position with that column, then discards the
edit. The frontend correctly represents the absent numeric operand with the
checked `parseFloat("NaN")` producer. The finite ABAP `f` lane previously
refused it.

A backend-private storage lowering now propagates the existing tagged Number
box from that checked producer through scalar locals, fields, constructor
parameters, method results, and related signatures. Unaffected numbers retain
native `f` storage. The input HIR is copied, so Go retains its original ABI.
Arithmetic propagates NaN; comparisons, isNaN, stringification, and
ToIntegerOrInfinity can observe it correctly. Only index conversion maps NaN
to zero. `TestNonfiniteScalarStorage` uses Node expectations and verifies both
NaN and finite values across a constructor, field, and getter.

This is not a blanket removal of numeric refusals. Uncertified nonfinite
boundaries, nonfinite scalar unboxing, tagged array min/max, and operations
whose signed-zero/nonfinite behavior is not established still refuse.

## Optional reference identity

Focused downport selection also exposed three open-abap FAILs. Live adapter
identity was normalized for required references but not optional references.
The same original release object could therefore compare unequal through two
adapters. Optional reference storage now takes the same backing-source
identity path. `TestLiveOptionalReferenceIdentity` covers two optional views
of one graph.

## Focused commands

```sh
ABAPITI_TESTS_TARGET=osg \
ABAPITI_TESTS_FILES=test/rules/downport.ts,test/config.ts,test/rules/line_only_punc.ts \
ABAPITI_TESTS_KEEP_ARTIFACTS=1 \
node tools/abaplint-tests-all.mjs "$ABAPLINT_HOME" "$EVIDENCE/focused.json"
```

Use `ABAPITI_OSG_ONE_BUILD=0` for independent shard builds, and
`ABAPITI_TESTS_TARGET=go` for the same-file Go comparison. All heavy commands
must use the shared flock, memory-limited scope, and resource admission.
