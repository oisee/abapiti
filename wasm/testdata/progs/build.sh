#!/usr/bin/env bash
# Rebuilds wasm/*.wasm (freestanding, no imports) and native-results.txt
# from the C sources. Needs clang with the wasm32 target and a host gcc.
set -euo pipefail
cd "$(dirname "$0")"
CLANG=${CLANG:-clang}
progs="base64 bf crc32 fnv fptr life mandel qsort queens recur sieve xorshift"
mkdir -p wasm
for p in $progs; do
  "$CLANG" --target=wasm32 -O2 -nostdlib -fno-builtin -Wl,--no-entry -Wl,--export-dynamic -o "wasm/$p.wasm" "$p.c" mem.c
done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for p in $progs; do
  gcc -O2 -o "$tmp/$p" "native/$p.c"
  "$tmp/$p"
done > native-results.txt
