# C test corpus

Twelve small freestanding C programs (no libc, no imports): qsort, base64,
CRC-32, FNV-1a 64, xorshift64*, Life, Mandelbrot, N-queens, sieve, recursion,
function pointers and a Brainfuck interpreter. Every export returns one
checksum, so a check compares one number.

- `*.c`, `common.h`, `mem.c`: the programs; `native/*.c`: native drivers.
- `wasm/*.wasm`: the built modules, committed so no C toolchain is needed.
- `cases.txt`: one check per line, `program export args...` (38 checks).
- `native-results.txt`: the native answer for every check.
- `build.sh`: rebuilds the modules and the native answers.

`go test ./wasm -run TestProgsCorpus` runs every check in wazero against the
native answers. With `ABAPITI_TEST_OUT=<dir>` it also writes the generated
ABAP classes and their ABAP Unit test classes there, one class per program,
plus a copy of each module (`<program>.wasm`), for running them elsewhere.
