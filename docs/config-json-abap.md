# Config JSON on ABAP

Base: `origin/feat/fit-config-json` at `fe22375`. The ABAP backend now emits
`json.stringifyConfig` instead of refusing the Config.getDefault round trip.
The writer uses the same source-pinned native property orders as Go, including
base-to-derived rule configuration fields, and numeric index keys before other
record keys. It omits undefined properties, writes undefined array slots as
null, preserves nesting, escapes controls and isolated UTF-16 surrogates, and
uses shortest round-trip numbers with V8's positional/exponent thresholds.
Exact binary-rational midpoint comparisons prevent double rounding and select
the even decimal at exact 17-digit ties; the discovered rounding differences
are covered by emitted ABAP regressions.
Parsed data shapes retain their complete source graph and unknown properties.
Native version inputs still require a property-order proof; cycles, unsupported
shapes and fields outside the pinned default literal raise explicitly.

The writer keeps float arithmetic explicitly in type f and reads decimal
parity from exact packed text, avoiding OSG-JS integer promotion and lossy MOD
evaluation. It uses a recursion stack, indexed reference-array reads through
`REF TO object`, and checked casts. Dynamic reference-array views inspect each element because
reference arrays share erased storage. These avoid the pinned osgo generator's
reference-table LOOP, typed READ and reference-key DELETE limitations. UTF-16
code units use converter bytes instead of OSG-JS's faulty high-byte UCCPI
calculation. Isolated surrogate escapes are decoded through a valid pair and
then sliced to one unit. These parser changes are emitted only with the config
writer, so the manifest-driven default output remains byte-identical.

`TestConfigJSONFixtures` constructs the native default graph, serializes it,
parses it into typed global/syntax/root shapes and dependency arrays, and serializes the copy. The
snapshot comes from a fresh original pinned upstream build via
`tools/config-json-oracle.mjs`: 194 rule configs, 22,011 bytes including its
terminal newline, with all declaration orders checked against V8 own keys.
Three additional fixtures cover nesting, controls, numeric index ordering,
empty/non-index keys, exponent boundaries, extreme finite and subnormal
numbers, precision-sensitive values, surrogate pairs and isolated units,
unknown properties, undefined omission/array nulls, and cycle refusal.
Every byte expectation is computed by Node during fixture generation.

All generated fixtures, including the config writer, parser and typed materializers, pass
hir-lint (ABAP 7.50) with no comments and lines at most 255 bytes. The former 7.02 gate for
the config classes was retired when cast probes moved from TRY to IS INSTANCE OF (7.40).
`.github/ci/hir-unit.sh` runs the four config cases on both osgo and OSG-JS,
alongside the existing HIR and lexer suites. The checkout is pinned at
`7720f6e29b41a6a12faee17ea65076ac6f046552`, with transpiler/runtime
`ee5c2cd1a45aaac6be503a46fc903ce95a946877` from its lockfile.

## Measurements and reproduction

The HIR work directory must contain the pinned checkout as `open-steamgate`.

[Diagnostic counts by message](evidence/config-json-abap.json) record the full
closure before and after. Total blockers remain **347**, including **136 root**
and **211 cascade** diagnostics, with **284 HIR errors**. The JSON frontend
category remains zero; this change fixes the one ABAP backend refusal, rather
than changing TS lowering or reachability. Other frontend blockers prevent
emission of the complete unpruned core, so the 11,060 mocha tests are not yet a
passing compiled-core gate.

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"
export GOMAXPROCS=2 GOFLAGS=-p=2

go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
.github/ci/hir-unit.sh /tmp/config-json-abap-final
ABAPITI_GRACECHECK=1 GRACE_FACTS_OUT=/tmp/config-json-abap/lexer.facts \
  go test -p 2 ./tsfront -run '^TestLexerFactsReport$' -count=1

/tmp/config-json-abap/base-bin abaplint --full -o /tmp/config-json-abap/before
/tmp/config-json-abap/branch-bin abaplint --full -o /tmp/config-json-abap/after
```

The base binary is built from a disposable archive of `fe22375`; the branch
binary is built from this checkout. Generate default outputs using the same
output spelling because generated READMEs contain that spelling:

```sh
/tmp/config-json-abap/base-bin abaplint -o /tmp/config-json-abap/default --target all,go
cp -a /tmp/config-json-abap/default /tmp/config-json-abap/default-before
rm -rf /tmp/config-json-abap/default
/tmp/config-json-abap/branch-bin abaplint -o /tmp/config-json-abap/default --target all,go
diff -r /tmp/config-json-abap/default-before /tmp/config-json-abap/default
```

The diff is empty. Vet, short tests and the Grace lexer facts check pass; no
lexer golden update is needed because HIR operation counts are unchanged.
`TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend` is absent on this base.
The existing `TestConfigJSONRefusals` now requires successful ABAP emission.
