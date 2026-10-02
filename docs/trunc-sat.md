The Go WASM compiler implements all eight `0xFC 0x00` through `0xFC 0x07`
saturating float-to-integer conversions. Both WASM float types use ABAP `TYPE f`.
Finite inputs are clamped before truncation. Float constants retain enough
decimal digits to round-trip, including values immediately below integers.
Unsigned results use signed integer bit patterns: the upper half of the range subtracts 2^32 or 2^64 in a float
stack slot before assignment to `i` or `int8`. Saturated i64 endpoints are
assigned from integer literals so that the maximum does not round to 2^63.

ABAP `TYPE f` cannot represent NaN or infinity. Non-finite f32/f64 constants
emit the standard WASM trap at the point where the float is pushed, so only
execution of that path traps. Direct integer constant/reinterpret pairs with
NaN or infinity bit patterns also emit that trap. Other reinterpret inputs
still use the existing helpers, which perform numeric assignments rather than
IEEE bit reinterpretation. An OSD fixture explicitly expects construction to
trap, including a function returning `trunc_sat(NaN)`; this differs from WASM,
where that conversion returns zero.

The pre-existing f32 model computes in double precision without rounding each
producer to f32. For example, `2147483520 + 64` followed by signed i32 trunc_sat
returns 2147483584 in generated ABAP, versus 2147483647 in WASM. A Go test pins
this known gap; f32 precision is outside this change's scope. Finite edge cases
for every opcode, including both halves of each i64 result, remain emitted by
`TestOSD_EmitUnitClasses` for all targets, with expected values from wazero.

`Compile`, `CompileWith`, and `CompileMultiClass` now return an error alongside
their output. Unsupported instructions, including SIMD and atomic prefixes,
abort compilation with the opcode and the WASM function index. No partial
source is returned. The CLI reports the error before writing files. The
QuickJS fixture contains SIMD and its compilation tests expect rejection.
