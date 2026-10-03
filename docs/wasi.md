# WASI preview1 in generated ABAP classes

The class backend implements `wasi_snapshot_preview1` inside the generated
class, using its linear memory helpers. Multi-class output keeps the same WASI
state and methods on the main class; chunks delegate imports and memory access
to that object. No application runtime class is needed for single-class output.
Generated source uses ABAP 7.02 syntax with `int8`, contains no comments, and
wraps lines to 255 characters.

`get_stdout()` and `get_stderr()` return accumulated bytes as `xstring`.
`set_stdin(iv)` replaces the input bytes and resets the read cursor. Reads
scatter into iovecs, report the actual byte count, and return zero bytes at EOF.
Arguments default to the generated class name; the environment defaults to
empty. `set_args(it)` and `set_env(it)` accept `string_table`; the WASI getters
encode UTF-8 strings with trailing NULs and wasm32 little-endian pointers.

Descriptors 0, 1 and 2 represent character devices. Seeking returns ESPIPE;
fdstat uses the 24-byte preview1 layout with wazero-compatible APPEND flags and
nonseekable character-device base rights for each stream; inheriting rights are zero.
Closing descriptors 0..2 disables subsequent operations on them; unknown or
already closed descriptors return EBADF (8). Pointer and iovec ranges are checked
as unsigned wasm32 values with int8 sums and return EFAULT (21) on invalid
memory ranges. There are no preopened directories. Unsupported imports, including
`poll_oneoff`, return ENOSYS (52). The FUGR and hybrid backends retain syntactically
valid ENOSYS stubs rather than implementing stream state.

Realtime uses a whole-second UTC `timestamp` (14 integral digits, exact even
on double-backed hosts), copied to `int8` before splitting with DIV/MOD.
Gregorian civil-day arithmetic produces seconds since 1970-01-01, multiplied
by 1000000000 in `int8`; fractional nanoseconds are zero. Realtime resolution
is 1000000000 ns. Tests can supply `mv_clock_override_ts` (default -1,
`YYYYMMDDhhmmss`) to exercise the same integer assembly path.
Monotonic time extends the unsigned `GET RUN TIME` microsecond counter with
persistent `int8` last-value and rollover state, then multiplies by 1000.
Its resolution is 1000 ns. `mv_runtime_override_us` (default -1) substitutes
raw counter samples for deterministic signed-boundary and rollover tests.
The counter cannot reveal multiple complete wraps between samples, so elapsed
time can be undercounted after gaps of about 72 minutes; returned values still
never decrease. `random_get` is a deterministic LCG
(seed 1, multiplier 1664525, increment 1013904223, modulus 2^32, high byte per
step). It is not cryptographic randomness.

## PR decision: process exit

`proc_exit` stores the requested code and raises
`cx_sy_dyn_call_illegal_method`, the existing WASM trap exception. Raising
unwinds nested WASM functions and prevents the exported caller from continuing.
The caller catches the exception and reads `get_exit_code()`; each outer export
invocation resets the code
to -1 and `mv_exited` to false. `proc_exit` sets `mv_exited` to true. Internal
direct and indirect calls use function bodies rather than export facades, so
nested calls do not reset exit state. This follows the task's accepted fallback
and avoids adding another global
exception object or relying on local exception definitions in a CCIMP include
that the `.clas.abap` emitter does not produce. The exception type itself does
not distinguish an exit from a trap; `mv_exited` is the exit indication and
the stored code is the exit status. Objects are not permanently disabled after an exit.

## Validation

A hand-assembled WASM fixture is included in `TestOSD_EmitUnitClasses` before
the optional clang corpus. Its generated ABAP Unit test checks repeated writes
with two iovecs, stderr, invalid descriptors, stdin scatter/EOF, argument and
environment pointers and UTF-8/NUL layout, clocks, deterministic random bytes,
fdstat rights, preopen errors, ENOSYS and nested exit. Wazero's preview1 host
provides independent stdout/stderr, stdin, default argument/environment memory,
monotonic clock resolution and errno results. Configured layouts and LCG vectors are
independent fixed expectations; fixed realtime instants check both 32-bit words,
including leap-day and century boundaries. Counter samples cover two monotonic
rollovers. All 24 fdstat bytes are compared with wazero for each descriptor
using character-device streams (buffer-backed wazero streams report block devices).
Invalid iovec tables, unsigned buffer lengths, output pointers, and descriptor
closure are compared with wazero. Exit followed by an ordinary trap verifies
that stale exit state is cleared.

Go tests also check class and multi-class WASI generation, FUGR stub output,
absence of comments and modern ABAP syntax, and line length. The generated WASI
class and unit test have additionally been checked with abaplint's v702 syntax
and run through the locally installed ABAP transpiler/runtime. CI runs the same
emitted fixture on osgo and OSD; local transpiler execution does not establish
SAP-kernel or osgo compatibility.
