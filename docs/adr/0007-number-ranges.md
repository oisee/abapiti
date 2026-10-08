# 0007. Proven integer storage during TS lowering

Status: implemented conservatively.

The TS lowering runs interval analysis after constructing semantic HIR and
before verification/emission. The domain distinguishes no evidence, finite
safe integers, and unknown binary64. Branches join, arithmetic computes bounds,
and loops and recursive/mutable summaries widen to integer thresholds before
falling back to Number. Comparisons restrict existing integer facts; they do
not establish integrality of public Number inputs. Length seeds use the
existing ABAP string/table runtime's signed 32-bit length domain.

I32 requires all stored values to fit signed 32 bits. I64 requires all values
and intermediate operations to remain JS safe integers, so int8 arithmetic
cannot silently change binary64 rounding. Division remains Number; only a
nonzero literal divisor can be emitted under ADR 0005. NaN/Infinity retain
existing blocking diagnostics. Fractional values, negative zero, and negative
remainders that could produce negative zero stay Number. Positive literal
remainders require a proven nonnegative integral dividend. Math.min/max of two
proven integer operands propagate intervals. Public binary64 signatures remain binary64. Parameters feeding private string-
index fields with only literal/affine writes use the explicitly permitted
checked-boundary policy: a dominating checked integer alias establishes the
range before storage is specialized. Other public parameters remain Number.
Each new boundary contract has a source-located note-number-boundary diagnostic;
fractional or out-of-range values raise cx_sy_range_out_of_bounds rather than
rounding or wrapping. At this explicit integer boundary, signed zero becomes
integer zero; untouched Number paths retain signed zero. This is a deliberate
input-contract restriction, not a
claim that all formerly accepted JS inputs still succeed.

Private method and unexported function parameters/results can specialize from
all lowered call sites. Virtual result summaries join lowered overrides; public
and interface signatures retain their ABI. Private/readonly numeric fields
require dominating constructor or static initialization and invariant writes.
Calls invalidate temporary mutable-field facts. A field bounded by a private
readonly string length may use a symbolic inductive upper bound only when that
string has one dominating constructor assignment; the readonly modifier alone
is insufficient. The numeric bound is validated at
every write. An equality guard can exclude that bound before an increment.
No class names or lexer-specific source patterns are special-cased.

NumericConvert records exact promotions and frontend interval evidence for
narrowing a flow-proven value read from wider storage. HIR verification checks
conversion kinds and proof bounds; the TS analysis is responsible for the
proof. Numeric min/max evaluates both operands once in source order. The
backend maps I32/I64/Number to i/int8/f and declares index-conversion bounds
and the safe-integer formatting bound once per method, outside loops.

Diagnostics count scalar numeric declaration sites (fields, parameters,
locals, and method results), including compiler-generated locals, per class.

LexerStream.offset has the inductive invariant -1 <= offset <= raw.length and
becomes I32. Its row/col remain Number: this interval domain cannot prove the
newline-count bound, and repeatedly advancing at EOF can decrement col without
bound. Lookahead offsets may require I64 for raw.length + 1/+2, with guarded
I32 narrowing at character access.

LexerBuffer.start/end become I32 using a checked alias for add(offset:number).
All writes are either integer literals or affine expressions of that parameter.
Their constraints intersect to [-2147483648,2147483646], because end=offset+1
must fit I32. The public parameter remains f; the guard rejects fractional
inputs and values outside that interval before converting. This includes
2147483647 (whose end would require int8) and deliberately restricts the
boundary rather than claiming an unchecked universal I32 proof. Existing valid
lexer corpus inputs satisfy the contract. A theoretical raw string with
2147483647 code units reaching that EOF offset would also hit this guard.
The runtime regression tests require both valid behavior and rejection of a
fraction and that upper-bound value. Buffer.length arithmetic may use I64 when
the independent start/end intervals cannot prove an I32 difference.
