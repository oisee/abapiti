# Monocypher 4.0.3

`mono.c` and `native/mono.c` preserve the measured driver: ten checks covering
BLAKE2b, X25519, ChaCha20, Poly1305 and EdDSA, each returning an i32 checksum.
`mem.c` supplies freestanding memcpy/memset, as in `../../progs/build.sh`.

Run `./build.sh` here, or see [the corpus instructions](../README.md).
`CLANG` selects the compiler (default `clang`); gcc rebuilds
`native-results.txt`. The wasm build uses:

```text
--target=wasm32 -O2 -nostdlib -fno-builtin -Wl,--no-entry -Wl,--export-dynamic
```

Official archive: [monocypher-4.0.3.tar.gz](https://monocypher.org/download/monocypher-4.0.3.tar.gz)

Pinned archive SHA256:
`8cc9bc341a66249016db9bd70e9142d8d0aef9945973744b1ac05dbc55d8ee66`

Expected `mono.wasm` SHA256 (scratch build, Ubuntu clang 18.1.3):
`d57ed7233203a0cad226e5fc7fc0a79347622d997372bc8f846cf26556ccce8b`

The local scripted build with Ubuntu clang 18.1.3 matches this hash exactly.
The script prints and compares it, warning on mismatch without failing.
