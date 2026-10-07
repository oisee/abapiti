# Plan: make the WebAssembly path fast

Decided 2026-10-03. Only the WebAssembly path is in a verification pipeline
(CI on wazero, osgo and OSG-JS, runs on a real kernel). `llvm/` has typed
locals, memory helpers, GEP lowering and a class layout, and five small C
functions ran once on SAP; the committed TypeScript lexer (`ts/testdata`)
passes a test on OSG-JS. But a smoke test on 2026-10-03 compiled none of
`llvm/`'s output for the 12-program C corpus on osgo or OSG-JS (memory helpers
emitted as FORMs and called as methods, array indices lost in GEP, raw IR
text in expressions, 32-bit arithmetic in `TYPE i`), and fresh `ts/` output
for a small sample did not compile either. So we keep WebAssembly and make it
fast, instead of switching IR.

## Why WebAssembly, not LLVM IR

- WebAssembly already gives what `llvm/` still lacks: structured control flow
  (`block`/`loop`/`if` map to `IF`/`DO`/`EXIT`; `llvm/` dispatches basic blocks
  through a `CASE` on a string), typed locals, function names in the `name`
  section, and a memory model, wrap-around and division that pass on the
  kernel.
- `llvm/` would have to make all of that correct (memory, GEP addresses,
  32-bit wrap, 7.02 syntax, a global class instead of a report) on a large, version-dependent text
  format. Its gain, typed pointers and structs, is real but not an order of
  magnitude for pointer-based C.
- `ts/` is source-to-source translation, a different niche; it is replaced
  by its own track, [TS-HIR-PLAN.md](TS-HIR-PLAN.md).

## Goal

On the C corpus and the QuickJS benchmark, compiled code within **3×** of
hand-written ABAP. Measured on A4H on 2026-10-03 (record to follow in
docs/history): Lars Hvam's hand-written zqjs is 4–60× faster than QuickJS
compiled from wasm on 12 of our 16 scripts (8.6× slower on sorting). QuickJS
timings: docs/history/2026-10-03-quickjs-speed.md (PR #24).

## Steps (each one PR, measured before and after)

1. **Profile.** Time per function on A4H (`GET RUN TIME`, background job) and
   osgo for the corpus and QuickJS; find where the time goes.
2. **Stack slots.** Declare only the slots a method uses (a review of the QuickJS
   split found methods that declare thousands of slots and use a few dozen;
   step 1 counts declared against used slots for every method) and stop copying values through them: keep them
   in typed locals.
3. **Memory inline.** Loads and stores as `gv_mem+off(n)` and
   `REPLACE SECTION` in place, without a method call per access.
4. **Wrap only where needed.** Range analysis so loop counters and indices
   stay plain `i` without the 32-bit wrap helper.
5. **Shifts** by a constant as multiplication or `DIV` by a power of two
   instead of `DO` loops.
6. **Inline** small functions; fuse compare and branch; copy propagation.

## Rules for every step

- Answers equal the native ones: the corpus (`wasm/testdata/progs`, added by PR #25) and
  QuickJS's 16 scripts, checked in wazero, osgo and OSG-JS, on the kernel
  for the final measurement.
- Generated code keeps the kernel rules: lines of at most 255 characters,
  nesting of at most 100, ABAP 7.02 plus int8, no comments, no writes to
  IMPORTING parameters.
- Each measurement records the OSG version (tag or commit) and the system.

## Later

- Use LLVM or DWARF information as a source of names and struct types for
  readable output, not as a second backend.
- The wider ideas are in [SUPER-PLANS.md](SUPER-PLANS.md) (added by PR #28).
