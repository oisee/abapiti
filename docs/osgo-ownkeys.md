# Own keys and primitive config views on TS-HA@OSGO

The implementation is `3cfe311`, based on `376ed1fe` from
`origin/feat/osgo-rules-init`, with `origin/main` (`08dd0e2`) merged to bring in
#136's collecting-fixture gate. The two inherited erased-element tests now use
`emitABAPNamed`; their collecting-helper regression passes.

## Original affected tests

**All 12 missing titles recovered; registrations 70/82 → 82/82;
initializer traps 3 → 0; FAIL 0 on both runs. Credited passes remain 0/82.**

| Original file | Node active | Registered before → after | Recovered | Pass before → after |
| --- | ---: | ---: | ---: | ---: |
| `line_length.ts` | 4 | 2 → 4 | 2 | 0 → 0 |
| `no_dynamic_stuff.ts` | 78 | 68 → 78 | 10 | 0 → 0 |
| **TOTAL** | **82** | **70 → 82** | **12** | **0 → 0** |

Body traps are **70 → 82**, unregistered/not-run titles **12 → 0**;
FAIL, unmatched, async-pending and skipped/focused counts are all **0**.
All three original escaping initializer actions now complete. The remaining
82 body traps are later failures; the all-file summary does not retain their
individual messages, so no unsupported-op attribution is inferred for them.
[Compact evidence](evidence/osgo-ownkeys.json) preserves the counters,
missing-title multiset, initializer actions, inventories and host verdicts.

These are whole original `test/rules/line_length.ts` and
`test/rules/no_dynamic_stuff.ts` files, selected with `ABAPITI_TESTS_FILES`.
The independent, freshly compiled Node oracle passes all 82 active titles.
Registration recovery is separate from test-body success. No synthetic probe
is credited as an upstream pass, and the full 3,149-title rules shard and the
whole 11,060-title suite were not measured here.

Both host-generated Go modules compiled and returned the original runner's
JSON. Host phases (frontend + Go emission + build + execution) total
**375.717 s before / 445.691 s after**; the memory controls and shared load make
this an uncontrolled performance comparison. ABAP envelopes are **2,320 →
2,322 files**, **255-byte** maximum line, **0 overlong** lines and **29-byte**
maximum object name. Both lowering gates have zero blocking diagnostics and
HIR errors. Both native runners exit 1 for honest traps; the reconciled file
orchestrator exits 0 with FAIL 0. This is registration parity, not a whole-suite
passing certificate.

## Emission and semantic boundary

`dynamic.keys` already had an ABAP catalogue entry, but its emitter always
raised `view.refusal`. It now enumerates ordered dynamic object entries and
certified native/structural references. Keys are strings; canonical decimal
array indices from 0 through 4,294,967,294 precede other keys, numerically sorted.
Other keys retain insertion order. Leading-zero spellings, the empty key,
Unicode keys and 4,294,967,295 remain ordinary string keys. The returned string
array is dense, including keys whose value is undefined.

Native layouts require public data-only ancestry and a constructor proof:
field writes must match declaration order, superclass initialization precedes
those writes, and initializer values cannot expose `this`. Inherited fields
are included. Unsupported subclasses override inherited adapters with explicit
refusals, so a base layout cannot yield a partial list for a derived instance.
Enumeration excludes methods, statics, prototypes and ABAP implementation slots.
The real NoDynamicStuffConf layout includes its two superclass fields and all
13 boolean flags. Computed reads/writes use the original reference and checked
field conversions; boolean mutation preserves the nonboolean values and aliases.

Structural records retain the constructor expression's per-instance `OwnKeys`
proof, rather than enumerating their complete physical layout. Literal values
are evaluated in source key order even if constructor parameters are arranged
in a different order. Typed writes and computed writes maintain presence and
insertion order; extras share that metadata. Writing undefined clears an
optional value while retaining its own key. Unknown presence/order metadata
keeps a refusal for enumeration and computed reads/writes.

Primitive-backed class views mirror Go's activated `PrimitiveReads` boundary
for strings, booleans and Number values, including inherited flags. They keep
the original primitive as their source and skip native constructor defaults.
Boxing the view recovers the original primitive. The assignment expression
`new LineLengthConf().length = 400` consequently remains Number 400, and its
missing properties remain missing. Optional readers preserve absence. Required
missing Number reads retain an explicit refusal because ABAP `f` cannot stand
in for undefined/NaN. Primitive writes, null, arrays and unapproved layouts
retain checked/refusing behavior.

A new literal-record probe exposed a shared frontend bug: tagged dynamic
values were folded as ordinary references when compared with a boolean.
`alias["2"] === true` returned false on Go and ABAP after storing true, while
Node returned true. The fix excludes tagged values from that object-only proof
and uses the existing `dynamic.strictEquals` operation. The unchanged strict
oracle now agrees on all three hosts; both operand orders, inequality,
undefined/object operands and operand effects are covered. No Go emitter was
changed, and fresh default Go output remains byte-identical to `origin/main`.

## Static closure accounting

The unchanged production `abaplint-tests --target osg --gap-only` counting tool
from [the suite gap measurement](osgo-suite-gap.md) was used with
`ABAPITI_INLINE=classic` on the same materialized 625-root closure:
**2,166 files / 7,052 classes / 85 interfaces**, zero blocking diagnostics,
HIR errors and frontend coverage traps.

The full production inventories are identical before and after:
**96 unsupported runtime-op sites** in 15 present operations, and **72 other
emitter refusals**: 66 dynamic/nonfinite Number parseInt sites, 4 i64 boxing
sites and 2 erased structural `object.has` sites. Nonfinite literal sites are
0 on both runs. The 17-operation unsupported catalogue is unchanged. These
figures supersede the older M0 partition for this branch; they do not claim
that the already catalogued runtime ABIs are universally executable.

The production inventory counts unregistered runtime operations before ABAP
passes and distinct emitter diagnostics after those passes. Already catalogued
operations that print runtime raises do not appear in either inventory. Thus
our items have **0 → 0 production inventory entries** despite the real runtime
initializer refusals. The supplementary read-only
[site counter](../tools/osgo-ownkeys-sites.go) uses the same pre-pass
expression-occurrence, owning-body, ID and ordinal convention; it does not
change the harness or claim runtime reachability.

| Item | Full pre-pass sites | Before | After |
| --- | ---: | --- | --- |
| `dynamic.keys` | 2 | unconditional runtime refusal at both sites | 0 unconditional refusals; checked storage/order boundary |
| `dynamic.classView` with `PrimitiveReads` ancestry | 17 | primitive backing selects the refusal branch | approved primitive backing retained; unrepresentable reads/writes still refuse |
| `dynamic.get` | 607 | shared operation with restricted native adapters | certified original-reference adapters and presence checks |
| `dynamic.put` | 170 | shared operation with restricted native adapters | checked mutation and presence/order maintenance |

The supplementary before/after counts agree exactly. The two key sites are `test/rules/no_dynamic_stuff.ts:107:33` and
`src/lsp/_lookup.ts:272:48`. The 17 view sites are potential primitive-backed
boundaries, not 17 observed primitive inputs. The get/put rows count operation
uses, not removed refusals. Runtime admission is conditional; no claim is made
that every input at every site is supported. Inventory phases and runtime test
counts are not added together.

Remaining refusals include unknown constructor/presence order, private or
behavior-bearing layouts, unsupported inherited layouts, unowned prototype
properties, unknown native writes, unsupported scalar boxing (including i64),
primitive mutation and missing Number reads requiring NaN. Parser arrays and
null are outside the primitive-view boundary. General `dynamic.arrayView`,
`array.has` and the remaining unsupported catalogue operations are unchanged.
No operation was removed from `unsupportedOps`; the owned operations were
already catalogued and their runtime ABI was incomplete.

## Executed regression probes and gates

Six authored ABAP Unit probes execute through pinned gogen/OSG, **6/6 SUCCESS**:

- Primitive source identity, optional absence, required Number/write refusals,
  and array/null view refusals.
- Parsed/view key order: zero, numeric keys, both uint32 boundary sides,
  leading-zero, empty and Unicode keys, for-in/entries, and live writes.
- Native inherited keys, computed boolean mutation, and preservation of the
  original exclusion array and severity.
- Native declaration-order, uninitialized-slot and unsupported-subclass
  refusals; structural absent/present-undefined keys, extras and prototype
  refusal checks.
- Literal constructor key metadata, source-order initializer effects, and
  typed/view/computed mutation through aliases of the original record.
- Tagged boolean equality, inequality, operand orders and evaluation effects.

All ABAP-emitting Go tests use collecting helpers. The probes compile and run
through the host, rather than relying only on abaplint syntax. New table reads
use exact row storage or `TRANSPORTING NO FIELDS`; there is no narrower-reference
`READ TABLE ... INTO` admission.

Passed `go vet -p 2 ./...`, `go test -short -p 2 ./...`,
`ABAPITI_GRACECHECK=1 go test -p 2 -count=1 -run TestLexerFactsReport ./tsfront/`,
`TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`, and
`./.github/ci/lint.sh gate origin/main` (**0 new**, canary **5/5**, lint 2.13.2).
No golden regeneration. Default `abaplint --target go` was generated from an
archive of exact `origin/main` and the final compiler at the same output path;
raw `diff -r` is empty: **9 files**, executable included.

## Reproduction

Use clean checkouts at upstream **`577f875ebec44cfaf64841cfe71c8ab8dc32622e`** and
OSG **`368baab28111a072c3f5779a0e07d259305f836f`**, with their pinned dependencies.
Supply checkout/cache locations through the caller's environment:

```sh
export ABAPITI_OSG_HOME="$OSG_HOME" TRANSPILER="$TRANSPILER_HOME"
export GOTOOLCHAIN=go1.26.0 GOMAXPROCS=4
export GOFLAGS='-p=2 -buildvcs=false -gcflags=-dwarf=false -ldflags=-w'
export NODE_OPTIONS=--max-old-space-size=20000
export ABAPITI_TESTS_TARGET=osg ABAPITI_TESTS_WORKERS=1
export ABAPITI_TESTS_FILES=test/rules/line_length.ts,test/rules/no_dynamic_stuff.ts
node tools/abaplint-tests-all.mjs "$ABAPLINT_HOME" focused.json

GOMEMLIMIT=18GiB GOGC=50 ABAPITI_INLINE=classic \
  abapiti abaplint-tests "$CLOSURE" --target osg --gap-only -o "$GAP_OUT"
go run tools/osgo-ownkeys-sites.go "$CLOSURE" > supported-sites.json
```

Run large host builds and full static audits sequentially. An unrestricted
baseline audit was stopped under memory pressure and the concurrent candidate
census was discarded; the bounded baseline completed successfully. Those
stopped attempts supply no result counts. Setup attempts that failed pin/library
checks likewise supply no registration or pass evidence. The actual before/after
runs use the same upstream/host pins and file selection. No runner, oracle,
upstream assertion, host source, exemption or harness was modified. Commits are
local; nothing was pushed.
