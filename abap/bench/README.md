# Memory-model benchmark

`zcl_abapiti_bench_mem` compares three ways to hold WASM linear memory in
ABAP on the same workloads, with the same dispatch cost:

| Model | Memory | i32 store |
|---|---|---|
| A | `xstring` | `REPLACE SECTION ... IN BYTE MODE` (today's generated code) |
| B | `STANDARD TABLE OF x LENGTH 4096` | `READ TABLE ... ASSIGNING <row>`, `<row>+off(4) = ...` in place |
| C | as B | `ASSIGN <row>+off(4) TO <i> CASTING` (platform byte order) |

Workloads: W0 allocation, W1 byte fill and sum (16384 bytes), W2 aligned i32
store and load (4096), W3 unaligned i32 across row boundaries (2048), W5 one
i32 per 4 KiB over the whole memory, W4 recursion (fib 20). The ABAP Unit
class checks correctness for A and B (checksums, byte order, row crossing);
run the class as a console app (`if_oo_adt_classrun`) for timings
(`GET RUN TIME`); `run_one( model, pages )` measures one model at one size.
Model C has no unit tests: it dumps on a kernel and would abort the class.

First run, OSD vscode-v0.6.1511 (JS runtime), `GET RUN TIME` units as
reported (they look like milliseconds there, not microseconds):

| | A | B | C |
|---|---|---|---|
| W1 | 5608 | 157 | 118 |
| W2 | 1331 | 59 | error: Conversion no number |
| W3 | 838 | 21 | error: Conversion no number |

B is 23-40x faster than A at 1 page. C fails on OSD with "Conversion no
number" on W2 and W3 (cause not investigated).

A4H (release 758, x86-64), 3rd of 3 runs in one session, microseconds,
1 page (measured by osg-research, 2026-10-02):

| | A | B |
|---|---|---|
| W1 | 7878 | 14024 |
| W2 | 2889 | 5877 |
| W3 | 1445 | 2946 |

On the kernel the picture is reversed: xstring + REPLACE SECTION is about
2x faster than the table on W1-W3 once warm, so the generated code stays on
xstring.
B's lead on OSD is a property of the JS runtime. Model C dumps on the kernel
(ASSIGN_BASE_WRONG_ALIGNMENT, already at offset 0), so it is out of the
console run and its tests live in their own test class.

The console run now also measures W0 (allocation) and W5 (one i32 per 4 KiB
over the whole memory) at sizes up to 8192 pages (512 MiB); `run_one( )`
measures one model at one size.

A4H scaling, `run_one( )` per size, each in its own session, microseconds
(osg-research, 2026-10-02, commit 3eb3796). All checksums correct, no memory
dumps up to 8192 pages. Checksums were compared by hand with the closed
forms in each run:

| model | pages | W0 | W1 | W2 | W3 | W5 |
|---|---|---|---|---|---|---|
| A | 1 | 35 | 19761 | 6252 | 3174 | 30 |
| A | 16 | 636 | 17121 | 6292 | 3275 | 410 |
| A | 256 | 8669 | 17093 | 4926 | 1431 | 2859 |
| A | 1024 | 37204 | 7760 | 2847 | 1432 | 11511 |
| A | 8192 | 359255 | 7631 | 2836 | 1433 | 92198 |
| B | 1 | 12 | 30962 | 12159 | 5119 | 45 |
| B | 16 | 118 | 30118 | 10901 | 5076 | 633 |
| B | 256 | 1573 | 29591 | 8776 | 2920 | 5870 |
| B | 1024 | 7006 | 28757 | 5869 | 2953 | 24799 |
| B | 8192 | 48486 | 13973 | 5825 | 2931 | 201832 |

A's W1-W3 do not grow with the memory size (they even fall about 2x from 1
to 8192 pages), which is consistent with the kernel replacing a same-length
section in place; that mechanism is inferred, not measured. A is 1.5-3.7x
faster than B on W1-W3 (W1 is noisy: it drops at large sizes for both
models, reason unknown). Against A: allocation (W0) is 3-7x slower than B
(359255 vs 48486 us at 8192 pages); the generated code will build memory by
doubling instead of 256-byte steps. The whole-memory sweep (W5) favours A,
about 2.2x.

OSD 0.6.1511 (JS), same console run (units as OSD reports them):

| model | pages | W1 | W2 | W3 |
|---|---|---|---|---|
| A | 1 | 5467 | 1386 | 691 |
| A | 4 | 23000 | 4898 | 2845 |
| A | 16 | 83873 | 16668 | 6467 |
| B | 1 | 106 | 43 | 19 |
| B | 1024 | 49 | 8 | (negative) |

On OSD A grows linearly with the size (every write copies the memory).
OSD timings are coarse (they look like milliseconds) and some differences
come out negative: its `GET RUN TIME` is unreliable, so read only the ratios,
and W3 at 1 page (691 vs 19) only roughly.
