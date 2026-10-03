# 2026-10-03: QuickJS speed on five runtimes

The same WebAssembly module (QuickJS 2024-01-13, built without SIMD by wasi-sdk 34, driver `qjs_eval(n)` with 16 fixed scripts) run on five runtimes. Each number is one `qjs_eval` call, which creates a QuickJS runtime and context, evaluates the script and frees them; every result equals native QuickJS. Raw values, long format (program, runtime, phase init|eval, value, unit, runs): [2026-10-03-quickjs-speed.csv](2026-10-03-quickjs-speed.csv). Each value is a single run.

**Machines and versions.** native, wazero and osgo: one host, Intel Core i7-10700K (8 cores / 16 threads, 3.8 GHz), Linux 6.8, Go 1.25/1.26, wazero 1.x, open-steamgate main ad3d1e8. A4H: SAP ABAP Platform developer system, kernel 7.58, on a separate remote host; its hardware is not part of this record.

| # | JavaScript | native (gcc -O2) | wazero JIT | wazero interpreter | osgo | A4H |
|---|---|---|---|---|---|---|
| 0 | 1+2 | 0.20 ms | 1.01 ms | 16.50 ms | 7.9 s | 151.78 ms |
| 1 | let s=0; for (let i=1;i<=100;i++) s+=i; s | 0.15 ms | 1.05 ms | 17.46 ms | 8.2 s | 205.75 ms |
| 2 | [5,3,9,1].sort((a,b)=>a-b) ... | 0.17 ms | 1.08 ms | 21.03 ms | 8.0 s | 162.37 ms |
| 3 | JSON.stringify({a:[1,2,{b:3}]}).length | 0.16 ms | 1.05 ms | 20.35 ms | 9.1 s | 159.52 ms |
| 4 | fib(15), recursive | 0.22 ms | 1.26 ms | 27.36 ms | 16.6 s | 952.16 ms |
| 5 | 'abapiti'.toUpperCase().charCodeAt(0) | 0.16 ms | 0.98 ms | 15.53 ms | 8.1 s | 149.24 ms |
| 6 | Math.floor(Math.sqrt(1e6)) | 0.17 ms | 1.00 ms | 16.84 ms | 8.2 s | 157.90 ms |
| 7 | new Map(...).size + /a+b/.exec(...) | 0.15 ms | 1.03 ms | 17.20 ms | 8.8 s | 157.29 ms |
| 8 | fib(22), recursive | 2.07 ms | 5.91 ms | 360.49 ms | — | 23.3 s |
| 9 | for 1e6: s=(s+i*7)%1000003 | 36.32 ms | 147.60 ms | 7.8 s | — | 677.0 s |
| 10 | string of 20,000 chars via += | 6.62 ms | 12.99 ms | 578.97 ms | — | 23.8 s |
| 11 | sort 10,000 numbers | 8.20 ms | 18.49 ms | 1.1 s | — | 60.2 s |
| 12 | 20,000 small objects | 10.69 ms | 24.73 ms | 1.6 s | — | 40.6 s |
| 13 | 100,000 closure calls | 8.02 ms | 16.93 ms | 1.0 s | — | 55.4 s |
| 14 | JSON.stringify/parse of 1,000 records | 5.21 ms | 9.92 ms | 622.96 ms | 359.7 s | 9.5 s |
| 15 | regexp match and replace on 10,000 chars | 2.27 ms | 6.86 ms | 419.83 ms | 206.7 s | 6.2 s |

## How it was measured

- **native**: QuickJS compiled with gcc -O2 on the host, `clock_gettime(CLOCK_MONOTONIC)` around `qjs_eval`.
- **wazero** 1.x, compiler and interpreter modes, fresh module instance per call, `time.Now()` around the call.
- **A4H** (SAP kernel 7.58): abapiti split output (13 interfaces, state class, 13 chunks, facade) activated in a throwaway package; a report run as a background job takes `GET RUN TIME` (microseconds on the kernel) around `CREATE OBJECT` + `_initialize( )` and around `qjs_eval( )`, writes the values to the spool. QuickJS start (`CREATE OBJECT` + `_initialize`) took ~0.6–1.2 ms (the first one 19 ms).
- **osgo** (open-steamgate main ad3d1e8, Go runtime): the same classes as ABAP Unit, `GET RUN TIME` inside the test, unit timeout raised for the run; the sum of the measured evals matches osgo's own run time, so the unit is microseconds there too. The heavy scripts 8–13 were not run on osgo (hours at its speed).

## Reading it

- The kernel is 770× (script 0) to 18,600× (script 9) slower than native QuickJS, and 15–90× slower than wazero's interpreter: every WebAssembly instruction becomes several ABAP statements.
- Scripts that spend their time inside large library routines (JSON, regexp) lose least; tight interpreter loops lose most.
- osgo is 17–57× slower than the kernel on execution.
