# Memory-model benchmark

`zcl_abapiti_bench_mem` compares three ways to hold WASM linear memory in
ABAP on the same workloads, with the same dispatch cost:

| Model | Memory | i32 store |
|---|---|---|
| A | `xstring` | `REPLACE SECTION ... IN BYTE MODE` (today's generated code) |
| B | `STANDARD TABLE OF x LENGTH 4096` | `READ TABLE ... ASSIGNING <row>`, `<row>+off(4) = ...` in place |
| C | as B | `ASSIGN <row>+off(4) TO <i> CASTING` (platform byte order) |

Workloads: W1 byte fill and sum (16384 bytes), W2 aligned i32 store and
load (4096), W3 unaligned i32 across row boundaries (2048), W4 recursion
(fib 20). The ABAP Unit class checks correctness; run the class as a console
app (`if_oo_adt_classrun`) for timings (`GET RUN TIME`).

First run, OSD vscode-v0.6.1511 (JS runtime), `GET RUN TIME` units as
reported (they look like milliseconds there, not microseconds):

| | A | B | C |
|---|---|---|---|
| W1 | 5608 | 157 | 118 |
| W2 | 1331 | 59 | error: Conversion no number |
| W3 | 838 | 21 | error: Conversion no number |

B is 23-40x faster than A. C fails on OSD: a write through a field symbol
assigned with `CASTING TYPE i` does not reach the bytes.
