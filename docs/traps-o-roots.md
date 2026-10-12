# Trap batch O — final abapiti-side roots

Base: `f78b973` (train 11). Original upstream remains
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. Only this worktree is edited;
no upstream source/test changes, site overrides, exclusions, push or transpiler
clone. Batch N's dynamic CDS methods, ToPrimitive and registrations, and Dell's
absence/deletion/typeof repairs are not included.

## Structural primitive compatibility: rename.ts:98 and :109

The factory receives an existing `VersionedTextDocumentIdentifier` record whose
`version` is numeric. Its destination `OptionalVersionedTextDocumentIdentifier`
uses the existing tagged primitive ABI for `number | null`, not an unrelated
nominal class or an optional-number field. Batch J's property-only interface
compatibility required exactly equal field types, so the factory's temporary
failed HIR verification before execution.

The general structural proof now admits primitive fields flowing to their
existing tagged or optional primitive slots. It still excludes native classes,
methods, hierarchy, container covariance and missing required properties. The
existing live data-class adapter preserves original backing storage, extra keys,
identity and mutation; no copy or fake version is made. The Node regression
checks identity, mutation in both directions and serialization of the same row.

**Dell handoff:** writing null through the wider alias into required numeric
backing storage remains an explicit conversion-domain refusal. Node retains
null there. This is an absence/tagged-storage ABI, not permission to fabricate
zero or silently widen container storage. The regression preserves that negative
boundary. The three original rename tests only use present numeric versions.

## semantic.ts:50 — enum value versus anonymous instance shape

`for (const t in LServer.SemanticTokenTypes)` uses the lowered namespace value
ABI. Checker flow names an anonymous property shape at that expression; it is
not a nominal-instance downcast. Unpruned narrowing retains an already lowered
class/enum value rather than producing invalid HIR ClassValue-to-ClassRef Narrow.
A Node differential covers enum key iteration alongside actual constructor reads.
The first focused repair exposed the imported namespace's separate ClassValue
representation. Constant ambient string enums now initialize one shared ordered
map in their ordinary module static initializer; namespace-member accesses use
the same path as identifier accesses. The original pinned LSP package supplies
an independent imported-enum key/value oracle. Pruned default lowering is
unchanged.

## CDS union constructor

`c.get().constructor.name` is a prototype descriptor read on native members,
not a common public data field. Every constituent must be a native class or method interface with
no own constructor slot (including ancestry). Such a union delegates to the
ordinary descriptor read, preserving the actual runtime constructor, inheritance
and evaluation. Its existing Go operation now requires the native constructor
marker: structural or live storage refuses rather than acquiring a synthetic
constructor. Structural/own-shadowed unions retain their shared-field proof
or refusal; no union payload layout is changed. The first focused repair
revealed the method-interface constituent; the retained oracle now covers it.

The follow-up CDS shard still has 355 passes, FAIL 0: this original body now
reaches the same `cds_parser.ts:3804` dynamic `get` / `getChildren` helper trap
owned by batch N. That group grows from one to two bodies; constructor lowering
is independently certified, but **no CDS body pass is credited** here. No dynamic
method bridge is added under O.

## AFF: malformed JSON, not JSON5 or a missing valid JSON feature

The actual input at `test/objects/domain.ts:92` is **`sdfsdf`**. Original Node
throws a native SyntaxError, caught by Domain's original catch. The old strict
runtime instead raised an uncatchable subset trap. Widening the JSON grammar
would be incorrect.

The existing strict parser now has an optional native-error callback. General
JSON.parse lowering supplies the synthesized native SyntaxError subclass and
Error ancestry, using the existing typed payload/catch ABI. AFF's original
adapter shares that runtime when its original catch provides the Error layout.
No original catch is bypassed, and syntax errors are not plain Error or host
faults. Node differentials compare exact constructor identity, Error/SyntaxError
membership, message bytes and successful JSON value bytes (duplicate properties,
`__proto__`, UTF-16, signed zero, finite decimal exponents).

The certified error domain is unexpected value/literal tokens in short inputs
(up to 20 UTF-16 units), native special undefined/NaN/Infinity diagnostics, and
unexpected end of input at a value/literal. Other diagnostic forms and valid
nonfinite-number conversion remain explicit traps, not invented SyntaxErrors.
The parser grammar stays strict: trailing object commas, invalid escapes,
revivers and coercion are not silently admitted. Existing trap-only parseJSON
and projection callers retain their prior refusal behavior.

## Native class JSON order

The general compact JSON writer now uses the existing ordered own-key bag for
native constructor-created instances, usage-gated by ordinary JSON.stringify.
Every embedded base shares the same bag. Ordinary constructor initializer/body
writes and later field/live-adapter writes append a key only on creation, never
on replacement. Optional undefined define-fields record their creation at the
source point when the compiler uses define-field semantics. Serialization reads
current values through the actual field owner, omits undefined and prototype/
static/#private members, and applies canonical array-index ordering before other
strings. TypeScript `private` keyword fields are ordinary enumerable JS keys.
Field-free objects emit `{}`. Native toJSON/cycles remain explicit refusals.
Frozen config-only adapters and pruned default emission retain their old path.

**Dell absence handoff:** ES2022 required fields without initializers are created
as undefined before later constructor assignments. Their current required
storage cannot retain that initial undefined value. A general HIR provenance
flag makes native serialization explicitly refuse such layouts (including
ancestry), rather than order by later writes or fabricate zero. A Node oracle
retains the exact `first,second` creation order and the Go refusal. Optional
undefined storage already exists and is supported without widening required
storage; ES2020 assignment-field semantics do not create these bare keys.

**Dell deletion integration:** deletion must remove the name from the shared
ordered bag. The writer never replays declaration order or stale values for a
removed name. An ordinary later assignment re-adds it at the end. The backend
Node oracle executes this exact storage contract directly, without changing
Dell's frontend delete lowering. The original LSP Help CDS test cannot pass on
this branch until Dell's delete-records fix arrives; it is not credited here.

## Genuine oracle mismatch — stopped and reported first

The first native-order oracle revealed that quoted class field names (`"10"`,
`"2"`, `"01"`) were non-blockingly skipped by the existing frontend. Go omitted
those fields while Node serialized them. Shard work stopped and this mismatch
was reported before follow-up. Unpruned literal-named property declarations now
use the ordinary hashed HIR field ABI; no computed-name effects are guessed.
The retained Node regression covers those keys, inheritance, field/body/later
assignment order, undefined optional creation, private/static/prototype exclusion,
Unicode literal keys, empty classes and single receiver evaluation. No oracle expectation was weakened
and no original test was changed.

Early regression setup failures: Node 26 no longer accepts the obsolete
transform-types flag; the enum oracle now uses the original installed TypeScript
compiler. The first serializer metadata write used an interface reference as a
Go struct; it now uses the existing owner getter. A serializer extras branch
incorrectly accessed structural extras on a native class; its structural guard
is restored. The negative null-write regression now checks the actual typed
rangeFault refusal, not the unrelated trap type. These were compile/setup or
negative-boundary errors, not accepted value mismatches. Two historical local
JSON tests asserted blanket refusal of the very features now implemented. Their
original source fixtures remain intact and now compare exact original Node
bytes; a required-undefined define-field storage negative is retained. No
upstream test is edited and no assertion is made weaker or removed.

## Certification

Frozen semantic source: **a827974** (full hash/tree identities in the gate
certificate), based on f78b973. Final documentation/evidence changes preserve
those semantic trees.

- Exactly **one** final **17-shard / 625-file** run: **11,037/11,060 active
  passes (99.79%), FAIL 0, 20 traps, zero async-pending, 35 not-run**. Original
  pinned Node: 11,060 passes, zero failures, 32 pending. Complete title multisets
  reconcile without exclusions, unmatched executions, wiring failures, borrowed
  FAILs or executed pending bodies. All 14 normalized reached-message groups
  are retained, not just a top-20 subset.
- **23 active non-passes = 20 traps + 3 unregistered + 0 active skips + 0 FAIL +
  0 async-pending.** Not-run 35 = 28 registered original pending skips + seven
  unregistered (three active + four pending). [All 23 titles/sites](traps-o-remaining.md)
  remain explicit handoffs.
- **+5 body passes:** rename three, semantic one, AFF one. Constructor repair
  reaches batch N's dynamic helper, not a CDS body pass; Help CDS still traps at
  Dell's delete-records boundary. Their independent oracles are not inflated
  into original-test passes.
- [Focused evidence](evidence/traps-o-focused.json) retains three selected
  development stages. Initial development measurement was launched before
  edits but compiled asynchronously during edits: its counts match train 11,
  but its diagnostics are not a frozen baseline certificate. Final followup
  uses a827974. No full baseline run was performed.
- All requested short, Grace-enabled tsfront/hir/cmd and vet gates pass, along
  with runner regressions and the new/retained exact Node oracles. Early local
  gates still expected obsolete blanket native/row JSON refusals; the original
  fixtures now assert strict Node bytes, and unsupported-storage negatives are
  retained. No original upstream fixture or assertion is touched.
- **No golden changes:** regenerated lexer facts are byte-identical. No new
  operation/effect catalogue entry. Default output versus an archived f78b973
  compiler has **raw diff -r exit 0, zero bytes, all 6,312 files**, including zip,
  at the same absolute path. No output changes require justification.
- Fresh emitted Go and fresh original pinned Node agree byte-for-byte on
  zabapgit **clean 0 / seeded 5**, also equal to the checksum-verified v0.1.1 kit;
  direct cmp and go-target-check.py both pass. [Gate certificate](evidence/traps-o-gates.json)
  retains output SHA256s, environment and semantic trees.

Generated builds, copied closures, archived sources/output, kit, profiles,
facts, raw records and logs are deleted. Only implementation/regressions and
compact evidence/docs remain. Clean commits, no push or transpiler clone.
