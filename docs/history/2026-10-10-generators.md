# Generator lowering: TS → HIR

Baseline: `origin/main` at `b04e07f`; upstream abaplint core is pinned at
`577f875e` (1,538 source files, plus the registry harness). `abaplint --full`
is absent on that baseline. Measurement therefore calls `LowerRegistry` with
**nil reachability**, including the entire pinned closure.

| Measurement | Before | After |
| --- | ---: | ---: |
| Blocking diagnostics | 644 | 641 |
| Messages mentioning `Generator` | 1 | 0 |
| Follow-on failures in the same consumer | 2 | 0 |
| HIR verification errors | 392 | 392 |

The removed root is the inferred local `icfServices` in
`src/objects/rename/renamer_helper.ts:281`. Its checker type is
`Generator<IObject, void, undefined>`; the prior lowering then rejected the
local's void storage and its use at line 283. Counts by **message** are recorded
in [before](2026-10-10-generators-before.tsv) and
[after](2026-10-10-generators-after.tsv).

## ABI and sites

The full build uses ordinary HIR interfaces and classes: a suspended object's
fields retain locals, arguments, receiver and program counter. `next()` returns
an object with `done: boolean` and an optional `value`. Creation evaluates
arguments/defaults but does not execute the body. Both Go and ABAP emit this
same HIR without backend changes. Generator delegation resumes the delegated
iterator; array delegation reads the current length at each step.

| Producer | Full-closure route | Why eager lowering cannot be assumed |
| --- | --- | --- |
| `Registry.getObjects`, registry.ts:122 | Iterator | `Registry.isDirty` returns on the first dirty object, registry.ts:402. |
| `Registry.getObjectsByType`, registry.ts:130 | Iterator | `SuperclassFinal.findInRelatedProgramIncludes` returns the first match, rules/superclass_final.ts:95. |
| `Registry.getFiles`, registry.ts:136 | Iterator | Public library boundary; full draining by external consumers is unproven. Calls to each object's `getFiles()` must remain deferred between yields. |
| `MethodDefinitions.getAll`, abap/types/method_definitions.ts:18 | Iterator | The lookup consumer returns from a nested method-parameter loop, lsp/_lookup.ts:137. |

All four producers take the iterator route; none is newly replaced by an eager
array-producing function. `Array.from` and spread explicitly drain at their
consumption sites. `for-of` preserves suspension and closes on owned break,
return and consumer exceptions. Optional generator `|| []` loop inputs keep
iteration lazy and enumerate nothing when absent. Iterable parameters accept
generators and arrays: arrays acquire fresh, live iterators for each enumeration,
while generators return the same single-use iterator.

Existing source-pinned snapshot overrides apply to the recorded default
workload only. Full lowering validates their source fingerprints but uses the
real generator bodies and matching Iterable signatures. The default retains its
three snapshot producers and the existing `getFiles` trap.

## Reproduction

The helper creates the pinned dependency packages, tsconfig and harness inside
the extracted source directory. It writes blocking diagnostics, generator routes
and verification details alongside the requested JSON output. Copy the helper
into the baseline checkout to run exactly the same command there.

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"
export GOMAXPROCS=2
mkdir -p /tmp/fit-generator/src
tar -xzf tsfront/testdata/registrycorpus/abaplint-core-577f875e.tar.gz \
  -C /tmp/fit-generator/src
go run -p 2 ./tools/generatorgap \
  /tmp/fit-generator/src/packages/core /tmp/fit-generator/gap.json \
  > /tmp/fit-generator/gap-counts.txt
```

The helper disables `ABAPITI_ASSUME_INT` for both revisions. The initial
integer-mode unpruned baseline attempt crashed in
`tsfront/assume_integer.go:298` on incomplete HIR. Binary64 measurement avoids
changing that unrelated pass. The default-output gate uses the normal integer
mode.

Default outputs were generated with `abapiti abaplint -o <dir> --target all,go`
from both revisions. Both invocations use the **same output directory**, copying
the baseline tree aside before the second run: README instructions embed that
path, and the Go binary contains source paths. `diff -r` of the saved baseline
and final tree is empty, including the binary and ABAP archives.

Validation:

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"
export GOMAXPROCS=2
export GOFLAGS=-p=2
go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
```

Tests execute generated Go against Node for suspension, interleaved consumer
mutation, exhaustion, early close, exceptions, defaults, generator/array
`yield*`, spread, `Array.from`, optional loop inputs and repeated Iterable
consumption, including caught consumer exceptions. The generator tests also
pass with `ABAPITI_ASSUME_INT=1`. They lower the existing registry iterator fixture from its
actual source without snapshot replacements, verify HIR, and emit ABAP.

## Remaining limits

The 641 full-closure blocking diagnostics and 392 verification errors prevent
compiling the entire core, so the 11,060 upstream mocha tests cannot yet run
against a full compiled core. Those diagnostics remain outside this generator
root; there are no generator-specific blocking diagnostics in the pinned
closure.

Only `Generator<T, void, undefined>` is supported. Sent values, explicit
`.return()`/`.throw()`, value-producing yield expressions, suspension with
try/finally or nested functions, consumer exception regions containing
return/break/continue, shadowed generator locals, destructured
iterator bindings, and delegation to other iterable protocols are refused with
located diagnostics. Reentrant `next()` raises a located runtime trap instead
of silently returning an incorrect result. General iterator cleanup handlers,
lexical capture cells and other protocols require further lowering support.
