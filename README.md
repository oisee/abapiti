# ABAPiti

**ABAPiti: because we can**
*a new identity for your code*

## Why? Because we can.

ABAPiti compiles WebAssembly, LLVM IR (so C, and in principle anything clang or rustc can lower) and TypeScript **into** ABAP, in the demoscene spirit of doing something just to see that it runs.
It is the reverse direction of [abaplint/transpiler](https://github.com/abaplint/transpiler), which takes ABAP out to JavaScript; ABAPiti brings other languages in.
None of this is production software. It is a research toy that happens to produce ABAP a real SAP system accepts.

## What?

One Go module, one CLI (`abapiti`), several frontends. Status as of the split (2026-10):

| Frontend | Input → output | What works today | What does not (yet) |
|---|---|---|---|
| **`wasm/`** (WASM → ABAP, Go) | `.wasm` → a global ABAP class (`METHODS` per export, linear memory as `xstring`). Backends: class, function group, hybrid, multi-class. | Unit tests pass. The 12-function suite (add, factorial, fibonacci, gcd, is_prime, abs, max, min, negate, sum_to, collatz, pow2) parses and compiles. QuickJS (Javy, 1.2 MB wasm, 1,410 functions) compiles to ~21 MB of ABAP. The tests run the inputs natively in [wazero](https://wazero.io) as a reference. add and factorial were verified on a real SAP system via `GENERATE SUBROUTINE POOL`. | QuickJS does **not** execute (needs a parse fix and WASI stubs). porffor-produced inputs compile but return 0. Not yet checked on OSD: the memory helpers write into an `xstring` at an offset, which a real kernel may reject for strings. i32 wrap-around vs ABAP `TYPE i` overflow is not handled. |
| **`abap/wasm_compiler/`** (WASM → ABAP, written in ABAP) | `.wasm` xstring → ABAP source, then `GENERATE SUBROUTINE POOL` on SAP | Self-hosting run on SAP (a 785-line compiler); it has an ABAP Unit class. | Not packaged for installation. Depends on `GENERATE SUBROUTINE POOL`, which OSD most likely cannot run. |
| **`llvm/`** (LLVM IR / C → ABAP) | `.ll` or `.c` (via `clang`) → typed `CLASS-METHODS`; optional abapGit zip; multi-class split | 3 tests pass (a 34-function C corpus, leaf functions, control flow). FatFS: 28 functions → 8,016 lines, 0 TODOs, verified 5/5 on SAP via `GENERATE SUBROUTINE POOL`. QuickJS: 537 methods / 124 K lines, 0 TODOs. | QuickJS-as-ABAP is verified only natively (via `llc`), not on SAP. Test coverage is thin. `.c` input needs `clang` on the PATH. |
| **`ts/`** (TypeScript → ABAP) | TS → JSON AST (node + `ts/ts_ast.js`) → ABAP classes | A lexer modelled on abaplint's was transpiled to ABAP (55 classes in `ts/testdata/abaplint_lexer/`) and ran on SAP. | Needs node and `npm install` at runtime, and finds `ts_ast.js` only from a source checkout (or `ABAPITI_TS_AST_PATH`). Its test skips without the TypeScript toolchain. |
| **`ts2go/`** (TypeScript → Go) | TS AST → Go | Produced valid Go from the same lexer. | No tests, no caller. |
| **`jseval/`** + **`abap/jseval/`** | A JavaScript interpreter in Go (the oracle) and in ABAP (`ZCL_JSEVAL*`) | The Go side passes 109 subtests; the ABAP side ran on SAP (v0.3.3). | No harness yet runs both on the same inputs. |
| Go → ABAP | | Does not exist. | |

Other bits: `cmd/abapgit-pack/` packs `.abap` files into an abapGit zip, `abap/abaplint-lexer.zip` is the transpiled lexer as an abapGit package, and `fun/` has examples and the QuickJS-on-SAP write-up.

### Usage

```sh
go install github.com/oisee/abapiti/cmd/abapiti@latest

abapiti compile wasm add.wasm -o out/          # out/zcl_wasm_add.clas.abap
abapiti add.wasm                               # shortcut for "compile wasm", to stdout
abapiti compile llvm prog.c --class zcl_prog   # needs clang
abapiti compile llvm prog.ll --zip -o prog.zip # abapGit zip
abapiti compile ts lexer.ts -o out/            # needs node + npm install, from a checkout
```

ABAPiti never talks to SAP. Deploy the output with [vsp](https://github.com/oisee/vibing-steampunk) (`vsp deploy out/zcl_wasm_add.clas.abap '$TMP'`) or abapGit.

## How it's verified

Today: Go unit tests (`go test ./...`), including a native run of the WASM inputs in wazero as the reference, plus manual runs on SAP systems recorded in [`docs/history/`](docs/history/).

The target pipeline, per test case:

1. Compile the input with `abapiti` and generate an ABAP Unit class whose expected values come from a native run (wazero for WASM, clang for C).
2. Deploy both with `vsp` to the [open-steamgate](https://github.com/oisee/open-steamgate) emulator (OSD), activate, and run the ABAP Unit tests.
3. Compare the ABAP results with the native ones; any difference fails the build.

OSD activates code through the abaplint transpiler, so a green OSD run proves the generated ABAP passes abaplint and behaves correctly under that runtime. It does not prove kernel behaviour (`TYPE i` overflow, `xstring` offset writes); that stays a manual check on a real system.

## Roadmap

- **M1:** `abapiti compile wasm` output for **add** and **factorial**, plus the generated ABAP Unit class, deploys, activates and runs green on a pinned OSD in CI. First check whether the memory helpers activate on OSD at all.
- **M2:** the whole 12-function WASM suite and the 34-function LLVM corpus under the same harness.
- **M3:** the TypeScript-transpiled lexer vs the `@abaplint/core` lexer on the same inputs.
- **M4:** jseval (Go) and ZCL_JSEVAL (ABAP) conformance.

## History

ABAPiti was split out of [oisee/vibing-steampunk](https://github.com/oisee/vibing-steampunk) (vsp) with its git history: the commits that touched the transpilers keep their original authors and dates, trimmed to the moved paths. The design notes and session logs from that time are in [`docs/history/`](docs/history/); paths there still say `pkg/wasmcomp`, `pkg/llvm2abap` and `vsp compile`, which are now `wasm/`, `llvm/` and `abapiti compile`.

## License

MIT, see [LICENSE](LICENSE). Third-party notices are in [NOTICE](NOTICE).
