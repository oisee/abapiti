# Chai and sized, hole-aware arrays

## Root cause and observation audit

At original upstream `577f875ebec44cfaf64841cfe71c8ab8dc32622e`, the relevant
construct is `new Array(this.nodeCount)` in
`src/abap/2_statements/result.ts:78` (`Result.getNodes`). It allocates the
result, then writes every slot backwards before returning it. There is no
`.fill`, `Array(n)` call, or spread of an unfilled sized array at this site.
The refusal happened during parsing, before the test's Chai assertion.

Affected `abap` tests include `abap_parser`, `ast`, `concat_tokens`,
`cross_object_macros`, `find_global_definitions`, `macros`, `macros_via_include`,
`native_sql_colon`, `nesting`, `qualified_name`, `sql_path_ast`, `statement_parser`, and
`structure_errors`. Typical observations are AST child/token counts, statement
classes, concatenated strings, issue counts, and error descriptions. For example,
`statement_parser` also compares token and pragma arrays deeply; those arrays
are fully populated. The types tests parse class/interface/form definitions and
observe names, flags, visibility, parameter lengths, and dense mapped name lists
(e.g. FOR LOCK importing/exporting signatures).

A diagnostic Node run against the harness's **fresh original build** records
positive-sized `getNodes` calls in exactly **96 abap and 23 types registrations**,
and **zero holes in any returned array**. [All affected titles and source files](evidence/chai-sized-observations.json)
are retained, including duplicate-title occurrence numbers. This instrumentation
only observes the original return values; the shard certification oracle is
separate and unmodified.

| Source under `test/abap/` | Affected registrations |
| --- | ---: |
| `abap_parser.ts` | 8 |
| `ast.ts` | 4 |
| `concat_tokens.ts` | 3 |
| `cross_object_macros.ts` | 1 |
| `find_global_definitions.ts` | 3 |
| `macros.ts` / `macros_via_include.ts` | 7 / 2 |
| `native_sql_colon.ts` | 7 |
| `nesting.ts` | 31 |
| `qualified_name.ts` | 1 |
| `sql_path_ast.ts` | 8 |
| `statement_parser.ts` | 9 |
| `structure_errors.ts` | 12 |
| `types/class_definition.ts` | 13 |
| `types/form_definition.ts` | 4 |
| `types/interface_definition.ts` | 4 |
| `types/method_parameters.ts` / `types/types.ts` | 1 / 1 |

| Observation | Certified behavior / retained boundary |
| --- | --- |
| Construction and indexed filling | `allocate` records missing slots; `put` clears presence metadata for each written slot. No construction-time Chai refusal. |
| Length | Counts all allocated slots, including holes; it does not read element padding. Native typed length and Chai's boxed-array length are covered. Existing unsupported casts still trap rather than inventing a length. |
| Deep equal | Real Chai 4 equates `new Array(1)` with `[undefined]`, in both directions. ABAP's typed and dynamic `array_items` adapters still refuse sparse storage before reading padding. Floating-number arrays containing unproven zero signs also retain `chai.equal` refusals; certified integer zero arrays compare correctly. Identity equality remains safe. |
| Shallow/deep include and members | Chai does **not** include undefined in an entirely hole-only array and does not equate its members with `[undefined]`. Retain the shared `chai.array_items` refusal, rather than densifying and confusing these operations with deep equality. |
| `forEach` / `every` | General presence-aware callback lowering snapshots length and checks `array.has`, so callbacks skip holes. Pinned dense callback overrides retain their independent density proofs. |
| `map` / other HIR `ForEach` consumers | The common loop ABI does not encode callback hole skipping or preservation of mapped holes. Chai-enabled ABAP refuses sparse storage at `array.iteration`, including holes introduced during a loop. This deliberately also narrows sparse spread/iterator consumers sharing that ABI. |
| Indexed reads | Optional/reference reads represent undefined; nonoptional primitive reads refuse missing slots instead of fabricating zero. |
| JSON | Typed-array serializers call `dense` and loudly refuse sparse input. Dynamic graph holes contain absent references and serialize as `null`, as does present undefined in array positions. JSON is not Chai's deep-equality adapter. |
| Inspect / diagnostic display | The harness does not implement Node inspect. Chai shim diagnostics deliberately use `[object]` / `[function]`; successful assertions do not perform diagnostic ToPrimitive conversion. No sparse diagnostic is used as an assertion result. |
| Throw matching | Only translated payload exceptions are matched. Chai refusals and native range/storage faults propagate as traps, never as an expected assertion throw. |

The important distinction is observation-specific: Chai deep equality reads a
hole as undefined, while inclusion/member algorithms have different presence
semantics. A single hole-to-default adapter cannot safely serve all of them.
This change is not unrestricted sparse Chai support.

## Regressions

- `TestChaiSizedArrayFixtures` uses `emitABAPNamed` and real Node/Chai answers:
  length 3, filled zero arrays deeply equal, and holes versus undefined in both
  directions. It executes the supported cases and explicitly requires typed
  Chai refusals for sparse equality, include, deep include, members, and loops.
- Existing typed/dynamic sparse-storage fixtures retain their narrow refusals.
- `TestChaiSizedArrayNodeOracle` compares seven frontend cases with real Chai.
  Go passes all seven. OSG passes length, nonzero fill/deep equality and sparse
  `forEach`, retaining two **narrow sparse equality traps** and two **unproven
  floating zero-sign traps**, with zero assertion failures. The integer-array
  emitter fixture separately proves zero-fill/deep equality. No signed-zero
  safety check is weakened merely to match Node's answer for literal `+0`.
- `TestSizedArrayFillABAPNodeOracle` checks full-array `.fill(value)` receiver
  identity, receiver/value evaluation order, and length capture after evaluating
  the value argument. Indexed writes visit holes; optional start/end forms remain
  unsupported.
- The frontend oracle uncovered a Go sized-construction gap: allocated slots
  were treated as present defaults. Go now records holes at construction too,
  allowing presence-aware `forEach` to skip them. Corpus parity is checked below.
- The host-dump/recovery fixture now expects the sized-array length case to pass,
  without weakening its intentional FAILs or host-trap identity checks.

## Verification

OSGO pin: `16c024c8fc175b5656d643034b0fe8dde484740f`.
The before counts are the supplied 16c024c8 measurement, also recorded in
[the runner resume report](osg-runner-resume.md).

Verified production code: `853feeb3968ead2e06a44a2c04bd1fc54ec87bb0`.
[Compact shard, host and gate evidence](evidence/chai-sized-shards.json).

Numbers below are **pass / FAIL / trap / active unregistered**. The before
measurement is the supplied TS-HA baseline; Go retains its prior all-pass totals.

| Shard | TS-HA before | TS-HA after | TS-HG after |
| --- | --- | --- | --- |
| `abap` (216 active) | 120 / 0 / 96 / 0 | **216 / 0 / 0 / 0** | **216 / 0 / 0 / 0** |
| `abap/types` (29 active) | 6 / 0 / 23 / 0 | **29 / 0 / 0 / 0** | **29 / 0 / 0 / 0** |
| `abap/lexer` (200 active) | 200 / 0 / 0 / 0 | **200 / 0 / 0 / 0** | **200 / 0 / 0 / 0** |
| Total | 326 / 0 / 119 / 0 | **445 / 0 / 0 / 0** | **445 / 0 / 0 / 0** |

Both final targets report **449 registrations: 445 active passes and four
Node-pending skips**. The pending cases belong to `abap`, remain outside the
active denominator, and are not credited as passes. There are no infrastructure
failures, unmatched results, initialization failures or async-pending results.

Passed `go vet ./...`,
`GOFLAGS=-buildvcs=false go test -short -p 2 ./hir/... ./tsfront/ ./cmd/... -count=1`,
and lint against `origin/main`: **0 new issues, canary 5/5, golangci-lint 2.13.2**.
All new/existing focused OSG fixtures pass: two HIR storage/Chai units, the
frontend fill/evaluation-order unit, the frontend Chai oracle with its four
explicit narrow refusals, and the host recovery fixture (**8 pass / 2 intentional
assertion FAIL / 4 trap**, preserving four real host dumps and all 14 results).
Those deliberate host-fixture FAILs are not corpus failures.

Heavy commands use the shared OSG flock, a user systemd scope with
`MemoryMax=48G`, nice 10, and load below 12. Scratch/output remains on RAM;
OSG sources are not edited. The focused shard invocation is:

```sh
ABAPITI_TESTS_SHARDS=abap/types,abap,abap/lexer \
  node tools/abaplint-tests-all.mjs "$UPSTREAM" "$EVIDENCE/shards.json"
```

Use `ABAPITI_TESTS_TARGET=osg` and the pinned `ABAPITI_OSG_HOME` for TS-HA,
then `ABAPITI_TESTS_TARGET=go` for TS-HG, with the required toolchain/cache env.
