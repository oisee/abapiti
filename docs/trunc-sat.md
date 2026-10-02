The Go WASM compiler implements all eight `0xFC 0x00` through `0xFC 0x07`
saturating float-to-integer conversions. Both WASM float types use ABAP `TYPE f`.
Finite inputs are clamped before truncation. Float constants retain enough
decimal digits to round-trip, including values immediately below integers.
Unsigned results use signed integer bit patterns: the upper half of the range subtracts 2^32 or 2^64 in a float
stack slot before assignment to `i` or `int8`. Saturated i64 endpoints are
assigned from integer literals so that the maximum does not round to 2^63.

Generated conversions test `value <> value` first and return zero for NaN if
the host float representation supports it. Comparisons would clamp positive
and negative infinity to the appropriate endpoint. However, the existing
constant emitter writes `NaN`, `+Inf`, and `-Inf` as quoted numeric strings;
these are not portable ABAP float literals and can fail during assignment,
before the conversion executes. The existing `f64.reinterpret_i64` and
`f32.reinterpret_i32` helpers perform numeric assignments rather than IEEE bit
reinterpretation, so they cannot construct these special values either.
This change does not claim portable ABAP NaN/Inf construction. Those cases
remain Go-only, using hand-assembled WASM executed by wazero. Finite edge cases
for every opcode, including both halves of each i64 result, are emitted by
`TestOSD_EmitUnitClasses` for osgo and OSD, with expected values from wazero.

`Compile`, `CompileWith`, and `CompileMultiClass` now return an error alongside
their output. Unsupported instructions, including SIMD and atomic prefixes,
abort compilation with the opcode and the WASM function index. No partial
source is returned. The CLI reports the error before writing files. The
QuickJS fixture contains SIMD and its compilation tests expect rejection.
