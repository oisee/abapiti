# Plan: make the WebAssembly path fast

Decided 2026-10-03. Only the WebAssembly path is verified (CI on wazero, osgo
and OSG-JS, runs on a real kernel); a smoke test of `llvm/` and `ts/` on the
12-program C corpus and a small TypeScript sample compiled nothing. So we
keep WebAssembly and make it fast, instead of switching IR.

## Why WebAssembly, not LLVM IR

- WebAssembly already gives what `llvm/` still lacks: structured control flow
  (`block`/`loop`/`if` map to `IF`/`DO`/`EXIT`; `llvm/` dispatches basic blocks
  through a `CASE` on a string), typed locals, function names in the `name`
  section, and a memory model, wrap-around and division that pass on the
  kernel.
- `llvm/` would have to solve all of that again (memory, GEP addresses,
  32-bit wrap, 7.02 syntax, a global class) on a large, version-dependent text
  format. Its gain, typed pointers and structs, is real but not an order of
  magnitude for pointer-based C.
- `ts/` is source-to-source translation, a different niche; frozen for now.

## Goal

On the C corpus and the QuickJS benchmark, compiled code within **3×** of
hand-written ABAP. Today Lars Hvam's hand-written zqjs is 4–60× faster than
QuickJS compiled from wasm on 12 of 16 scripts.

## Steps (each one PR, measured before and after)

1. **Profile.** Time per function on A4H (`GET RUN TIME`, background job) and
   osgo for the corpus and QuickJS; find where the time goes.
2. **Stack slots.** Declare only the slots a method uses (one QuickJS method
   declares 7,305, uses 19) and stop copying values through them: keep them
   in typed locals.
3. **Memory inline.** Loads and stores as `gv_mem+off(n)` and
   `REPLACE SECTION` in place, without a method call per access.
4. **Wrap only where needed.** Range analysis so loop counters and indices
   stay plain `i` without the 32-bit wrap helper.
5. **Shifts** by a constant as multiplication or `DIV` by a power of two
   instead of `DO` loops.
6. **Inline** small functions; fuse compare and branch; copy propagation.

## Rules for every step

- Answers equal the native ones: the corpus (`wasm/testdata/progs`) and
  QuickJS's 16 scripts, checked in wazero, osgo and OSG-JS, on the kernel
  for the final measurement.
- Generated code keeps the kernel rules: lines of at most 255 characters,
  nesting of at most 100, ABAP 7.02 plus int8, no comments, no writes to
  IMPORTING parameters.
- Each measurement records the OSG version (tag or commit) and the system.

## Later

- Use LLVM or DWARF information as a source of names and struct types for
  readable output, not as a second backend.
- The wider ideas are in [SUPER-PLANS.md](SUPER-PLANS.md).
