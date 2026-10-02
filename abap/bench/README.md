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

A4H (release 758, x86-64), 3rd of 3 runs in one session, microseconds,
1 page (measured by osg-research, 2026-10-02):

| | A | B |
|---|---|---|
| W1 | 7878 | 14024 |
| W2 | 2889 | 5877 |
| W3 | 1445 | 2946 |

On the kernel the picture is reversed: xstring + REPLACE SECTION is about
2x faster than the table once warm, so the generated code stays on xstring.
B's lead on OSD is a property of the JS runtime. Model C dumps on the kernel
(ASSIGN_BASE_WRONG_ALIGNMENT, already at offset 0), so it is out of the
console run and its tests live in their own test class.

The console run now also measures W0 (allocation) and W5 (one i32 per 4 KiB
over the whole memory) at sizes up to 8192 pages (512 MiB); `run_one( )`
measures one model at one size.
