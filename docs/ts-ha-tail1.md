# TS-HA@OSGO: tail batch 1

Upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`; OSGO: `16c024c8`.
The original upstream Mocha/Chai oracle is independent of the translated runner.
All counts below exclude Node-pending tests from the active denominator.

## Repairs

* **Character classes:** SOTS uses
  `/^(\/[A-Z_\d]{3,8}\/)?[A-Z_-\d<> ]+$/i`; W3MI uses
  `/^[A-Z_-\d/<> ]+$/i`. In non-Unicode JavaScript, `_ - \d` is the
  Annex B literal-hyphen case, not a range from `_` to a shorthand set.
  Expanding `\d` first had produced the invalid class `[A-Z_-0-9...]`.
  Normalize class atoms and consumed ranges **before** shorthand expansion;
  move literal hyphens to an edge, preserving genuine ranges. Apply the same
  normalization to static emission, runtime construction, and PCRE spelling.
  Escaped brackets, escaped hyphens, negation, and BMP range endpoints retain
  their meaning. RegExp source/toString is not rewritten by construction.
* **NROB:** `NumberRange.parse()` stores `parseFloat(attributes?.PERCENTAGE ||
  "")` in an optional numeric field. Empty text produces **NaN**, not
  undefined or zero. Route that optional store through a tagged nonfinite lane:
  `has` remains true; NaN and both infinities survive boxing, materialization,
  truth tests, strict equality, Chai deep equality, and JSON serialization.
  Native `f` arithmetic/unboxing of nonfinite values still explicitly refuses;
  this is not a claim of a general IEEE Number arithmetic ABI.
* **Domain AFF:** distinguish strict-JSON grammar errors from valid JSON whose
  value exceeds the representation boundary. Only the grammar signal becomes a
  translated throw carrying a native `SyntaxError` (with `Error` ancestry), so
  source catches and settled async rejection use their existing payload ABI.
  Numeric representation refusals and native faults are not swallowed. The
  diagnostic is generic `Invalid JSON`, not V8's version-specific position text.
* **Semantic highlighting:** the comparison is the plain numeric token array
  against `[0,0,5,17,0,0,6,7,20,0,0,7,1,0,0]`. Native numeric-array zeros lacked
  source-proven sign metadata. Observe an unknown zero's actual sign with the
  checked `DECIMALS = 0` template instead of refusing. Number unary negation
  also uses a genuine unary minus, not binary `0 - x`, to retain IEEE zero
  signs. Deep equality continues to distinguish positive and negative zero;
  sparse-array refusals remain.

## Not-run audit

All **26** not-run records correspond exactly to original Node `it.skip`
registrations: **10 rules, 10 syntax, 5 LSP, 1 object**. There are **0 active
skips, 0 unregistered active tests, and 0 async-pending cases** in this selection.
No registration/accounting change or forced execution of pending bodies is
appropriate.

| File | Count | Pending titles (file/suite prefix omitted) |
| --- | ---: | --- |
| `test/rules/downport.ts` | 5 | REDUCE FOR GROUPS; FOR loops, identical variable names, step 1; SELECT AS; SELECT UNION; basic value FOR |
| `test/rules/prefer_inline.ts` | 2 | Dont inline, type P, this will change the type?; Types should not change when inlining |
| `test/rules/uncaught_exception.ts` | 2 | no error, super classed, local exceptions; PROG with INCL, expect error |
| `test/rules/unknown_types.ts` | 1 | No class/method implementation but DEFAULT not found |
| `test/abap/syntax/syntax.ts` | 10 | function module definition not found; error cannot define length and decimals for date; Error, overlapping names; access instance variable from interface in static method, error; Error, insert, types incompatible; accessing me is not possible in static methods; fields inside TYPES cannot be generic; return type doesnt match importing; STRUCTURE typing gap, FORM parameters typed against program-local names are never checked; STRUCTURE typing gap, TABLES accepts sorted and hashed tables |
| `test/lsp/hover.ts` | 3 | keyworld should not resolve; hover method parameter; Hover inline SELECT data voided |
| `test/lsp/references.ts` | 2 | method references; find references for macro definition |
| `test/objects/proxy_object.ts` | 1 | adhoc |

Async callbacks alone are not Node-pending: the other async tests execute and
use the certified settled-promise ABI. The independent oracle's `pendingTitles`
multiset, not callback syntax, determines exclusion from the denominator.

## Measurements

The combined baseline has **1778 pass / 0 FAIL / 8 trap**, with **26 pending**
and **1812/1812 registrations**. Five traps are owned by this batch; the other
three are the two BDEF negative-lookbehind cases in `unknown_types.ts` and the
static-modifier hover record-conversion case.

Counts are **pass / FAIL / trap**, with explicit pending shown separately.
The focused totals exclude the lexer; the combined verification includes it.

| File (under `test/`) | OSG before | OSG after | Go after | Pending |
| --- | --- | --- | --- | ---: |
| `rules/allowed_object_naming.ts` | 27 / 0 / 3 | 30 / 0 / 0 | 30 / 0 / 0 | 0 |
| `objects/domain.ts` | 13 / 0 / 1 | 14 / 0 / 0 | 14 / 0 / 0 | 0 |
| `lsp/semantic.ts` | 0 / 0 / 1 | 1 / 0 / 0 | 1 / 0 / 0 | 0 |
| `rules/downport.ts` | 298 / 0 / 0 | 298 / 0 / 0 | 298 / 0 / 0 | 5 |
| `rules/prefer_inline.ts` | 23 / 0 / 0 | 23 / 0 / 0 | 23 / 0 / 0 | 2 |
| `rules/uncaught_exception.ts` | 21 / 0 / 0 | 21 / 0 / 0 | 21 / 0 / 0 | 2 |
| `rules/unknown_types.ts` | 136 / 0 / 2 | 136 / 0 / 2 | 138 / 0 / 0 | 1 |
| `lsp/hover.ts` | 110 / 0 / 1 | 110 / 0 / 1 | 111 / 0 / 0 | 3 |
| `lsp/references.ts` | 12 / 0 / 0 | 12 / 0 / 0 | 12 / 0 / 0 | 2 |
| `objects/proxy_object.ts` | 1 / 0 / 0 | 1 / 0 / 0 | 1 / 0 / 0 | 1 |
| `abap/syntax/syntax.ts` | 1137 / 0 / 0 | 1137 / 0 / 0 | 1137 / 0 / 0 | 10 |
| **Focused total** | **1778 / 0 / 8** | **1783 / 0 / 3** | **1786 / 0 / 0** | **26** |
| `abap/lexer` regression | — | **200 / 0 / 0** | **200 / 0 / 0** | 0 |
| **Combined after** | — | **1983 / 0 / 3** | **1986 / 0 / 0** | **26** |

The combined run registers **2012/2012** tests on both targets, covering
**1986 Node-active** cases plus the **26 Node-pending** cases. All five owned
traps are repaired; the three remaining traps have unchanged titles/reasons.
Source and synthetic native checks include five successful OSG unit executions:
class-member membership against a Node oracle, optional parseFloat including
NaN/infinities and unsafe-unboxing refusal, ordinary/async syntax catches, and
computed signed-zero scalar/array comparisons.

## Gates

All gates pass with Go **1.26.0**:

* `go vet ./...`
* `GOFLAGS=-buildvcs=false go test -short -p 2 ./hir/... ./tsfront/ ./cmd/... -count=1`
* All **15** `tools/*-test.mjs`, each invoked individually with `node --test`
  (not the runner itself).
* Native `TestChaiSizedArrayNodeOracle`: **5 pass / 0 FAIL / 2 intentional
  sparse-array equality traps**; the independent Node oracle passes all seven.
* `.github/ci/lint.sh gate origin/main`: **0 new issues**, canaries **5/5**,
  pinned golangci-lint **2.13.2**.

The global-definition golden includes `number_special` for every numeric
optional ABI, including I32/I64 wrappers that may share runtime storage.
New emitter fixtures use `emitABAPNamed`; the golden uses its existing tracked
emitter helper. No fixture emission bypass was added.

### CI lint follow-up

The initial gates above missed the generated-ABAP lint set in `hir-unit.sh`.
After the class-section/numeric-optional repairs, subset JSON still discovered
its synthesized ClassValue handler too late: builtin Error implemented the
described interface, but that interface had not been emitted. Usage scanning
now enables the descriptor ABI before emitting interfaces or any class.
A regression covers an early class, the JSON caller, and synthetic builtin Error.

The config JSON v702 gate also could not parse a shared array dependency's
live-box `IF ... IS INSTANCE OF` block. An equivalent checked cast preserves
boxing and identity without that newer structural syntax. A second regression
covers the generated shared array helper.

`HIR_RUNTIMES=" " GOFLAGS=-buildvcs=false .github/ci/hir-unit.sh "$work"` exits **0**:
all **34** ABAP lint reports have **0 issues**, including config JSON v702.
The checkout matching the branch's `16c024c8` OSG pin is needed for the final
lexer validation. No lint rules or fixture selection were weakened.

The eleven-file focused OSGO recheck after `8025ea0` remains **1783 pass /
0 FAIL / 3 known traps**, with **1812/1812 registrations**, **26 Node-pending**,
and **0 active skips / unregistered / async-pending**. The scoped build and
executions used the shared heavy-run lock and the same pinned runtime.
`go vet`, the short Go suite, all 15 individual Node test files, and the
0-new-issues lint gate are green. OSG source remained clean; no generated
`tools/gogen/.out/go-cache` was left behind.

## Reproduction and resource discipline

Select the eight pending-audit files above plus
`test/rules/allowed_object_naming.ts`, `test/objects/domain.ts`, and
`test/lsp/semantic.ts` (eleven focused files). Add the three
roots of `test/abap/lexer` for the lexer regression. Use comma-separated
`ABAPITI_TESTS_FILES`, `ABAPITI_TESTS_KEEP_ARTIFACTS=1`, and a union build size
covering the selected shards (`ABAPITI_OSG_SHARDS_PER_BUILD=5`). Run each target:

```sh
for target in osg go; do
  ABAPITI_TESTS_TARGET="$target" node tools/abaplint-tests-all.mjs \
    "$ABAPLINT_HOME" "$SCRATCH/$target.json"
done
```

Each heavy command uses the shared flock, a user systemd scope with
`MemoryMax=48G`, and `nice -n 10`, admitted only below load 12. The specified
OSGO/transpiler/toolchain environment is retained. Evidence stays on RAM.
When the shared SSD cache filled, subsequent commands used a process-local RAM
mount at the specified `GOCACHE` path, without replacing or clearing the shared
cache directory. Only the baseline's own disposable generated-package cache
blobs were removed. No OSG source files were changed.
