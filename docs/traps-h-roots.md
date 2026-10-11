# Trap batch H — harness and method-callback audit

Base: `d45f3c174fcf5e143f9d9899de694d1c5c989d00` (F+G reconciled plus main,
train 6 candidate). Original upstream remains
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

## The 64 unregistered titles: root ownership, not pass estimates

All test roots remain selected. No registrations are synthesized from Node's
inventory, and no upstream test is modified, weakened, deleted or excluded.
The original F inventory has four roots:

| Missing titles | Source | Actual first boundary | H disposition |
| ---: | --- | --- | --- |
| 57 active | `test/abap/expressions/reuse.ts:10,70` | A heterogeneous matcher table tries to store FieldLength (then the other concrete expression classes) in the first nominal FieldChain slot. HIR rejects `optional<FieldChain> <- FieldLength`; the later `test.ready.tests` refusal is consequential. | Dell: heterogeneous/common Expression storage and concrete method dispatch, not a registration-loop rewrite. |
| 4 pending | `test/abap/structure_errors.ts:15` | Concrete If row storage cannot be narrowed to the callback's distinct structural `IStructure` row view. | Dell: live structural field/method views. Preserve all four original pending statuses. |
| 1 active | `test/abap/structures/_combi.ts:276` | Block-local `class Normal` needs a fresh per-invocation constructor identity and captures. | Dell: local class/constructor ABI. Hoisting would change the program. |
| 2 active | `test/rules/line_length.ts:17` | `new LineLengthConf().length = 400` returns **400**, which the nominal config setter cannot store. | Dell: primitive erased configuration storage. Do not replace it with `{length:400}`. |

These roots are type/ABI work under the task's ownership boundary, not a lost
file or missing harness API. Focused fresh executions audit the affected
`abap`, `abap/expressions`, `abap/structures` and `rules` shards; final evidence
retains the exact missing-title multiset (including its duplicate TypeTable title).

## Property-access callbacks

The reached `src/lsp/references.ts:40` shape is
`locs.map(LSPUtils.identiferToLocation)`: **a declared static method**, not an
instance method that should inherit its source object's receiver. The shared
array adapter now recognizes a checker-resolved class static method whose
implementation does not use `this`. It uses the existing physical callee ABI:
element, numeric index, original array, optional/default tails and rest packing.

Also supported: `obj.method.bind(obj)` and `this.method.bind(this)` with the same
simple class receiver, one bind argument and no partial arguments. The receiver
expression is evaluated before callback preparation; the bound receiver is saved
once outside the loop. A missing receiver refuses during method lookup even on
an empty array. Invocation preserves virtual dispatch and the original receiver,
not a snapshot object. No general callable storage or new runtime op is added.

Unbound instance methods are **not silently rebound**. They lose `this` in JS;
our receiver ABI explicitly refuses that shape. Receiver-using unbound static
methods, arbitrary callable properties/getters, computed references, different
bind receivers, partial bind arguments and arbitrary callback calls also refuse
with typed diagnostics. This is the bounded shape used by abaplint, not a claim
of general Function.prototype.bind or first-class method-value support.

`tsfront/method_callbacks_test.go` compares fresh Node and Go for static map and
forEach callbacks, original index/array arguments, empty arrays, bound receiver
reads and receiver-before-bind evaluation (a producer reassigns a captured
receiver). Node independently confirms an unbound instance callback throws a
TypeError; explicit lowering tests retain the unsupported-shape refusals.

## Mutable module bindings and hook ordering

The pinned `test/abap/types/form_definition.ts` has **no before hook**: line 8 is
`let reg: IRegistry = new Registry()`, and `runProgram` reassigns it. Its initializer
was rejected solely by the blanket mutable-module-variable restriction. The
five test bodies then encountered the existing readiness guard.

In opt-in trap/test lowering, test-module `let` declarations now use the same
original-statement initialization actions and live static storage as `const`.
An initializer still runs before suite registration; a hook still runs later in
the existing Mocha host. Reassignment does not copy or capture the old value.
`let x;` initializes to undefined only when the physical storage can represent
absence (reference, optional or Dynamic); primitive zero fabrication refuses.
Default/pruned module lowering keeps its previous mutable-binding refusal.

`tsfront/test_module_mutable_test.go` checks initial values before hooks,
before/beforeEach assignments, subsequent helper reassignment and separate
body executions against Node. A refused initializer keeps readiness false;
a later independent suite still registers and executes. The readiness/recovery
guard is not weakened, and no hook is executed prematurely to manufacture values.

## Fresh touched-shard top-20 follow-up

The six-shard aggregate ranking after the callback/module repair contains two
owned small lowering roots (see the retained focused audit):

- **RemoveDescriptions (3 reached cases):** `new XMLParser({parseTagValue:false,
  ignoreAttributes:true, trimValues:false}).parse(xml)` is now recognized by
  its resolved fast-xml-parser declaration and routed to the **existing reviewed
  XML subset**. The exact three literal options are required; mutable/default/
  extra options, nonstring coercion and escaping parser instances refuse.
  This is shared call lowering, not a source-specific replacement or a rewrite
  of the rule's catch/extraction/assertions. `xml_parser_call_test.go` checks
  nested/repeated nodes, ignored attributes, preserved whitespace, entities,
  CRLF and UTF-16 against original pinned fast-xml-parser 5.10.1 on Node, and
  retains explicit option-domain negatives. Runtime subset refusals cannot be
  swallowed as ordinary TypeScript parser errors. All three rule bodies then
  reach `reported[d.CMPNAME]`: native string-key storage still receives Dynamic.
  That map-key ABI remains Dell-owned; none of these three is credited as a pass.
- **UnusedDDIC fresh JSON literals (2 reached cases):** a fresh object passed
  directly to `JSON.stringify` was unnecessarily stored in a nominal shape
  with no original own-key-order proof. Only that fresh literal gets the
  existing Dynamic/tagged-record context, recursively preserving its original
  property construction order. The existing serializer is unchanged. No
  aliased native object, class instance, config graph, or typed row-array gets
  invented keys/order. `json_literal_test.go` compares nested AFF-shaped data,
  numeric-string key ordering, undefined omission, effects and alias mutation
  with Node. Its array-of-native-row case is retained as a typed order-domain
  negative, with Node's actual supported output independently checked; the
  existing native-instance order negative remains unchanged. Additional fresh
  literals below the initial cutoff share this fix. Execution advances to
  Domain/DataElement conversion refusals, not fabricated body passes.

A small per-LSP ranking additionally exposed **one ordinary string split**:
the external-call dispatcher treated `Hover.value.split` as an LSP namespace
factory solely because the field's declaration came from that npm package.
It now requires an actual merged namespace declaration. String fields use the
ordinary checked string implementation; constructors and unsupported .is calls
retain their previous behavior. `lsp_string_callback_test.go` certifies a
TextEdit string-field split against Node. No LSP storage/own-key code changes.

A final per-rules top-20 also has three low-count regex boundaries. Two exact
non-u naming patterns use a literal hyphen next to `\\d`:
`/^(\\/[A-Z_\\d]{3,8}\\/)?[A-Z_-\\d<> ]+$/i` and
`/^[A-Z_-\\d/<> ]+$/i`. Their reviewed translations explicitly include ASCII
letters/digits, underscore, hyphen and the original slash/angle/space grammar,
with namespace bounds and strict end anchors. Arbitrary legacy class ranges
remain refused. The empty **case-insensitive** constructor used by
ClassAttributeNames is now an exact reviewed empty-match language; its public
source/toString use Node's canonical `(?:)`. Generic empty grammar/unsupported
flags still retain their existing refusal tests. `regex_h_test.go` compares
literal and dynamic test/exec, optional captures, namespace bounds, character
membership, non-ASCII exclusions, final line terminators, empty subjects,
source/toString and nonstateful lastIndex against live Node. The existing whole
reviewed-pattern Node differential automatically includes the three additions.

The other aggregate top-20 items are native/optional conversion or structural
storage domains: recommendations; LSP extra own keys; ABAPObject Dynamic keys;
nesting; Domain; getParameters/importing; primitive find; rules_runner; rename
parameters; recursive SQL-set tuples; optional Location; pretty-printer;
DataElement; optional TypedIdentifier; class_data; Config; dynpros; and
UnusedVariablesConf.skipAbstract. They remain Dell-owned. NumberRange's
unlowered parseFloat also needs the numeric result domain (including NaN and
prefix parsing); it is not certified by replacing it with a finite whole-string
Go conversion.

The low-count Help diagnostic combines optional-native `delete parsed?.tree`
with pretty JSON of native graphs. Its first boundary is the delegated live
own-property/delete storage, not a proven tagged graph needing only indentation.
No native serialization proof or synthetic field enumeration is fabricated.

## Dell-owned boundaries (write-up only)

No repairs are made to `_abap_object`'s native string-key/Dynamic handoff or
missing HIR expression; nesting, Domain, Config, pretty-printer and rule-config
conversion domains; LSP original own-key/enumerable/structural storage;
TextDocumentEdit.is/RenameFile.is or renamer return/assignment views;
Downport getParameters/importing; captured parse/parseView; or general type/ABI.
The F/G root documents retain their concrete contracts and Node probes.

`identical_conditions` primitive find still needs absence-preserving callback
arguments/results, not a string implementation that converts undefined to an
empty string. NameValidator's `patternKind` is a structural configuration field,
not a regex parser failure. Recursive SQL-set-op tuples are a type declaration
boundary, not permission to invent an array element type.

## Certification

Frozen semantic source **c863bec736a0ee15a2a1e19e2b2beb49b60ab589** (final
amendment is documentation/evidence only). Exactly one final full 17-shard /
625-file run: **10,474/11,060 passed, FAIL 0, 526 typed/domain traps, zero
async-pending, 92 not-run = 64 unregistered + 28 skipped/focused + 0 async/done**.
Fresh original Node: 11,060 passed, zero failed, 32 pending. No exclusions,
infrastructure failures, unmatched titles or borrowed FAILs; complete title
multisets reconcile. The missing inventory remains 60 active + four pending.

The directly measured repaired bodies are 12 References, five FormDefinition,
one LSP string split and three regex naming cases. XMLParser and fresh JSON
advance to typed ABI refusals, with no estimated gains. Remaining Chai refusals
in the full top 20 are at `Assertion.throw` (three root tests, one objects test):
checked unboxing from stored any to the void-thunk ABI, not a failed assertion
or permission to skip calling the action. The full Table assignment and optional
TypedIdentifier buckets likewise remain general type/ABI handoffs for Dell.

All final short/full Grace-enabled Go tests use `-timeout 40m`; vet and reporter/
partition tests pass. The lexer facts export is byte-identical to the committed
golden: **no golden diffs**. Default output versus an archived d45f3c1 compiler
at the same absolute output path is byte-identical by **diff -r (6,312 files,
including zip)**. Fresh emitted Go, original pinned Node and the release kit
agree byte-for-byte on zabapgit **clean 0 / seeded 5**. See
[final all-shard evidence](evidence/traps-h-final.json),
[focused/root audit](evidence/traps-h-focused.json),
[gate and output certificate](evidence/traps-h-gates.json), and
[the current test table](abaplint-tests.md).

Final counts, newly reached top 20, exact missing titles, gates, default-output
comparison and clean/seeded workload comparisons are recorded with that final
measurement. Exploratory negative-fixture assertions initially expected blocking
rather than note-trap diagnostics, and one probe incorrectly imported a Go helper
from another generated file. These test harness errors were corrected and rerun.
An initially broader fresh-JSON probe reached a typed native-row order refusal,
not a Go/Node value mismatch: it remains a negative with Node's exact output.
No genuine assertion/value mismatch was encountered or accepted. Only compact Markdown/JSON is retained; generated compilers,
closures, binaries, copied kit, raw records, facts exports and logs are deleted.
No push or transpiler clone.
