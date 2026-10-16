# Step-4 replacement and type lowering

Baseline: origin/integ/selfhost-s7 at
683d0e3036a2358946c4b84873ab89bf1d616b7e; branch dell/types.

## Replacement

RED Node oracle: the original lowerer rejects literal dollar substitutions;
the first backend run also refuses the pilot's /^(-?)0+(?=\d)/ grammar.
The final oracle compares Node and generated Go, and exports ABAP fixtures.
Coverage includes original-input prefix/suffix, first/all, empty needles and
matches, UTF-16 units, capture indices 1–99 and two-digit fallback, absent
captures, named captures, absent names, and literal dollars/backslashes.
The exact zero-prefix grammar consumes a final digit internally, excludes
that digit from the exposed match, and hides its implementation capture.
Named groups admit unique ASCII names; other regex forms retain checked
emitter/runtime refusals. Callback/coercing replacements and replaceAll with
a nonliteral or nonglobal RegExp receive located frontend diagnostics.

Specification: https://tc39.es/ecma262/multipage/text-processing.html#sec-getsubstitution

Baseline pilot (local writable mirror of the supplied OSG checkout):
reference a13702d55b1eecdb; files 56, loaded 51, bodies 422, executed 226,
unmatched 3; 304 blocking (206 root, 98 cascade), 1691 HIR errors in 37 files;
118.40 seconds, 1397244 KB. /logs/pilotsetup is used read-only.

## Generic nested append (two type-reference root sites)

A: append.ts:2:35 and :2:70. Both are correctly typed uses of T in native
array storage, not missing generator annotations. The RED oracle reproduces
both declaration failures. The supported complete two-loop body specializes
at direct calls, independent of identifier names, retaining native element
storage and evaluating all arguments before mutation. Primitive, object and
nested-array aliases and late argument mutation match Node. Other generic
bodies and indirect function values retain located refusals.

## Bigint (three checker-type root sites)

A: emit-int8.ts:11:7, :11:27, :13:30. TypeScript bigint is correctly typed;
changing it to number or int64 would lose precision or overflow at the bounds.
The RED oracle reproduces the checker/annotation failures and BigInt conversion
refusal. HIR now has a distinct bigint primitive, canonical arbitrary-length
signed decimal storage, decimal-string construction, constant +/−/multiply/left
shift, runtime negation, comparisons, formatting and truthiness. Tagged storage
preserves bigint versus Number/String/null. Runtime arithmetic beyond negation,
nondecimal conversion strings and numeric/boolean conversion arguments stay
outside the checked subset. Shift folding caps allocation at 1,048,576 bits.
Go matches Node across both int64 boundaries and 40-digit integers; ABAP emits
string-based sign/length/digit comparisons without fixed-width conversion.

## Optional callable intersections (OwnershipObject)

A: frontend-owned.ts:14:27 and :15:14. ArtifactObject intersected with
Partial<Pick<ABAPObject, 'getABAPFiles'>> is correctly typed TypeScript.
Structural intersection synthesis now maps callable slots by their checker
signatures and uses the existing optional-data-member fallback. The RED oracle
reproduced the alias refusal; absent and present callable slots now match Node
and export ABAP. Behavior-bearing native class conversion remains subject to
the existing checked class-view ABI, rather than copying method values.

## Generic property values and string keys

A: ir-children.ts:10:110 and :53:81 (PropertyValue<T>), plus the two
keyof-T-and-string diagnostics at :55:28. T is constrained to object. The
supported conditional is structurally T extends object ? T[keyof T] : never;
it maps to tagged values, and keyof T & string maps to string keys. Arbitrary
conditional types retain their refusals. Generic indexed reads use original
object storage and Object.keys snapshots. Tuples containing erased values use
real tagged arrays, preserving entry arrays and explicit undefined elements.
The RED oracle reproduced both conditional failures and key intersections;
children and entries now match Node, including JSON array shape, and emit ABAP.
