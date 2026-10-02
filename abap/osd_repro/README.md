# OSD compatibility repros

Small ABAP Unit classes (ABAP 7.02, ASCII) that pin the kernel behaviour
ABAPiti's generated code relies on. They are handed to the open-steamgate
team as tests; A4H is the reference, and where it disagrees with an
expectation here, A4H wins and the expectation is fixed.

| Class | Item | What |
|---|---|---|
| `zcl_abapiti_repro_bytes` | 1, 2 | `REPLACE SECTION` / `FIND ... IN BYTE MODE` on xstring |
| `zcl_abapiti_repro_x4i` | 4 | `x` <-> `i` conversion (sign, byte order, truncation) |
| `zcl_abapiti_repro_ovfl` | 3 | `TYPE i` overflow raises |
| `zcl_abapiti_repro_mem` | 1, 4 | the exact memory helpers from `wasm/memhelpers.go` |

All 26 expectations were measured green on A4H on 2026-10-02 (`EXPECT = A4H`).

On OSD vscode-v0.6.1511 and on main (JS runtime): 18 of 26 pass. Failing:
`FIND ... IN BYTE MODE` (f1, f2, f4) and every `TYPE i` overflow case except
the boundaries. On osgo `REPLACE SECTION ... IN BYTE MODE` does not compile yet.
