# Trap batch G: checked regexp/string work and ABI handoff

Base: `350034c52a3544f5a2ca742abeb4d51f7e51270f` (yolo-4 plus live views 2).
Upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`. No upstream source edits,
transpiler checkout, or push. Final whole-suite accounting is in
[abaplint-tests.md](abaplint-tests.md) and `evidence/traps-g-final.json`.

## Owned fixes

* `catch_and_raise.ts:73`: a RegExp was handed to the string-separator runtime
  signature. `string.splitRegex` now carries a RegExp and numeric limit, with
  non-u JS sticky-clone splitting and capture splicing. Source enumeration
  found exactly one other regexp split, `cds_field_order.ts:128`, with
  `/[\s.,:(]+/`; it uses the same op, not a rule-specific rewrite.
* `catch_and_raise.ts:69` (`/\.$/` replacement), XML's dynamically constructed
  `/<abapGit\b[^>]*\bversion="([^"]+)"/`, and `/^v\d\.\d\.\d$/` certify
  through the shared translator. Dynamic Go constructors activate the
  translator even without an out-of-table literal. `\s`, `\S`, class `\s`,
  `\b` and `\B` retain JS whitespace/ASCII-boundary semantics.
* The exploratory rules/LSP/root top-20 exposed the table include-name grammar
  `/^((\/[A-Z_\d]{3,8}\/)|[a-zA-Z0-9]{3}|CI_)\w+$/` (19 rules tests) and
  `/^[a-zA-Z]{1,3}_.*$/i` (10 local-variable, six method-parameter, three
  class-attribute tests). Exact reviewed translations were added, **not** a
  general bounded-repeat or arbitrary RE2 fallback. LSP's reached `/$^/i`
  also now certifies: it matches only the empty input.

Live Node differentials: `tsfront/regex_split_test.go`,
`regex_split_edges_test.go`, and `regex_naming_test.go`. They cover empty input
and matches, captures (including empty captures), leading/trailing separators,
limits cutting off capture insertion, ToUint32 truncation/wraparound/nonfinite
values, lastIndex preservation, evaluation order, supplementary characters,
all JS whitespace, non-whitespace U+0085, ASCII/non-ASCII boundaries/case,
namespace bounds, final line terminators, dynamic construction, and typed
refusal outside the certified domain.

### Oracle mismatch found before certification

The first split prototype copied exec's missing-capture metadata into ordinary
`string[]`. Node's `','.split(/(,)(x)?/)[2]` is undefined, but the ordinary typed
indexed read returned the string zero value and its undefined comparison was
false. This was reported immediately; no shard assertion mismatch was accepted.
A broader optional-string indexed-read experiment exposed unrelated callable
return-ABI refusals and was **removed**, rather than modifying dell's ABI work.

Final split refuses **at capture insertion**, with a typed nullable
string-element ABI trap, if an actually unmatched capture would be appended.
This prevents slice, iteration, aliasing and string-only returns from silently
losing undefined. Limits that stop before that capture remain supported.
Older reviewed exec patterns with repeated captures are also refused for split
unless they certify in the capture-safe grammar. The negative Node-oracle tests
assert these explicit domain refusals. No claim of full JS split/species or
nullable string-array ABI support is made. Dell's future nullable-array work
needs oracles for indexed reads, own membership (undefined is not a hole),
iteration, slice/reverse/splice, boxing, callbacks, aliases and return escapes.

Final review also found that calling exec at each split cursor would reuse its
`.{start}` prefix wrapper, invalid in Go for start > 1000. Before certification,
split switched to equivalent full-subject leftmost-first enumeration in the
checked regular language, with JS's adjacent-empty suppression and final-boundary
rule. A live Node differential uses 1101-unit subjects with whitespace, empty
matches, captures and word boundaries. The shared exec implementation/ABI was
not changed.

## Dell-owned reached roots (unchanged)

Counts below are **final all-shard reached** counts, not estimates of additional
passing cases. Full rankings and exact diagnostics are recorded in the evidence.

| Count | Root | Boundary / required follow-up |
| ---: | --- | --- |
| 173 | `function_module_recommendations.ts:125` domain refusal (123 rules, 23 LSP, 10 root, 17 abap) | `runParsed` reads the nominal config's `Recommendations[]`, searches row `name`, optional `from`, and release values. The generic range/conversion diagnostic does not identify a numeric fault. Inspect the config/row view and absent `from` conversion; preserve default row identity and user-supplied config writes. Do not replace the recommendations with a hard-coded snapshot. |
| 37 | `_abap_object.ts:148`, string <- dynamic | `findTexts` accepts parsed XML `any`; row `ENTRY` feeds `unescape(t.ENTRY)` and `LENGTH` feeds parseInt. `typeof` checks of ID/KEY do not prove ENTRY is a string. Needs dynamic primitive conversion/narrowing contracts, not another regexp. |
| 33 | `lsp/_lookup.ts:238` own-key enumeration | `referenceHover` calls `Object.keys(ref.extra)` before JSON.stringify. IReference's structural extra metadata must retain original own enumerable fields, absent vs explicit undefined and live writes. Fabricating declared-field keys or suppressing JSON output is not a fix. |
| 50 | `object_naming.ts:126` RegExp pattern not string | `config[abapType]` at :120 is heterogeneous nominal class-field indexing. The resulting dynamic/optional view does not become a lowered pattern string; `regex` missing at :128 cascades. This is not the runtime translator refusing a pattern. Needs original config identity and checked string-field reads. |
| 27 + 16 | `downport.ts:730` captured `fix`; :3556 dynamic getParameters | Closure storage and erased method receiver/result ABI; left untouched. |
| 38 | `domain.ts:114/122` empty array context | `JSON.parse` returns a dynamic graph; `parsed.fixedValues ?? []` and `fixedValueIntervals ?? []` feed for-of. Needs contextual element/view typing of the fallback and live parsed-array reads; do not inject synthetic domain values. |
| 12 | `lsp/references.ts:40` property-access callback | `locs.map(LSPUtils.identiferToLocation)` is a namespace/static callable, not an inline arrow. Callable-value adapter and return-view ABI work, not a regex/string shim. |
| 9 | `identical_conditions.ts:23` primitive-array find | The predicate receives a string; find's result must distinguish undefined from `""`. Presence-preserving callback/result ABI remains refused. |
| 8 | captured `getConfig` (six registry, two severity) | Lowered captured function types/storage, not registration recovery or JSON text substitution. |
| 4 | `lsp/implementation.ts:22` Location <- optional Location | `if (lookup?.implementation) return [lookup?.implementation]`: repeated optional property reads need a proven narrowing/view of the actual value, without dropping absence or reading a stale snapshot. |
| 4 | `lsp/rename.ts:72` prepareRename parameter mismatch | IRenameParams passed to ITextDocumentPositionParams; structural/interface bridge identity and inherited field ABI. No nominal cast fabrication. |

Also left to dell: `data_definition.ts:221`'s `new Set(asNode ? ...map(...) : [])`
contextual empty string-array construction; DDIC Math/local types; captured
parse/parseView; rename implementation-result bridges; and LSP presence
metadata. Async, rule-config conversions and source-local synthesis were not
changed. JSON replacer/space support and arbitrary JS ToPrimitive remain
explicit refusals rather than debug-string approximations.

## Gate audit

The only Grace golden change is runtime-catalogue counts for the new op:
`op_aliases/allocates/may_raise/writes` 121 → 122 and `op_reads` 182 → 184.
Lexer method facts, 61 call sites and conservative count are unchanged.
ABAP explicitly lists `string.splitRegex` as unsupported; its legacy split ABI
and the default pruned ABAP output are unchanged.

Final measurement on `b0570c4`: **10,317/11,060 (93.28%), FAIL 0**, 644 trapped,
131 not-run (0 async/done, 26 skipped/focused, 105 unregistered). All 17 shards,
625 files, no exclusions/infrastructure failures/unmatched titles. Initial
exploratory runs and aborted gate retries are not counted in this measurement.
Grace-enabled short/full tests, vet and reporter tests pass. Raw default
`diff -r` against `350034c` finds all 6,312 files identical. Optional Go output
changes only its checked runtime and binary; the translator and two naming
reviews justify every difference. Zabapgit clean/seeded agree byte-for-byte
with fresh original Node and the kit (0/5 issues). Only compact evidence remains;
all generated/intermediate artifacts were removed. No push.
