# selfhost-s6: five OSG traps

Train: `70e4baf73e4303a66d9edb16aece1d5df233079c`.
Baseline: `9b7aef51cf7846249021cc502f558bed251253bb` (`origin/selfhost`).
Fixed code tested: `5bf2c7d92a4bf9d684e14f352aeb7220e5ce4807`.
Only the `objects` and `utils` shards were selected for the integration reruns.

## Failing titles and first bad commits

Utils, first bad `2f2fcaff5da2e5e43f15c7804184d53b3d878f4d`
(`fix(tsfront): complete cyclic captures and boxed array bindings`):

- `Name validator does not violate a rule if the pattern is ignored`
- `Name validator does not violate a rule if the name is ignored`

Objects, first bad `6bd1fa72b4d40414377875a0db51820934e718ab` (`feat(tsfront): lower resolved aliases and anonymous data shapes`):

- `Funcion Group, parse main xml test, findTextFile, Z case`
- `Funcion Group, parse main xml test, findTextFile, namespace case`
- `malformed object XML ignores structured values where text is expected`

Both groups were bisected with `git bisect run` over the requested merge graph.
Each candidate executed only its group's exact original test titles on OSG.
The scratch Mocha registration hook selected those titles; application and test
bodies stayed unchanged. The closure, runtime bridge and OSG pin stayed fixed
while the compiler changed. Unknown infrastructure/other failures return 125;
neither completed bisect skipped a revision.
The tested refs and verdicts are recorded in
[selfhost-s6-traps-bisect.txt](selfhost-s6-traps-bisect.txt).

## Root causes and fixes

**Utils is abapiti-zone: TypeScript frontend lowering.** The boxed-array change
forced canonical string-array literals into `array<dynamic>` when their
destination was dynamic. ABAP consequently allocated native reference-array
storage. The `NamingRuleConfig.ignorePatterns` live getter needed native
string-array storage and its `arrayView` cast correctly refused the mismatch:
`live_array.go: arrayView: incompatible native array reference`.
Preserve canonical array element storage while retaining the dynamic-element
fallback for contextual tuples. The runtime view checks remain intact.
See [selfhost-s6-utils.md](selfhost-s6-utils.md).

**Objects is ABAP storage/emission.** The HIR change captures structural-literal
property values in source order before assembling constructor arguments.
`parseInt(t.LENGTH, 10)` in `ABAPObject.findTexts`, and the corresponding
initializer in `findTextsTranslations`, are captured as plain Numbers before
they reach the shape's optional numeric storage. Missing XML `LENGTH`
converts to `"undefined"`, whose parsed value is NaN. The scalar ABAP parser's
finite-only guard raises `CX_SY_RANGE_OUT_OF_BOUNDS` before that value can
reach the numeric box. The diagnostic stack and emitted ABAP identify this
explicit parser guard as the source of the exception.

Propagate a tagged numeric destination backward through local, sequence and
optional-conditional captures, for both existing optional storage and storage widened by the ABAP
pass. Emit `int_optional`/`float_optional` before a native scalar is evaluated.
Keep the evaluation-order captures and the Go HIR unchanged. Retain the
original program object when the pass makes no changes, preserving audit
owning-method identity. The production fix is entirely in `hir/abap`; the TypeScript source regression
is in `tsfront/numeric_parse_storage_test.go`.

## Results

Cells show **passed / trapped**; every row has FAIL 0 and unregistered 0.
The one Node-pending `objects` test is outside the 159-test active denominator.

| Code | Runtime | objects | utils |
| --- | --- | ---: | ---: |
| origin/selfhost | OSG | 144 / 0 | 15 / 0 |
| original train | TS-HG | 144 / 0 | 15 / 0 |
| original train | OSG | 141 / 3 | 13 / 2 |
| fixed | TS-HG | 144 / 0 | 15 / 0 |
| fixed | OSG | 144 / 0 | 15 / 0 |

Both fixed runtimes pass **159/159** active tests, with registration parity.

- `go test -short -p 2 ./hir/... ./tsfront/... ./cmd/abapiti/... ./internal/abapfixture/...`: passed on the final code.
- Canonical-array storage regression fails on the untouched train and passes
  after repair; recursive tuple and capture Node oracles pass.
- Numeric parser capture regression fails on the untouched train. Native OSG
  passes both a widened destination and an already-optional destination,
  including sequence preludes, optional constructor wrappers, multiple captures,
  NaN presence and finite 42. An anonymous-record TypeScript regression also
  passes Node/Go/native-OSG parity and fails on the untouched train.
- Exact extra gate, summed `grep -rc 'assignment type mismatch'` over the
  generated default abaplint Go: **baseline 2; final 2**.

## Environment and evidence

Abaplint `577f875ebec44cfaf64841cfe71c8ab8dc32622e`, with `npm ci` at its root
and `packages/core`. Clean OSG
`f83dc5c3e05bfaea71268fd6ad0e9ff88532cce6` from `.github/ci/osgo.ref`, with
`npm ci` and `node tools/osd-libs.mjs`. Node 26.9.0; Go 1.26.0 for bisect, final compilers and generated test programs.
Integration commands use `tools/abaplint-tests-all.mjs` with
`ABAPITI_TESTS_SHARDS=objects,utils`; OSG adds `ABAPITI_TESTS_TARGET=osg` and
`ABAPITI_OSG_HOME`.

Heavy processes ran with `ulimit -v 16777216`; Go package parallelism was at
most 2. A local Node wrapper uses explicit WebAssembly bounds checks and a
256 MiB WebAssembly memory maximum so Node's TypeScript loader does not
reserve multiple GiB of unused address space. Its ordinary JS heap remains
12 GiB. Heavy jobs were serialized after discovering the container's separate
24 GiB memory limit; later Go builds used `GOMEMLIMIT=11GiB`.
Infrastructure retries are excluded from the result table.

Raw reports, frozen bisect/run scripts and counterfactual logs are retained
under `/tmp/s6traps`. The baseline report completed before a local shell
wrapper footer error; its complete JSON was independently checked for
registration parity, zero infrastructure failures, FAIL 0 and traps 0.
The original train objects replay printed diagnostic stacks in a copied Go
runtime; final and baseline native runs used the clean pinned runtime.

No push.
