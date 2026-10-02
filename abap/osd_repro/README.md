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

Lines marked `[A4H?]` still need the A4H oracle.
