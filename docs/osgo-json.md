# ABAP JSON subset and indentation

Base: `origin/main` at `428a8d5acbd4e3be6331b01d5d9e0afc71df0dc4`.
`json.stringifySubset` and `json.indent` now have ABAP emitters and catalogue
entries. This is a JSON backend step toward TS-HA@OSGO, not a certificate that
all 11,060 upstream tests pass on the ABAP host.

## Static full closure

The unchanged `MeasureABAPTestGap` tool was run with production
`ABAPITI_INLINE=classic` on all 625 roots / 17 shards: 2,166 files,
7,014 classes, 85 interfaces, zero blocking diagnostics and HIR errors.
Main has 38 fewer classes than the historical M0 closure; this work changes
no frontend lowering. [Compact evidence](evidence/osgo-json.json) preserves
both inventories and the intermediate refusal measurement.

| JSON refusal inventory | Before | Final |
| --- | ---: | ---: |
| `json.stringifySubset` catalogue sites (pre-pass) | 34 | 0 |
| `json.indent` catalogue sites (pre-pass) | 3 | 0 |
| JSON shape refusals (post-pass) | 0 | 1 |
| `IConfig` shape refusals (post-pass) | 0 | 0 |

The first writer iteration exposed 27 post-pass shape refusals, including
23 `IConfig` sites. Per-instance structural own-key storage removes 26 of
those refusals, including all 23 Config sites. The final refusal is
`JSON.stringify(t)` at `src/lsp/help.ts:57:35`, with receiver
`AbstractToken`: its nominal token hierarchy has no ABAP own-property
presence/order storage. It remains an explicit owning-body refusal.

Pre-pass operation occurrences and post-pass, source/body-deduplicated emitter
refusals are different inventories; the table is not a subtraction that
certifies 36 newly passing tests. Runtime shape/hook refusals and full-suite
impact remain unmeasured. Other emitter groups remain unchanged: 66 dynamic
Number `parseInt` sites and four I64-boxing sites. The catalogue has 22
unsupported operations, down from 24.

An initial full audit was killed at the container memory limit before it
returned counts. Both final inventories completed serially with
`GOMEMLIMIT=8GiB GOGC=30`; no counts from the killed run are credited.

## Semantics and refusal boundary

The subset writer reuses the existing Config writer's UTF-16 escaping and
shortest binary64 formatting, with separate methods for subset semantics.
Tagged records preserve insertion order, with canonical array-index keys first.
Undefined and constructor-function properties are omitted; their array slots
become `null`. A string result cannot represent an undefined/function root, so
that root raises explicitly. Integers outside the JS safe-integer domain,
unsupported dynamic tags and cyclic graphs also raise.

Structural literals store their existing HIR `OwnKeys` proof per instance.
Constructor arguments are evaluated and captured in source property order;
successful field and live-adapter writes retain or append keys, including
undefined keys and unknown live-view properties. The writer enumerates those
keys and reads current values. Missing constructor proofs, control characters
in structural field names, and narrowed wide-constructor ABIs refuse.

Parsed data shapes enumerate their retained source keys but serialize current
field values, including nested mutations and undefined omission. Unknown keys
survive. One newly present field has a provable insertion position; multiple
new fields refuse because their write order is not recorded. Native instances
without source metadata refuse instead of borrowing Config's pinned literal
orders. Live class views serialize their shared backing graph. Declared or
inherited `toJSON` methods/getters and function-valued `toJSON` properties
refuse. Constructor functions skip only after their descriptor chain proves
absence of static/inherited hooks; opaque, accessor/computed-hook or lexical
metadata refuses. Tagged `__proto__` entries also refuse because the storage
does not distinguish data properties from prototype changes.
This work does not invent a callable ABI or skip a reached hook.

Unproven nominal layouts, constructor proofs, interfaces, sets, RegExp and
arrays of unproven element types retain emitter refusals. Boxed instances without an
accepted backing representation retain runtime refusals. HIR map storage is
also refused: it does not prove whether the source was a record or an ES `Map`,
whose Node serialization differs. Refusals in dynamically reached values
remain runtime checks and are not counted as static emitter diagnostics.

Indentation scans the compact writer output without re-parsing its leaves or
changing key order, escaping or number spellings. It follows the Go backend's
quote/escape state machine and truncates the gap to ten UTF-16 code units,
including a truncation between surrogate units. Numeric literal gaps are
already clamped/truncated by the unchanged frontend.

## Executed proof

Eighteen ABAP Unit fixtures passed on pinned OSG
`f557e12f4217addcec7834b0607234aecf9b723a`, through its gogen frontend,
generated Go compilation and kernel execution: six `TestJSONSubsetFixtures`,
six `TestJSONSubsetMaterializedFixtures`, `TestJSONSubsetConfigFixture` and
`TestLiveClassViewJSONNodeOracle/json-live`, plus four
`TestABAPJSONStructural*NodeOracle` fixtures (argument/key order, shared
unknown-field writes, Config-shaped updates and a 45-field literal). Each
report has one compiled test,
one SUCCESS, and zero failures, errors or NOT_COMPILED rows. This is actual
OSGO execution, not an abaplint-only syntax claim. These cases are added to the
existing HIR CI execution kit; no upstream suite harness, runner, assertion
classifier or counting implementation is changed. Its catalogue-count
regression now expects 22 entries rather than 24.

Fresh Node oracles supply exact expectations for index/non-index ordering,
empty keys, escaping, controls, surrogate pairs and isolated units, `-0`,
exponent thresholds, subnormals, maximum finite values and precision-sensitive
rounding cases. Expectations are transported as JSON strings where necessary
so a lone indentation surrogate is not replaced during UTF-8 stdout encoding.
Direct primitive, optional and typed-array operations are exercised too.
Negative ABAP executions check undefined/function roots, unsafe I64, unknown
tags, cycles, unproven native `toJSON`, function-valued `toJSON`, missing source
metadata, multiple new keys, opaque function metadata and `__proto__`
ambiguity. Parsed Config and live-view cases verify
nested/shared mutations against Node. Reference-table reads use a raw
`REF TO object` followed by `?=`, including arrays of parsed data shapes.

## Gates

Validated: `go vet ./...`; `go test -short -p 2 ./...`;
`ABAPITI_GRACECHECK=1 go test -p 2 -count=1 -run TestLexerFactsReport ./tsfront/`;
`TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`; and
`./.github/ci/lint.sh gate origin/main` (zero new issues, all five canaries).
No lexer-facts golden regeneration is needed: frontend lowering is unchanged.
The short suite initially exceeded its default timeout under concurrent
compiler work, passed with a longer timeout, then passed with the requested
command. Vet/tests and the lint canary run are serialized for the final gates.

Default zabapgit output, generated with the same output spelling using base
and branch binaries and `--target all,go`, has an empty raw `diff -r`: zero
bytes across 6,324 files, including the zip. Go emission code is unchanged.

## Reproduction

Use disposable `SCRATCH`, a clean pinned upstream checkout in `UPSTREAM`, and
the pinned OSG checkout in `OSG`. Generate the unchanged full closure with
`discoverShards` from `tools/abaplint-tests-all.mjs` and
`materializeRegistryClosure` from `tools/registry-closure.mjs` (all 625 roots,
17 shards). Run the original counting command on each revision:

```sh
GOMEMLIMIT=8GiB GOGC=30 ABAPITI_INLINE=classic \
  abapiti abaplint-tests "$SCRATCH/closure" --target osg --gap-only \
  -o "$SCRATCH/gap"

ABAPITI_TEST_OUT="$SCRATCH/hir" go test -p 2 -count=1 ./hir/abap \
  -run '^TestJSONSubset'
ABAPITI_TEST_OUT="$SCRATCH/live" go test -p 2 -count=1 ./tsfront \
  -run '^(TestLiveClassViewJSONNodeOracle|TestABAPJSONStructural.*NodeOracle)$'
```

For each emitted fixture directory, run the existing syntax gate, then:

```sh
(cd "$OSG" && GOTOOLCHAIN=go1.26.0 GOFLAGS='-p=2 -buildvcs=false' \
  npm run -s osgo:unit -- FIXTURE --json --jobs 2)
```

Go test fixture generation
alone does not execute ABAP; the OSG command supplies the compile/execution
proof. OSG-JS and the full upstream ABAP-host suite were not run here.
