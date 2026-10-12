# Trap batch J — small type/ABI roots

Base: `8bff252` (main `ab026e7`, train 7, plus I). Original upstream remains
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. No upstream tests, assertions,
registrations or exclusions are changed. Measurements and final gates are
recorded below after certification.

## `in`: concrete structural storage, not declared-field truthiness

The exact reached expression is **`"newText" in e` at renamer.ts:132**, inside
`applyEdit`, not another implementation of TextDocumentEdit.is/RenameFile.is.
The edit-element union is erased to `builtin.object`.

Erased object/union `in` now uses the existing **`object.has`** operation and
property-lookup emitter. It reuses original tagged-record keys or the existing
per-instance structural own-key metadata; live views unwrap to their original
storage before lookup. It does not enumerate declared fields or infer presence
from a field's value. An explicitly assigned undefined property is present; an
omitted optional property is not. Ordinary Object.prototype members are inherited,
not own. Receiver and key execute once in the original order.

Supported LSP factories other than RenameFile record their proven original key
set on their ordinary HIR New, using the existing OwnKeys ABI. These factories
produce required fields and, for Command, an arguments array only when supplied.
RenameFile's options are conditionally constructed (even a supplied empty options
object is omitted by Node); no fixed field-presence proof is fabricated there.
Native prototype-bearing objects, unknown constructor-presence metadata and
erased prototype-function reads remain explicit refusals. ABAP explicitly refuses
the erased lookup rather than treating object-root storage as a nominal class.
There is no second class/config view, reflection API, or property-presence ABI.

`TestErasedInPresenceBoundaryNodeOracle` checks omitted versus assigned undefined,
later assignment, inherited Object.prototype membership and a retained native
prototype refusal. `TestLSPErasedInNodeOracle` uses original pinned LSP 3.18.0
on Node, including TextEdit/TextDocumentEdit factory storage. The structural
interface oracle also tests `in` through a live view whose original object has
extra keys. The preexisting fixed nominal/table lookup domain is unchanged.

The focused rename shard still has **zero passes, FAIL 0**. Its 23 former `in`
refusals now reach **`renamer.ts:135: unresolved field range`**: the checker-narrowed
TextEdit/AnnotatedTextEdit/SnippetTextEdit element union still has object-root HIR
storage. Completing its common-field view requires the general heterogeneous
common-element/union view machinery owned by Dell. Under the stop rule, no union
storage widening, new matcher/row ABI, fake TextEdit instance or source override is
added. The `in` lowering is independently executed by the Node regressions, but
whole-body trapping means these 23 renamer bodies do not yet execute it; no pass
or runtime progress is credited to those bodies.

## Primitive find: extend the existing dense callback/result ABI

The unpinned primitive find path reuses fit-rules' `arrayCallbackLoop` and
Optional result storage. A found `""`, `0` (or false) remains distinct from no
match. Length is captured once; current values and membership are read at each
visit, the original array/index are passed, appended slots are not visited, and
short-circuiting preserves the matching value.

The certified extension is **checked dense primitive find**, not an implementation
of arbitrary sparse/undefined callback arguments. A missing slot refuses before
callback invocation; it is never skipped and never bound as a zero primitive.
Pinned dense proofs keep their original fast path. No callback signature,
callable representation, runtime operation or effect catalogue is added.

### Genuine prototype mismatch — stopped, reported and repaired first

The first sparse oracle exposed a real prototype bug: without an `array.has`
operation, the ordinary Go array storage did not enable its existing hole metadata.
An out-of-range assignment could fill a visited primitive slot with `""`; the
proposed find path failed to refuse it. Work stopped and the mismatch was reported
before shard follow-up. The new path now checks `array.has` at each visit, enabling
the existing presence-aware array ABI, and traps on a missing slot. Independent
Node evidence confirms that find visits that slot as undefined and continues to
`"z"`. The regression retains this as an explicit unsupported-domain case, not a
fabricated match. Node/Go dense probes certify no match, empty arrays, found zero
and empty string, effect counts, index/original-array arguments, replacement and
append behavior. The former blanket unpinned refusal is replaced by this bounded
acceptance oracle plus the stronger missing-slot negative; no upstream test is
weakened or deleted.

The first focused run reaches **`lastIndexOf` (nine tests)** after removing the
find ABI barrier. The follow-up adds a bounded primitive-array lastIndexOf with
one typed search argument and the default starting index. It snapshots length
after argument evaluation, searches backward, uses live Has/Get, skips holes and
uses strict primitive equality. Empty/missing searches return -1. Arbitrary
fromIndex/coercion and reference-element searches refuse rather than ignore extra
arguments. This uses existing HIR loops/array operations, not a new runtime op.
Its Node oracle checks duplicates of empty strings/zero, holes, empty/missing
searches, search-argument mutation of the receiver, the actual combined duplicate
find predicate, and an independently measured fromIndex refusal boundary.

## Property-only structural interface compatibility

IRenameParams is a structural subtype of ITextDocumentPositionParams, despite
both having nominal HIR data-class layouts. Coercion now proves an invariant
field-name/type subset between **property-only interface layouts** and reuses the
existing live data-class view. Fields are not copied: identity, extra keys,
mutations through both aliases and nested object references are retained.
Equality between these compatible layouts uses the existing boxed identity
operation. Integer-mode views exposed a latent missing `math` import in generated
checked integer field helpers. Only import-use tracking is added to the existing
helper emitter; its conversions, bounds and runtime refusal semantics are unchanged.
The same structural-interface Node oracle now runs in ordinary and integer modes. Native classes/configs, methods, inheritance on runtime classes,
optional-to-required fields and nested row/array covariance are not admitted.
The HIR verifier's nominal acceptance rules are not relaxed.

The Node regression initially rejected the different nominal operand types at
an equality expression; this was a verifier refusal, not an accepted value
mismatch. The final regression checks identity and alias mutation without deleting
that expression. No live configuration-row or configuration-array machinery is
changed.

## Flow presence and construction arguments

Optional-chain reads are no longer blanket-excluded from checker flow narrowing.
A chain is narrowed only when flow eliminated **declared** absence on the property
or receiver. A nonoptional annotation alone is insufficient: an erased undefined
still yields undefined from optional access. This handles
`if (lookup?.implementation) return [lookup?.implementation]` in Implementation,
while retaining the existing erased-undefined optional-access Node regression.
These extensions use the existing unpruned-feature mode; reachability-pruned default
lowering retains its prior output. An exploratory default diff exposed unnecessary
optional-chain temporary changes; the original pruned behavior was restored before
final certification, rather than accepting unrelated default changes.

SpaghettiScope's exact boundary is **line 280**, the shape constructor argument
in `ret.push({name: v, identifier: vars[v]})` in listDefinitions. It is not a
normal method-call argument. Required object-literal fields now use the same
checker-directed checked presence adapter as ordinary fixed call arguments.
Optional target fields retain absence; missing required values refuse rather
than becoming fabricated objects. The record/constructor Node oracle executes
this general pattern; no source-specific guard or override is introduced.

## Table: exact four sites and optional nominal assertions

The four Table assignment refusals are at:

- **950:** `tabl.parseType(reg) as UnknownType | undefined`, transparent key order;
- **965:** the same assertion, USER3/cloud;
- **1021:** `tabl.parseType(reg) as StructureType | undefined`, local descriptions;
- **1096:** the same StructureType assertion, DTEL/DOMA descriptions.

Table.parseType actually declares a **nonoptional AbstractType** result; asserting
it to an optional subclass previously left that original HIR type unchanged. The
adapter also covers genuinely optional base operands. A generally checked optional
nominal assertion now evaluates its operand once, preserves undefined when the
source admits it, wraps present base operands correctly, and checks only a present
downcast through the existing trap-only
`object.checkedClassView`. Unrelated assertions and unproven runtime payloads
remain refused. The Node regression checks present values, absent values and
single evaluation. This does not repair Domain/DataElement conversion or authorize
casting structurally unrelated rows.

## Reached follow-up boundaries and handoffs

- The renamer edit-element union's `range` view remains **23 whole-body traps**,
  as detailed above. It requires the shared heterogeneous/common-element view
  machinery, not another property-name table or nominal fake.
- The IRenameParams parameter mismatch and Implementation optional Location
  bucket disappear. LSP still reaches **three assignment mismatches at
  `rename.ts:109:52`**, the `TextDocumentEdit.create(doc, [...])` call. Its source
  `VersionedTextDocumentIdentifier` has required numeric `version`; the factory
  accepts the optional/nullable-version identifier layout. The invariant
  field-type subset adapter deliberately does not admit that mutable storage
  widening. No copy or nullable-version fabricated value is supplied.
- SpaghettiScope's constructor-argument mismatch disappears, including one
  additional syntax pass in the complete run. This does not claim that all callers
  of listDefinitions complete; unrelated reached boundaries remain explicit.
- All four located Table assignment-type refusals disappear. Two description
  bodies now refuse at existing **zero-argument `expect.fail` / subsequent
  `getComponents` lowering** (two distinct one-count compound diagnostics).
  Those are not credited as passes. No Chai termination/panic/harness widening is
  attempted. The total object-shard gain is seven, not a claim that each of the
  four initially trapped Table bodies passes.
- Recommendations **195**, Domain **19**, RulesRunner **18**, live Config and
  pretty-printer assignments, NamingRuleConfig.patternKind, recursive tuples and
  heterogeneous matcher registrations remain Dell-owned. No domain conversion,
  live-row/array mutation, async behavior or panic classification is changed.

## Certification

The frozen semantic commit is **0694974dc4c989d41124237e515db05806a2cc73**;
final amendment changes documentation/evidence only. Machine evidence retains
semantic subtree hashes so this can be checked after amendment.

- Development baseline: **3426/3728**, FAIL 0, 300 traps across the five selected
  shards. First repair: **3434/3728**, FAIL 0, 292 traps; LSP +8, other selected
  shards unchanged. The rules/objects follow-up: **3096/3293**, FAIL 0, 195 traps;
  rules +9 and objects +7 versus baseline. These are selected-shard measurements,
  not whole-suite estimates. Dirty-parent provenance is disclosed in the compact
  [focused record](evidence/traps-j-focused.json).
- **Exactly one final 17-shard / 625-file run**: **10,668/11,060 active passes
  (96.46%), FAIL 0, 332 traps, zero async-pending, 92 not-run**. Fresh original
  pinned Node: 11,060 passes, zero failures, 32 pending, 11,092 total. Whole-suite
  title multisets reconcile, no exclusions/unmatched executions/infrastructure
  errors/borrowed FAILs. See [all shard counts, missing multisets, initialization
  refusals and top 20](evidence/traps-j-final.json).
- Not-run remains **0 async/done, 28 skipped/focused, 64 unregistered**
  (**60 active + four Node pending**). No test or registration is synthesized.
- `go test ./... -short -timeout 40m`, Grace-enabled
  `go test ./tsfront/... ./hir/... ./cmd/... -timeout 40m`, `go vet ./...`, and
  runner/report regression tests all pass with the requested Go 1.26.0/cache/
  buildvcs/Grace environment. Shard runs use `nice -n 10`, two workers.
- Regenerated lexer facts compare byte-identically to the existing golden. There
  is no runtime catalogue change and no justified golden update.
- Default output versus locally archived **8bff252**, at the **same absolute
  output path**, has raw **`diff -r` exit 0 across 6,312 files**, including the zip.
  The exploratory temporary differences were removed before this comparison;
  nothing is normalized or accepted as an unrelated output change.
- Fresh emitted Go, fresh original pinned Node and the v0.1.1 release kit agree
  byte-for-byte on zabapgit **clean 0 / seeded 5**. The kit ZIP's verified SHA256
  and raw stdout hashes are retained in the [gate certificate](evidence/traps-j-gates.json).

Generated builds, closures, executables, baseline archive, kit, raw records,
facts and logs are deleted; only compact Markdown/JSON remains. No tests are
weakened/deleted, no source overrides added, no push or transpiler clone.
