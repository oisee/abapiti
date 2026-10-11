# Trap batch C roots

## Third round: remaining owned roots and integration

### Selection-screen assignment: optional default construction

The 17 selection-screen tests refused in `checkFile`'s prologue, not in a
registry assignment or class bridge: `mainProgName: string | undefined =
undefined` emitted an absent optional-dynamic literal assigned to an
optional-string parameter. `applyDefaults` now coerces each default to the
parameter's storage type, as ordinary assignments already do. This is shared
frontend lowering, not a rule override or ABI relaxation.
`TestDefaultUndefinedNodeOracle` covers omitted/explicit undefined, empty and
present strings, and lazy once-only default evaluation; Go and ABAP emission
pass with Grace. Focused original selection-screen tests: **17/17, FAIL 0**.

### LSP own-key enumeration: Dell-owned presence/order ABI handoff

No own-key compiler/runtime code was changed. `Lookup.reference` reaches
`Object.keys(ref.extra)` at `src/lsp/_lookup.ts:272`; `extra` is the native data
shape `IReferenceExtras` from `src/abap/5_syntax/_reference.ts`, whose three
members are optional. The current shape ABI cannot distinguish omitted keys
from own keys holding undefined, or retain literal insertion/write order.
Independent Node evidence:

```js
for (const extra of [{}, {ooName: undefined}, {foundQualified: false},
                    {ooType: "CLAS", ooName: "X"},
                    {ooName: "X", ooType: "CLAS"}]) {
  console.log(Object.keys(extra));
}
// [], ["ooName"], ["foundQualified"],
// ["ooType", "ooName"], ["ooName", "ooType"]
```

`tsfront/lower_types.go:synthFromProperties/synthFromAlias` use optional field
values for missing interface members; their constructors have no own-key metadata.
`hir/golang/emit.go:class` stores those values as native fields, while
`shape_fields.go:shapeFields` returns undefined for both missing and absent
optional values. `objectStatic` in `lower_expr2.go` correctly lacks a native
shape key-order proof. Enumerating Go fields invents absent JS properties;
filtering undefined loses present-undefined keys, **including the count** used
by LSP. Parsed-source key order alone is not a live native own-key ABI.
Dell needs property presence and insertion/deletion/reinsertion order carried
through native shapes and views, plus mutation/materialization rules. Required
oracles: empty/present-undefined/false optionals, both literal orders, alias
writes, delete/reinsert and integer-index ordering. The historical 53-test
bucket remains a refusal, not a fabricated pass; final reached counts are in
[the all-shard report](abaplint-tests.md).

### Chai deep assertions: exact supported data graph, explicit refusals

The harness shim supports `.deep.equal/equals`, `.eql/eqls`, and
`.deep.include/contain`. Tests-only calls resolve the shim's actual helper
symbols, never arbitrary user methods with matching names. Usage-gated Go
operations implement Chai 4/deep-eql 4's SameValue primitives (NaN and signed
zero), positional arrays, pair memoization for cycles, key-set equality
independent of order, and object subset assertions requiring **presence** as
well as deep value equality. Nested subset values are full deep equality, not
recursive partial matching. Sparse array inclusion skips holes; equality reads
holes as undefined. Empty object subsets do not assert even when negated.

Native enumeration is certified only for required-field data shapes: their
key set is known and equality does not depend on insertion order. Optional or
nominal/native enumerable storage, inherited prototype properties, unsupported
exotic types and string coercions, and direct parsed-to-native snapshot escapes
remain typed refusals. No identity/live-view ABI, closure or upstream body was
changed. ABAP explicitly refuses the three harness operations. Effect records
and backend catalogue checks include these refusals and the earlier SHA-1
operation; the inspected Grace golden changes only operation fact counts.

**A genuine shim mismatch was found and reported before continuing.** Initially
`.deep` returned a new assertion without persisting the flag on the original.
Actual Chai keeps the original's flag and returns a distinct wrapper; `.not`
is persistent too, and `.eql` must not set `.deep`. The incomplete shard run
was stopped. The retained `TestChaiDeepNodeOracle` now independently checks
these reuse/identity cases against actual upstream Chai, alongside 24 positive
and negative data comparisons and fluent aliases. A negative equality stays
failed in the oracle; no failed record is relabeled trapped.
`TestChaiDeepRuntimeNodeOracle` additionally compares NaN, infinities, signed
zero, cyclic graphs, holes versus undefined and missing-key semantics, and
requires typed refusals for unproven/inherited properties. Focused original DDL
is **32/32, FAIL 0**; selection-screen plus DDL is **49/49**.

## Second-round stop (historical): rule configs require Dell-owned class/view ABI

**Stopped at `479373b`, before changing compiler/runtime code.** The erased
setter conversion cannot be repaired locally while retaining JS identity and
live writes. This meets the task's explicit type/ABI stop condition; the
remaining selection-screen assignment, LSP own-key and chai.deep roots, final
gates and all-17-shard measurement were **not executed in this round**. No pass
gain, whole-suite FAIL count or integration certification is inferred.

### Reproduction and observed boundary

A temporary ordinary-TS probe used the same interface signatures as upstream
`IRule`: `setConfig(conf:any):void`, `getConfig():void`. Its implementation
stores a `RuleConf extends BasicConf` in a nominal field, returns that field
from its value-returning `getConfig`, and reads/writes its inherited `value`.
The following is the Node reproduction (each function uses a fresh rule):

```ts
class BasicConf { value = 1; }
class RuleConf extends BasicConf {}
interface IRule { setConfig(conf: any): void; getConfig(): void; }
class Rule implements IRule {
  private conf = new RuleConf();
  setConfig(conf: RuleConf): void { this.conf = conf; }
  getConfig(): RuleConf { return this.conf; }
  read(): number { return this.conf.value; }
  write(value: number): void { this.conf.value = value; }
}
function live(conf: RuleConf | {value: number}): string {
  const rule = new Rule();
  const slot: IRule = rule;
  slot.setConfig(conf);
  const current: any = rule.getConfig();
  const same = current === conf;
  conf.value = 2;
  const observed = rule.read();
  rule.write(3);
  return same + ":" + observed + ":" + conf.value;
}
console.log(live(new RuleConf())); // true:2:3
console.log(live({value: 1}));      // true:2:3
const parsed = JSON.parse('{"value":1}');
const rule = new Rule();
const slot: IRule = rule;
slot.setConfig(parsed);
const current: any = rule.getConfig();
const same = current === parsed;
const observed = rule.read();
rule.write(3);
console.log(same + ":" + observed + ":" + parsed.value); // true:1:3
```

The actual frontend probe duplicated these bodies as static methods (no
captured functions or closure work). A fresh Node type-stripping oracle printed
exactly the three results above. Go matched the native case; **both the plain
and parsed cases raised `rangeFault` at the erased setter**, not assertion
failures. The emitted bridge contains
`castRef[<RuleConf reference>](dynRef(parameter))`. The temporary test separately
required those typed refusals, and passed with:

```sh
GOCACHE="$HOME/.cache/go-build" GOTOOLCHAIN=go1.26.0 \
GOFLAGS=-buildvcs=false ABAPITI_GRACECHECK=1 \
go test ./tsfront -run '^TestTrapsCConfigBridgeProbe$' -count=1 -v
```

The temporary probe and its generated Go were removed. This is diagnosis of an
unsupported boundary, **not** a passing Node/Go parity regression or a shard
run. An initial probe additionally encountered unsupported arbitrary `any`
property assignment and an `any`-return interface bridge verification refusal;
the final probe avoided both and used upstream's void-return signature. Neither
was repaired or reclassified, and no genuine runtime assertion mismatch arose.

### Exact handoff

- `tsfront/lower_generic.go:eraseGenericOverrides` emits `dynamic.asRef` from
  the interface's erased `any` parameter to the implementation's `ClassRef`.
  `hir/golang/ops.go` emits the checked `castRef(dynRef(...))` boundary.
- `hir/golang/runtime.go:dynRef` accepts native references/arrays, not parsed
  `tagObject` records; `castRef` also rejects a native plain shape that is not
  the nominal config representation. Relaxing only either check does not
  supply compatible storage.
- `hir/golang/emit.go:class` represents leaf classes as concrete pointers and
  other classes as interfaces requiring nominal getters returning concrete
  structs. `lvalue` uses those getters for direct native field access. The
  setter implementation, `conf` field, return view and inherited field reads
  therefore still require nominal storage after the bridge.
- `hir/golang/materialize.go:materializer` rejects inherited config classes and
  constructs fresh native storage for permitted data shapes. Its `source`
  pointer is not a live field proxy. Even extending that materializer would
  snapshot fields: caller writes would not reach the rule, rule writes would
  not reach the caller, and identity/prototype behavior could be fabricated.

Dell needs a structural/live reference view (or a coordinated erased storage
representation) across the class/union ABI and property accesses. Required
oracle cases: `getConfig() === supplied`, writes in both directions after
`setConfig`, two rules sharing one supplied object, inherited/optional/missing
fields without injecting constructor defaults, plain/native/parsed configs,
and `instanceof RuleConf` remaining false for plain/parsed objects. Existing
checked nominal-view refusal tests must remain intact. Widening just the setter
parameter or special-casing SevenBitAscii is not a general fix.

The batch-B counts 173/72/22/17 are historical reached buckets, not confirmed
remaining totals: this branch already fixes ancestry tests in
obsolete_statement/avoid_use (85 focused passes, 24 traps below). Config's
release/language conversions must be checked separately from setter casts;
this probe does not claim to resolve all 22 Config tests.

## Call receiver construction context

Downport's four arrays of ExpressionNode are flattened to ExpressionNode[].
The call result's array construction hint leaked into its receiver literal,
causing each inner array to be pushed as an ExpressionNode. This is a general
frontend context bug, not a runtime flat or argument ABI change. Library-call
receiver lowering now clears/restores the result hint, as property reads already
do. `TestGoArrayReceiverContextNodeOracle` checks nested literal flattening,
reverse, evaluation counts, fresh storage and references against Node; ABAP
emission passes. Grace-enabled small-builtins tests pass.

Focused upstream Downport: 1 passed, 297 trapped, 5 not-run, **FAIL 0**.
The 194-test argument mismatch is gone; newly reached cases chiefly trap at
crypto (157) or Dell-owned EditHelper (85). No pass gain is inferred from
removing an earlier trap. Generated module/closure deleted after execution.

## Conversion triage: ancestry operand versus parsed rule configs

The generic conversion message grouped different roots. Stack inspection of a
throwaway generated module identified checked nominal casts *before* ancestry
tests in obsolete_statement and avoid_use; Config/7bit_ascii instead encounter
parsed object-to-rule-config boundaries. These are not one shared conversion.

`instanceof` now tests the operand's natural storage type rather than applying
the checker's flow type first. Empty sibling classes are structurally identical
in TS but nominally distinct in JS; compound/repeated tests could otherwise cast
to the wrong sibling before returning false. The general Node/Go regression
covers A/B/C/base instances, repeated and short-circuit tests, and once-only
receiver evaluation; ABAP emission and Grace checks pass. Explicit user class
assertions still retain checked-view refusal behavior.

Focused obsolete_statement + avoid_use: **85 passed, 24 trapped, FAIL 0**.
Remaining traps are Dell-owned EditHelper (20) and registry assignment (4).
Generated builds and temporary instrumentation removed.

## Fresh Node SHA-1 pipeline

The frontend recognizes only a fresh namespace-imported `crypto`/`node:crypto`
`createHash("sha1").update(string).digest("hex")` chain. No intermediate Hash
state escapes; aliases of the namespace work, local lookalikes do not. Other
algorithms, encodings, multiple updates, buffers and coercion remain explicit
traps. Go uses SHA-1 on UTF-8 with Node's replacement of isolated surrogates;
separate Node-oracle tests exercise empty/ASCII/NUL/Unicode strings, supplementary
pairs and isolated surrogates, plus once-only input evaluation. ABAP explicitly
refuses the new operation. This is a generic imported intrinsic, not a Downport
body rewrite. Pruned/default lowering is unchanged.

Focused Downport remains **1 passed, 297 trapped, 5 not-run, FAIL 0**; both earlier
roots are gone. EditHelper now blocks 238 tests. No speculative pass credit.

## Grouped-switch bindings in numeric range renaming

DDIC's decimal cases lower one shared TS block into mutually exclusive HIR
branches. The numeric pass renamed each copy of a checked binding separately,
then rewrote *all* reads to the last copy's name, leaving earlier branches with
unresolved locals. Each checked symbol now gets one name reused by its copies;
different shadowing symbols remain distinct. The general grouped-switch
Node/Go oracle reproduces the exact unresolved-local refusal before the fix,
and tests all grouped entries plus an independent shadow binding. Existing
parameter-shadow and alias-boundary regressions pass with Grace. ABAP refuses
its independent Math.ceil operation explicitly.

Focused basic_types + sql_value_conversion: **15 passed, 2 trapped, FAIL 0**;
remaining traps are chai's unsupported `instanceOf` alias. Full syntax/rules
DDIC gains are measured only in the final all-shards execution.
