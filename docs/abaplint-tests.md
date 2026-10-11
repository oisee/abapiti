# abaplint's own tests on TS-HG@Go

## Latest measurement — traps batch A (partial)

Pinned upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`
(@abaplint/core 2.120.56). Measured compiler/runtime: **`a1c19a7`**, based on
train-2 **`ce2f789`**. All four shards were freshly rebuilt and executed against
fresh original-Mocha Node oracles; selections include every `.ts` file in sorted
path order. These are actual executions, not estimates or smoke-subset totals.

| Shard | Files | Passed | FAIL | Trapped | Not-run | Registered | Node total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `test/rules/` | 192 | 1,415 | 0 | 1,722 | 22 | 3,147 | 3,159 |
| `test/abap/lexer/` | 3 | 200 | 0 | 0 | 0 | 200 | 200 |
| `test/abap/statements/` | 286 | 4,942 | 0 | 20 | 0 | 4,962 | 4,962 |
| `test/abap/structures/` | 30 | 138 | 0 | 0 | 2 | 139 | 140 |
| **Unique total** | **511** | **6,695** | **0** | **1,742** | **24** | **8,448** | **8,461** |

**6,695/8,461**, up **180** from train-2's 6,515. **FAIL stays zero.**
Unsupported lowering and runtime domains remain traps, never fabricated passes.
Passed + FAIL + trapped + not-run equals Node total. Every shard has zero
unmatched multiset titles and zero async-pending tests. Node has 8,450 passes,
11 pending, and zero failures. The denominator remains 8,461: the isolated
Downport oracle's 303 registrations did not change the complete-shard totals.

This is **partial batch A**, not completion of every assigned root. The
structural-config representation, explicit version conversions and lost
registrations remain unsupported. The 2,599 tests outside these four shards
were not measured. Compact execution evidence is
[`abaplint-tests-latest.json`](abaplint-tests-latest.json). Historical baselines
and integration details are in [`train-1.md`](train-1.md) and
[`train-2.md`](train-2.md).

## Implemented roots and exactness boundaries

### JSON helper and live parsed fields

The first assigned site is **`test/rules/downport.ts:13`**, not the rule's
source import at `src/rules/downport.ts:13`. `buildConfig` changes
`Config.getDefault().get().syntax.version`, serializes the whole graph, and
constructs a new Config. The generic frontend now recognizes the default-library
JSON binding, with exactly one argument. Shadowed bindings are not intrinsics.
String parsing uses the existing strict JSON adapter; input coercion, revivers,
replacers and indentation explicitly refuse.

The usage-gated Go writer preserves tagged-record insertion order (numeric
index keys first), array positions including holes, undefined-property omission,
UTF-16 escaping, primitive encoding and recursive data graphs. Unsupported root
undefined values, cycles, toJSON hooks and unproven native own-key order trap.
ABAP explicitly refuses `json.stringifySubset`; this is not an identity clone
or debug-string serializer.

A newly reached bug replayed a parsed shape's stale input graph after direct
native field writes. That lost the requested Downport release and caused issue
mismatches in an intermediate experiment. Serialization now retains source key
order but reads **live fields**, preserving unprojected properties. Multiple
new keys without write-order metadata refuse. Boxing a changed parsed native
projection also refuses rather than expose stale source values to a later
serializer. Node/Go and runtime regressions cover mutation, key order, holes,
Unicode escaping and native-order refusal. The final reruns have no mismatches.

The JSON/conf702 helper trap is gone. Downport now reaches later, independently
unsupported syntax/crypto/EditHelper/registry paths; this is not 296 new passes.

### Artifacts inventory and dynamic constructor arguments

The five-type factory remains the frozen deployment adapter. Unpruned lowering
executes the pinned source's **complete Objects namespace inventory**: ordered
export enumeration, one dummy construction per export, `getType`, last value
wins for duplicate type keys, lazy publication, then construction with the
requested name. Unknown ordinary keys construct UnknownObject. Inherited
Array/Object properties and unsupported argument domains explicitly refuse
rather than pretending they are absent inventory keys.

General descriptor-valued `new` now supports evaluated argument arrays,
constructor defaults, inherited constructors and ignored extra arguments.
Receiver and arguments are evaluated once in source order. Required storage
conversions are checked; unsupported rest/spread/storage domains trap. Go
factories are usage-gated; ABAP explicitly refuses `classvalue.newArgs`.
A Node/Go differential exercises inherited constructors, defaults and extras.
The pinned artifacts refusal is no longer a reached trap in the four shards.

### Optional references and EmptyStructure

Known undefined literals retain absence when entering a different optional
storage type. Optional boxed reference projections check presence before
unboxing, including map references, instead of forcing absence through a
present-only conversion. This removes ObsoleteStatement's text-edit-map HIR
verification barrier, **but 72 executions still reach runtime conversion
refusals**; that root is not claimed complete.

Reference equality against an optional accepted interface now converts the
present operand to that interface without narrowing the optional operand.
Undefined compares unequal rather than throwing, and same/different instance
identity is retained. This removes EmptyStructure's StatementNode versus
optional union-interface comparison barrier. Generated-Go regressions cover
absence, live map values and interface identity.

### DDIC and ABAP Doc regexps

A small general regexp grammar translates literal ASCII atoms, dot,
non-folded character classes, anchors and `?`, `*`, `+`. Dot is one UTF-16 unit
excluding LF, CR, U+2028 and U+2029; non-u ASCII case folding is expressed with
explicit ASCII alternatives, not Go Unicode case folding. The same source
implementation certifies literal patterns and handles runtime patterns.

Groups, alternation, backreferences, Unicode/class escapes, folded classes,
lookarounds, unsupported flags and empty-source canonicalization refuse.
Node/Go exec differentials include both DDIC exception-name patterns and all
three ABAP Doc patterns, match text/index, astral input and line terminators.
The DDIC/ABAP Doc regexp traps are gone. The new runtime is usage-gated; ABAP
retains its explicit reviewed-pattern refusal for these additional patterns.

## Remaining assigned roots and lost registrations

| Root | Final reached tests / lost registrations | Status |
| --- | ---: | --- |
| `src/rules/7bit_ascii.ts:38` | 121 | Plain JSON rule-config values cannot become nominal config references without a live identity/alias-aware structural view; still refuses |
| `src/rules/obsolete_statement.ts:157` | 72 | Verification barrier removed; checked runtime conversion still refuses |
| `src/config.ts:11` | 20 statements | Explicit release/language object conversion remains unsupported |
| `test/rules/keyword_case.ts:289`, `:304` | 10 lost | Spread plain objects cannot become nominal KeywordCaseConf references |
| `test/rules/line_length.ts:17` | 2 lost | Assignment expression correctly yields scalar 400; config storage cannot retain that scalar |
| structures `_combi.ts:276` | 1 lost | Block-local class needs fresh per-invocation constructor identity/capture ABI |

No nominal config object was manufactured from a plain object or scalar, no
local class was hoisted, and no assertion was suppressed. The 24 not-run tests
are exactly **11 skipped + 13 unregistered**. Registry `findIssues` and
EditHelper's forEach were left untouched, as requested.

## New top 10 reached traps

Counts are actual trapped test executions, excluding initialization failures;
normalization and representative full messages are retained in the JSON.

| Rank | Tests | Reached trap |
| ---: | ---: | --- |
| 1 | 986 | `src/registry.ts:370`: HIR assignment type mismatch (parallel workstream) |
| 2 | 194 | `src/rules/downport.ts:713`: ExpressionNode versus ExpressionNode-array runtime argument mismatch |
| 3 | 187 | `src/edit_helper.ts:220`, `:227`: foreach requires matching array (parallel workstream) |
| 4 | 121 | `src/rules/7bit_ascii.ts:38`: config reference conversion refusal |
| 5 | 72 | `src/rules/obsolete_statement.ts:157`: runtime conversion refusal |
| 6 | 30 | `src/xml_utils.ts:1`: runtime conversion refusal |
| 7 | 29 | `src/rules/downport.ts:3161`: unsupported crypto/hash values |
| 8 | 20 | `src/config.ts:11`: explicit release/language conversion refusal (statements) |
| 9 | 18 | `src/rules/line_only_punc.ts:46`, `:61`: regexp groups outside the supported grammar |
| 10 | 17 | `src/rules/avoid_use.ts:72`: runtime conversion refusal |

Registry/EditHelper counts grow because execution reaches them through removed
barriers, not because their implementations changed.

## Gates, reproduction and cleanup

All Go tests used `GOCACHE=$HOME/.cache/go-build`, `GOTOOLCHAIN=go1.26.0`,
`GOFLAGS=-buildvcs=false`, and `ABAPITI_GRACECHECK=1`.

| Gate | Result |
| --- | --- |
| `go test ./... -short` | PASS |
| Grace-enabled `go test ./tsfront/... ./hir/... ./cmd/...` (non-short) | PASS |
| `go vet ./...` | PASS |
| Node report/summary multiset contract | PASS |
| Node test-host runtime contract | PASS, eight scenarios; not TS-HG@Go |
| Default `abaplint --target all` against `origin/integ/train-2` **236da86** | PASS: empty `diff -r`, 6,339 byte-identical files |
| `abaplint --full` | Measurement only: 33 blocking (27 root + 6 cascade), 143 HIR errors in 39 files |

The branch base is ce2f789; origin/integ/train-2 advanced during this work.
Default output was also compared against the earlier reference executable, then
freshly compared with 236da86. Both comparisons were empty. No reference-source
changes were merged into this branch. Full closure numbers above describe this
branch, not the subsequently advanced integration reference; the supplied base
had 35 blocking / 151 HIR errors.

Grace golden updates change **only operation-catalogue counts** for the JSON
writer and constructor-argument operation. Lexer method/call-site facts remain
unchanged. Frozen deployment lowering retains its original serials and output;
new generic semantics apply to unpruned lowering. ABAP refuses unsupported new
operations explicitly.

Reproduction follows the train-2 recipe: materialize each sorted complete shard
with `tools/abaplint-tests-closure.mjs`; build a fresh original oracle with
`tools/abaplint-tests-oracle.mjs`; execute `abapiti abaplint-tests`; reconcile with
`tools/abaplint-tests-report.mjs`; aggregate with
`tools/abaplint-tests-summary.mjs`. Harness-only builds reuse the measurement-only
declaration index described in `docs/evidence/train-2-verifier-index.patch`:
all original bodies are checked in order, with no production verifier changes.
Code gates and default/full measurements use ordinary source-built executables.

Generated Go/ABAP, binaries, copied closures, oracle builds, raw results, debug
logs and baseline source extractions were deleted after execution/summary.
Only compact JSON/markdown evidence is retained. No push or transpiler clone
was performed.
