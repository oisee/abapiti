# Identifier JSON on TS-HA@OSGO

This work starts at `origin/integ/shinkansen` (`90a02a5`) and addresses the
remaining `LSP, help class` strict-JSON refusal. It does not certify the entire
11,060-test suite. The implementation is `b31884e`; no Go backend, frontend,
operation catalogue, harness, or runner was changed.

## Constructor-order certificate

The native JSON certificate now includes
`src/abap/4_file_information/_identifier.ts.Identifier`, alongside the existing
lexer-token and Position families. It uses the existing
`json.stringifySubset` writer, rather than inventing a structural class view.
The source Identifier declares `token`, then `filename`, and its constructor
assigns them exactly once in that order. Fresh original Node compilation
confirms those enumerable own keys in the class-help JSON. Its token contains
`start`, `str`, `strUpper`; Position contains `row`, `col`.

The certificate checks every constructor from base to subclass. Each class must
install all its nonstatic fields once, in declaration order; explicit base
construction must precede assignments. Initializers must not expose `this`.
Conditional, missing, repeated, or reordered assignments refuse. Unknown roots,
accessors, scoped layouts, unsafe coercion, undefined define-field storage,
private `#` slots, duplicate inherited keys, `__proto__`, and deletable optional
or dynamic slots remain outside the certificate. Instance/inherited `toJSON`
hooks refuse. Each actual subclass must independently prove its complete layout;
an unproven subclass raises the strict-JSON exception through the base writer.

A structural assertion can mark a class `ViewUnsafe`; the constructor proof
permits nominal serialization without granting structural adaptation. The writer
keeps the existing undefined, cycle, nonfinite-number, and `toJSON` boundaries.
It reads fields in certified order, recursively writes their real values, and
uses the ordinary ordered JSON graph writer. No refusal becomes a default value.
No operation was removed from `unsupportedOps`.

## Executed regressions

`TestLSPIdentifierJSONDeclarationOrder` uses `emitABAPNamed` and
`storageHostFixture`. Pinned gogen and the OSG kernel compile and execute the
emitted ABAP. It compares nested Identifier/token/Position JSON with a fresh
Node oracle byte-for-byte, including escaped quotes, Unicode, and a trailing
blank in the filename. The same emitted fixture requires an unproven Identifier
subclass to raise `exception.RegistryJSONSubsetError`. Constructor-certificate
negative cases cover order, missing/duplicate/conditional assignments, escaping
`this`, hooks, accessors, define-field storage, optional/dynamic fields, and an
unknown root. The existing token JSON and refusal regressions also pass on OSG.
The new emitted fixture separately passes CI HIR lint with zero issues.

## Static and execution evidence

The full closure uses the unchanged `LowerAbaplintTests` /
`MeasureABAPTestGap` tool from [osgo-suite-gap.md](osgo-suite-gap.md), with
`ABAPITI_INLINE=classic`. Runtime-operation occurrences are counted before ABAP
passes; emitter diagnostics are distinct transformed-HIR refusal sites. A
runtime guard is not an emitter diagnostic.

The fresh baseline full census has 625 roots, 2,166 files, 7,052 classes,
85 interfaces, zero blocking diagnostics, zero HIR errors, zero unsupported-op
occurrences, zero emitter refusals, and zero nonfinite literals. The candidate
selected-file closure has 1,541 files, 2,069 classes, 75 interfaces, with the same
zero diagnostic inventories. The fresh candidate full census matches the full
baseline: 2,166 files, 7,052 classes, 85 interfaces, and zero diagnostic
inventories.

The fresh untouched `90a02a5` baseline on the same OSG pin is **2/3 passed,
FAIL 0, one trap**: `LSP, help class` raises
`exception.RegistryJSONSubsetError` with the strict-JSON-subset message.
The candidate removes that runtime refusal.

| Assigned item | Before static sites | After static sites | Before execution traps | After execution traps |
| --- | ---: | ---: | ---: | ---: |
| Identifier native JSON certificate | 0 | 0 | 1 | 0 |

The static columns count emitter diagnostics, not runtime guards; the execution
columns count the one original class-help test, not the number of Identifier
instances serialized. The pre-existing unused `string.parseFloat` catalogue
refusal is outside this work.

The unchanged original `test/lsp/help.ts` is **3/3 passed on OSG, FAIL 0,
traps 0**, and **3/3 passed on Go, FAIL 0, traps 0**. Fresh original Node is
3/3. There are no pending, missing, unmatched, initialization-failure, or
unregistered titles. The OSG run emits 2,325 ABAP files, maximum line length
255 bytes, zero overlong lines, and maximum object-name length 29 bytes.
Frontend and binary caches are disabled.

A separate supplemental command calls the unchanged gogen-emitted functions,
using the original emitted class-help setup and registration initialization.
Only its observation entry point differs; the measured runner, emitted ABAP,
and emitted function bodies are unchanged. Fresh original Node compilation
supplies the oracle. Raw byte comparisons, without whitespace normalization,
match both the JSON substring and the entire returned HTML:

| Output | Node bytes | OSG bytes | SHA-256 on both hosts |
| --- | ---: | ---: | --- |
| Info Dump JSON | 1,052 | 1,052 | `7ec943dd842bb3bf7cc022dfca024a3578326a889b5094ca7f9e99bd3012e2c0` |
| Complete class-help HTML | 6,801 | 6,801 | `689060fa1769728ab975d929dc0e52c5c07a4bf2d573d578262974962fd46c0f` |

The class-information dump includes two separate Identifier instances, with
token positions `(2, 7)` and `(4, 7)` and filename `zcl_foobar.clas.abap`.
Both retain `token` before `filename` in the exact Node output.

## Gates and scope limit

The complete CI `hir-unit.sh` suite passes with `HIR_RUNTIMES=" "`, using the
current OSG and matching transpiler/parser pins. `go vet ./...`,
`go test -short -p 2 -timeout 30m ./...`, the uncached Grace lexer-facts test,
`TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`, and
`lint.sh gate origin/main` pass. Lint reports zero new issues; the Grace golden
is unchanged and was not regenerated.

The first short-test attempt reached its ten-minute timeout during concurrent
verification. A subsequent run completed `tsfront` but overlapped lint's
creation/removal of its temporary canary package. The final complete run uses a
stable tree and passes. The first host setup attempt detected npm's incidental
scratch lockfile rewrite; it is excluded from execution evidence. The lockfile
was restored before the successful clean-pin run. No gate or assertion was
weakened.

Default zabapgit Go output is generated at the same path with identical flags
using fresh main, untouched baseline, and candidate compilers. Raw `diff -r`
against `90a02a5` is empty, including the binary. The required comparison against
`origin/main` (`fd77046`) **fails on inherited differences**: `hir.go` and
`hir_profile.go` use `sizedArray` on shinkansen; its `runtime.go` tracks sparse
array holes; the binary also differs. These differences are present in the
untouched baseline. This ABAP-only step does not repair that integration gate or
claim main parity.

## Reproduction

Use clean original upstream `577f875ebec44cfaf64841cfe71c8ab8dc32622e`, OSG at
`.github/ci/osgo.ref` (`f83dc5c3e05bfaea71268fd6ad0e9ff88532cce6`), and its
matching transpiler/runtime `f34e9bf4947daaf6464566b42b1e3ef1eaeb369c` with
parser 2.120.73. Paths below are caller-supplied. Do not enable OSG caches.

```sh
export GOMAXPROCS=2 GOTOOLCHAIN=go1.26.0
export GOFLAGS="-p=2 -buildvcs=false -gcflags=all=-dwarf=false '-ldflags=-s -w'"
export NODE_OPTIONS=--max-old-space-size=24576
unset ABAPITI_OSG_CACHE
ABAPITI_OSG_HOME="$OSG" go test -p 2 -count=1 -v ./hir/abap \
  -run 'TestLSP(Token|Identifier)JSON'
ABAPITI_TESTS_TARGET=osg ABAPITI_TESTS_FILES=test/lsp/help.ts \
  ABAPITI_OSG_HOME="$OSG" node tools/abaplint-tests-all.mjs "$UPSTREAM" "$OSG_RESULT"
ABAPITI_TESTS_TARGET=go ABAPITI_TESTS_FILES=test/lsp/help.ts \
  node tools/abaplint-tests-all.mjs "$UPSTREAM" "$GO_RESULT"
ABAPITI_INLINE=classic abapiti abaplint-tests "$FULL_CLOSURE" \
  --target osg --gap-only -o "$GAP_OUTPUT"
HIR_RUNTIMES=" " TRANSPILER="$TRANSPILER_BUILD" \
  .github/ci/hir-unit.sh "$CI_WORK"
```

All changes are committed locally with the requested identity and trailers.
Nothing is pushed. Compact portable measurement evidence is in
[osgo-identjson.json](evidence/osgo-identjson.json).
