# Independently activatable WASM classes

`abapiti compile wasm module.wasm --split --class zcl_example -o src`
produces `zcl_example_st.clas.abap`, `zcl_example_c01.clas.abap` (and further
chunks), and the `zcl_example.clas.abap` facade. Activate the state class first,
then chunks in any order, then the facade. Small modules retain single-class
output; splitting also happens automatically above `--class-lines` (default
20000) or 500 WASM methods. Split output defaults to the current directory.

The state holds static memory, globals, function tables, required arithmetic
and memory helpers, and WASI output/exit state for imported modules. Its `init`
resets memory, globals, data segments and tables. Constructing a facade calls
`init`; multiple facade objects share this state, so constructing another
facade resets the shared module. The facade preserves export aliases.

Chunks have static WASM methods with local parameter copies and typed stacks.
Calls within a chunk are static. Cross-chunk calls read class/method names from
the state's function registry and use ABAP 7.02 dynamic `CALL METHOD` with
`EXPORTING` and `RECEIVING`. Registry names are assembled from literal stems
and suffixes, so neither state nor chunk source names another complete chunk
class. Indirect calls validate table bounds, null entries, and structural
function signatures before using the same dynamic mechanism. No external
`zcl_wasm_rt` class is emitted or required.

Tarjan strongly connected components group recursive functions. Greedy packing
keeps callers near callees and respects conservative generated line estimates
and a 500-method limit. Components larger than a chunk budget are divided.
A single function cannot be divided between classes; if it alone exceeds the
budget it occupies its own chunk. Shared initialization/data and facade exports
are also indivisible. The size report lists actual lines, methods, and longest
routines for every class, including any such budget excess.

WASI provides byte output via `wasi_output` and exit status via `wasi_exit` on
the facade. Reading returns EOF, args/environment are empty, and unimplemented
host operations return ENOSYS. SIMD retains the existing unsupported-op trap;
splitting QuickJS does not make its SIMD instructions executable.

`TestOSD_EmitUnitClasses` emits both existing single-class tests and split
variants, with expectations computed by wazero. Split variants cover add/i32
wrap, factorial, memory growth, i64 wrap/helpers, direct cross-chunk calls with
parameter writes, and indirect calls with null/type/bounds traps. OSD deploys
state, chunks, then facades and tests. The pinned osgo frontend cannot compile
dynamic `CALL METHOD (lv_cls)=>(lv_meth)`; its exact diagnostic is listed in
`osgo-known-gaps.txt`. Those cases remain emitted and become enforceable on
osgo when that frontend gains dynamic dispatch support. OSD executes the same
assertions without this exemption.
