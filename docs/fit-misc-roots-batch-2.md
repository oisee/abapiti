# Small lowering roots, batch 2

Measured against embedded abaplint `577f875e` (core 2.120.56), with
`GOCACHE=$HOME/.cache/go-build GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false`.

## Implemented

| Root / site | Model |
| --- | --- |
| `Conditions.findFirstDuplicate`, identical_conditions.ts:23 | Pin the **whole Conditions class**, including all writes to its private dense string array. Existing dense callback loop keeps the find result optional: a found empty string is not absence. General primitive find with removed/absent slots remains refused. |
| contains_tab.ts:51; catch.ts:94 | Fingerprinted density/stability proofs. Split creates a private local dense array; nested lineage arrays are built by push and predicates never expose their identities. Reuse the existing captured-length dense callback HIR. |
| renamer_helper.ts:39 and other early-return comparators | Callback-local result/done temporaries; blocks and if branches preserve an independent return boundary. Subsequent statements are guarded; declaration scope and conditional expression preludes remain local. Return inside loop/try/switch is explicitly refused, not turned into an enclosing return/break. |
| rename_icf_service.ts:22 | Immediately invoked parameterless lexical arrow, no escaping closure. Same return boundary as callbacks. Its separate string-match roots remain another branch's responsibility. |
| downport.ts:82 | Lift top-level-in-function nested declarations lazily, capturing references/current lexical values at the call. Declaration can follow first call. Refuse recursion, block-scoped/shadowed bindings, dynamic `this`, and suspension. |
| function_group.ts:158 and other immediate XML calls | Resolve fast-xml-parser's imported constructor (including aliases); recognize exactly parseTagValue:false, ignoreAttributes:true, trimValues:false and one string parse argument. Emit existing `xml.parseSubset` HIR/runtime in Go and ABAP. Other options/validation/binary inputs remain diagnostic; XML outside the reviewed subset traps. This is the same subset as AbstractObject.parseRaw2, not a complete external library implementation. |
| Follow-on identical_conditions root | One-argument array lastIndexOf: search a reversed **copy**, preserve receiver identity/order and return -1 for absence. fromIndex remains diagnostic. |
| dump_scope.ts:80 | Standard string-array ToString via comma join, including empty arrays/empty strings; not JSON serialization. Arrays with user-modified prototypes/own coercion hooks are outside the lowered array model. |

These mechanisms use existing verified HIR operations and both emitters rather
than Go-only rewrites. Tests execute generated Go and check ABAP emission;
source-pin tests read the embedded archive and reject stale fingerprints.

## Explicitly refused / investigated

* **ddl_parser.ts:97 constructor name on erased object root:** requires runtime
  constructor identity/name metadata, including dynamically retrieved objects.
  A static declared type is not the runtime constructor.
* **number_range.ts:51 parseFloat:** requires JS longest-prefix binary64 parsing,
  NaN, Infinity, signed zero and ToString of its input. Integer parseInt is not
  equivalent. The global intrinsic now has that precise diagnostic.
* **object_naming.ts:120 indexed nominal config:** needs heterogeneous
  own/prototype lookup, including method/inherited/unknown keys. A finite field
  switch returning undefined for every other string would be unsound.
* **semantic.ts:50 for-in over external SemanticTokenTypes shape:** requires
  enumerable own/inherited property metadata and order, not map-entry iteration.
  No change to the other branch's vscode constructor work.
* **help.ts:46 delete parsed?.tree:** fixed nominal fields lack configurable
  property presence/omission metadata. Clearing a field is not deletion,
  especially for the following JSON.stringify. Ordinary map-backed records
  retain their existing delete model.
* **dump_scope.ts:31,32,86 object coercion:** valueOf/toString dispatch is not
  modelled for heterogeneous class/interface values. Do not substitute toText,
  JSON, or a guessed `[object Object]` without proving hook absence. These
  accesses include dictionary `.length` lookups, whose values can be objects.

### Synchronous drain proposal (not implemented)

`Registry.parseAsync` **is needed**: registry, skip_logic, LSP references and
stats tests explicitly await it. `IProgress.tick` returns Promise<void>; replacing
parseAsync with parse drops progress/performance behavior and error timing.
A future bounded model should introduce an explicit settled Promise<T> HIR
(success/rejection), a deterministic synchronous drain at await/test-harness
boundaries, and preserve rejection propagation and callback evaluation order.
Only certified already-settled progress providers may participate. Pending
promises, arbitrary thenables, and observable microtask interleaving must remain
refused until a scheduler is modelled. This is not a small lowering-only change.

The reported rename_helper.ts:281 site is actually
**objects/rename/renamer_helper.ts:281**, a local holding getObjectsByType("SICF").
Existing registry generator adapters already return ordered snapshots for
certified consumers, but the inferred checker local still has Generator type.
A small future adapter can pin this entire consumer and carry the adapted array
result into the local, after proving complete drain without membership mutation
and preserving object identities. Do **not** map all Generator<T> to Array<T>:
JS starts enumeration lazily, and next/return/throw and mutation between yields
need a real iterator state machine. The current diagnostic states this boundary.

## Gates and measurement

* Uncovered closure: **644 -> 629 blocking diagnostics**; accompanying HIR
  errors **392 -> 382**. This does not claim all core tests pass yet.
* This branch has no --full switch. Measurement used the non-committed change
  `LowerRegistry(work, files, registry, nil)` in cmd/abapiti/abaplint.go, then
  `abaplint --target a4h --assume-int=false --evidence`. The hack was restored.
  Binary64 was used for both measurements because the baseline uncovered
  assume-int pass panics at assume_integer.go:298 on malformed blocked HIR.
* `go test ./tsfront/... ./hir/... ./cmd/...` passes, plus per-feature tests.
* Default a4h output versus origin/main (`b04e07f`): **diff -r empty**, including
  the deterministic zip. Default validated workload remains unchanged.
* No SAP runtime execution was available; ABAP coverage here is verified HIR
  plus emitter tests, not a claim of on-system ABAP Unit execution.
