# 2026-10-03: QR, Lua, and donut demos on A4H

Three WebAssembly-to-ABAP demos run on the real kernel. Generator: the
WebAssembly path (`abapiti compile wasm`), with the signed-division fix from
PR #23 where noted.

## QR

### Input

- Nayuki's qrcodegen (MIT), compiled to 16 KB of WebAssembly.
- QR version 3, 29×29 modules, for `https://github.com/oisee/abapiti`.
- The native C rendering was the comparison oracle.

### Output

`abapiti compile wasm` produced class `ZCL_ABAPITI_T_QR` (8.8K lines) and
report `ZABAPITI_QR`, which prints the QR code to the spool.

### A4H (SAP kernel 7.58), 2026-10-03

Encoding took 0.29 s in a background job. The matrix was identical to the
native C rendering bit for bit. The signed-division fix from
`fix/signed-div` commit `73f5c1e` produced the identical matrix again.

Rendering used full blocks (`U+2588`) and spaces with a four-module quiet
zone; a phone scanned the spool output.

The throwaway package was deleted afterwards. At Alice's request, package
`$ZABAPITI_QR` (`ZCL_ABAPITI_QR` and `ZABAPITI_QR`) was then installed
permanently. Its background run had status `F`, and its matrix was also
identical to the native rendering.

### osgo

Not recorded in the available facts.

## Lua 5.4

### Input

- Lua 5.4 interpreter, compiled to 710 KB of WebAssembly.
- Eight ABAP Unit tests, each driving Lua scripts.

### Output

`abapiti compile wasm --split` produced five chunk classes, five interfaces,
one state class, and facade `ZCL_LUA`.

### A4H (SAP kernel 7.58), 2026-10-03

The eight ABAP Unit tests passed 8/8. One fix was needed: translating the
float constant `math.huge = HUGE_VAL` (Infinity) traps by design, so Lua was
rebuilt with `lmathlib_abap.c` using `DBL_MAX`.

### osgo

The eight tests also passed 8/8 at the osgo CI pin below.

## donut.c

### Input

- Andy Sloane's `donut.c`, using doubles and `sin`/`cos` from the WASI libm.
- wazero's character rendering was the comparison oracle.

### Output

Report `ZABAPITI_DONUT` renders frame 30 to the spool.

### A4H (SAP kernel 7.58), 2026-10-03

Frame rendering took 5.32 s per frame. The 1,760 output characters were
identical to wazero character for character.

The first run printed a blank picture because a character-conversion line was
lost in the demo report. Its checksum already matched wazero:
`-1227956848`.

### osgo

Not recorded in the available facts.

## OSG versions

- osgo ran at the abapiti CI pin `ad3d1e87cd3c3545b32dd4ba2ddeceb25708f05f`;
  `.github/ci/osgo.ref` contained that pin at the merges of PR #22 and PR #23.
- OSG-JS version 0.6.1531 was the pinned OSD release used by CI on
  2026-10-03 (PR #6 pinned `vscode-v0.6.1531`).

## Practical files

- `docs/img/qr-a4h.png`: README QR image; it decodes to the repository URL.
- `docs/img/donut-a4h.png`: README donut image.

## Not in the repository yet

- QR source, driver, build script, generated ABAP, and expected matrix.
- Lua source, `lmathlib_abap.c`, driver, build script, generated ABAP, and
  test scripts.
- donut source, driver, build script, generated ABAP, and expected frame
  output.
