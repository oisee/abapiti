# Trap batch F — registration audit and owned roots

Base: `origin/integ/yolo-4@7c2d33f`. Upstream is pinned at
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. No full base run was performed here:
five selected shards were executed with an archived base compiler and fresh
original Node oracles. [Exact missing-title multisets](evidence/traps-f-registration-base.json)
include duplicates and pending cases; no titles or statuses were synthesized.

## Step 0: 105 missing on this base, not the historical 115

| Root | Missing | Construction / actual blocker | Disposition |
| --- | ---: | --- | --- |
| `test/abap/structure_errors.ts:15` | 4 | `cases.forEach`, annotated structural callback row, dynamic `it.skip(c.abap)`; native row-shape narrowing | Dell, all four are Node pending |
| `test/abap/expressions/reuse.ts:10,70` | 57 | heterogeneous expression instances in a module table, `forEach`, title calls `r.getName()`; initializer nominal-union storage mismatch, then failed readiness | Dell |
| `test/abap/structures/_combi.ts:276` | 1 | nested describe with block-local `class Normal` | Dell, local constructor identity/capture ABI |
| `test/abap/syntax/syntax.ts:16428` | 41 | ordinary suite-local XML fixture builders and direct `it` registrations; lifted helpers incorrectly required a module receiver in a suite thunk | repaired; actual execution required below |
| `test/rules/line_length.ts:17` | 2 | helper registers a table after `new LineLengthConf().length = 400`; assignment correctly yields **400**, setter requires a config reference | Dell |

The live-class-view merge already recovered the ten historical `keyword_case`
registrations. They are not credited again to F. Fresh base selected-shard totals:
4,378 passed, FAIL 0, 497 trapped, 124 not-run (105 unregistered + 19 skipped).
The 41 STRUCTURE titles include two Node pending cases.

The reporter now retains the exact missing multiset as `unregisteredTitles`;
its tests cover duplicate missing/pending titles and unchanged accounting.
This is reporting only, never an input to Go registration.

## Registration and test fixture helpers

The interrupted WIP is retained, with stronger binding-based API recognition
and Node regressions. Receiver-free suite-local helpers use the existing static
method/capture-parameter lowering; no module-typed `this` is fabricated inside
a closure. The first fresh WIP execution registered all 41 titles, but 39 active
bodies then honestly trapped on captured `runFixtures` (two stayed skipped).
Registration was not counted as a body pass.

Named fixture helpers carrying immutable suite data now expand that data into
the test closure's existing capture fields. Initialized immutable suite arrows
can be direct static callees too (`nativeOf` is this pattern). Only original
Mocha `it` callbacks receive this allowance; checked symbols distinguish the
API and helper bindings. A whole-file non-call-reference screen rejects helper
writes, aliases and identity/escaping uses. Mutable future data bindings remain
refused. Calls to suite const arrows lexically preceding their initializer
refuse before argument preludes; no catchable ReferenceError is fabricated.
Receiver-dependent suites keep their existing refusal. The original array/object
references are retained, so fixture mutations are visible through helper calls.

The first completed owned syntax rerun then isolated a second owned root at
`syntax.ts:16492`: `fixtures.concat([program], extra ?? [])`. The old single-
argument array lowering emitted a nil operand for two arguments. The unpruned
Go path now evaluates receiver and **all** compatible-array arguments once in
JS order, saves their live references, and only then concatenates using existing
array operations. Zero arguments uses the existing fresh slice operation;
scalar/rest/incompatible-storage arguments refuse. A Node regression covers
argument mutations of the receiver and an earlier argument, receiver identity,
fresh result/empty concat, sparse copy/JSON, structural fixture rows and omitted
optional extra data. The registration-helper regression also runs this exact
multi-array pattern through a captured named helper after live fixture mutation.
No new array storage, type ABI or runtime operation was added.

This does **not** recover capture-free named parser/config helper declarations,
Downport's core closures, arbitrary callable objects or asynchronous scheduling.
Dell's `parse`, `parseView` and `getConfig` roots remain outside this certificate.
Regressions compare nested descriptions, XML-derived titles, aliases of the real
Mocha API, defaults, transitive helpers, direct immutable arrows and live fixture
reads against Node, and require boundary refusals explicitly. The WIP test's
initial failure was only JSON escaping (`\u003c` versus `<`); titles are now
compared after JSON decoding, not by serialized spelling.

## Items 2 and 3: precise Dell handoffs

### Nesting — sibling nominal assertion, not SevenBitAscii/config

All 31 reached refusals are reported at the test thunk's creation location
`nesting.ts:253:72`, not necessarily the failing expression's line.
The body creates `znesting.prog.abap`, then asserts `reg.getFirstObject() as Class`
before calling the shared ABAPObject method `getMainABAPFile()`. Fresh original
Node diagnosis for both `IF foo = bar. ENDIF.` and `WRITE foo.` reports
`constructor.name === "Program"`, `instanceof Class === false`, and one child.
Registry.parse/getFirstObject preserve that Program instance; an explicit
nominal Class view cannot be obtained from the sibling Program storage.
This is the checked assertion/class-method view ABI, **not** an erased rule
setter or live configuration conversion. No cast/verifier relaxation was made.
Dell needs a structural/common-method view preserving the Program identity and
its prototype, not a fabricated Class. Required oracle: Program and Class
receivers, unchanged identity/instanceof, common method dispatch, undefined
getFirstObject, and incompatible objects refusing. See the final certification
for the reached count; no 31-pass estimate is made.

### Pretty-printer — private Config graph / live own-property writes

The seven cases at `pretty_printer.ts:161` cast Config to any, then assign
`config.config.rules.keyword_case = test.config`. This goes through Config's
private native `IConfig` graph and an erased rules-object property, not a Chai
assertion or title generator. `assignTarget` cannot supply this Dynamic lvalue.
Fresh original Node confirms the stored config is the **same supplied object**.
Two other cases also assign `sequential_blank`; one deletes that own key. These
need live own-property storage/write/delete support across the native graph and
config views, including present-undefined versus absent properties. A snapshot,
static setter rewrite or invented nominal config instance would not implement it.
Dell owns this boundary; no Config, dynamicPut, class-view or own-key code changed.
The 11 other pretty-printer refusals at its constructor remain separate.

## Other remaining registration ABI details for Dell

- **reuse.ts:** base HIR rejects `optional<classref<FieldChain>> <-
  classref<FieldLength>` in construction of a row at line 17. Distinct matcher
  classes inhabit one `r` union; selecting the first nominal class cannot store
  the others. The later `test.ready.tests` verifier refusal is a consequence of
  the refused module initializer, not evidence of missing test discovery. A
  sound heterogeneous/common Expression view must preserve concrete getName /
  getRunnable dispatch and identity. Test all matcher classes, duplicate titles,
  initializer refusal and later independent suite recovery.
- **structure_errors.ts:** the table stores concrete If instances, while the
  callback parameter's row uses `structure: IStructure`. HIR rejects narrowing
  one native `shape.*` reference to the other; matching keys alone do not supply
  a live field/method view. All four cases are skipped on Node. Preserve that
  pending status; do not synthesize registrations from their titles.
- **_combi.ts:** block-local Normal extends AbstractStructure and is passed as
  a constructor to structureType. Hoisting it globally changes per-invocation
  class identity/captures. Local-class/descriptor support is not repaired here.
- **line_length.ts:** the setter receives a primitive number, not an object;
  JavaScript stores 400 as `this.conf`. Dell must model that erased field/use
  boundary honestly. Converting 400 into `{length:400}` changes the program.

## New top-20 owned repairs

Selection uses freshly executed WIP/selected-shard rankings, not the old train-3
counts. All removed roots are rerun; downstream traps never become estimated
passes.

- **Domain JSON array loops (24 selected reached cases):** `parsed.fixedValues
  ?? []` and `fixedValueIntervals ?? []` now construct a fresh existing any[] in
  an erased context. For-of consumes the existing checked any[] reference
  adapter, evaluates its receiver once, and observes live length/values. No
  iterator or typed-array ABI was added. Parsed arrays retain shared storage;
  nonarray and incompatible native typed-array payloads still refuse. Node
  tests cover missing/null/empty/nonempty data, once-only effects and mutation
  through shared native any[] aliases. ABAP emission uses existing operations.
- **Regex grammar:** non-u ASCII `\b`/`\B`, exact ECMAScript `\s`/`\S`, `\s`
  within classes, and capture-safe bounded repetitions up to RE2's 1000 limit.
  Unicode flags, lookarounds, backreferences, repeated captures, lazy repeats,
  malformed/oversized bounds and unsupported class languages still refuse.
  Dynamic constructors request runtime certification independently of unrelated
  literals. Erased string patterns/flags use the existing checked scalar unbox;
  objects and erased RegExp values do not acquire coercion/copy semantics.
  Node oracles cover XML header boundaries/captures, greediness, full whitespace
  membership (including nonmembers U+0085/U+180E/U+200B), UTF-16, bounds, flags,
  lastIndex and exactly-once erased argument evaluation.
- **Regexp string split:** the 9 CatchAndRaise + 9 CDSFieldOrder verifier roots
  passed a RegExp to string-only split. A usage-gated Go operation handles
  capture-free, nonempty delimiter matches with UTF-16 positions, fresh arrays,
  leading/trailing empty fields and unchanged separator lastIndex. Capturing or
  zero-width delimiters refuse; limits and exotic RegExp semantics are not
  fabricated. ABAP explicitly refuses this operation. Node checks both original
  delimiter languages and g/y/i flags, Unicode, empty input and boundary refusals.
- **Chai `instanceOf`:** a real alias delegating to the existing `instanceof`
  assertion, not a no-op. Actual pinned Chai checks inheritance, sibling/primitive
  negatives, null/undefined and negation; genuine failed assertions retain failed
  host records.

Unchanged type/ABI roots include ClearExportingParameters' parsed `skipNames`
array-to-native-string-array live bridge, IReferenceExtras own keys, Registry
assignment, constructor/interface and rename return bridges, NameValidator,
primitive-array find callback ABI, and ABAPObject's erased native record keys.
Downport fix/getParameters, captured parser/config declarations, async and panics
remain Dell-owned. Existing E/C write-ups supply their detailed oracle contracts.

### ABAPObject.findTexts — native string-key record versus Dynamic key

The selected 13-test `string <- dynamic` bucket is **not** unescape or parseInt.
A temporary untrapped lowering of the original byte-pinned core isolates the
first invalid operation: `map.get(this.texts, id)` at
`src/objects/_abap_object.ts:148:11`, with key source `:148:22`.
RuntimeSignature requires String; the `id` local still has Dynamic storage.
The same pair occurs in `map.set` at 149 and `map.get` at 151. `id` was produced
by `typeof t?.ID === "string" ? t.ID.toUpperCase() : undefined`, then checked
against undefined before these accesses. `this.texts` is native
`Record<string, Record<string, {entry:string, maxLength:number}>>` storage.
The scalar flow/storage bridge and record PropertyKey boundary need to agree;
this is not permission to pass arbitrary boxed values directly into a native
string-key map, nor to silently snapshot the XML graph. Required Dell oracles:
string/nonstring/missing ID, uppercase IDs, R without KEY, non-R missing KEY,
empty/optional keys and text/length values, and incompatible key domains
refusing rather than creating invented properties. No map-key/coercion ABI code
was changed. The raw probe and its source closure are deleted after diagnosis.

## Grace / certification status

Fresh base export already differs from its committed lexer golden by one runtime
catalogue record (`dynamic.classView`), with no lexer method/call facts changed.
F adds one `string.splitRegex` record (two read facts); the inspected final golden
changes only six catalogue counts. No lexer oracle was edited.
The base also omitted the already-implemented ABAP `dynamic.classView` operation
from its backend capability map. Only that catalogue entry is restored, after
checking the existing emitter/runtime; no class-view storage, conversion or
property code is changed. The old Go regexp refusal test is updated to reject
repeated captures/oversized bounds instead of the now-certified `a{2}` and `\s`.
The inherited qualified-alias checked-view test used two unrelated classes,
although the existing implementation requires declared nominal ancestry. Its
positive fixture now declares `Child extends Base`; a new negative fixture keeps
the original sibling assertion refused. No assertion/class-view code is changed.
The first full/owned certification attempts were stopped before completion when
the owned multi-array concat trap was found (not a genuine FAIL). Final execution
prioritizes rules and syntax via `ABAPITI_TESTS_FIRST` but still executes and
reconciles every shard/whole-suite title; the ordering helper has partition and
unchanged-total regression tests.

Final all-shard execution, default-output audit, workload and gates will be
recorded here after fresh final-source runs. No passing whole-suite claim is made
from the incomplete diagnostic fixture run: it built during an intermediate
source edit and refused emission for missing `execSpace`; its zero registrations
were an explicit infrastructure failure and are not final coverage evidence.
