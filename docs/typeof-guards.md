# Typeof and structured value guards

Branch `feat/fit-typeof-guards` starts at
`origin/feat/fit-last-absence@6d0ac42` and merges
`origin/feat/fit-delete-records@4632865` with `--no-ff`.
The frozen merged baseline is `a9192086575431d082da14dbebd08b3836ca7c28`.
Origin/main is `f78b973b6848682830779875644f2045f30e9fc0`.
Original upstream is `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

**Objects: 142 → 143 passes / 144 Node-active tests; traps 2 → 1;
FAIL 0 → 0.** One Node-pending case stays pending. There are no active
unregistered, skipped, async-pending or unmatched tests. Only the objects shard
was measured; this is not a whole-suite result.

The last-absence Proxy handoff and final-ABI Chai handoff both identify
`malformed object XML ignores malformed inner object metadata`.
They are two references to **one unique original upstream test**, counted once.
Original Node and the final Go compiler both pass that unchanged title.
Its structured `R3_NAME` is an object, so `generateABAPObjects()` returns an
empty list through its own `typeof intfItem.R3_NAME !== "string"` guard.
The earlier key assertions in that same test also complete.

## Repair

In unpruned TS→HIR closures, typeof field operands use the existing raw property
read before a live data view can convert them to their nominal primitive type.
Sequence preludes and receiver evaluation remain intact. Only the guard's
operand takes this path: subsequent string consumers keep their checked reads.
No XML row is dropped, no string default is supplied, and no original source,
assertion, registration or FAIL guard is changed.

Standalone typeof and comparisons share tag lowering for strings, numbers,
booleans, absence, objects and functions. Optional values are evaluated once and
retain absence. Null keeps the object tag. Compiled thunks and value-returning
closures retain their identity and existing call ABI while reporting function,
including after dynamic erasure. Native class descriptors also report function.
The new `dynamic.typeofValue` operation carries the full-value domain; the
existing pruned operation retains its emitted bytes. Its Go emitter recognizes
real compiled closure classes; the ABAP emitter uses their concrete instance
identity. No void ABI, object-literal policy, small builtin, RegExp, Math or
WASM implementation is changed.

The committed Node differential probe covers structured and primitive field
values, missing fields, null, native optional fields, successful string reads
after the guard, raw dynamic arrays/records/primitives, function descriptors,
thunks, value-returning functions, negative comparisons and receiver counts.
Existing optional typeof and runtime primitive probes also pass. The shared
ABAP syntax-feature fixture exercises the new operation.

## Measurements and remaining work

[Evidence](evidence/typeof-guards.json) records executable fingerprints, exact
Go measurement source blobs, the final per-title result, fresh shard accounting,
every blocking message and located diagnostic, and gate commands.
The Go measurement uses the frozen candidate binary. The later ABAP emitter
completion does not alter those recorded TS/HIR/Go blobs; the release compiler
also reproduces the exact default output.

Runtime Proxy conversion refusals: **1 → 0**. Total objects traps: **2 → 1**.
The remaining original title is `Domain, parse AFF json parser error`, which
refuses at `src/objects/domain.ts:100:3` with
`input is outside the supported strict JSON subset`.
That parser domain is outside this guard repair.

`abaplint --full` is available in origin/main. Fresh measurements with the
recorded default CLI number policy show **25 → 25 blocking diagnostics:
16 roots + 9 cascades**, and **67 → 67 HIR errors in 19 files**.
Typeof blocking diagnostics are **0 → 0**; this task removes a runtime boundary,
not a static blocker. The old handoff documents' 26-blocker count is not the
fresh merged-baseline result for the command below. Remaining static roots are
integer evidence, method lowering and structured string coercion; their exact
messages and counts stay unchanged.

```sh
export GOMAXPROCS=2
export GOFLAGS=-p=2
SCRATCH=${TMPDIR:-/tmp}/typeof-guards
UPSTREAM=${TMPDIR:-/tmp}/abaplint-upstream
mkdir -p "$SCRATCH"
git worktree add --detach "$SCRATCH/base" a919208

go -C "$SCRATCH/base" build -p 2 -o "$SCRATCH/base-abapiti" ./cmd/abapiti
go build -p 2 -o "$SCRATCH/candidate-abapiti" ./cmd/abapiti
BASE="$SCRATCH/base-abapiti"
CANDIDATE="$SCRATCH/candidate-abapiti"
"$BASE" abaplint --full -o "$SCRATCH/full-before"
"$CANDIDATE" abaplint --full -o "$SCRATCH/full-after"

ABAPITI_TESTS_COMPILER="$BASE" ABAPITI_TESTS_COMPILER_COMMIT=a919208 \
ABAPITI_TESTS_SHARDS=objects ABAPITI_TESTS_WORKERS=1 \
node tools/abaplint-tests-all.mjs "$UPSTREAM" "$SCRATCH/objects-before.json"
ABAPITI_TESTS_COMPILER="$CANDIDATE" ABAPITI_TESTS_COMPILER_COMMIT=HEAD \
ABAPITI_TESTS_SHARDS=objects ABAPITI_TESTS_WORKERS=1 \
node tools/abaplint-tests-all.mjs "$UPSTREAM" "$SCRATCH/objects-after.json"
```

The upstream checkout must be clean at the pin above, with locked dependencies
installed in its root and `packages/core`. Both original Node oracles were
freshly built with the original locked TypeScript 6.0.3.
The measured candidate runner revision argument was the merged base; executable
and semantic source fingerprints identify the candidate independently of that
manifest argument. No runner metadata is presented as its implementation SHA.

## Gates and default output

All final gates pass with `GOMAXPROCS=2` and package concurrency at most two:

```sh
go vet -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -timeout 30m -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 go test -short -timeout 30m -p 2 ./tsfront \
  -run 'Test(LexerFactsReport|EmitRegistryFeatures|EmitRegistrySingletons)$' -count=1
go test -short -timeout 30m -p 2 ./cmd/abapiti \
  -run '^TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend$' -count=1
./.github/ci/lint.sh gate origin/main
```

Lint reports zero new issues, canary 5/5, pinned golangci-lint 2.13.2.
Implementation: `0458db565fc9f0a82c23e165761092441020d299`.


Grace retains all 61 lexer call sites. Its only golden changes are the new
operation's catalog counts: four counts **138 → 139**, receiver reads
**207 → 208**, and conservative count unchanged at 21.
A preliminary broad run identified the missing ABAP emitter for the new
operation; it was completed before the final gate. A preliminary closure-helper
prototype changed default bytes and was superseded by the dedicated full-value
operation. Neither superseded result is used as a green certificate.

Default output is **6,321 files, raw diff exit 0, zero bytes**, against both the
merged task baseline and origin/main, including the Go executable and ABAP
archives. Each compiler emitted at the same output path before moving its
output aside for comparison:

```sh
"$BASE" abaplint -o "$SCRATCH/default" --target all,go
mv "$SCRATCH/default" "$SCRATCH/default-base"
"$CANDIDATE" abaplint -o "$SCRATCH/default" --target all,go
diff -r "$SCRATCH/default-base" "$SCRATCH/default"
```

Origin/main was independently built and compared in the same way. The final
release compiler also matches the measured candidate output byte for byte.
All commits remain local on the task branch; no credentials, push or PR is used.
