# Structural discriminated-union ABI: gogen reconnaissance and plan

Alice, 2026-10-14. Goal B, self-hosting step 2: compile OSG's generator through
TypeScript → HIR → Go (TS-HG), then use that generator to emit Go. The union ABI
is a prerequisite; supporting it alone will not make the generator executable.

**Decision: plan only.** The largest inferred pilot blocker group is recursive
aliases (31 findings). The smallest useful implementation must register recursive
types, retain metadata and payload fields, verify their uses, emit Go, and execute
a recursive Node differential. Estimated scope is 2–4 engineering days, including
review and the requested gates. This exceeds the approximately one-day limit.
A placeholder alias or a trap-only Go build would not establish the ABI.
No compiler, runtime, fixture, golden, or OSG source changes accompany this plan.

## Inputs and current typing

Abapiti base: `6e94206864fa596231840bcaeee3bf9984ef4e20` (`origin/main`).
The earlier spike is on `origin/spike/gogen-lowering`,
`11b3a55ecd97e3138307791dace0dd9914e226e1`; its
[report](https://github.com/oisee/abapiti/blob/11b3a55ecd97e3138307791dace0dd9914e226e1/docs/gogen-spike.md)
is absent from main. It measured an older compiler and found 68–72 findings in
four typed modules. Its compiled Go contained trapped/omitted functions and
dropped IR fields. Those historical counts are not today's baseline.

OSG was cloned into `.osg`, at clean `origin/main`
`0abbef3f13941bf98b29531982092012110ce3ce`. It remains read only after cloning;
dependencies and generated files live in `.tmp/union-abi`.
The requested [PR #718 baseline](https://github.com/oisee/open-steamgate/pull/718)
is `36bc310be8eeb487838c6064bbcc1e56cb9e1ad1` (merged as `f557e12f`). Both revisions
were measured. Current typing has advanced only by three IR additions since that
baseline: `refattr.kernelRtti`, `exc_ref | exc_previous`, and `Method.sinkUnsafe`.

[tsconfig.gogen.json](https://github.com/oisee/open-steamgate/blob/0abbef3f13941bf98b29531982092012110ce3ce/tsconfig.gogen.json)
includes exactly four files, with `strict`, `erasableSyntaxOnly`, `allowJs: true`,
and `checkJs: false`:

| File under `tools/gogen` | Typed runtime exports / role |
| --- | --- |
| `ir.ts` | Type-only IR contract; no executable entry point |
| `symbols.ts` | `typeName`, `funcName`, `buildCommit`, `symbolCollector`, `writeSymbols`, `writeGo`, `reportSymbols`, `refreshSymbolBuild` |
| `emit-constructors.ts` | `constructorFactories` |
| `frontend-find-results.ts` | `inlineFindResultTypes`, `lowerFindResults` |

There are no other `.ts` files or `.mjs` files with `@ts-check`, `@typedef`, or
typed `@param`/`@returns` annotations in `tools/gogen`. The main functions remain
untyped: `emit-go.mjs:emitGo`, `frontend.mjs:compileProgram`, and the `osabap.mjs`
and `osg-build.mjs` drivers. `emitGo` imports the typed symbol/constructor leaves;
`frontend` imports the typed FIND leaf. Conversely, the typed leaves import those
untyped implementations, so the pilot has a runtime import cycle.
The stoker conversion is still a pilot, not a typed generator closure.

The baseline IR has 19 Type, 70 Expr, and 107 Stmt alternatives, tagged by
`k`, `e`, and `s`. Current Expr has 71 alternatives and 72 literal tag values;
one alternative admits both `exc_ref` and `exc_previous`. Type/Expr intersect
metadata. Condition is another recursive family (`c`, 15 alternatives / 18
literal values). No repeated literal tag occurs across Type/Expr/Stmt alternatives
today, but shared tags and multi-literal tags must be supported by the ABI.

All 11 phase-2 mixed-absence fields remain:
`Argument.place`; call-expression `receiver`, `sup`, `exceptions`, `receiving`;
call-enqueue `exceptions`; FIND `secOff`, `secLen`; FIND-results `count`;
`Method.returning`; `TemplateOptions.pdec`.
Their comments document real null-versus-omission producers. Preserve those
semantics until OSG performs its own phase-2 normalization. Other required-null
fields, including `Class.constructor`, `Class.super` and `Catch.intoType`, are
also part of the contract.

## Measurement

Complete pilot diagnostics, source manifests with SHA-256, inferred root labels,
probe sources, and both revision summaries are in
[union-abi-recon.json](evidence/union-abi-recon.json). All locations are normalized
to project-relative paths. Environment: Go 1.26.9, Node 26.9.0, npm 11.19.1,
TypeScript 5.9.3, Node declarations 22.20.2. Core is 2.120.59 at the baseline,
2.120.73 at current main, matching the respective OSG locks.

The public command uses the same `LowerProjectWithOptions` and `hir.Verify`
policy as `abaplint --full`: no pruning, no failure traps, and no overrides.
Binary64 was selected explicitly to avoid attributing integer assumptions to
union support. `--scope` filters the report, not the loaded/lowered closure.
The only source-config adjustment is explicit `types: ["node"]`, also needed by
the historical spike. Both the tsgo checker and original tsc report zero errors.

| Measurement | PR #718 baseline | Current OSG main |
| --- | ---: | ---: |
| Typed roots lowered; actual JS imports loaded | 4 | 4 |
| Scoped blocking diagnostics | 76 | 76 |
| Scoped compiler root / cascade counts | 70 / 6 | 70 / 6 |
| Scoped untrapped HIR errors | 23 | 23 |
| Whole-report blockers, including declaration boundaries | 87 | 87 |
| Whole-report untrapped HIR errors, including synthetic locations | 26 | 26 |
| Actual TS/JS project closure files | 43 | 51 |
| Full-closure untrapped command | panic, exit 2 | panic, exit 2 |
| Full-closure repair-note refusal records | 6,377 | 6,692 |
| Full-closure HIR errors after trap repair | 1 | 1 |

The 11 additional blockers are module declarations in
`node_modules/@abaplint/core/build/abaplint.d.ts`, reached through namespace
boundary handling despite not being explicit lowering inputs. They are not
implemented abaplint functions. In the pilot, 44 findings are dropped interface
members; the other categories are unsupported expression (13), call (7), type
(6), constructor (3), array (2), and short circuit (1).

Primary-cause grouping below is **inferred from the source contract and failure
messages**, not a measured promise that one fix removes every finding. Groups
are disjoint; nullable and callable dependencies often overlap recursive aliases.
The compiler's cascade flag identifies only some downstream failures.

| Inferred root, four typed files | All findings | Compiler roots | Representative dependency |
| --- | ---: | ---: | --- |
| Recursive aliases / metadata intersections | 31 | 31 | `Field.type`, `Argument.value`, `Method.body`, `Catch.body`, helper signatures using Type/TypedExpr |
| Tag narrowing | 0 | 0 | Real leaf bodies are concealed by failed signatures; simple nonrecursive probe lowers |
| Shared discriminants | 0 | 0 | Simple shared-`e` probe lowers; still a required recursive ABI case |
| `Extract` / indexed access | 6 | 6 | `Program.structs`, `ExprMetadata.fromP`, `EventHandler.call`, FIND result, `Layers.structs/consts` |
| Nullable typed fields / truthiness | 1 | 1 | Constructor leaf: “mixed tagged operands require value-preserving truthiness” |
| Typed callables / imports | 22 | 22 | abaplint node/registry types, Node fs/process, missing imported emitter/frontend values |
| Other | 16 | 10 | Map/Set iterable initialization, spreads, property `??=`, downstream failed locals |
| **Total** | **76** | **70** | Six recorded cascades |

In the full current closure, repair notes retain 69 pilot findings (31 recursive,
6 utility, 1 nullable, 17 callable/import, 14 other), 11 declaration-boundary
findings, and **6,612 untriaged untyped implementation findings**. The baseline
has the same 69 + 11 and 6,297 untyped findings. Importing implementations removes
some missing-import diagnostics but exposes their bodies. These totals count
records, including repeats and cascades, not distinct causes.

Both full untrapped runs panic in
`tsfront.(*numberPass).rewriteExpr`, `tsfront/number_ranges.go:1265`, while visiting
partial HIR (`x.X.Type` is read without an operand). There is no complete raw
diagnostic inventory after that panic. Supplemental trap-mode counts include
`note-trap-failure` and `note-dropped-declaration`; filtering all `note-*` would
misreport zero blockers. Repair still leaves “missing interface implementation
call” at `tools/osd-dialog-step.mjs:336:36`, and both Go and ABAP emitters refuse
there. Full-closure counts are observed refusals, not evidence of executable Go.

The repaired four-file output builds as a Go library, with zero repaired verifier
errors. That only proves the skeleton. The raw program has invalid signatures
and void container arguments, and essential IR fields are missing.

### Reproduction

Run from the abapiti repository root. A fresh scratch directory is required;
the spike driver refuses to reuse an output directory. Use the already cloned
`.osg`; reproduction reads it without installing packages or checking out commits
there. The driver is recovered from its immutable spike commit, not merged into
the compiler.

```sh
mkdir -p .tmp/union-abi
git show 11b3a55ecd97e3138307791dace0dd9914e226e1:cmd/gogen-spike/main.go > .tmp/union-abi/measure.go
go build -o .tmp/union-abi/abapiti ./cmd/abapiti
go build -o .tmp/union-abi/measure .tmp/union-abi/measure.go

for revision in baseline current; do
  case "$revision" in
    baseline) osg_commit=36bc310be8eeb487838c6064bbcc1e56cb9e1ad1; core_version=2.120.59 ;;
    current) osg_commit=0abbef3f13941bf98b29531982092012110ce3ce; core_version=2.120.73 ;;
  esac
  scratch=".tmp/union-abi/$revision"
  mkdir -p "$scratch"
  git -C .osg archive "$osg_commit" tools tsconfig.gogen.json | tar -x -C "$scratch"
  python3 - "$scratch" <<'PY'
import json, sys
from pathlib import Path
p = Path(sys.argv[1])
(p/'package.json').write_text('{"type":"module","private":true}\n')
c = json.loads((p/'tsconfig.gogen.json').read_text())
c['compilerOptions']['types'] = ['node']
(p/'tsconfig.gogen.json').write_text(json.dumps(c, indent=2)+'\n')
PY
  npm install --prefix "$scratch" --cache .tmp/union-abi/npm-cache \
    --ignore-scripts --no-audit --no-fund --save-exact \
    "@abaplint/core@$core_version" @types/node@22.20.2 typescript@5.9.3
  node "$scratch/node_modules/typescript/bin/tsc" -p "$scratch/tsconfig.gogen.json"
  .tmp/union-abi/abapiti lower --full --tsconfig "$scratch/tsconfig.gogen.json" \
    --scope tools/gogen --assume-int=false -o "$scratch/cli-ts" \
    tools/gogen/ir.ts tools/gogen/symbols.ts tools/gogen/emit-constructors.ts tools/gogen/frontend-find-results.ts
  .tmp/union-abi/measure -config "$scratch/tsconfig.gogen.json" -out "$scratch/result-ts"
  .tmp/union-abi/measure -config "$scratch/tsconfig.gogen.json" -js -out "$scratch/result-closure"
done
```

The pilot command prints `76 blocking (70 root, 6 cascade), 23 HIR errors`.
`cli-ts/evidence/closure-gap-summary.json` retains the 87 / 26 whole-report counts.
Its original diagnostic multiset equals the supplemental driver's raw findings
at both revisions. To repeat the public command on the actual resolved closure:

```sh
python3 - <<'PY'
import json, subprocess
from pathlib import Path
for revision in ['baseline', 'current']:
    p = Path('.tmp/union-abi')/revision
    d = json.loads((p/'result-closure/result.json').read_text())
    files = [str(Path(f).relative_to(p.resolve())) for f in d['Files']]
    command = ['.tmp/union-abi/abapiti', 'lower', '--full', '--tsconfig',
               str(p/'tsconfig.gogen.json'), '--scope', 'tools/gogen',
               '--assume-int=false', '-o', str(p/'cli-closure'), *files]
    with (p/'cli-closure.log').open('w') as log:
        status = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT).returncode
    print(revision, status)  # both currently 2; inspect the recorded panic
PY
```

All project implementation files resolved by the real config are passed; library
declarations and node_modules are excluded from the explicit source list.
Tests and other gogen CLI drivers outside that import closure are not implied
to be measured. Checker success with `checkJs: false` does not make those JS
bodies typed. Raw and repaired result streams remain available in scratch;
checked-in evidence retains full pilot diagnostics and full-closure histograms.

## ABI decision

Use **a closed union in semantic HIR, backed by the original structural object
reference on Go**. Reuse the existing Dynamic box and live data-view machinery
for physical storage; retain the family, literal tags, alternative membership,
and field types in HIR. Do not flatten every payload into optional fields or
generate an unrelated nominal class hierarchy to represent TS type membership.

### Type graph and operations

Add program-level union declarations and named `UnionRef` types. A declaration
records its discriminant property and alternatives; each alternative records a
stable identity, literal tag domain, and a structural payload layout. Named
subset views refer to the same family and layouts, with the allowed alternatives
and checker-proven refinements. Literal tags are semantic metadata even though
their field storage is HIR String. No recursive graph is expanded inside
`hir.Type.Args` or `Type.String()`.

Predeclare all alias/shape identities in each recursive component before mapping
members. Resolve `TypeMetadata & (...)` / `ExprMetadata & (...)` by distributing
compatible metadata into each alternative using checker-resolved property types.
Include indirect cycles through Field, Argument, Condition, arrays, and maps.
Cache by normalized declaration identity and resolved type arguments, not by a
transient checker pointer or the use site's synthesized-shape hash. Anonymous
layouts use a deterministic structural key with named recursive back references.
Finalize each component atomically; unsupported members invalidate the component
with a diagnostic rather than publishing a truncated definition.

HIR must distinguish construction/admission, widening, tag reads, and narrowing
to a subset/payload view, whether these are new expression kinds or verified
operations. Verification requires completed declarations, valid subset inclusion,
legal field reads for every surviving alternative, and matching constructor/call
types. Verify recursive references by identity with a visited set. Dumps, rewrite
facts, inlining, serialization, runtime-op discovery, and backend guards must
traverse the new definitions and preserve those contracts.

### Go representation and identity

A union reference uses the existing `*dynamic` envelope containing a structural
record or generated data object; a generated Go type alias can name the family.
Static union membership is checked in HIR, despite the shared Go representation.
The Dynamic runtime tag describes the JS value category. The union discriminant
is the object's actual `k`/`e`/`s` property, read as `jsString` with the existing
UTF-16 semantics. It is not a synthetic integer stored beside the payload.

Construction evaluates properties once, left to right, and retains own-property
presence and insertion order. Widening boxes the same object; narrowing attaches
an existing live data view. Neither operation copies payloads or runs a class
constructor. Existing `dynamicSource`/`box` and dynamic identity comparison are
the starting point; test equality across repeated boxes, multiple views, map
keys, nested arrays, and round trips. Reference equality must compare the backing
object, never a temporary wrapper/view pointer. No view may fabricate prototype
methods, getters, or missing required fields.

Do not cache a discriminant as object identity or assume that TS `readonly`
prevents writes through another alias. Common reads use the intersection of
available fields with a compatible read type. Payload reads require the
checker's narrowed type. Mutable array/map/set slots remain invariant; widening
must not enable writes through an incompatible alias. Initially refuse union
field writes without a valid narrowed destination. Extend mutation only with
oracles for aliasing and invalidation of prior tag proofs.

### Narrowing, shared tags, and utilities

Use the checker flow type at the receiver for `if`, `switch`, negative tests,
and nested guards. A tag value maps to **a set of alternatives**, never to exactly
one payload layout. Shared-tag variants remain a union until another guard
resolves their incompatible fields. An alternative whose tag is itself
`'exc_ref' | 'exc_previous'` retains that logical TS alternative and tag domain.
Do not split it in a way that changes conditional-type distributivity.

Normalize checker-resolved `Extract`, indexed access, intersections and
`ReturnType` to existing family/subset/field types. For example,
`Extract<Type,{k:'struct'}>` shares Type's backing object and struct layout;
`Layers['structs']` shares the declared map ABI. Respect the checker's `never`
result, especially when an alternative's entire tag union is not assignable to
an `Extract` filter. An empty subset is not a storable payload. External
`ReturnType<abaplint...>` may still require an external callable implementation;
utility normalization cannot manufacture that implementation.

### Null and absence

Keep `Nullable<T>` distinct from `Optional<T>` in semantic types. On Go the
existing null box and undefined representation can remain physical primitives;
typed getters/projections must retain T and check the value category. Required
`T | null` admits present-T or explicit null. `x?: T | null` additionally admits
undefined; own-key metadata separately distinguishes an omitted property from
an own property containing undefined. Test all four object states.

Do not map null to Go nil when nil already means undefined, and do not turn
nullable payloads into absent zero values. Preserve `=== null`, `=== undefined`,
`in`, `Object.keys`, JSON omission/null emission, `??`, optional chains, and
value-preserving `&&`/`||`. A recursive field can initially use the existing
Dynamic nullable path only where its operations are already proven; unsupported
nullable operations must remain explicit blockers. The current nullable probe
builds with Dynamic storage, so zero diagnostics there does not prove a typed
nullable ABI.

### ABAP later

Use the same HIR declarations and subset checks. Map union references to the
existing dynamic object-reference envelope and `view.storage` get/put/has/source
interface; generated payload views read the same storage. Recursive edges become
`REF TO` references, not recursively embedded ABAP structures. Runtime null has
an explicit tag, while undefined is the absence representation; own-property
membership remains separate. Check/downcast payload views without constructing
copies or treating plain objects as prototype-bearing classes.

Go is the first execution oracle. Until ABAP implementations exist, the ABAP
emitter and backend guard must explicitly refuse the new operations at their
source locations. Emitting files is not ABAP execution evidence. Later ABAP
acceptance requires the same Node differential through OSGO and kernel syntax/
runtime checks as appropriate; default zabapgit paths must continue to work on
every existing backend throughout.

## Ordered slices and proof

Counts below are candidate findings exposed by the slice, not additive predicted
reductions. Remeasure the frozen pilot and complete closure after each slice;
newly retained signatures may reveal additional body blockers.

| Slice | Scope and expected leverage | Required proof |
| --- | --- | --- |
| 1. Recursive family registration and minimal executable Go ABI | Highest inferred group: 31 recursive findings. Named graph registration, metadata intersections, recursive record/container fields, completed HIR definitions, construction/widening and unique-tag payload access. Estimate 2–4 days; deferred in this task. | Mutually recursive tree/Expr/Stmt oracle, metadata retained, nested arrays and map values, cross-module aliases, no dropped required fields, stable dumps under source-order permutations. Zero blockers and HIR errors on the accepted fixtures, real recursive execution equals Node, no traps. |
| 2. Subsets, shared tags, utility normalization | Six explicit Extract/indexed findings plus tag-dependent bodies hidden today. Extend existing nonrecursive common-field views; preserve tag domains and arbitrary checker-selected subsets. | Shared-tag variants with different payload types, multi-literal alternatives, secondary and negative guards, switch fallthrough, Extract selecting one/many/none, indexed fields, TypedExpr intersections. Both valid oracle cases and invalid-payload refusal cases. |
| 3. Honest typed runtime boundaries | 22 pilot callable/import findings and 11 declaration-boundary records, after type dependencies settle. Retain existing callable lowering; implement specific missing Node/abaplint/import contracts. Typing the main emitter/frontend in OSG is a separate prerequisite. | Imported functions and circular module initialization, multi-argument captured callbacks, real emitter signatures and exceptions. Actual adapter executions equal Node. Declarations or `as unknown as` assertions do not count as implementations. |
| 4. Typed nullability and presence operations | One explicit nullable-truthiness finding, overlapping recursive fields, and all 11 phase-2 contracts. Preserve useful typed payloads and value-returning short circuits. | Null, omitted, own-undefined, present values; optional nullable scalars/references; keys, JSON, equality, `??`, chains and short circuits, with single evaluation and side effects compared against Node. |
| 5. Remaining containers and implementation closure | Ten other pilot compiler roots plus six cascades, then the currently untriaged JS closure. Iterable Map/Set/tuple initialization, spreads and property `??=` have separate semantics. Harden the raw measurement panic as a separate fail-closed repair. | Ordered container/aliasing oracles; a raw closure run that returns original diagnostics without panic. Typed `emitGo` runs on frozen real Program fixtures, emitted Go equals Node byte for byte and compiles/runs the same gogen fixtures. No source rewrites or body traps credited as passes. |

Simple nonrecursive tag, shared-tag, Extract/indexed, nullable and const-callable
probes already have zero raw blockers / HIR errors and build on current abapiti.
The recursive probe has two blockers (`Program.root`, `Tree` parameter) and two
raw verifier errors; its repaired Go builds only by dropping/trapping the failed
code. Probe source and exact findings are in the evidence. Probe runtime
differentials were not performed; existing `TestUnionField.*NodeOracle` tests
were run and passed, demonstrating the reusable common-field view machinery.

The implementation hot spots are `tsfront/lower_types.go` (alias/checker mapping
and registration), `tsfront/union_fields.go` (flow-based shared reads),
`hir/hir.go` and verification/traversal, and both backends' live-view code.
`synthFromAlias` predeclares plain type-literal shapes today, but rejects union/
intersection aliases; `synthFromPropertyTypes` maps fields before registering
an anonymous layout. Simply relaxing either check risks caching partial layouts,
infinite recursion, or accepting a payload with the wrong tag.

## Regression contract and risks

Every code slice requires Go-vs-original-Node execution, **FAIL 0**. Never
relabel an assertion mismatch as a refusal. Compare supported behavior, trap
counts and registration totals as well as build success. A repaired verifier
pass or a reduction in diagnostic count alone is insufficient.

Before committing a compiler/runtime slice, run the requested gates:

```sh
go vet ./...
go test -short -p 2 ./...
ABAPITI_GRACECHECK=1 go test -p 2 ./tsfront -run '^TestLexerFactsReport$' -count=1
go test -p 2 ./cmd/abapiti -run '^(TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend|TestSelectedTargetRuntimeOpsFailLoudly)$' -count=1
bash .github/ci/lint.sh gate origin/main
HIR_RUNTIMES=" " bash .github/ci/hir-unit.sh .tmp/union-abi/hir-unit
```

For hir-unit, provision a writable dependency-equipped OSG copy at
`.tmp/union-abi/hir-unit/open-steamgate`; keep `.osg` read only. A space disables
the runtime loops but still runs fixture generation, lint and the transpiler
pin check. This copy must be clean at `.github/ci/osgo.ref`, with the pinned
transpiler/runtime build, as required by `lexer-unit.mjs`. Do not count a
dependency/setup failure as a passed lint suite.
Leave Grace goldens unchanged unless an independently reviewed semantic change
requires otherwise.

Generate default `abapiti abaplint --target go` output from origin/main and the
slice using the same frozen inputs/toolchain. Require an empty recursive diff
of the generated Go source/module files, excluding the built executable, and
retain the existing clean/seeded zabapgit Node oracle checks. New union semantics
must respect the existing `coverage == nil` boundary so the source-pinned default
deployment's identities and serials do not drift.

Run the TS-HG test harness on affected shards (`abap/types`, `abap/syntax`,
`abap/nodes`, `objects`, `rules`, `lsp`, and any newly implicated shard), before
and after, with
`ABAPITI_TESTS_SHARDS` and `tools/abaplint-tests-all.mjs` against pinned abaplint
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. Select real shard names from that
checkout rather than assuming a directory name. Require unchanged per-title
registration/pass/trap results and FAIL 0. Before integration of the ABI, run
all 17 shards: **11,060 / 11,060 Node-active passes on Go**, 0 failures, 0 traps,
0 unregistered active tests; Node-pending tests remain outside the denominator.
A focused shard run cannot certify the 11,060 total.

For example, with the pinned abaplint checkout and its dependencies prepared:

```sh
ABAPITI_TESTS_TARGET=go \
ABAPITI_TESTS_SHARDS=abap/types,abap/syntax,abap/nodes,objects,rules,lsp \
ABAPITI_TESTS_SCRATCH=.tmp/union-abi/test-shards \
node tools/abaplint-tests-all.mjs .tmp/abaplint .tmp/union-abi/targeted-shards.json
```

The main risks are unstable recursive identities across checker instances;
invalid incomplete components; confusing a tag with a unique variant; treating
readonly as runtime immutability; losing aliases/own-key presence in views;
unsound container widening; conflating null and undefined; and accepting external
declarations without runtime bodies. Explicit contracts, negative verifier tests,
source-order reproducibility and Node differentials address these separately.
The 6,612 untyped closure findings and raw-lowering panic remain separate work;
this plan does not estimate them away as union blockers.

Validation of this documentation-only change: both public pilot measurements,
both full-closure panic reproductions, supplemental raw/trap measurements and
Go builds, original tsc checks, six isolated lowering probes, and existing
union-field Node oracle tests. `abapiti abaplint --full` also reproduced the
main baseline: 19 blockers (10 root / 9 cascade), 54 HIR errors. The 11,060 result
is the existing main certification, not a suite rerun here. Conditional code-change
gates above remain acceptance requirements for implementation; this task changes
only the plan and normalized evidence.
