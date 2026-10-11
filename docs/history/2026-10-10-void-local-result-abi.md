# Void local result ABI (2026-10-10)

Base: origin/main `69c79ee73d7d04a70bb0e20826876d1b0e995e3f`.
Branch: `feat/fit-voidres`. Upstream abaplint: `577f875e`.

## Behavior

Lifted local arrows and function expressions, nested function declarations, and
stored parameterless callbacks use the existing HIR void method ABI. ABAP emits
no `RETURNING`; Go emits no return value. A void expression body runs for its
effects. Explicit void returns evaluate their expression before returning. Existing
`return voidCall()` HIR remains unchanged to preserve deployment Go output and
site-map locations.
Expression-body preludes execute within the callee on each invocation, rather
than during its first call's lowering in the caller. Contextual void signatures
also discard the expression result. Separate lexical functions with the same
name receive distinct method names within their generated class.

A local initialized by a genuine no-result call evaluates the call exactly once
and stores undefined using `optional<dynamic>`. An initializer whose lowered
signature exposes a value retains that value.

The old `void local requires a value-preserving result ABI` diagnostic also
appeared when type mapping failed: HIR void doubles as its failure sentinel.
All 16 baseline occurrences are cascades, with a blocking unsupported-type
diagnostic at the same location. This change removes that misleading follow-on
diagnostic while retaining the unsupported type and initializer diagnostics.
It does not make these 16 upstream bindings translatable.

## Static measurement

Main has no `abaplint --full`. The existing full-closure gate materializes and
verifies all 1,538 pinned files, then calls `LowerRegistry` with nil reachability.
No deployment harness is added; assume-int is off. The closure gate is expected
to fail until all remaining compiler gaps are fixed; it writes evidence first.

```sh
node tools/registry-closure.mjs /tmp/voidres-closure \
  ~/.cache/kits/abaplint-577f875e

env -u REGISTRY_REACHABILITY ABAPITI_ASSUME_INT= \
  REGISTRY_CLOSURE=/tmp/voidres-closure \
  ABAPITI_TEST_OUT=/tmp/voidres-evidence/before \
  go test ./tsfront -run '^TestRegistryClosureGate$' -count=1 -v \
  > /tmp/voidres-evidence/before.log 2>&1

env -u REGISTRY_REACHABILITY ABAPITI_ASSUME_INT= \
  REGISTRY_CLOSURE=/tmp/voidres-closure \
  ABAPITI_TEST_OUT=/tmp/voidres-evidence/after-accepted \
  go test ./tsfront -run '^TestRegistryClosureGate$' -count=1 -v \
  > /tmp/voidres-evidence/after-accepted.log 2>&1
```

The before command ran on unchanged origin/main, before edits. The after command
ran on the final compiler. Blocking diagnostics grouped by exact message are
recorded in `void-result-abi/blocking-by-message.json`. Each of the original 16
void cascades and its underlying type error is recorded in
`void-result-abi/baseline-void-cascades.json`.

## Results and remaining work

| Measurement | origin/main | Branch |
| --- | ---: | ---: |
| Full-closure blocking diagnostics | 644 | 625 |
| Full-closure HIR errors | 392 | 386 |
| Misclassified void-local diagnostics (cascades) | 16 | 0 |
| Nested declaration diagnostic | 1 | 0 |
| Calls to the nested `has` function | 2 | 0 |
| Duplicate lifted void callback members | 2 | 0 |

The nested `has` declaration in `src/rules/downport.ts:82` now lowers, as do
its calls at lines 92 and 100. This removes three blocking diagnostics and four
HIR errors (two missing expressions and two nonboolean conditions). The
parameter/result ABI is retained for this value-returning nested function.

The separate void `push` callbacks in `src/rules/xml_consistency.ts` now have
unique lifted method names; the collisions at lines 245 and 260 disappear,
removing two HIR errors. There are no added blocking diagnostic groups or HIR
errors. The new genuine void-result behavior is exercised by the focused tests.

The 16 suppressed diagnostics were **not 16 fixed source roots**. Their
underlying unsupported-type diagnostics remain: five `DocumentSymbol`, five
`RegExpExecArray`, five `RegExpMatchArray`, and one
`Generator<IObject, void, undefined>`. RegExp/match and LSP types are outside this
batch's authorized scope; Generator lowering requires a separate iterator ABI.
The identifier-call diagnostic that remains is `parseFloat` in
`src/objects/number_range.ts:51`, unrelated to local function results.
No genuine void-local result ABI diagnostic remains in the measured corpus.
The full closure remains untranslatable because of the 625 other blockers and
386 HIR errors; the default deployment workload remains the acceptance gate.

## Gates and reproduction

```sh
go vet ./...
go test -short ./...
go test ./tsfront -run '^TestVoid' -count=1
```

The focused executable regressions check calls execute once, stored callbacks,
contextual void results, nested callees, repeated lexical names, effect-only
returns, assignments in arrow bodies, per-invocation preludes, and preservation
of unsupported-type roots. They verify HIR, emit ABAP, and build/run emitted Go.

The main binary was built before any source edits; the final binary was built
after the nested-capture and void-call output-preservation corrections. Both
execute the default pruned workload.
`--target all,go` preserves the default ABAP targets and also emits Go. The same
absolute output path is used for both runs because generated READMEs contain
that path, and Go executables contain their build directory. The main output is
copied before the final compiler overwrites the same output directory. No
normalization or exclusions are applied to the recursive diff.

```sh
# On unchanged origin/main:
go build -o /tmp/voidres-evidence/abapiti-main ./cmd/abapiti
# On this branch:
go build -o /tmp/voidres-evidence/abapiti-accepted ./cmd/abapiti

/tmp/voidres-evidence/abapiti-main abaplint \
  -o /tmp/voidres-evidence/same-output --target all,go
cp -a /tmp/voidres-evidence/same-output /tmp/voidres-evidence/same-main
/tmp/voidres-evidence/abapiti-accepted abaplint \
  -o /tmp/voidres-evidence/same-output --target all,go
diff -r /tmp/voidres-evidence/same-main /tmp/voidres-evidence/same-output

python3 tools/go-target-check.py \
  ~/dev/dell-work/kits/zabapgit-check-kit \
  /tmp/voidres-evidence/same-output/go/zabaplint-go \
  --output /tmp/voidres-evidence/kit-accepted.json
```

The pinned TypeScript source kit lives at `~/.cache/kits/abaplint-577f875e`;
the populated zabapgit check kit lives at the path above. A fresh original
TypeScript build using `buildUpstream` from `tools/statements-upstream.mjs` ran
against both kit variants, producing byte-identical expected dumps (0 clean
issues, 5 seeded issues). No preexisting Node build was trusted and neither kit
was modified.

Final acceptance: `go vet ./...` and `go test -short ./...` passed. The
same-path recursive diff was empty, including the default ABAP targets,
Go sources, site maps, and generated Go executable. The compiled Go kit runs
were byte-identical to pinned Node: clean 0 issues, seeded 5 issues.
