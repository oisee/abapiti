# Independently activatable WASM classes

`abapiti compile wasm module.wasm --split --class zcl_example -o src`
produces `zif_example_c01.intf.abap` (and further interfaces),
`zcl_example_st.clas.abap`, `zcl_example_c01.clas.abap` (and further chunks),
and the `zcl_example.clas.abap` facade. Activate interfaces first, then state,
chunks in any order, and the facade last. Small modules retain single-class
output; splitting also happens automatically above `--class-lines` (default
20000) or 500 WASM methods. Split output defaults to the current directory.
The WASM CLI writes ABAP source files without XML metadata for either classes
or interfaces. Its size report includes every emitted interface and class.

The state holds memory and its page counter as private `CLASS-DATA`, without
`FRIENDS`. Chunks access memory only through public static state methods for
loads, stores, size, grow, copy, and fill. Data segments are initialized inside
the state. Globals, function tables, required arithmetic helpers, and WASI
output/exit state also live in the state. Its `init`
resets memory, globals, data segments and tables. Constructing a facade calls
`init`; multiple facade objects share this state, so constructing another
facade resets the shared module. The facade preserves all export aliases,
including re-exported imports.

Each chunk implements instance methods through its global interface. All WASM
functions are exposed in the interface to keep generation simple. Within a
chunk, methods call their own interface implementations. Cross-chunk and facade
calls use typed interface references such as `zcl_example_st=>go_c02->f123( )`.
The state creates each instance once with `CREATE OBJECT ... TYPE (lv_name)`
using an uppercase class-name string. State and chunks have no static reference
to any other chunk class. Bases beginning with `zcl_` use `zif_..._cNN` interface
names; other bases use `<base>_iNN`. The shared stem is shortened to reserve
suffix space and avoid collisions with the facade. Generated names are distinct
and fit in 30 characters. `CompileResult.Files` returns an error for duplicate
object names instead of overwriting output; callers must handle its error.

Indirect calls validate table bounds, null entries, and structural signatures.
The function registry stores chunk number, function id, and signature id. A
shared CASE dispatcher per used signature calls the appropriate interface
reference or an imported-function wrapper in the state. No dynamic method
calls or external runtime class are generated. Parameter copies and typed
stacks keep actual argument types identical to the method's formal types.
Split compilation returns a clear Go error for local or imported functions
with multiple results; multi-result transport is not implemented.

Tarjan strongly connected components group recursive functions. Greedy packing
keeps callers near callees and respects conservative generated line estimates
and a 500-method limit. Components larger than a chunk budget are divided.
A single function cannot be divided between classes; if it alone exceeds the
budget it occupies its own chunk. Shared initialization/data, indirect dispatch,
and facade exports are also indivisible. The size report lists actual lines,
methods, and longest routines for every output, including budget excesses.

Split helpers use the same kernel-valid bodies as single-class compilation,
with direct access to private memory inside the state methods. Function bodies,
interface methods, calls, dispatch targets, and facade exports use the shared
unique naming allocator; export aliases use its remaining namespace.

WASI preview1 buffers, arguments, environment, clocks, random state, and the
dispatcher live in the state. The facade exposes `get_stdout`, `get_stderr`,
`get_exit_code`, `set_stdin`, `set_args`, and `set_env`; `wasi_output` and
`wasi_exit` remain compatibility aliases. Facade exports reset exit status once
per host invocation; internal calls preserve it. Compile APIs return errors for
unsupported opcodes, including SIMD, instead of emitting partial output.

`TestOSD_EmitUnitClasses` emits single-class tests into its root folder and
forced-split variants into `split/`, with expectations computed by wazero.
Both osgo and OSD CI consume both sets, including the interfaces. Split tests
cover i32/i64 arithmetic, shared memory, direct cross-chunk calls, indirect
import calls, re-exported imports, and null/type/bounds traps. OSD deploys
interfaces, state, chunks, then facades and tests. osgo must pass every split
test; its known-gap exceptions apply only to the single-class fixtures.
