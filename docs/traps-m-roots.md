# Trap batch M — exact conversion-site audit

Base: **8b2511b**. Semantic repair: **b67770b**. Original upstream:
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. Original source, assertions,
selection and registration are unchanged. No site override or ABI relaxation.

## Method: enclosing-method locations are not failing expressions

Four focused baseline shards (`root`, `lsp`, `objects`, `rules`) measured
**3,518 passes, FAIL 0, 55 traps**. Their nine target bodies are below.
Generated Go was retained temporarily. A scratch-only `newRangeFault` diagnostic
printed runtime frames without changing the refusal or catching it. All owned
result records matched the uninstrumented executions exactly; LSP's six extra
probe records were the original passing borrowed SQL-conversion tests.

A separate fresh original-Node build executed the eight relevant test modules:
**78 passing, FAIL 0**. Observation-only wrappers recorded original parser
results, absent property presence, config values and constructor names. The
wrappers delegated to the original operations, never substituted values.
Only compact observations/counts are retained in
[evidence/traps-m-focused.json](evidence/traps-m-focused.json).

The generic diagnostic names the **enclosing emitted method**, not necessarily
an arithmetic operation or even a conversion. The exact baseline Go frames,
variables and operations below distinguish reached faults from later possible
faults. Line numbers refer to the retained baseline shard modules, not TS lines.

## 1. Dynpros: two absent `NAME` values, not numeric parsing

Both `rules` bodies:

- `Rule: dynpro_checks Element SDFSDF length longer than 132`
- `Rule: dynpro_checks overlapping dynpro fields`

The emitted `z_body_src_obj_3adafa39348c9c` (`parseDynpros`) refuses at
`rules-out/go/hir.go:869076`:

```go
v99 := v97.get(str("NAME"))
v100 := dynString(v99)
```

`v97` is the **OKCODE** field row, at field index **1 / 2**, respectively, in
screen `2000`. Both XML fixtures omit `<NAME>` on that row. Original Node has
`f.NAME === undefined`; the resulting field object has an **own `name` key with
value undefined**. It is not an absent row/key in the destination object.
`dynString` requires a nonnil dynamic tagged as an actual UTF-16 string.
Go receives nil/undefined and refuses before the remaining field construction.

The row's type is `"OKCODE"`, container type is `"SCREEN"`, length/vislength are
20, height is 1, and absent line/column correctly become 0 through the original
`parseNumber` guard. **No nonfinite parseInt is reached here.**

**Dell: absence/union storage.** The declared `DynproField.name: string` does
not validate or coerce JavaScript's erased input. Preserve explicit undefined
in the row storage, not `""`, a removed key, a skipped OKCODE row or a new parser
default. Header and other field conversions are only possible follow-ups.

## 2. MessageClass: both failures are the first constructor argument

Bodies:

- `objects`: `malformed object XML ignores structured values where text is expected`
- `rules`: `Message rule no number`

`z_body_src_obj_ac5aaa20f7866c` (`MessageClass.parseXML`) refuses at
`objects-out/go/hir.go:659067` / `rules-out/go/hir.go:910953`:

```go
v256 := v254.get(str("MSGNR"))
v257 := dynString(v256)
// Later, not yet executed:
v261 := unescape(v258.get(str("TEXT")))
v265 := dynString(v262.get(str("ARBGB")))
newMessage(v257, v261, v265)
```

Both source XML rows **omit MSGNR**, so Node passes **undefined** as `num` to
`new Message(num, text, arbgb)`. The required Go string argument refuses first.
The malformed-object row is `{TEXT: {nested: ""}}`, also lacking ARBGB. Original
`unescape(unknown)` already returns `""` for nonstrings; **structured TEXT is
not the reached conversion** and needs no XML/string repair. The rule fixture
has valid TEXT `"hello world"` and ARBGB `"ZAGTEST_EMPTY"`, but still no MSGNR.

**Dell: absence/union argument storage.** Crucially, the original Message
constructor **explicitly checks undefined and then sets number/className to
`""`**. Node's final number is therefore `""`, not undefined. Go refuses at the
caller before that legitimate source guard can run: its required string
parameter cannot carry the original undefined input. Preserve that input through
the call ABI so the original constructor performs its own defaulting; do not
invent a caller-side empty string, drop the message, coerce to `"undefined"`, or
replace the text guard. Optional topName and description conversions are not
the failing sites in these two bodies.

## 3. View: both failures are `KEYFLAG`, before join construction

`objects` bodies:

- `View, parse XML test`: first missing KEYFLAG is field **KORRNUM**.
- `View, parse XML database view with 2 append views`: first missing KEYFLAG is
  main-view field **FIELD2** (not a skipped `.APPEND` marker or a join row).

Both refuse in `z_body_src_obj_15e171d57f48fd` (`View.parseXML`),
`objects-out/go/hir.go:695193`:

```go
v313 := v311.get(str("KEYFLAG"))
v314 := dynString(v313)
newField(viewField, tableName, fieldName, v314)
```

The three guarded text fields are strings. KEYFLAG is **undefined**, because
those XML elements are omitted. Node stores an **own KEYFLAG property valued
undefined**, and its later `=== "X"` check treats the row as non-key. Go's
required native `KEYFLAG: jsString` argument instead requires a real string.

**Dell: absence/union row storage.** Not a parser bug, invalid numeric value or
missing join conversion. Preserve the value and property presence; supplying
`""` is not an exact storage repair even if that one equality would agree.

## 4. SpaceBeforeDot: live config omission, not a default boolean

`root` bodies:

- `exclude list will exclude issues based on the local exclude patterns`:
  original config is `{exclude: ["FOO.prog.abap"]}`.
- `severity an issue will have the severity specified in the config`:
  original config is `{severity: "Info"}`.

The emitted `runParsed` body calls `z_view_read_sr_2639abf67a0ff8`; that live
getter at `root-out/go/hir.go:911556` refuses:

```go
d := self.viewSource.get(str("ignoreGlobalDefinition"))
return dynBoolean(d)
```

The original property is **omitted**, not false, and Node reads **undefined**.
JavaScript's `if (this.conf.ignoreGlobalDefinition)` simply does not take the
branch. `dynBoolean` requires the actual boolean tag, so nil/undefined refuses.
The required field exists only in the nominal declaration/default constructor;
`setConfig` actually replaces the config with the supplied partial object.

**Dell: live config views/absence**, including the ongoing config `object.get`
work. Do not return false, reconstruct defaults, snapshot/copy the view, or
change the supplied partial config. `ignoreExceptions` and severity are not the
first refusing reads. Both original cases remain explicit handoffs.

## 5. Help: a repairable existing-descriptor read, not `dynString`

`lsp`: `LSP, help first character = DO`.
`z_body_src_lsp_c369cdd34a8258` (`Help.linkToStatement`),
`lsp-out/go/hir.go:609654`, first executes:

```go
v4 := v2.get(str("constructor")) // reached refusal
v6 := v4.get(str("name"))        // not yet reached
v7 := dynToString(v6)            // not yet reached
```

The runtime frame is **dynamic.get's tag-2 missing-adapter branch**, not its
nil-receiver branch. `v2` boxes the real, field-free native **Do** instance.
Original Node reads the inherited constructor **function Do**, whose name is
**the string `"Do"`**. The instance has no own constructor property; its
prototype has one. Generated Do already has the stable descriptor
`z_descriptor_s_2e3d5f812f0615`, `Name: str("Do")`, and a `descriptor()` method,
but no dynamic field adapter. Thus the actual native value/identity/name exists;
the dynamic read simply lacks access to it. This refines K/L's broad reflection
handoff: this bounded read needs no new constructor/union storage ABI.

**General runtime/emitter repair:** nonstructural native classes expose their
existing descriptor through `constructorDescriptor()`. Dynamic get uses it only
for `constructor` when no field adapter exists. Field-backed native adapters
also expose it after own-field cases. Existing live-view delegation runs first
and remains unchanged. Only real descriptor values permit a `name` read;
opaque function tokens and other properties still refuse. The same descriptor
pointer is returned, not a reconstructed class or string-only constructor shim.
Structural synthesized records never acquire a nominal constructor, and live
rows retain original backing storage. No enumeration, prototype mutation,
function call, name write, config getter or sibling assertion ABI is widened.

`tsfront/constructor_name_test.go` executes the complete emitted path against
Node: field-free Base/Derived, interface-erased instance, field-backed class,
constructor identity/difference, original JSON own-constructor shadow, and one
receiver evaluation. Separate original-Node probes report Object for structural
and live rows; Go still explicitly refuses those unimplemented prototype views.
Opaque function name/prototype reads and descriptor name writes remain refused.
The focused original Help body advances through this repaired operation but
then reaches **DumpScope's existing TypedIdentifier class-ToPrimitive refusal**.
LSP stays **178 passed, FAIL 0, 10 trapped**. The independent descriptor operation
is certified; **no recovered body pass is credited**. That subsequent ABI
boundary is left untouched, and its reached group grows from one to two bodies.

## Development stops and scope

No genuine Go/Node value or assertion mismatch was encountered or accepted.
The observation script first used the wrong installed mocha path; that setup
failure was corrected before obtaining the 78-pass Node observation. New test
prototypes tried computed/quoted class fields named constructor and an angle-
bracket assertion; existing lowering refused those constructs. The supported
shadow probe now uses original JSON own storage, while no original regression
or assertion is weakened/deleted. An early interpretation of the Help stack as
a nil second read was explicitly corrected after inspecting the exact frame:
it is the **first native-instance get**, as documented above.

None of the excluded CDS, renamer/range, Domain, ClassData, RenamerHelper,
ProxyObject, ABAPObject parseInt, Chai keys, config-shape or pretty-printer
boundaries is changed. Four baseline shards plus one repaired LSP shard precede
exactly one final 17-shard run. No complete base run or second final run occurred.

## Final certificate and complete remaining list

Frozen semantic source: **b67770b98271251b8d5c823eef67e778fcacef79**. All **17
shards / 625 files**: **10,949/11,060 Node-active passed (99.00%), FAIL 0, 108
traps in 32 groups, zero async-pending**. **111 active non-passes = 108 traps +
3 unregistered + 0 skipped + 0 FAIL + 0 async-pending**. Not-run is **35 = 28
skipped + 7 unregistered**, zero async/done. The 32 original Node-pending titles
are 28 skipped registrations plus four unregistered titles, all outside the
active denominator. Fresh original Node passes all 11,060 active bodies.
No exclusions, wiring failures, unmatched executions or borrowed FAILs occur;
whole-suite and owned title multisets reconcile.

No body pass is claimed from the repaired constructor read. Compared with
8b2511b, only Help's reached group disappears and DumpScope's group grows 1→2.
All other trap groups, initialization refusals and per-shard counts agree.
[Final JSON](evidence/traps-m-final.json) retains every complete normalized
message, located example, count, shard contribution, missing title and pending
audit; the following **32 groups sum to 108**. Initializers are separate.

| Count | Remaining reached root |
| ---: | --- |
| 29 | Renamer unresolved common edit-union `range` |
| 19 | Domain native/optional conversion |
| 7 | PrettyPrinter live `keyword_case` assignment |
| 7 | NameValidator `patternKind` shape + dependent local |
| 3 | ClassData absence/statement-structure union conversion |
| 3 | LSP Rename versioned/optional identifier assignment |
| 2 | Chai native ordered-object enumerable keys |
| 2 | ABAPObject MAXLENGTH nonfinite parseInt adapter |
| 2 | Dynpros absent `NAME` native-row conversion (section 1) |
| 2 | MessageClass absent `MSGNR` constructor input (section 2) |
| 2 | RenamerHelper erased sibling/common-method conversion |
| 2 | View absent `KEYFLAG` native-row conversion (section 3) |
| 2 | SpaceBeforeDot absent live-config boolean (section 4) |
| 2 | LSP ApplyEdit unresolved common edit-union `range` |
| 2 | Test/Domain erased sibling nominal conversion |
| 2 | Config live `applyUnspecifiedRules` assignment |
| 2 | PrettyPrinter `keyword_case` + `sequential_blank` assignments |
| 2 | CDS erased-node `get` / `getChildren` + dependent local |
| 2 | UnusedVariables inherited `skipAbstract` shape + dependent local |
| 2 | DumpScope class ToPrimitive (two coercions); includes Help follow-up |
| 1 | Semantic class descriptor narrowed to structural instance |
| 1 | Domain strict JSON.parse SyntaxError payload boundary |
| 1 | EnhancementSpot erased/absent XML-field conversion |
| 1 | NumberRange parseFloat NaN/nonfinite result |
| 1 | ProxyObject live erased row/array assignment |
| 1 | LineOnlyPunc live partial-config conversion |
| 1 | Types helper Program/Class common-method live view |
| 1 | CDS erased-node `getFirstToken` + dependent local |
| 1 | Help native own-property deletion |
| 1 | PrettyPrinter native config deletion |
| 1 | Registry native structural deletion |
| 1 | UnnecessaryPragma inherited `allowNoTextGlobal` shape + dependent local |

[Gate evidence](evidence/traps-m-gates.json) certifies the short suite, full
Grace-enabled tsfront/hir/cmd suite, vet, runner regressions and native-descriptor
Node oracle. **No golden changes:** regenerated lexer facts are byte-identical
(SHA256 `619b2efac72d9f0344c2f6e2114b97334ac06eba9604d0467ccf8ad19b99ee07`).
No runtime operation/effect record was added; all 61 Grace call sites are
unchanged. The repair reuses existing `dynamic.get` and descriptor storage.

Default output versus archived **8b2511b** at the same absolute output path has
**raw `diff -r` exit 0, zero diff bytes, 6,312 files**, including the zip. There
are no default differences to explain. The opt-in Go emitter adds only the
reviewed descriptor-access capability; no ABAP/default backend is changed.
Fresh Go and original Node agree byte-for-byte on zabapgit **clean 0 / seeded
5**, independently matching the SHA256-pinned v0.1.1 kit. Direct raw `cmp` and
`go-target-check.py` both pass; compact evidence retains both output hashes.

Documentation-only certification preserves all four measured semantic subtree
hashes. Generated executables, copied closures, archived source/output, kit,
profiles, facts, raw records and logs are deleted, including the temporary
oracle directory left by an early interrupted scratch setup. Only compact
Markdown/JSON and implementation/regression source remain. Clean commits,
no push, no transpiler clone, no upstream test weakening/deletion, and no
accepted genuine mismatch.
