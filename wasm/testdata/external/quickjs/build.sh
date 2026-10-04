#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
source ../build-common.sh
download quickjs-2024-01-13.tar.xz \
  https://bellard.org/quickjs/quickjs-2024-01-13.tar.xz \
  3c4bf8f895bfa54beb486c8d1218112771ecfc5ac3be1036851ef41568212e03
download wasi-sdk-34.0-x86_64-linux.tar.gz \
  https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-34/wasi-sdk-34.0-x86_64-linux.tar.gz \
  b761e3a0721dbae9c09a0059e5fdb2bf917d1b4a8a7b430fb3b5aafb0984b2c4
tar -xJf "$cache/quickjs-2024-01-13.tar.xz" -C "$tmp"
tar -xzf "$cache/wasi-sdk-34.0-x86_64-linux.tar.gz" -C "$tmp"
src="$tmp/quickjs-2024-01-13"
# Match the scratch build's WASI portability fix: WASI has no directed
# floating-point rounding modes used by CONFIG_PRINTF_RNDN.
sed -i 's/^#if !defined(_WIN32)$/#if !defined(_WIN32) \&\& !defined(__wasi__)/' "$src/quickjs.c"
sources=(qjs_eval.c "$src/quickjs.c" "$src/libregexp.c" "$src/libunicode.c" "$src/cutils.c" "$src/libbf.c")
cp qjs_eval.c "$src/qjs_eval.c"
(
  cd "$src"
  "$tmp/wasi-sdk-34.0-x86_64-linux/bin/clang" \
    -mcpu=mvp -msign-ext -mbulk-memory -mmutable-globals \
    -mno-simd128 -mno-nontrapping-fptoint -mno-multivalue -mno-reference-types \
    -O2 -mexec-model=reactor -Wl,--export-dynamic -Wl,-z,stack-size=1048576 \
    -D_GNU_SOURCE -DCONFIG_VERSION='"2024-01-13"' -DEMSCRIPTEN \
    -D_WASI_EMULATED_SIGNAL -D_WASI_EMULATED_PROCESS_CLOCKS \
    qjs_eval.c quickjs.c libregexp.c libunicode.c cutils.c libbf.c \
    -lwasi-emulated-signal -lwasi-emulated-process-clocks \
    -lm -o "$tmp/qjs_bench.wasm"
)
mv "$tmp/qjs_bench.wasm" "$output/quickjs.wasm"
gcc -O2 -Wno-attributes -D_GNU_SOURCE -DCONFIG_VERSION='"2024-01-13"' \
  -I"$src" native/qjs_eval.c "${sources[@]}" -lm -lpthread -ldl -o "$tmp/native"
"$tmp/native" > "$tmp/native-results.txt"
# Keep the measured answers independent of the freshly built module.
diff -u native-results.txt "$tmp/native-results.txt"
cp "$tmp/native-results.txt" native-results.txt
cat native-results.txt
module_checksum "$output/quickjs.wasm" a181744b10153ecf37143da30dd7e7df1112bc54fdde8c27a2ccc5a76aef6406
