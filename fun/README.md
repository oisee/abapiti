# fun/ — Try It Now

Build vsp once, then play:
```bash
go build -o vsp ./cmd/vsp
```

---

## 1. C → ABAP (one command)

```bash
# Your function → typed ABAP
echo 'int square(int x) { return x * x; }' > /tmp/sq.c
./vsp compile llvm /tmp/sq.c
# → CLASS-METHODS square IMPORTING a TYPE i RETURNING VALUE(rv) TYPE i.

# With ZIP for SAP import
./vsp compile llvm /tmp/sq.c --class zcl_math --zip -o math.zip

# Fibonacci, factorial, gcd, lerp
./vsp compile llvm fun/examples/fibonacci.c --class zcl_fib

# FatFS filesystem (7249 lines C → 8016 lines ABAP)
./vsp compile llvm ~/dev/minz/examples/c89/fatfs/ff.c --class zcl_fatfs --zip

# QuickJS JS engine (55K lines C → 124K lines ABAP, 537 functions)
./vsp compile llvm /tmp/quickjs-2024-01-13/quickjs.c --class zcl_qjs --zip
```

## 2. LLVM IR directly

```bash
# Compile C to LLVM IR yourself, then to ABAP
clang -S -emit-llvm -O1 mycode.c -o mycode.ll
./vsp compile llvm mycode.ll --class zcl_mycode

# Rust → LLVM IR → ABAP (needs rustc)
echo 'pub fn add(a: i32, b: i32) -> i32 { a + b } fn main(){}' > /tmp/r.rs
rustc --emit=llvm-ir /tmp/r.rs -o /tmp/r.ll
./vsp compile llvm /tmp/r.ll --class zcl_rust
```

## 3. WASM → ABAP

```bash
# Any .wasm file
./vsp compile wasm pkg/wasmcomp/testdata/factorial.wasm --class zcl_fact

# Available test WASMs:
ls pkg/wasmcomp/testdata/*.wasm
# add, factorial, fibonacci, collatz, sum_to, pow2, suite, extended
```

## 4. TypeScript → ABAP (via Porffor → WASM)

```bash
npm install -g porffor
echo 'const x: number = 2 + 3;' > /tmp/demo.ts
npx porffor wasm /tmp/demo.ts /tmp/demo.wasm -t
./vsp compile wasm /tmp/demo.wasm --class zcl_ts_demo
```

## 5. ABAP offline tools (no SAP needed)

```bash
# Lint
./vsp lint --file myprogram.abap

# Parse to AST
./vsp parse --file myprogram.abap --format json

# Tokenize
echo "DATA lv TYPE i. lv = 42." | ./vsp parse --stdin
```

## 6. SAP tools (needs SAP_URL/SAP_USER/SAP_PASSWORD)

```bash
# Execute ABAP
./vsp execute "DATA(x) = 2 + 2. WRITE x."

# Search code
./vsp grep "FACTORIAL" --package '$TMP'

# Run unit tests
./vsp test CLAS ZCL_MY_CLASS

# System info
./vsp system info
```

## 7. Pre-built ZIPs (import via abapGit)

```
pkg/llvm2abap/output/zllvm_test_01.zip   — 34 functions + 17 unit tests
pkg/llvm2abap/output/quickjs_llvm.zip    — QuickJS 537 functions, 124K lines
```

---

## What gets compiled

| Source | Command | Output |
|--------|---------|--------|
| `int add(int a, int b)` | `vsp compile llvm` | `CLASS-METHODS add IMPORTING a TYPE i b TYPE i RETURNING VALUE(rv) TYPE i` |
| `double lerp(...)` | `vsp compile llvm` | `IMPORTING a TYPE f b TYPE f ... RETURNING VALUE(rv) TYPE f` |
| `long long add64(...)` | `vsp compile llvm` | `IMPORTING a TYPE int8 ... RETURNING VALUE(rv) TYPE int8` |
| `.wasm binary` | `vsp compile wasm` | `FORM f0 USING p0 TYPE i CHANGING rv TYPE i` |
| TypeScript `.ts` | porffor → `vsp compile wasm` | WASM-style ABAP |

## Quick comparison: same function, two paths

```bash
# Path 1: C → LLVM → ABAP (typed, clean)
./vsp compile llvm fun/examples/fibonacci.c --class zcl_fib

# Path 2: C → WASM → ABAP (untyped, assembly-like)
clang --target=wasm32 -O1 -nostdlib -Wl,--no-entry \
  -Wl,--export=fibonacci fun/examples/fibonacci.c -o /tmp/fib.wasm
./vsp compile wasm /tmp/fib.wasm --class zcl_fib_wasm
```
