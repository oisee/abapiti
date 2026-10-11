# Trap batch E roots and Dell handoffs

Merged base `f0e8efe` (C `b11e522` + registrations `a8ee028`): fresh execution
of all 17 shards, 625 files, **8,860/11,060, FAIL 0**, 2,091 trapped,
141 not-run (115 unregistered + 26 skipped/focused). Original Node: 11,060
passed, zero failed, 32 pending. No infrastructure/title-accounting failures.
Exact base evidence: [abaplint-tests-traps-e-base.json](abaplint-tests-traps-e-base.json).

## Owned repairs

### `expect.fail`: harness namespace sugar, not a captured local

`test/rules/_utils.ts:154` calls the merged function/namespace's `expect.fail`.
The namespace was not emitted, so generic member-call lowering tried to load
`expect` as a value. Tests-only lowering now resolves **both** runtime/chai's
`fail` and its `expect` base (including aliased imports), and calls the emitted
`chaiFail` helper. Arbitrary classes named expect/fail are not intercepted.
Only the one-message form is supported; other arities/spreads explicitly refuse.
This does not alter the upstream helper or implement a general function-object ABI.
`TestChaiSmallNodeOracle` uses actual pinned Chai, checks caught failure messages,
aliased imports, lookalikes, and proves an uncaught fail stays a genuine failed
host record rather than being reclassified as a refusal.

### Shallow Chai array containment: no `any[]` conversion

The seven `runtime/chai.ts:43` refusals occur in `contain`, not the config
setter/live-class-view root. Its dynamic-to-`any[]` cast asks for different
physical storage from a boxed native typed array. The tests-only
`chai.arrayInclude` operation instead reads the existing boxed array through
its already-supported length/presence/element adapter; it never snapshots,
mutates, or weakens a class conversion. Identity uses the existing strict
boxed equality, including nested arrays/objects. Nonarrays refuse, rather than
pretending to implement arbitrary Chai object-subset/prototype behavior.
ABAP explicitly refuses this harness operation; default programs do not use it.

**Genuine shim defect discovered by an independent oracle, reported before
continuing:** actual Chai's shallow array containment uses `indexOf` semantics,
not `Array.includes`. `[NaN]` does not contain NaN, and holes are skipped rather
than treated as undefined. The repaired operation preserves those rules,
including signed-zero equality. `TestChaiDeepRuntimeNodeOracle` retains actual
Chai checks for NaN, zero, holes/present undefined, object identity, nested array
identity, parsed/native arrays and nonarray refusal. `TestChaiSmallNodeOracle`
checks full lowering/fluent negation, strings and native data arrays. No suite
failure was relabeled; the completed base had FAIL 0.

### DataDefinition conditional: Set result context leaked into its input

The eight `src/objects/data_definition.ts:221` refusals are construction-context
lowering, not a collection ABI mismatch. A minimal ordinary-TS reproduction of
`new Set(node ? node.getTokens().map(t => template) : [])` emitted an
`array<string>` true arm and a **`set<string>` empty arm**, then tried to copy
the malformed conditional as a Set. `newExpression` now clears/restores the
outer result hint while lowering the Set's iterable argument. Both arms retain
their natural array storage, followed by ordinary `set.fromArray` construction.
No Array-to-Set cast, verifier relaxation, or upstream body override is added.
`TestSetArgumentContextNodeOracle` reproduces the old verification refusal and
checks template-derived keys, both branch orders, lazy once-only input effects,
empty/nonempty arrays, duplicate elimination, insertion order and fresh Set
mutation against Node, plus ABAP emission with Grace.

## Type/ABI items handed to Dell (not repaired here)

Counts below are reached tests on the merged base, not inferred future passes.
Cascading unresolved locals are consequences, not separate helper fixes.

### Downport `getParameters` on dynamic — 13

`src/rules/downport.ts:3574` explicitly declares `findConstructor(...): any`.
Its return is the method-definition object found by `getMethodDefinitions()` /
`getByName("CONSTRUCTOR")`, or undefined. At line 3556 the optional chain unwraps
this erased dynamic result, but `getParameters()` is an arbitrary user method,
not a recognized runtime intrinsic. Generic member-call lowering in
`tsfront/lower_expr.go:call` cannot dispatch a method on Dynamic. The declaration
of `importing` therefore fails; the two unresolved reads at 3557/3558 cascade.
Do not synthesize a free local or special-case the method's spelling.
Dell needs a checked erased object/method view or sound return-shape recovery
across the helper, its optional chain, and MethodParameters result. Required
oracles: absent constructor, found/inherited constructor, repeated superclass
lookup, once-only receiver evaluation, and an erased incompatible object
refusing rather than invoking a fabricated method. The captured Downport `fix`
and argument-mismatch roots are separate and unchanged.

### AmbiguousStatement constructor result — 9

`src/rules/ambiguous_statement.ts:89` receives `type1: new () => IStatement`;
line 95 constructs it and immediately calls `getMatcher()`. The descriptor's
erased factory result is `IStatementRunnable`; the emitted narrowed view is
`IStatement`. HIR rejects that interface-to-interface narrowing, exactly:
`invalid narrowing interfaceref<...IStatementRunnable> to interfaceref<...IStatement>`.
`tsfront/lower_expr.go:newExpression` selects the use-site result, while the
classvalue factory/constructor result and interface relation must agree.
This is general constructor/interface representation work, not a rule defect.
Dell should preserve the actual instance and validate the checked view; do not
bless every interface cast. Oracles must include multiple concrete statement
constructors, matcher virtual dispatch, constructor identity/effects, and an
incompatible descriptor refusal. No narrowing/verifier/descriptor code changed.

### RenameGlobalInterface assignment — 7

`src/objects/rename/rename_global_interface.ts:18` receives `obj: IObject`, checks
`obj instanceof Interface`, and uses its narrowed `getMainABAPFile()` and later
its original identity in RenamerHelper calls. Verification reports synthetic
`node 0 (): assignment type mismatch` in `buildEdits`, not a source assignment
of `changes` at line 45. This is the flow-view parameter/assignment boundary
between IObject and nominal Interface. The analogous RenameGlobalClass bucket
is six tests. Dell needs to inspect the synthesized flow-view assignments and
preserve alias identity across both original-interface and class uses; widening
the method parameter or copying an Interface is not sound. Required oracles:
valid/wrong object types, negative instanceof branch, same instance in helper
calls, inherited fields/methods and writes through both views. These assignment
roots and the registry assignment are left untouched.

### NameValidator NamingRuleConfig shape — 7

`test/utils/name_validator.ts:7` passes a plain literal `{patternKind:"required"}`
to `NameValidator.violatesRule(..., params: NamingRuleConfig)`. NamingRuleConfig
is an **abstract nominal class extending BasicRuleConfig**, with optional
`patternKind`, `ignoreNames`, `ignorePatterns` fields and constructor defaults.
Object-literal construction (`tsfront/lower_expr2.go`) reports that patternKind
is not in its selected nominal shape; the missing `ruleViolated` then cascades.
This boundary needs structural/plain-to-inherited-class storage, not a missing
helper registration. It overlaps the config live-view handoff in
[traps-c-roots.md](traps-c-roots.md): plain objects must not gain constructor
defaults or become instanceof NamingRuleConfig. Required oracles: omitted vs
explicit undefined patternKind, required/forbidden, ignoreNames and dynamic
ignorePatterns, inherited fields, alias writes and unchanged object identity.
No abstract class, object-literal shape or config conversion code changed.

### Other new top-20 type/ABI roots

Batch-D registration makes 31 `test/abap/nesting.ts:253` conversion refusals and
seven pretty-printer `keyword_case` property assignments reachable. They remain
unsupported; registration gains are not estimated as body passes. The latter
uses the live IConfiguration/rule-config object boundary. Neither is selected
as a harness/string repair. Captured registry `getConfig`, parse/parseView,
LSP own-key metadata, DDIC locals, local-class lowering, async and panic work
remain outside this batch.

Final measurements and gate/output audit are recorded in
[abaplint-tests.md](abaplint-tests.md).
