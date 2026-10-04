#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
source ../build-common.sh
download monocypher-4.0.3.tar.gz \
  https://monocypher.org/download/monocypher-4.0.3.tar.gz \
  8cc9bc341a66249016db9bd70e9142d8d0aef9945973744b1ac05dbc55d8ee66
tar -xzf "$cache/monocypher-4.0.3.tar.gz" -C "$tmp"
src="$tmp/monocypher-4.0.3/src"
"${CLANG:-clang}" --target=wasm32 -O2 -nostdlib -fno-builtin \
  -Wl,--no-entry -Wl,--export-dynamic -I"$src" \
  -o "$output/mono.wasm" mono.c "$src/monocypher.c" mem.c
gcc -O2 -I"$src" -o "$tmp/native" native/mono.c mono.c "$src/monocypher.c"
"$tmp/native" > "$tmp/native-results.txt"
cp "$tmp/native-results.txt" native-results.txt
cat native-results.txt
module_checksum "$output/mono.wasm" d57ed7233203a0cad226e5fc7fc0a79347622d997372bc8f846cf26556ccce8b
