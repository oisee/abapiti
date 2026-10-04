# QuickJS 2024-01-13 without SIMD

`qjs_eval.c` preserves the scratch driver and its 16 JavaScript scripts.
The corpus calls `qjs_eval(0)` through `qjs_eval(15)`; the driver also exports
`qjs_print`. `native/qjs_eval.c` checks the same scripts with host gcc and the
build compares its answers against the committed `native-results.txt`.

Run `./build.sh` here, or see [the corpus instructions](../README.md).
It downloads these official releases, verifying the SHA256 before extraction:

| Archive | SHA256 |
| --- | --- |
| [quickjs-2024-01-13.tar.xz](https://bellard.org/quickjs/quickjs-2024-01-13.tar.xz) | `3c4bf8f895bfa54beb486c8d1218112771ecfc5ac3be1036851ef41568212e03` |
| [wasi-sdk-34.0-x86_64-linux.tar.gz](https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-34/wasi-sdk-34.0-x86_64-linux.tar.gz) | `b761e3a0721dbae9c09a0059e5fdb2bf917d1b4a8a7b430fb3b5aafb0984b2c4` |

The wasm build is an `-O2 -mexec-model=reactor -Wl,--export-dynamic` WASI reactor
with a 1 MiB stack (`-Wl,-z,stack-size=1048576`), matching the scratch module:

```text
-mcpu=mvp -msign-ext -mbulk-memory -mmutable-globals
-mno-simd128 -mno-nontrapping-fptoint -mno-multivalue -mno-reference-types
```

It defines `_GNU_SOURCE`, `CONFIG_VERSION="2024-01-13"`, `EMSCRIPTEN` (disabling
QuickJS's OS atomics and stack checks), `_WASI_EMULATED_SIGNAL` and
`_WASI_EMULATED_PROCESS_CLOCKS`, and links the corresponding WASI emulation
libraries and libm. Sources are `qjs_eval.c`, `quickjs.c`, `libregexp.c`,
`libunicode.c`, `cutils.c`, and `libbf.c`. The last is required for core BigInt
references even without `CONFIG_BIGNUM`.

The script applies the scratch source's one-line portability fix in the
temporary tree: disable `CONFIG_PRINTF_RNDN` on WASI, whose fenv headers do not
provide the directed rounding modes this path requires. Native builds retain
that path. Relative source filenames avoid embedding temporary directory names
in assertion strings. It links as `qjs_bench.wasm` to preserve the scratch
module's name section, then moves the output to `quickjs.wasm`.

Expected scratch `qjs_bench.wasm` SHA256:
`a181744b10153ecf37143da30dd7e7df1112bc54fdde8c27a2ccc5a76aef6406`

The local scripted build with the pinned wasi-sdk matches this hash exactly.
The script prints and compares the output hash, warning on mismatch without
failing. The native and wazero answers must still match all 16 committed results.
