# 2026-10-03: QuickJS runs on a real SAP kernel

The run that made QuickJS evaluate JavaScript inside ABAP, recorded with the
numbers as measured. Generator: the commits of PR "feat(wasm): QuickJS split
classes and bounded nesting" on top of main bb6da22.

## Input

- QuickJS 2024-01-13 (bellard.org), built for WebAssembly by wasi-sdk 34
  (clang 23) without SIMD: `--target=wasm32-wasip1 -O2 -mcpu=mvp -msign-ext
  -mbulk-memory -mmutable-globals`, reactor model, with a small driver that
  exports `qjs_eval(n)` (evaluates one of a fixed list of scripts and returns
  the result as an integer) and `qjs_print(n)` (calls `printf`). 1.0 MB of
  wasm, 1,024 functions, 5 WASI imports.
- The same module in wazero gives the expected values.

## Output

`abapiti compile wasm qjs.wasm --split --class zcl_qjs`: 13 global interfaces
`ZIF_QJS_C01..C13`, the state class `ZCL_QJS_ST` (private static memory, WASM
globals, the function table, WASI state), 13 chunk classes `ZCL_QJS_C01..C13`
of up to 19,946 lines, and the facade `ZCL_QJS`: 253K lines in total, no line
over 255 characters, nesting depth at most 63.

## A4H (SAP kernel 7.58), 2026-10-03 07:40 UTC

Throwaway package, deployed and activated one object at a time in the order
interfaces, state class, chunks, facade:

| Step | Time |
|---|---|
| 13 interfaces | 0.5–1.7 s each |
| state class | ~5.5 s |
| chunk classes (~20K lines each) | 2.7–6.8 s each |
| facade | ~1 s |
| all 28 objects | 87 s |
| 9 ABAP Unit tests | ~2.2 s, 9/9 passed |

Tests (each creates a fresh QuickJS runtime): `1+2` = 3; a loop summing 1..100
= 5050; `[5,3,9,1].sort((a,b)=>a-b)` checksum 4009;
`JSON.stringify({a:[1,2,{b:3}]}).length` = 19; recursive `fib(15)` = 610;
`'abapiti'.toUpperCase().charCodeAt(0)` = 65; `Math.floor(Math.sqrt(1e6))` =
1000; `Map` size plus a `RegExp` match = 42; `printf("abapiti says %d\n", 42)`
through WASI arrives in `get_stdout( )`. All equal to wazero. Afterwards every
object and the package were deleted; nothing remained.

## osgo (open-steamgate's Go runtime, main ad3d1e8)

Same folder, same 9 tests: 9/9. ABAP frontend 156 s, Go build 36 s, run 89 s.

## What it took

- Runtime helpers rewritten into ABAP the kernel accepts (`BIT-AND` needs `x`
  operands; no offset writes into an `xstring`), which abaplint and the
  emulators accept either way.
- Unique method names within 30 characters (two long exports truncated to the
  same name broke activation).
- Nesting below the kernel's limit of about 128 control structures: a C switch
  had become 288 nested `DO 1 TIMES` ("Structure stack full").
- A split whose chunk classes depend only on interfaces and the state class, so
  each activates on its own; calls between chunks go through interface
  references created once with `CREATE OBJECT ... TYPE (name)`.
- On the open-steamgate side: `ipow`, int8/x conversions, writes to another
  class's static attributes, a near-linear frontend and a fast in-place buffer
  for private memory, each checked against A4H oracles.

Not checked yet: QuickJS on OSG-JS (the abaplint JavaScript runtime).

Speed on five runtimes (native, wazero JIT and interpreter, osgo, A4H): [2026-10-03-quickjs-speed.md](2026-10-03-quickjs-speed.md).
