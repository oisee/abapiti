# 0010. Explicit integer arithmetic contract

The tsgo → HIR → ABAP lowering accepts `LowerOptions.AssumeOnlyIntegerCalculations`,
`tslower -assume-only-integer-calculations`, or `ABAPITI_ASSUME_INT=1` in test
and existing lowering entry points. It is opt-in. Explicit `LowerWithOptions`
uses its options rather than the environment. The default path remains unchanged.

Run the existing proven-range pass unchanged. Its I32 proofs still select `i`;
remaining Number storage, including public signatures and nested collection or
optional types, defaults to I64 / `int8`. This is an integer input contract,
not a binary64 semantics optimization. All values must stay within JS safe
integers (±9007199254740991).
Integer arithmetic must raise when a result leaves that domain. Negative zero
is not preserved: `-0` becomes `0`; division is blocked unless explicitly approved.
An explicit HIR arithmetic marker requests range guards in generated ABAP so
bigint-backed execution also rejects values outside the safe-integer domain.
Native ABAP can raise before those guards. Operands are evaluated once, in source order.

Potential fractional sources are blocking diagnostics at their exact TS spans:
division, negative/non-literal exponentiation, fractional/exponent literals,
literals outside the safe-integer range,
Math functions that need integral-result evidence, parseFloat/Number conversions,
and fractional numeric formatting. Unsupported TS operations remain blocking;
an exception does not authorize implementing unsupported operations incorrectly.
Detection reads original token spans because tsgo normalizes numeric token text.

`-integer-exceptions FILE` accepts a JSON array of `{file, start, sha256, reason}`.
File names are relative to the tsconfig; start is the token's byte offset; SHA256
covers the exact TS span; reason is mandatory review evidence. Duplicate,
changed, deleted, or ambiguous targets in selected files are errors. Entries for
other closures are not applied. There is no automatic approval or wildcard.
Diagnostics print every required or approved site, its location, expression,
reason, and fingerprint; JSON exposes the uncapped `integerExceptionSites` list.

Supported approved floating expressions retain Number at that expression and
keep intermediate fractions inside approved floating consumers, then
cross into integer storage through CheckedNumericConvert: a non-integral or
out-of-range result raises cx_sy_range_out_of_bounds. Binary64 crossings are
limited to exact JS safe integers (±9007199254740991). This conservative bound
avoids rounding an int8 operand into a different float. Integer decimal formatting
uses int8 directly within the safe-integer domain. Existing string
index boundaries retain their checked I32 domain rather than silently truncating.

The vendored lexer closure and statements/structures closure require an empty
floating exception list. This claim is tested by lowering and HIR verification,
and independently exercised by their differential ABAP Unit corpora.
