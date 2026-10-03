# abapiti super-plans

Written 2026-10-03 from the question: "imagine nothing blocks us and wasm is
finished — what interesting can we build?" Not a queue. Ideas ranked by how
much they change what ABAP can do. Each line: the idea, why it matters, the
first small proof.

## 0. Production path: a typed IR, not wasm

wasm proved breadth (QuickJS, Lua, QR, donut bit for bit on the kernel) but is
slow by construction: a stack machine with i32/f64 only, every C local whose
address is taken lives in linear memory, every load/store is a helper call on
an xstring, i32 wrap needs helpers, control flow becomes DO/EXIT chains, and
one method declares thousands of stack slots. Hand-written ABAP (zqjs) is
4–60x faster than QuickJS compiled from wasm.

We already have the typed alternatives:
- **abapiti `llvm/`** (LLVM IR → ABAP, see [the 2026-03-29 report](history/reports/2026-03-29-003-llvm-abap-compilation-journey.md)): typed
  CLASS-METHODS with real signatures, `i`/`int8`/`f`, alloca → DATA, structs;
  34-function C corpus, FatFS 8,016 lines verified 5/5 on SAP, QuickJS
  537 methods / 124K lines (never run on SAP). Weak spot: basic blocks are a
  `CASE lv_block` string dispatch inside DO — slow, needs structured control
  flow recovery (Relooper / stackifier) into IF/DO/EXIT.
- **MinZ MIR2** (the MinZ compiler's `pkg/mir2abap`, `pkg/qbe2mir2`): one SSA for 8
  backends, ABAP among them; QBE IR can be lowered into MIR2. Today a toy for
  ABAP (REPORT + FORMs, GOTO-style branches, rejects sdiv/smod); valuable as a
  second, independent typed path and as a cross-check.

Decisive experiment (first thing after the freeze): the 12-program C corpus
(`wasm/testdata/progs`, PR #25) through both `compile wasm` and `compile llvm`,
same native answers, measured on A4H and osgo. If llvm is several times faster
on loops and memory, it becomes the production path and wasm stays the
compatibility path (any language, huge programs).

What a production-grade typed backend still needs: structured control flow;
non-escaping arrays as internal tables or typed fields instead of xstring;
structs without pointer arithmetic as ABAP structures; range analysis to drop
i32 wrap helpers; inlining; libc routines mapped to kernel statements; the same
kernel rules as today (lines <= 255, nesting <= 100, 7.02, no comments).

## 1. Self-hosting: abapiti inside SAP

- **abapiti compiled to wasm, running as ABAP.** Go (TinyGo) → wasm → abapiti →
  ABAP class `ZCL_ABAPITI`. The SAP system then turns any `.wasm` (upload,
  HTTP, table) into ABAP classes itself and activates them with
  `GENERATE SUBROUTINE POOL` / `INSERT REPORT` or via the ADT API.
  Proof: compile one fixture on the system, compare with the CLI output byte
  for byte.
- **abaplint inside SAP.** abaplint is JavaScript → our QuickJS-in-ABAP runs it
  → linting, formatting (pretty printer), downport 7.40 → 7.02 on the system,
  no Node. Proof: lint one class through ZCL_QJS, same findings as Node.
- **The open-abap transpiler inside SAP** → ABAP → JS → back: a round trip that
  checks itself.

## 2. Libraries the ABAP world never had (as plain classes)

- **SQLite in ABAP**: an in-memory SQL engine in a class — CSV/Excel uploads
  queried with real SQL, offline analysis, test fixtures, "DB in a variable".
- **Full regex** (PCRE2 or RE2): lookbehind, Unicode classes, named groups, same
  results as in the browser.
- **Compression**: zstd, brotli, xz next to the kernel's gzip.
- **Crypto without kernel extensions**: Monocypher (Ed25519, X25519,
  BLAKE2b) already runs; signed documents and verifiable audit trails in pure
  ABAP, also on BTP.
- **Documents**: libxlsxwriter (real .xlsx), a small PDF writer, md4c/cmark
  (Markdown → HTML for mails and docs), QR/barcodes for forms (QR is done).
- **Parsers**: tree-sitter grammars (parse JSON5, YAML, TOML, SQL, any
  language) inside ABAP; JSON Schema validation.
- **Images**: stb_image + resize (thumbnails for attachments, no GOS plugins).

## 3. ABAP Cloud / Steampunk

- **C and Rust libraries on BTP ABAP Environment**, where no kernel extension or
  native code is allowed: abapiti output is plain classes. Needs an ABAP Cloud
  dialect of the generator (released APIs only). This is probably the biggest
  business story.

## 4. One calculation engine, browser and SAP

- **Same .wasm in the Fiori app and in the backend**: pricing, tax, scoring,
  configuration rules — one binary, identical results on both sides,
  because wasm is deterministic. No more "the UI and the backend disagree".
- **Rust crates in ABAP**: serde, chrono, decimal, rule engines; Go via TinyGo;
  Zig; AssemblyScript for TypeScript developers.

## 5. Languages for business logic

- **Lua** (done, 8/8) and **QuickJS** (done) as embedded scripting in ABAP:
  customer formulas, user-defined rules, plug-ins, stored as text in tables.
- **Python** (MicroPython or a WASI CPython subset): data scripts in SAP.
- **Prolog** (Trealla) for configuration and eligibility rules; **Datalog** for
  authorisation analysis.
- **Bindings**: wasm imports mapped to ABAP calls — JS or Lua scripts that call
  BAPIs, read tables through a safe facade, write the application log.

## 6. Resumable computations

- **Hibernate a VM in a table**: the whole wasm memory is one xstring, so a Lua
  or JS interpreter can be saved after any step and resumed in another dialog
  step, job or system. Long calculations across LUW boundaries; coroutines
  across user interactions; "pause the job, ship it to another server".

## 7. abapiti as the ultimate oracle generator

- **Differential testing at scale**: Csmith/YARPGen random C → wasm → ABAP;
  wazero gives the expected answer; run on the kernel, osgo and OSG-JS. Every
  difference is a bug in one of them, found automatically, minimised by
  creduce. Today's manual hunt (zqjs, abap-wasm, division) becomes a nightly
  job feeding open-steamgate's inbox.
- **Kernel feature probes**: generate ABAP for every corner (int8 overflow,
  packed rounding, float specials) and record the kernel's answers as the
  reference set other runtimes test against.

## 8. Speed: make compiled wasm beat hand-written interpreters

- Today zqjs (hand-written ABAP) is 4–60× faster than QuickJS compiled from
  wasm. Goal: the reverse. Typed locals, only used stack slots, real loops
  instead of block/br, copy propagation, compare+branch fusion, memory access
  inlined, i32 kept in `i` where overflow cannot happen (range analysis),
  function inlining, profile-guided splitting.
- **Native hot paths**: replace known libc routines (memcpy, memset, strlen,
  qsort) with kernel statements on xstrings.

## 9. Demos that make people stop scrolling

- **Zork (Z-machine)** playable in SAP GUI — text adventure in SE38.
- **CHIP-8 / NES emulator** rendering into an ALV grid or HTML viewer.
- **Chess engine** (a small C engine) answering moves in a dialog.
- **DOOM** frames rendered as ASCII in the spool (the classic "it runs DOOM").
- **Live Mandelbrot / donut animation** in an HTML control.

## 10. The other direction

- **ABAP → wasm**: abaplint AST → wasm, so ABAP logic runs in the browser and
  in any wasm runtime; together with abapiti this closes the loop.
