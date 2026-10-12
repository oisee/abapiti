# Trap batch L — JSON5 and the active denominator

Base: `d6c4f4a`. Frozen semantic source: `6410553`. Original upstream:
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. No upstream source, test,
assertion, selection or registration is modified. No new site override,
transpiler clone or push.

## JSON5: package semantics, not strict-JSON preprocessing

The only original package call is `src/config.ts:99`, with its compatibility
shim at 95–98. The frontend recognizes the resolved `json5` package `parse`
binding, including renamed/named imports; a local function with the same name
is not an intrinsic. Exactly one string argument is supported. Revivers,
coercion, spread and stringify remain explicit lowering refusals.

`json5.parse` returns the existing tagged graph, retaining own-key order,
duplicate-key replacement, arrays, null and UTF-16 strings. The parser supports
comments, JSON5 whitespace, trailing commas, unquoted/escaped identifier keys,
single/double quotes, hex numbers, signed/leading-point/trailing-point decimal
numbers, exponents, string hex/Unicode escapes, escaped line continuations
(including CRLF and U+2028/U+2029), and raw controls allowed by JSON5. Quoted
lone surrogates survive. Unicode identifier ranges come from **json5 2.2.3's
actual tables**, not the host Go Unicode version. The small reproducible table
builder is `tools/json5-unicode-generate.mjs`; its data attribution refers to
the existing package license.

The original strict operation and ABAP adapter are **not widened**. Full-source
closures keep only the existing compatibility-shim statement pattern; the
legacy strict expression replacement is no longer selected there. The original
Config assignment, defaults and version checks use ordinary lowering and the
existing graph projection. Reachability-pruned workloads retain the original
fingerprint, expression adapter and bytes. This is not a config-shape ABI fix.

`hir/golang/json5_test.go` differentially executes the real installed package
from `node_modules`, checks its 2.2.3 pin, and compares finite output bytes and
invalid-input **SyntaxError messages/UTF-16 line-column diagnostics**. Cases
include the actual abaplint config language, malformed comments, containers,
numbers, literals, escapes and identifiers. The Unicode oracle independently
enumerates all 1,114,112 code points against the package's actual regex tables.
Signed-zero checks use Node's `Object.is`, not JSON.stringify's zero collapse.
`tsfront/json5_call_test.go` executes namespace and renamed package calls,
checks unsupported signatures, and proves a catch cannot swallow an ABI trap.
The operation has a verified signature, conservative effects and explicit ABAP
refusal. Strict-JSON regressions remain unchanged.

### Exact refusal boundaries handed to Dell

- **NaN / ±Infinity / overflow:** lex and validate the complete document first,
  then raise a typed `JSON5 nonfinite Number representation is unsupported`
  refusal before any graph escapes. Private placeholders cannot be observed by
  callers. Thus `Infinity!` still has the package's invalid-character diagnostic,
  not a premature nonfinite refusal. The real Node oracle checks ±NaN,
  ±Infinity, overflow and nested nonfinite leaves. No zero, null, omitted field
  or fabricated finite value is returned. Number representation belongs to Dell.
- **SyntaxError payload:** the parser computes the package diagnostic exactly,
  but emitted TS-HG@Go raises a typed `JSON5 SyntaxError payload ABI is
  unsupported: <diagnostic>` refusal. It does **not** pretend that an Error or
  host panic is the package's SyntaxError with `lineNumber`/`columnNumber` own
  fields. Differential error-message certification is not a claim that the
  native exception object ABI is implemented. Domain's separate **JSON.parse**
  SyntaxError boundary remains unchanged and is still Dell-owned.

Focused original-shard execution recovers exactly one body:
`Config should support JSON5 (comments, trailing comma)`. Root is **89 passed,
FAIL 0, five traps**, versus K's 88/0/6. Rules, abap and structures retain their
counts. These four development shards are not counted as a full base/final run.

## All seven unregistered titles: concrete reached roots

All files are selected and materialized. These are three initialization
refusals, not discovery omissions, async/done callbacks or missing registrations
that can safely be synthesized. Exact title multisets and located messages
remain in the final JSON.

| Missing title/row | Node status | Root and ownership |
| --- | --- | --- |
| line_length: 269-character `hello … hello`, expected one issue | active | `test/rules/line_length.ts:17` passes **the numeric assignment result 400** to `testRule`, then `src/rules/line_length.ts:35` stores it in nominal `LineLengthConf` storage. Dell: primitive/native config-field storage ABI. |
| line_length: `WRITE: / 'abc'.`, expected zero issues | active | Same module initialization action and primitive config storage. It is a second occurrence of an existing active title, not a deduplication error. |
| `structure combi, complex1 complex1 match` | active | `_combi.ts:277–282`: block-local `Normal implements IStructure` passed as a constructor to `sub`. General lowering requires fresh **per-evaluation lexical constructor/class-value identity** and a captured factory frame. Dell: scoped class-value/capture ABI; no global shared-class hoist. |
| `Structure, test error messages, specific IF foo = bar.` | pending (`it.skip`) | `structure_errors.ts:15–17`: matcher-factory row to explicitly annotated structural callback row; verifier refuses incompatible native shape narrowing. Dell: original erased class/shape view and invariant field ABI. |
| `Structure, test error messages, specific IF foo = bar. moo = boo.` | pending (`it.skip`) | Same callback/shape boundary. |
| `Structure, test error messages, specific IF foo = bar. ENDWHILE.` | pending (`it.skip`) | Same callback/shape boundary. |
| `Structure, test error messages, specific IF foo = bar. ENDWHILE. ENDIF.` | pending (`it.skip`) | Same callback/shape boundary. |

The first normal LineLength registration already succeeds. Assignment-expression
lowering already returns its RHS, **400**, as JavaScript requires; changing that
to the newly allocated config object would silently miscompile the test.
StructureErrors loses even pending registrations before reaching `it.skip`;
the callback body's assertions are commented in original Node, not removed by
this harness. Bypassing its initializer or harvesting oracle titles would hide
the reached ABI failure. All seven therefore remain explicit handoffs, with
**three active + four pending**, and none is credited as a recovered pass.

## Denominator audit: each of the 28 skipped/focused registrations

Original mocha independently reports **11,060 active passes, FAIL 0, 32 pending**.
Each row below is an **executed Node oracle pending title** and an original
**`it.skip`** source call. There are **no xit, describe.skip, body-less or
focused-only exclusions** among these 28. All are outside the 11,060 denominator.
The two helper `it.only` branches in `abap/_utils.ts` and `rules/_utils.ts` are
not activated by this pinned suite. **Zero Node-active tests are skipped by Go.**
The four additional Node-pending StructureErrors titles are unregistered above.

Paths are under `packages/core/test/`. The final JSON's
`skippedOrFocusedTests` retains exact full titles and original Node classification
per shard; the table keeps suite-local titles readable.

| # | Source call | Suite-local title | Node / denominator |
| ---: | --- | --- | --- |
| 1 | `abap/structures/form.ts:20` | FORM, many statements | pending / outside |
| 2 | `abap/syntax/syntax.ts:899` | function module definition not found | pending / outside |
| 3 | `abap/syntax/syntax.ts:4161` | error cannot define length and decimals for date | pending / outside |
| 4 | `abap/syntax/syntax.ts:6491` | Error, overlapping names | pending / outside |
| 5 | `abap/syntax/syntax.ts:6550` | access instance variable from interface in static method, error | pending / outside |
| 6 | `abap/syntax/syntax.ts:7737` | Error, insert, types incompatible | pending / outside |
| 7 | `abap/syntax/syntax.ts:8241` | accessing me is not possible in static methods | pending / outside |
| 8 | `abap/syntax/syntax.ts:11290` | fields inside TYPES cannot be generic | pending / outside |
| 9 | `abap/syntax/syntax.ts:12923` | return type doesnt match importing | pending / outside |
| 10 | `abap/syntax/syntax.ts:16946` | gap, FORM parameters typed against program-local names are never checked | pending / outside |
| 11 | `abap/syntax/syntax.ts:16961` | gap, TABLES accepts sorted and hashed tables | pending / outside |
| 12 | `cds/cds_parser.ts:1159` | define custom entity with association | pending / outside |
| 13 | `lsp/hover.ts:133` | keyworld should not resolve | pending / outside |
| 14 | `lsp/hover.ts:140` | hover method parameter | pending / outside |
| 15 | `lsp/hover.ts:1714` | Hover inline SELECT data voided | pending / outside |
| 16 | `lsp/references.ts:58` | method references | pending / outside |
| 17 | `lsp/references.ts:118` | find references for macro definition | pending / outside |
| 18 | `objects/proxy_object.ts:9` | adhoc | pending / outside |
| 19 | `rules/downport.ts:3862` | REDUCE FOR GROUPS | pending / outside |
| 20 | `rules/downport.ts:4204` | FOR loops, identical variable names, step 1 | pending / outside |
| 21 | `rules/downport.ts:4684` | SELECT AS | pending / outside |
| 22 | `rules/downport.ts:4696` | SELECT UNION | pending / outside |
| 23 | `rules/downport.ts:4875` | basic value FOR | pending / outside |
| 24 | `rules/prefer_inline.ts:281` | Dont inline, type P, this will change the type? | pending / outside |
| 25 | `rules/prefer_inline.ts:291` | Types should not change when inlining | pending / outside |
| 26 | `rules/uncaught_exception.ts:210` | no error, super classed, local exceptions | pending / outside |
| 27 | `rules/uncaught_exception.ts:321` | PROG with INCL, expect error | pending / outside |
| 28 | `rules/unknown_types.ts:1537` | No class/method implementation but DEFAULT not found | pending / outside |

The report now reconciles Node-active and Node-pending **multisets separately**,
retains the skipped title audit, and explicitly reports active non-passes by
trap/unregistered/skipped/FAIL/async-pending. Ambiguous active/pending duplicate
collisions refuse rather than guess. A Go execution of a Node-pending body is
an accounting error, not a pass. Runner regressions cover active skip detection,
missing active/pending titles, duplicate multisets, and forbidden pending-body
execution. No original failed record is reclassified.

## Remaining reached traps and final certificate

The single final **17-shard / 625-file** run measured semantic commit
`6410553505feaa175ac2b2050ac4e7d69057d512`: **10,949/11,060 active passes
(99.00%), FAIL 0, 108 traps in 33 groups**. Not-run is **35 = 28 skipped/focused
+ 7 unregistered**, zero async/done and zero async-pending. **111 active tests
are not passing = 108 traps + 3 unregistered + 0 skipped**. Original Node has
32 pending: 28 skipped registrations plus four unregistered titles, all outside
the active denominator. Every owned/whole-suite title multiset reconciles;
there are no exclusions, wiring failures, unmatched executions or borrowed FAILs.

[Final JSON](evidence/traps-l-final.json) retains **allTraps**, with exact
compound diagnostics, located examples, counts and shard contributions; its
33 groups sum to 108 reached bodies. Initializer refusals remain separate.
All existing Dell ownership from [batch K](traps-k-roots.md) remains; no
absence/union, enumerable-key, config-shape or renamer/LSP range ABI is relaxed.
No additional non-ABI group is credited as repaired.

| Count | Complete remaining reached-root list |
| ---: | --- |
| 29 | Renamer unresolved common edit-union `range` |
| 19 | Domain native/optional conversion |
| 7 | PrettyPrinter live `keyword_case` assignment |
| 7 | NameValidator `patternKind` config shape + dependent local |
| 3 | ClassData absence/statement-structure union conversion |
| 3 | LSP Rename versioned/optional identifier assignment |
| 2 | Chai native ordered-object enumerable keys |
| 2 | ABAPObject MAXLENGTH nonfinite parseInt adapter |
| 2 | Dynpros erased/absent XML row conversion |
| 2 | MessageClass optional/structured XML conversion |
| 2 | RenamerHelper erased sibling/common-method conversion |
| 2 | View erased/optional XML row conversion |
| 2 | SpaceBeforeDot live partial-config conversion |
| 2 | LSP ApplyEdit unresolved common edit-union `range` |
| 2 | Test/Domain erased sibling nominal conversion |
| 2 | Config live `applyUnspecifiedRules` assignment |
| 2 | PrettyPrinter `keyword_case` + `sequential_blank` assignments |
| 2 | CDS erased-node `get` / `getChildren` + dependent local |
| 2 | UnusedVariables inherited `skipAbstract` shape + dependent local |
| 1 | Help constructor/class-descriptor reflection conversion |
| 1 | Semantic class descriptor incorrectly narrowed to structural instance |
| 1 | Domain strict **JSON.parse** SyntaxError payload boundary |
| 1 | EnhancementSpot erased/absent XML-field conversion |
| 1 | NumberRange parseFloat NaN/nonfinite result |
| 1 | ProxyObject live erased row/array assignment |
| 1 | LineOnlyPunc live partial-config conversion |
| 1 | Types helper Program/Class common-method live view |
| 1 | CDS erased-node `getFirstToken` + dependent local |
| 1 | DumpScope class ToPrimitive (two coercion sites) |
| 1 | Help native own-property deletion |
| 1 | PrettyPrinter native config deletion |
| 1 | Registry native structural deletion |
| 1 | UnnecessaryPragma inherited `allowNoTextGlobal` shape + dependent local |

### Gates, mismatch handling and cleanup

The [gate certificate](evidence/traps-l-gates.json) records the short suite,
full Grace-enabled tsfront/hir/cmd suite, vet, runner regressions, real JSON5
oracles and reproducible Unicode tables. Every gate passes. The only golden
change is one operation-catalogue record: counts **132→133**, reads **200→201**,
conservative count still 21; lexer behavior and all 61 Grace call sites remain
unchanged. Regenerated facts equal the golden byte-for-byte.

Default output versus locally archived **d6c4f4a** has raw **`diff -r` exit 0,
zero bytes across 6,312 files**, including the zip, at the same absolute output
path. No default change needs justification. Fresh Go, fresh original pinned
Node and the SHA256-pinned v0.1.1 kit agree on zabapgit **clean 0 / seeded 5**;
direct raw `cmp` and the independent kit checker both pass.

No genuine Go/Node value or assertion mismatch was encountered or accepted.
A prototype runtime compile error referenced `fmt` without its runtime import;
the dependency was removed and all real oracles rerun before certification.
The initial expected operation-catalogue golden difference was inspected and
justified. An early scratch comparison before the candidate binary existed
returned diff status 2 and was discarded; the certified comparison returns 0.
These setup errors are not semantic mismatches or accepted certificates.

Documentation/evidence commits preserve the four measured semantic subtree
hashes. Generated executables, closures, archived source/output, kit copies,
profiles, facts and logs are deleted; only compact JSON/Markdown, implementation
and regression source remain. No upstream tests weakened/deleted, synthetic
registrations, new site override, transpiler clone or push.
