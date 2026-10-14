# LSP runtime views on the ABAP host

Work starts at `origin/integ/shinkansen` (`169582b`). The scope is
`test/lsp/help.ts` and `test/lsp/hover.ts` on TS-HA@OSGO, compared with the
unchanged original Node tests and the Go target. This is a focused measurement,
not a certificate for all 11,060 tests.

## Certified emission changes

- `dynamic.recordString` accepts tagged ordinary records only when no own
  `toString`, `valueOf`, or `__proto__` entry exists. Presence is checked even
  when the stored value is undefined. This follows Go's `recordString` domain
  and returns Node's `[object Object]`. Arrays and nominal class references
  retain explicit refusals; primitive coercion remains checked.
- Native lexer tokens and their `Position` values have a narrow JSON
  certificate. Every constructor must install exactly its declared fields,
  once each in declaration order, with base construction first and no escaping
  `this`. Accessors, unsafe coercion layouts, undefined define-field storage,
  missing/conditional/reordered assignments, deletable optional/dynamic slots,
  and unproven subclasses refuse.
  A sealed `ViewUnsafe` flag still refuses structural adaptation; nominal
  JSON instead checks the constructors and separate accessor/coercion flags.
  Subclass dispatch checks the actual layout before writing fields. The writer
  retains existing undefined, cycle, nonfinite, and `toJSON` boundaries.
- `dynamic.get` reads `constructor` from nominal instances through the existing
  native descriptor protocol, and `name` from boxed descriptors. The first
  DO trap reads `statement.constructor.name` on the nominal
  `src/abap/2_statements/statements/do.ts.Do` returned by `StatementNode.get()`
  in `Help.linkToStatement()`. The statement's real constructor descriptor proves this property read
  while preserving the refusal of structural adaptation.
  The same path covers the structure/expression siblings, and the
  `combi.mapInput()` constructor-name read reached by class help. Structural
  records keep their own-property reader. Shadowed names, unsafe constructor
  accessors, and unknown nominal storage keep explicit refusals.
- Shared reference arrays can carry boxed constructor descriptors. Their typed
  element reads need `ClassValue` unboxing through the checked `as_classvalue`
  ABI, preserving the descriptor object rather than inventing a class view.
- Immediate `dynamic.toString(dynamic.of(Array<string>))` uses the existing
  checked comma join, evaluating the array once. Empty and unbound arrays,
  empty elements, trailing blanks, commas, Unicode, and mutation are covered.
  The class-help `CX_SY_MOVE_CAST_ERROR` was raised by `dynamic.to_string`
  on `IdentifierMeta[]` in `DumpScope.dumpNode`, rather than an invalid
  class cast. Erased references and other element types keep their existing
  refusal; sparse input retains the checked join's explicit refusal.

These operations already have ABAP catalogue entries. No operation is removed
from `unsupportedOps`, and no Go backend or harness/runner is changed.

## Static closure

The unchanged `LowerAbaplintTests` / `MeasureABAPTestGap` counting tool from
[the suite-gap workflow](osgo-suite-gap.md) is used
with all 625 roots: 2,166 files, 7,052 classes, 85 interfaces. Baseline blocking
and HIR errors are zero, with zero unsupported-operation occurrences and one
transformed-HIR emitter refusal: `json.stringifySubset` at
`src/lsp/help.ts:57:35` for `AbstractToken`.

Runtime guards are not emitter diagnostics. Consequently the baseline audit
counts zero sites for the view exception, record-string runtime guard, and
native cast exception. Execution impact must be reported separately rather
than added to the static census. The unused `string.parseFloat` catalogue
refusal remains outside this work.

| Assigned item | Before static sites | After static sites | Counting boundary |
| --- | ---: | ---: | --- |
| `AbstractToken` JSON order refusal | 1 | 0 | Distinct transformed-HIR emitter diagnostic |
| View exception | 0 | 0 | Runtime guard; measured by execution |
| Record-string primitive-only guard | 0 | 0 | Runtime guard; measured by execution |
| Native move-cast exception | 0 | 0 | Runtime exception; measured by execution |

The final full census has zero unsupported-operation occurrences, zero emitter
refusals, zero nonfinite literal sites, zero blocking diagnostics, and zero HIR
errors. This static zero does not certify every runtime input.

## Execution and gates

The emitted fixtures use `emitABAPNamed` and execute through the repository's
`storageHostFixture`, pinned gogen frontend and OSG kernel. Token JSON output
is compared with a fresh Node oracle, and an unproven token subclass is tested
through the certified base writer. Constructor-descriptor and native-array
fixtures also compare with fresh Node. Record-string cases cover ordinary
records, undefined coercion hooks, primitives, arrays, and class references.

The selected baseline at `169582b` is **110/114 active tests passed, FAIL 0,
four traps**, with three Node-pending tests outside the denominator. There is
one trap for each assigned item. The baseline binary was built from untouched
`169582b`; its runner summary sampled the later `4fbab47` repository HEAD
during execution. The evidence preserves that summary value separately from
the actual compiler revision. The older `05969f7` handoff's three view traps
are not substituted for this fresh selected-file baseline.

The Go target is **114/114 active tests passed, zero failures or traps**, with
three pending titles and no unregistered titles. All Go production files are
unchanged by these commits.

The final host result is **113/114 active tests passed, FAIL 0, one trap**.
All four original trap messages are gone. The remaining test is
`LSP, help class`: after metadata string coercion succeeds, `Help.dumpInfo()`
reaches an explicit strict-JSON-subset refusal for the native
`src/abap/4_file_information/_identifier.ts.Identifier` in class-information
records. An isolated kernel stack trace confirms the chain
`Help.dumpInfo -> typed dump record -> InfoClassDefinition -> Identifier`.
Its writer has no native own-key/toJSON certificate and refuses bound
instances. This is separate JSON-owner work, outside the specifically requested
lexer-token certificate; it remains refused rather than producing a guessed
object or omitted value.

| File | Before OSG pass / active | Before traps | After OSG pass / active | After traps | Go pass / active |
| --- | ---: | ---: | ---: | ---: | ---: |
| `test/lsp/help.ts` | 0 / 3 | 3 | 2 / 3 | 1 (class-information JSON) | 3 / 3 |
| `test/lsp/hover.ts` | 110 / 111 | 1 | 111 / 111 | 0 | 111 / 111 |
| Total | 110 / 114 | 4 | 113 / 114 | 1 | 114 / 114 |

Every row has zero failures. Three pending hover tests remain outside the active
denominator. Six borrowed dependency tests are excluded by the unchanged
ownership tool. The complete final OSG run uses the original selected-file
runner, fresh original Node tests, the CI OSG pin, and disabled frontend/binary
caches. An earlier interrupted infrastructure attempt and its staged diagnostic
execution are not used as the final pass certificate. Isolated kernel stack
traces identify refusal origins without changing the measured artifacts or
committed harness.

| Gate | Final result |
| --- | --- |
| Pinned gogen/OSG emitted fixtures and selected files | Compiled and executed; selected files 113 pass, 1 explicit JSON refusal, 0 fail |
| `go vet ./...` | PASS |
| `go test -short -p 2 ./...` | PASS |
| Uncached `ABAPITI_GRACECHECK=1` `TestLexerFactsReport` | PASS; golden unchanged, no regeneration |
| `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend` | PASS |
| `lint.sh gate origin/main` | PASS; 0 new issues |
| Default zabapgit Go raw `diff -r` against `169582b` | Empty, including binaries |
| Default zabapgit Go raw `diff -r` against `origin/main` | **FAIL: inherited difference** |

The `origin/main` snapshot is `fd77046`. Its default Go output differs from
untouched shinkansen before this work: `hir.go` and `hir_profile.go` use the
older array allocation; shinkansen uses `sizedArray`, and its `runtime.go`
already tracks sparse-array holes. The generated binary also differs. Both
comparisons generate at the same output path with identical build flags.
The raw base-to-final comparison is empty, and this branch changes no Go
backend, frontend, or harness code. The required main-parity gate is therefore
reported as failed, not hidden by a normalized comparison or an unrelated Go
backend change.

Portable counts, pins, per-file outcomes, fixture names, and gate results are
in [the evidence extract](evidence/osgo-lspviews.json). Measurements use code
commit `9889ff7`; the final commit adds only this report and the evidence extract.

## Reproduction

Use a clean upstream checkout at `577f875ebec44cfaf64841cfe71c8ab8dc32622e`,
OSG at the revision in `.github/ci/osgo.ref`, and transpiler/runtime at
`ee5c2cd1a45aaac6be503a46fc903ce95a946877`. Caller-supplied paths remain outside
committed evidence. Session builds run serially with at most two Go package
workers.
No `ABAPITI_OSG_CACHE` is set.

```sh
export GOTOOLCHAIN=go1.26.0 GOMAXPROCS=2
export GOFLAGS="-p=2 -buildvcs=false -gcflags=all=-dwarf=false '-ldflags=-s -w'"
export NODE_OPTIONS=--max-old-space-size=24576
export ABAPITI_TESTS_FILES=test/lsp/help.ts,test/lsp/hover.ts
unset ABAPITI_OSG_CACHE
ABAPITI_OSG_HOME="$OSG" ABAPITI_TESTS_TARGET=osg node tools/abaplint-tests-all.mjs "$UPSTREAM" "$OSG_RESULT"
ABAPITI_TESTS_TARGET=go node tools/abaplint-tests-all.mjs "$UPSTREAM" "$GO_RESULT"
ABAPITI_INLINE=classic abapiti abaplint-tests "$FULL_CLOSURE" \
  --target osg --gap-only -o "$GAP_OUTPUT"
```

Go's `GOFLAGS` parser requires quoting the entire flag containing spaces,
as shown above. A preliminary invocation with only the `-ldflags` value
quoted failed before compilation and is not counted as a measurement.
