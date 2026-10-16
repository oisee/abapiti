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
