# Tech debt

Things we decided to ship as a draft or to leave out for now, so the main
path (WebAssembly → ABAP that runs correctly on a real kernel, QuickJS first)
keeps moving. Each line says what is missing, why it is acceptable today, and
where the decision or the evidence lives. Remove a line when it is fixed.

## Code generation

| What | Today | Why it is acceptable | Where |
|---|---|---|---|
| f32 arithmetic | Computed in double precision; results can differ from IEEE f32 rounding (e.g. 2147483520 + 64 then trunc_sat). | Pre-existing in the whole f32 model; C programs rarely depend on f32 rounding. Pinned by a test. | feat/trunc-sat |
| NaN and ±Inf | ABAP `TYPE f` cannot hold them. Constants and reinterpreted bit patterns trap instead of computing. | Honest failure instead of a wrong value. JavaScript engines (QuickJS) will need a real answer here. | feat/trunc-sat |
| Exceptions (`try`/`catch`/`throw`) | Trap. | C does not need them; C++ and Rust unwinding do. | wasm/codegen.go |
| Tail calls (`return_call`) | Emitted as a call plus `RETURN`. | Correct, but deep tail recursion can exhaust the ABAP stack. | wasm/codegen.go |
| SIMD, atomics, memory64, GC | Compilation fails with a clear error. | We build our own modules without them (QuickJS: wasi-sdk `-mcpu=mvp +sign-ext +bulk-memory`). | feat/trunc-sat |
| Multi-value results across split classes | Rejected with a compile error in split mode. | Not used by our modules so far. | feat/multi-class |
| FUGR backend | Keeps PERFORM helpers; known issues with `RETURN` of results. | Not a priority (class backend is the target). | PLAN.md |
| C corpus in CI | The 12 C programs run locally and on A4H, not in CI (no clang/wasm-ld on the runner). | Add `apt-get install clang lld` as a reusable step, keep prebuilt `.wasm` as a cache. | PLAN.md |

## WASI (draft)

| What | Today | Why it is acceptable | Where |
|---|---|---|---|
| Files and preopens | Only stdin, stdout, stderr; `fd_prestat_get` returns EBADF, everything else ENOSYS. | QuickJS eval needs only stdout and the clock. | feat/wasi |
| Real time | One-second resolution; the fraction is 0. | Exact on every runtime (packed is a double in the JS runtime). | feat/wasi |
| Monotonic clock, rights masks | Draft quality; edge cases (counter wrap, per-descriptor rights) may differ from wazero. | Not on the main path. | feat/wasi round 2 critic |
| `random_get` | Deterministic pseudo-random bytes. | Not cryptographic; fine for tests and QuickJS `Math.random`. | feat/wasi |
| `proc_exit` | Stores the exit code and raises the trap exception; callers read `get_exit_code( )`. | A normal exit and a trap look the same to a caller that does not check the code. | feat/wasi |

## Runtimes (open-steamgate), tracked in their inbox

| What | Runtime | Status |
|---|---|---|
| Frontend is quadratic in class size (5K lines ~650 s) | osgo | stoker, in progress |
| Writes to another class's static attributes | osgo | stoker, in progress (inbox 011) |
| `round( )`, `SHIFT ... IN BYTE MODE`, `APPEND INITIAL LINE` | osgo | stoker, queued (inbox 004, 006) |
| Packed is a double; packed to text; decimals of a calculated packed | JS | dell, parked (inbox 002, 012, 013, open-steamgate #500) |
| `int8 = i * i` computed in a double | JS | dell (inbox 005); abapiti loads one factor into int8 first |
| Code the kernel rejects but OSG runs (`BIT-AND` on integers, offset writes into an xstring) | osgo, JS | stoker: a `kernel-reject` warning plus `--kernel-strict`; abapiti CI should turn it on |
