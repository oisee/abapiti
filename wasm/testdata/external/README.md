# External WebAssembly corpus

These drivers reproduce the measured Monocypher and QuickJS modules from a
checkout of abapiti. Third-party sources, toolchains, archives and built modules
are not committed. Builds fetch pinned official releases; tests never use the
network.

From the repository root:

```sh
wasm/testdata/external/monocypher/build.sh
wasm/testdata/external/quickjs/build.sh
go test ./wasm/ -run '^TestExternalModules$' -count=1 -v
ABAPITI_TEST_OUT=/tmp/external-abap go test ./wasm/ -run '^TestExternalModules$' -count=1 -v
```

The scripts require Bash, curl, tar (gzip/xz), sha256sum, gcc and standard Unix
utilities. Monocypher additionally needs clang with wasm32 support and wasm-ld.
QuickJS downloads the pinned wasi-sdk for **x86_64 Linux**. Archives are cached
in `.cache/` and checked on every build; extraction and native executables live
in temporary directories cleaned on exit. Once the cache is populated, builds
also work offline.

Default outputs are `wasm/mono.wasm` and `wasm/quickjs.wasm` under this directory.
Set `ABAPITI_EXTERNAL_DIR` to an absolute directory to use another output
location for both scripts and tests. Without built modules, each missing module
gets a clear test skip. `ABAPITI_TEST_OUT` exports beneath
`TestExternalModules/monocypher/` and `TestExternalModules/quickjs/`: a module,
generated class files and an ABAP Unit test class using the committed native
answers. QuickJS uses the CLI's `--split` implementation and default 20,000-line
budget, including state, chunk classes and interfaces.

Each fixture has `cases.txt` (`module export args...`), `native-results.txt`
(`module export args... = answer`), a C driver, a native driver and `build.sh`.
The builds print native answers and module SHA256 values. A module hash mismatch
warns without failing, since compiler versions or build metadata can change
the bytes; the Go test verifies the answers independently.

## Licences of the downloaded code

Nothing below is committed; `build.sh` downloads it into the ignored cache.

- Monocypher 4.0.3: dual-licensed BSD-2-Clause or CC0-1.0 (see `LICENCE.md` in its release).
- QuickJS 2024-01-13 (Fabrice Bellard, Charlie Gordon): MIT (see `LICENSE` in its release).
- wasi-sdk 34.0: Apache-2.0 with the LLVM exception; its bundled wasi-libc and
  other components carry their own notices inside the release archive.

The drivers in this directory (`mono.c`, `qjs_eval.c`, the native drivers)
are part of abapiti and use its licence.
