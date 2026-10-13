# Emitted ABAP fixture compile gate

## Results on the pin

OSG `f557e12f4217addcec7834b0607234aecf9b723a`:

- Full gate: **413 collected emissions, 326 unique programs, exit 0** (559 s
  on the shared development host, including the Go tests).
- Scratch copy with `hir/abap/class_abi.go` from `3ab143c^`: **exit 1**.
  All generating tests passed; the Go build rejected `TestClassArgsFactoryABAP`
  and `TestClassArgsWideFieldConstructorABAP` with
  `cannot use p1.items[...] (variable of interface type any) as
  *Z_RUNTIME_DYNA_A429568D5BE89E ... need type assertion`.
- Repeated the factory-only counterproof after the full green run: exit 1.
  Restoring the checked cast makes all six factory emissions (three unique
  programs) compile again. The working tree's factory fix was never reverted.
- `go vet ./...`, `go test -short -p 2 ./...`, tagged legacy test compilation,
  shellcheck and Node syntax checking passed. The shared OSG checkout remained
  clean; neither gogen output nor its Go module is written there.

## Running it

Supply an already prepared, clean OSG checkout at `.github/ci/osgo.ref`:

```bash
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
nice -n 10 .github/ci/abap-fixture-compile.sh "$OSG" "$work"
```

The caller supplies the usual Go cache/toolchain and transpiler environment.
No checkout, clone, dependency installation or activation happens in this script.
`ci.yml` calls the existing `osgo` job via the now-reusable `osd-m1.yml`; legacy
OSD deployment remains manual-only. The compile gate precedes `hir-unit.sh`.

## Collection and compilation

`ABAPITI_ABAP_FIXTURE_DIR` enables the shared test-only collecting helpers:

- Default Go tests cover HIR ABAP, HIR fixtures, TypeScript frontends, the OSG
  test harness and successfully emitted Chai/JSON fixtures when available.
  Existing refusal assertions and optional-fixture skips are unchanged.
- LLVM report fixtures and the legacy WASM ABAP Unit generator are included.
  Other opt-in legacy emitter tests also export when invoked with the variable.
- Layout is `<dir>/<package>/<test/subtest>/emit-NNN/`. Independent emissions
  stay separate because generated runtime names overlap. Cleanup captures final
  maps, including subsequently attached ABAP Unit classes. Legacy
  `ABAPITI_TEST_OUT` exporters retain their original contract.
- The inventory test rejects direct modern emitter calls that bypass collecting
  helpers. New tests should use `emitABAP`/`emitABAPNamed` (or the package's
  equivalent); new emitter packages need adding to that inventory.

One Node process invokes the pin's `tools/gogen/frontend.mjs` and `emit-go.mjs`,
using the train's `unit-inputs.mjs` overlays and native dependency closure.
Library visibility includes transitive DDIC/type-pool dependencies and the
pin's implicit RTTI reference-descriptor allocator. Identical file sets compile
once, with all test aliases retained. Class-containing LLVM reports use an
empty host class pool: only the REPORT header is replaced; their local class
bodies are not rewritten. Standalone reports fail closed.

One `go build -p 2` checks every generated package. No generated Go is executed.
Syntax diagnostics are fatal so tolerant mode cannot silently omit a broken
fixture; named runtime refusals remain allowed, as in the compile-only train
bridge. This is compilation evidence, **not execution or kernel certification**.

Failures print test names alongside gogen/Go diagnostics. `fixtures.json` maps
packages to aliases; `build.log` records the Go build. CI uploads both. Temporary
ABAP, adapted inputs and generated Go are removed on success or failure.

The first complete collection also exposed and repaired optional root-object
`classOf`, inherited async promise forwarding, late descriptor-interface
creation from signatures, and LLVM memory helpers emitted as FORMs despite
method calls. Existing test assertions were not weakened.
