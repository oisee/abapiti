# Structural discriminated-union ABI: gogen reconnaissance and plan

Alice, 2026-10-14. Goal B, self-hosting step 2: compile OSG's generator through
TypeScript → HIR → Go (TS-HG), then use that generator to emit Go. The union ABI
is a prerequisite; supporting it alone will not make the generator executable.

**Reconnaissance decision (before Slice 1): plan only.** The largest inferred pilot blocker group is recursive
aliases (31 findings). The smallest useful implementation must register recursive
types, retain metadata and payload fields, verify their uses, emit Go, and execute
a recursive Node differential. Estimated scope is 2–4 engineering days, including
review and the requested gates. This exceeds the approximately one-day limit.
A placeholder alias or a trap-only Go build would not establish the ABI.
The reconnaissance commit changed no compiler, runtime, fixture, golden, or OSG
source. Slices 1 and 2 implementation and measured acceptance are recorded in PROGRESS
below; the remaining slices are still separate work.

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
| 1. Recursive family registration and minimal executable Go ABI | Highest inferred group: 31 recursive findings. Named graph registration, metadata intersections, recursive record/container fields, completed HIR definitions, construction/widening and unique-tag payload access. Implementation on `feat/union-abi-s1`; see PROGRESS for acceptance and remeasured counts. | Mutually recursive tree/Expr/Stmt oracle, metadata retained, nested arrays and map values, cross-module aliases, no dropped required fields, stable dumps under source-order permutations. Zero blockers and HIR errors on the accepted fixtures, real recursive execution equals Node, no traps. |
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

## PROGRESS — Slice 1 (Alice, 2026-10-14)

Started on `feat/union-abi-s1` from `origin/feat/union-abi`. OSG remains clean
at the frozen current revision `0abbef3f13941bf98b29531982092012110ce3ce`.
Read the plan and reconnaissance inventory. Initial measured baseline remains
76 scoped pilot blockers (70 root / 6 cascade), 23 scoped HIR errors;
87 whole-report blockers, 26 whole-report HIR errors at both frozen revisions.
Full closure previously panicked untrapped, with 6,377 / 6,692 supplemental
repair-note refusals. These are baseline observations, not Slice 1 passes.

Implementation is in progress: explicit completed HIR family declarations and
verified operations first, followed by atomic recursive frontend registration,
Go execution oracles and the regression gates. No acceptance claim yet.

### Checkpoint 1 — HIR family contract and Go storage operations

Added named `UnionRef` edges, completed family declarations retaining tag domains
and payload layouts, explicit verified admission/widening/tag/common-read/unique
projection operations, dump/serialization retention, runtime discovery and
conservative rewrite effects. Go uses the existing Dynamic and live-view storage;
checks re-read actual tags and require own fields. ABAP's runtime guard refuses
the new operations and declaration boundary. Existing HIR, rewrite and Go emitter
short suites passed. Negative HIR tests cover incomplete/missing layouts, optional
discriminants, wrong tags, wrong payloads and missing constructor arguments.
Frontend registration and recursive execution are still pending. This is a WIP
checkpoint, not an accepted compiler slice; full regression gates remain pending.
Baseline blocker counts above have not yet been remeasured.

### Checkpoint 2 — Atomic registration and recursive execution

The frontend now stages named family/layout identities and all resolved members
privately, distributing checker-resolved metadata intersections. It publishes
only complete reachable graphs. Unsupported required members invalidate the graph;
retrying its aliases cannot retrieve a partial layout. Recursive edges stay named
through nested arrays and map values. Existing nonrecursive union paths and the
coverage-pruned deployment boundary remain in place.

Both recursive Node differentials passed with zero lowering blockers/HIR errors:
a binary tree and mutually recursive Tree/Expr/Stmt with metadata, nested arrays,
map values, imported aliases, own-key order, single evaluation and backing identity.
Declaration/input/alternative/metadata-property order permutations produce equal
HIR dumps and equal execution. Negative tests confirm atomic callback-member
refusal and wrong-tag runtime refusal. ABAP explicitly refuses the recursive ABI.

Regression work is ongoing. Vet passed. An initial short-suite/pilot run exposed
a nongeneric checker reference-target precondition; the targeted closure-inventory
rerun passed after correcting it. Initial post-change pilot registration still
refused unsupported readonly/mixed fields (78 scoped blockers, 23 HIR errors on
the frozen baseline); that intermediate count is not acceptance evidence. Existing
readonly-array and Dynamic mixed-field paths have since been retained; final
frozen measurements, full short suite, Grace, backend/lint/hir-unit gates, default
Go diff and before/after shards remain pending. No traps or source rewrites are
credited as passes.

### Checkpoint 3 — Preservation and regression checks

Added family/alternative/tag rewrite facts while preserving unrelated Grace facts;
the unchanged lexer golden passes. Serialization/inlining retain the named graph
and recursive execution. Distinct anonymous layouts and distinct resolved generic
arguments now have independent identities, with passing Node differentials.
Fixed the anonymous-checker-symbol collision rather than treating `__type` as a
named declaration. The ABAP catalogue test now explicitly expects the five
Go-only union operations to remain unsupported.

Passed: vet, backend guard, unchanged Grace golden, lint gate against fetched
`origin/main` (0 new issues, canary 5/5), and the complete hir-unit generation/lint/
lexer/pin suite with `HIR_RUNTIMES=" "` (0 lint issues). Initial hir-unit attempts
failed on missing locked libraries and package-relative output paths; neither is
credited as a pass. The full short suite found the stale ABAP catalogue assertion;
its focused rerun passes and the final complete rerun remains pending.

The frozen zabapgit Go source/module diff is empty, excluding only the executable.
Fetched `origin/main` is now `18cc014` (v0.5.0): its unqualified CLI default changed
to full core. The same legacy workload is selected with `--scope zabapgit` on that
revision and the feature branch's original default, with the same frozen inputs.
No upstream CLI/ABAP changes were merged into this Go ABI slice.

Before-change syntax/rules/statements shards: 9,474/9,474 Node-active passes,
9,494 registered, 20 Node-pending, 0 failures/traps/unregistered. Per-title
comparison with the slice is still running. Both frozen public pilots measure
61 scoped blockers (55 root / 6 cascade), 27 scoped HIR errors at this checkpoint;
complete raw/supplemental closure evidence is being finalized. These real pilot
errors are refusals, not accepted fixtures or executable-generator passes.

### Checkpoint 4 — Live tag-proof lifetime

A final audit found a retained-view hazard: a local holding a projected payload
could outlive a discriminant change through `any`. Payload getters now validate
the actual tag on every access, including stored views. A separate native slot
reader prevents validation from recursively calling its own guarded getter.
Unknown and known-but-wrong tags are refused both immediately and after caching
a view. Payload writes remain explicit refusals until alias invalidation is
proven; construction still initializes the same backing object. Atomic layout
mapping also rejects accessor/private/protected members rather than fabricating
public data slots. ABAP test emission now refuses family declarations before it
can replace operations with body traps. Its refusal catalogue is explicit.

All recursive positive/negative tests pass, including retained views. The prior
seven-shard before/after comparison is unchanged: 9,837 Node-active passes,
0 failures/traps/unregistered, 26 Node-pending. Final guard-era gates and compiler
remeasurement are pending. Earlier complete short runs caught two catalogue/fact
expectations and the fixture-export-helper policy; the corresponding focused
checks now pass. No earlier failed complete run is credited as a gate pass.
Frozen pilot counts remain measured at 61 / 27 scoped and 72 / 30 whole report;
full closure still panics raw, with 6,362 / 6,677 repair refusals and one repaired
HIR error at the last measured checkpoint.

### Slice 1 frozen remeasurement — final compiler `fcc15e0`

The recorded public command was rerun on the four frozen typed roots and all
resolved TS/JS closure files, without pruning, overrides, failure traps or OSG
source edits. Checker errors remain zero. Supplemental repair streams are
refusal inventories only; their build results are never execution passes.

| Measurement | PR #718 baseline after Slice 1 | Frozen OSG main after Slice 1 |
| --- | ---: | ---: |
| Scoped pilot blockers | 61 | 61 |
| Scoped compiler root / cascade | 55 / 6 | 55 / 6 |
| Scoped untrapped HIR errors | 27 | 27 |
| Whole pilot report blockers | 72 | 72 |
| Whole pilot report root / cascade | 66 / 6 | 66 / 6 |
| Whole pilot report HIR errors | 30 | 30 |
| Actual TS/JS closure files | 43 | 51 |
| Public full-closure raw command | panic, exit 2 | panic, exit 2 |
| Full-closure repair refusal records | 6,362 | 6,677 |
| Repair `note-dropped-declaration` / `note-trap-failure` | 152 / 6,210 | 163 / 6,514 |
| Full-closure repaired HIR errors | 1 | 1 |

Compared with reconnaissance, pilot blockers decrease by 15, while retained
signatures expose four additional scoped HIR errors. Full-closure refusal records
also decrease by 15. The raw panic remains in `numberPass.rewriteExpr` at
`tsfront/number_ranges.go:1265`; a complete raw closure inventory is unavailable.
The remaining repaired error is still the missing interface implementation call
at `tools/osd-dialog-step.mjs:336:36`. Full generator execution is not established.
This is compatible with the remaining utility, runtime-boundary, nullability and
implementation-closure slices; no count reduction is credited as an execution
proof. Final per-title artifact comparison and the lint gate are still pending
at this safety checkpoint; all other final gates have completed successfully.

### Slice 1 acceptance — complete, Go backend only

Final compiler `fcc15e0712bb7478cfb6203a4031e9402dc1b48a` satisfies the Slice 1
row. The binary-tree and mutually recursive Tree/Expr/Stmt oracles retain all
required metadata/payload fields, nested arrays, map values and imported aliases.
Accepted fixtures have zero blocking diagnostics, zero HIR errors and zero traps;
their actual recursive Go execution equals original Node. Declaration, file,
alternative and metadata-property permutations produce identical HIR dumps.
Serialization and inlining preserve the executable graph. Separate anonymous
records and resolved generic arguments retain distinct layout identities.

Negative proofs refuse incomplete graphs, unsupported required members, wrong
admission tags/types, missing required fields and payload writes. Atomic graph
publication exposes no partial layout. Every projected payload access revalidates
its live tag, including views retained across `any` alias mutation. ABAP ordinary
and test emission explicitly refuse this ABI before body traps. Subset/shared-tag
projection, typed nullability, payload mutation and remaining runtime boundaries
remain subsequent-slice work; they are not silently accepted here.

All final requested gates passed on that compiler:

| Gate | Final result |
| --- | --- |
| `go vet ./...` | Pass |
| `go test -short -p 2 -timeout 40m ./...` | Pass, complete suite |
| Grace lexer facts golden | Pass, tracked golden unchanged |
| Default/selected-target backend guard | Pass |
| Lint gate versus fetched `origin/main` (`18cc014`) | 0 new issues; canary 5/5, golangci-lint 2.13.2 |
| Complete hir-unit suite, `HIR_RUNTIMES=" "` | Pass, 0 lint issues, exact dependency pins |
| Frozen default zabapgit Go source/module comparison | Empty recursive diff, executable excluded |
| Original Node versus Go clean/seeded zabapgit kit | Identical 0 / 5 issue results |

The default comparison uses `origin/main --scope zabapgit` and this branch's
original default, because upstream changed its CLI default to full core. Both
use the same frozen sources and toolchain. Hir-unit uses a separate writable OSG
copy at its CI pin; the source `.osg` remains clean and read only.

All seven targeted abaplint shards at
`577f875ebec44cfaf64841cfe71c8ab8dc32622e` have unchanged per-title
registration/pass/trap results against the before-change compiler:

| Shard | Node-active passes before / after | Registered | Node-pending |
| --- | ---: | ---: | ---: |
| `abap/nodes` | 2 / 2 | 2 | 0 |
| `abap/statements` | 4,962 / 4,962 | 4,962 | 0 |
| `abap/syntax` | 1,363 / 1,363 | 1,373 | 10 |
| `abap/types` | 29 / 29 | 29 | 0 |
| `lsp` | 188 / 188 | 193 | 5 |
| `objects` | 144 / 144 | 145 | 1 |
| `rules` | 3,149 / 3,149 | 3,159 | 10 |
| **Total** | **9,837 / 9,837** | **9,863** | **26** |

Failures, traps, async-pending and unregistered active tests are all zero.
Retained result streams were compared as title/status multisets, including the
final rules rerun with artifacts retained. This focused run does not certify
all 17 shards or the historical 11,060 total; that remains an integration gate.

[union-abi-s1.json](evidence/union-abi-s1.json) records the compiler/source pins,
source hashes, normalized measurements, shard comparison hashes and acceptance
results. Final frozen counts are the table immediately above: 61 scoped blockers
and 27 scoped HIR errors at both revisions; 72 blockers and 30 HIR errors in the
whole pilot report. Raw full closure still panics, so its raw counts are unknown;
6,362 / 6,677 supplemental repair refusals and one repaired HIR error remain.
The full generator is not executable yet. These remaining failures belong to
the later slices and are not counted as Slice 1 passes. No OSG source rewrites,
body traps or partial layouts were used to establish acceptance. Work is
committed on `feat/union-abi-s1` with Alice's identity and the requested trailers;
no push was performed.

## PROGRESS — Slice 2 (Alice, 2026-10-15)

Started `feat/union-abi-s2` from `origin/selfhost`. Read the Slice 1 acceptance
and evidence. Work is limited to recursive/union code and new files, with small
integration hooks if required so concurrent collection-iterable work can proceed.
The implementation will retain checker-selected subset membership and original
recursive layouts, normalize utility/indexed types, and validate shared-tag
payloads against secondary literal domains and field types. Go execution and
explicit ABAP refusal remain the boundary. Node oracles, invalid-payload proofs,
a named core-line mutant, pilot remeasurement and every requested gate are
pending; no Slice 2 acceptance claim is made at this checkpoint.

### Slice 2 checkpoint 1 — retained subsets and executable guards

Implemented checker-selected subsets sharing retained payload layouts, including
utility-first Extract registration, indexed aliases, distributed TypedExpr
intersections, secondary string domains and multi-literal tags. Common scalar
reads preserve Dynamic storage for checker-proven `typeof` refinements. Shared-tag
construction requires checker assignability; Go payload validation checks required
own fields, scalar types and secondary literal domains on every access, including
retained views. ABAP ordinary/test emission still refuses before body traps.
Nonempty fallthrough is enabled only for recursive-union tag switches, retaining
the original checker flow for each executed clause. The integration changes are
one lowerer catalogue field, a subset-narrowing hook, and a small switch hook.

The complete recursive positive/refusal suite passes, including the new seven
Node differential fixtures, seven static invalid-membership cases and two live
shared-payload corruption oracles. Accepted fixtures have zero blockers/HIR
errors. All final gates, the core-line mutant and pilot counts remain pending.
The baseline shard attempt first lacked core-package npm declarations; it was
restarted after installing the locked dependencies and that setup failure is
not credited as a pass. OSG main fetched for remeasurement is
`4fa08e8e4b156b5022081ffcd20e13d3fbc8120d`.

### Slice 2 checkpoint 2 — layout/refinement audit and gate progress

Fixed registration-order-dependent intersection loss: subset reuse now requires
matching retained field layouts; intersections adding fields register their own
complete layouts. The TypedExpr oracle first registers Expr, then checks the
additional required type field. Indexed subset reads use the same verified
common-read operation; indexed writes refuse under the existing alias policy.
Constructor admission validates secondary/subset literal domains as well as
payload assignability. HIR tests cover directional inclusion, invalid subset
projection, malformed/empty/duplicate domains and incompatible literal storage.
Go validation also checks array/reference categories without expanding cycles.

Vet, unchanged Grace golden, backend guard and lint against `origin/selfhost`
passed (0 new issues; canary 5/5). Hir-unit's first invocation used package-local
relative fixture output and failed setup; its resolved-directory rerun is in
progress, with zero issues in completed lint groups. The full short suite,
baseline/after shard comparison and default Go diff remain in progress. The
interim pilot was 60 scoped blockers (54 root / 6 cascade), 29 scoped HIR errors;
71 whole-report blockers and 32 HIR errors. These counts precede the final layout
audit and will be superseded by the final measurement. The isolated one-line
subset-widening mutant is building/running; it is not yet credited as killed.

### Slice 2 checkpoint 3 — execution/refusal proof and final measurements

All seven positive Node fixtures pass with zero blockers/HIR errors and no
body/runtime traps. Invalid-payload proof now covers eight membership cases,
six additional per-feature Node contract/refusal cases (multi-literal payload/tag,
negative guard, fallthrough payload, indexed field, required TypedExpr field),
and two live shared-payload corruption cases. ABAP ordinary/test emission refuses.
The isolated core-line mutant replaces the retained subset result with its full
family in `familySubsetType`; it fails
`TestRecursiveSubsetsNodeOracle/extract-one-many-indexed` with a payload-read
narrowing refusal and missing return. The unmutated oracle passes.

Final vet, lint against `origin/selfhost` (0 new issues, canary 5/5), Grace and
backend guards pass. Complete hir-unit passes with zero lint issues and exact
OSG/transpiler/runtime/library pins, with `HIR_RUNTIMES=" "`. The default zabapgit
Go source/module diff versus `origin/selfhost` is empty. Fresh original pinned
Node and Go both equal the frozen release-kit issue bytes: clean 0, seeded 5.
The complete short suite and seven-shard title comparison are still in progress.

The final four-root public pilot against OSG main
`4fa08e8e4b156b5022081ffcd20e13d3fbc8120d` measures 60 scoped blockers
(54 root / 6 cascade), 29 scoped HIR errors; whole report 71 blockers
(65 root / 6 cascade), 32 HIR errors. Original tsc reports zero errors. A new
same-source `origin/selfhost` measurement is 59 scoped blockers (53 / 6),
29 scoped HIR errors; Slice 2 adds one explicit already-unsupported Stmt-family
refusal at the Extract source argument, with no removed blockers. This is
separate from the historical Slice 1 61 / 27 measurement. Recursive Stmt still
contains unsupported members; normalized utilities cannot publish truncated
layouts. Full-closure supplemental remeasurement remains in progress. These
pilot refusals do not establish executable gogen.

### Slice 2 checkpoint 4 — complete short gate and raw closure evidence

The complete `go test -short -p 2 -timeout 40m ./...` run now passes, including
all final refusal tests. Vet and lint were refreshed on that complete test tree
and pass. All other non-shard gates above are complete. Original per-title
artifacts are retained independently of the harness's successful-run cleanup.
Completed nodes/statements/syntax before/after Go title/status/message multisets
are equal, with zero Go failures/traps. Remaining shard comparisons are running.
One original Node after-run ADD test timed out at its existing 250 ms limit under
concurrent compiler load (baseline Node passed). This Node run is not credited as
FAIL 0; an independent fresh-original Node retry with the same timeout and source
will run after heavy jobs finish. The original failure evidence is retained.

The supplemental full closure now resolves 41 implementation files at current
OSG main, with zero checker errors, 5,423 raw blockers and 4,663 raw HIR errors.
Its original API diagnostic inventory is complete. Repair records are 5,424
(134 dropped declarations / 5,290 trapped failures), with zero repaired HIR errors;
Go still refuses emission at the unimplemented lookahead regexp in
`tools/gogen/frontend-analysis.mjs:45:17`, and ABAP explicitly refuses union
family declarations. No Go/ABAP files or executable-generator pass result.
The public full-closure command exits 2 while writing evidence at
`RegistryLowering.Evidence`, `tsfront/registry_build.go:308`, on both the unchanged
base and Slice 2. This differs from the historical Slice 1 number-pass panic;
the API inventory and public-command failure are recorded separately. Neither
repair-mode verifier success nor the changed main closure size is credited as
Slice 2 execution proof.

### Slice 2 acceptance — complete, Go backend only

Slice 2 satisfies its row: checker-selected subsets retain the original recursive
payload layouts; Extract and indexed-access annotations normalize through their
resolved checker types, including utility-first registration. Shared primary tags
retain secondary literal domains and different payload types. Multi-literal tags,
negative/secondary/typeof guards, nonempty switch fallthrough, Extract selecting
one/many/none, indexed field reads and TypedExpr intersections execute correctly.
Intersections adding required fields retain their own completed layouts even when
Expr was registered first. Subset projection and widening keep the original live
object storage. Mutable payload writes remain explicit refusals. ABAP ordinary
and test emission explicitly refuse the ABI before body traps.

Implementation checkpoint `3f3350144842021162f336317ce3f34ca25f795c` has the same
compiler source tree as the measured artifact; later checkpoints add refusal
tests and evidence. [Normalized evidence](evidence/union-abi-s2.json) records the
build revision, equivalent exact-identity commit, implementation/source hashes,
full pilot diagnostics, closure histograms, per-title hashes and gate results.
All compiler implementation files remain byte-identical to the measured build.

`TestRecursiveSubsetsNodeOracle` passes all seven original Node differentials:
`shared-secondary-negative`, `multi-literal-fallthrough`,
`extract-one-many-indexed`, `extract-none`, `utility-first`, `shared-typeof`, and
`typed-expr-intersection`. Every accepted fixture has zero blocking diagnostics,
zero HIR errors, zero body/runtime traps and equal Go/Node stdout. Serialization
retains declarations, refinements and equal dumps. The complete Slice 1 recursive
suite also passes. Fourteen source-level invalid contract cases, two live
shared-payload corruption cases and the HIR subset/literal-domain negative tests
refuse invalid payloads, illegal reads or malformed membership. No assertion
mismatch is credited as a refusal.

The core-line mutant widens `familySubsetType`'s retained result back to its full
family (`Name: name` → `Name: u.Name`). It is killed by
`TestRecursiveSubsetsNodeOracle/extract-one-many-indexed`: the payload read
requires narrowing and the resulting method lacks a return. The same unmutated
test passes. The mutation ran in an isolated worktree and is absent from this
branch's implementation.

#### Slice 2 pilot remeasurement

The recorded public command was rerun with `--full --scope tools/gogen
--assume-int=false` on the four original typed roots, without pruning, overrides,
repair traps or OSG source edits. OSG main is
`4fa08e8e4b156b5022081ffcd20e13d3fbc8120d`, clean; dependencies and the explicit
Node-types config adjustment live only in scratch. Original TypeScript 5.9.3 and
the compiler checker both report zero errors. The comparison below uses the same
OSG main inputs for `origin/selfhost` (`fa2ffc9`) and Slice 2.

| Measurement, current OSG main | Before Slice 2 | After Slice 2 |
| --- | ---: | ---: |
| Scoped pilot blockers | 59 | 60 |
| Scoped compiler root / cascade | 53 / 6 | 54 / 6 |
| Scoped untrapped HIR errors | 29 | 29 |
| Whole pilot report blockers | 70 | 71 |
| Whole pilot report root / cascade | 64 / 6 | 65 / 6 |
| Whole pilot report HIR errors | 32 | 32 |
| Checker errors | 0 | 0 |
| Resolved TS/JS implementation files | 41 | 41 |
| Public full-closure raw command | evidence panic, exit 2 | same evidence panic, exit 2 |

Slice 2 exposes one additional explicit refusal for the already-unsupported Stmt
family at the Extract source argument; no previous pilot blocker disappears.
Utilities depending on unsupported recursive members still refuse rather than
publish truncated layouts. The historical Slice 1 61-blocker / 27-HIR-error pilot
uses an older compiler/main snapshot and is not this same-source baseline.
These measurements do not establish executable gogen.

The supplemental original API run returns a complete raw closure inventory:
5,423 blockers and 4,663 HIR errors. The public command instead panics in
`RegistryLowering.Evidence`, `tsfront/registry_build.go:308`, on both compilers;
its missing diagnostic inventory is not reported as zero. Supplemental repair
has 5,424 refusal records (134 dropped declarations, 5,290 trapped failures),
zero repaired HIR errors and zero emitted Go/ABAP files. Go emission refuses the
unsupported lookahead regexp at `tools/gogen/frontend-analysis.mjs:45:17`; ABAP
refuses union declarations. Repaired verification is not execution acceptance.

#### Slice 2 regression gates

| Gate | Result |
| --- | --- |
| `go vet ./...` | Pass |
| `go test -short -p 2 -timeout 40m ./...` | Pass, complete final tree including all new refusal tests |
| Grace lexer facts golden | Pass, tracked golden unchanged |
| Default/selected-target backend guard | Pass |
| Lint against `origin/selfhost` | 0 new issues, canary 5/5, golangci-lint 2.13.2 |
| Complete hir-unit, `HIR_RUNTIMES=" "` | Pass, 0 lint issues, exact OSG/transpiler/runtime/library pins |
| Frozen zabapgit Go source/module comparison | Empty recursive diff; executable excluded |
| Fresh original Node / Go clean and seeded release kit | Both byte-identical: 0 / 5 issues |

The default deployment comparison uses `--scope zabapgit --target go` on both
compilers because `origin/selfhost`'s unqualified CLI default is full core.
Hir-unit uses a separate clean writable OSG copy at
`f83dc5c3e05bfaea71268fd6ad0e9ff88532cce6` and the exact
`f34e9bf4947daaf6464566b42b1e3ef1eaeb369c` transpiler/runtime build. The main OSG
source clone remains clean and read only. Setup failures and superseded runs are
not credited as passed gates.

All seven targeted shards at `577f875ebec44cfaf64841cfe71c8ab8dc32622e` have
identical before/after title/registration/pass/trap/message multisets. Go has
zero failures, zero traps and zero unregistered tests. A separate fresh original
Node build certifies every selected shard with zero failures and unchanged
source/timeouts after compiler jobs finish.

| Shard | Node-active passes before / after | Registered | Node-pending |
| --- | ---: | ---: | ---: |
| `abap/nodes` | 2 / 2 | 2 | 0 |
| `abap/statements` | 4,962 / 4,962 | 4,962 | 0 |
| `abap/syntax` | 1,363 / 1,363 | 1,373 | 10 |
| `abap/types` | 29 / 29 | 29 | 0 |
| `lsp` | 188 / 188 | 193 | 5 |
| `objects` | 144 / 144 | 145 | 1 |
| `rules` | 3,149 / 3,149 | 3,159 | 10 |
| **Total** | **9,837 / 9,837** | **9,863** | **26** |

The initial after-run original Node statements oracle had one ADD timeout at its
existing 250 ms limit during concurrent compilation. That failure is retained
and not credited as FAIL 0. Fresh original Node certification reran all seven
shards with the same source and timeouts: 9,837 passes, 0 failures, 26 pending.
No test source or timeout was changed, and no failure was relabeled a refusal.
This targeted run does not certify all 17 shards or the full 11,060 denominator.
All changes are committed locally with the requested identity/trailers; no push.
