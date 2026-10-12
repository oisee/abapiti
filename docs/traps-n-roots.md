# Trap batch N — native dispatch, bounded coercion and lexical classes

Base: **8288338** (train 10 + L + M). Frozen semantic source: **469fd1d**.
Original upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`. No upstream
source, assertions, selection or registrations are changed. No new site override,
transpiler clone or push.

## Re-check the actual base, not M's historical certificate

Four focused base shards measured **3,814 passes, FAIL 0, 17 traps, three
active unregistered**. In particular LSP is **180/188**, rules **3,141/3,149**,
CDS **355/358**, and structures **138/139**. M's historical LSP 178 and rules
3,121 are not this base: intervening live-property work has already recovered
22 bodies. N does not claim those gains. The compact
[focused evidence](evidence/traps-n-focused.json) retains counts, original Node
active/pending partitions, initialization refusals and all reached messages.
Focused development measurements precede the final certificate. Two incomplete
all-shard attempts were stopped for genuine refusal-boundary mismatches, as
disclosed below; neither is reported as a full measurement.

## 1. CDS: two roots, three blocked bodies

The direct root is `test/cds/cds_parser.ts:3745`, in the table-field annotation
test:

```ts
parsed!.getChildren().filter(c => c.get().constructor.name === "CDSAnnotation")
```

The other root is the recursive, erased `findAll` helper at **3799–3810**:
`x.get?.()?.constructor?.name` at **3804**, followed by
`x.getChildren?.() ?? []` at **3806**. Its two blocked callers include the
second table-field test and the quoted-alias test; the latter subsequently
reads **3852**, `nameNodes[0].getFirstToken().getStr()`. Thus repairing the two
shared roots can recover three original bodies, not just two diagnostics.

Children carry the original concrete `ExpressionNode`/`TokenNode` references.
Their `get` results have different ABIs (`IStatementRunnable` versus
`AbstractToken`); a single constituent cast or shared native return layout is
incorrect. Existing checked union views are reused when their method signatures
are compatible. Otherwise the Go-only **`dynamic.call0`** operation dispatches a
literal zero-argument method against the closed emitted native-class catalogue.
Each case invokes the concrete virtual method and boxes its actual result.
No object, row, array or constructor is copied or fabricated. Native erased
array-element reads box original slots rather than casting the raw object to an
already-boxed tag. This is a read adapter, not a changed union/absence layout.

The operation refuses nonnative/callable receivers, unknown methods/classes,
computed, accessor-dependent or field-shadowed method layouts, and argument-
bearing calls. Source getter/setter metadata is separate from data-view storage
unsafety; rejecting unrelated storage constraints would wrongly reject native
nodes. An empty eligible catalogue emits an explicit runtime refusal, not an
unused-switch-variable Go build error. Structural/JSON
function properties, arbitrary callable property storage and missing optional
methods are not implemented by guessing. Optional primitive results retain
presence; reference results retain identity. Erased void slots use an existing
value-preserving bridge when available; otherwise that result ABI refuses rather
than fabricating undefined. The
[`native_void_bridge_test.go`](../tsfront/native_void_bridge_test.go) oracle
executes a real generic/native constructor-result bridge and proves that a void-
typed callback may actually return **7** on JS, requiring refusal without such a
bridge. No result-field/config layout or enumerable-key adapter is widened.
Effects conservatively read/write
global state, allocate, may raise and may alias the receiver. ABAP explicitly
refuses the new operation.

A second reached boundary was not dispatch at all: union-field lowering treated
**`constructor` as a data slot** on the `get()` result union. Native/method-
interface constructor metadata now reaches the existing descriptor operation,
not a synthesized data-field getter. Structural records and own-field shadow
cases keep their existing boundary. The proof consults the already-populated
**shape registry**, not the Structural flag finalized after body lowering.
[`union_constructor_refusal_test.go`](../tsfront/union_constructor_refusal_test.go)
certifies the structural-union rejection against Node's actual Object name. Batch M's stable descriptors supply names
and constructor identity unchanged.

[`erased_method_test.go`](../tsfront/erased_method_test.go) executes covariant
class/interface results, concrete constructor names, callback/receiver effects,
recursive erased optional calls and native children against original Node. A
JSON own `get` string raises TypeError on Node and explicitly refuses on Go;
no function-property adapter is invented. Dedicated final CDS measurement is
**358/358 Node-active passes, FAIL 0, zero traps**, with its one original pending
test still pending.

## 2. Help/DumpScope: JS conversion semantics and the reached follow-up

The base reaches `DumpScope.traverseSpaghetti`, **dump_scope.ts:31–32**:
`node.getData().types.length` and `.vars.length` participate in concatenation.
These are **dictionary records, not arrays**. A fresh original-Node
[observation-only wrapper](evidence/traps-n-coercion-node.json), delegating the
original method and running all three unchanged Help bodies (**3 passes, FAIL
0**), proves both records have **no own `length` key**. Both reads are
**undefined**; String and `+` produce **`"undefined"`**. Go's diagnostic names the
static map value type **TypedIdentifier**, not a present runtime instance. The
original native reference read retains nil, which boxing/string conversion now
preserves as undefined. No array length, zero count or empty string is supplied.

For present TypedIdentifier objects, the reviewed class has `toText()` and a
debug-description symbol but no toString/valueOf/Symbol.toPrimitive override:
the correct ordinary-object fallback is `[object Object]`, not `toText()` or
its class name. That bounded general coercion is independently Node-regressed;
it is not falsely described as the value actually reached by those two reads.

Reviewed semantics:

- `String(obj)` and template substitutions use the string hint; ordinary objects
  try `toString` then `valueOf`. String-producing native `toString()` overrides
  dispatch on the actual runtime class, including inherited overrides.
- `+` uses the default hint: ordinary objects try `valueOf` first. The supported
  classes have only Object.prototype.valueOf (returning the object), so their
  string-producing `toString` result is also correct for `+`. **Custom valueOf
  or Symbol.toPrimitive is refused**, not silently treated as a string hint.
- Ordinary reviewed native/structural objects without coercion hooks produce
  `[object Object]`; parsed objects with own coercion-hook keys refuse. Existing
  live views delegate to original backing storage.
- Array conversion joins with commas, recurses into supported elements, and uses
  empty fields for null/undefined/holes. Primitive conversion remains unchanged.
  Cyclic/deeper-than-64 arrays, functions, maps/sets, dynamic Error prototype
  behavior and other unreviewed shapes refuse. The existing primitive-only
  `array.join` operation is not widened.

Go's concrete `nativeString` capability invokes source overrides or certifies
Object.prototype's result. Omitted computed hooks are retained as explicit HIR
coercion-unsafety metadata; the known `Symbol.for("debug.description")` spelling
is not a coercion hook. Own field shadows, coercion accessors and incompatible
method signatures refuse. No fabricated `[object Object]` fallback is used for
an unknown runtime object. Native coercion is enabled by ordinary unpruned
lowering, not a source-site override; pruned default HIR is unchanged.

[`to_primitive_test.go`](../tsfront/to_primitive_test.go) compares native/plain,
interface-erased and inherited-hook values, parsed/nested arrays, null/undefined,
String/template/+ contexts and evaluation counts against Node. Independent Node
probes certify that valueOf changes `+` to `7`, Symbol.toPrimitive observes the
`string` hint, and a noncallable own toString raises TypeError; Go explicitly
refuses those unimplemented domains. Hook effects are conservatively global,
not misclassified as a pure primitive formatter.

The repaired Help path next reaches **help.ts:243**, `node.get().getStr()` under
its runtime constructor-name switch. The same concrete zero-argument dispatcher
handles that actual token, not an invalid nominal cast of INode's broader get
result. The original **`LSP, help first character = DO` passes**. The separate
**`LSP, help class`** body also advances past its DumpScope refusal, but then
reaches the existing **native Identifier JSON own-key order refusal** at
help.ts:93. That is Dell's enumerable-key/storage handoff and **is not credited
as a class-Help pass**. LSP improves **180 → 181**, FAIL 0. Its CDS-Help native
delete and class-data/range/semantic boundaries remain unchanged.

## 3. Three Node-active unregistered titles, re-executed

| Title/row | Reached root on 8288338 | N disposition |
| --- | --- | --- |
| LineLength 269-character `hello … hello` row | Numeric assignment result **400** passed from line_length.ts:17 to testRule; native LineLengthConf storage refuses at src/rules/line_length.ts:35 | Still Dell-owned primitive/config storage. |
| LineLength `WRITE: / 'abc'.` zero-issue row | Same initialization action; a second occurrence of an active title | Still unregistered, not deduplicated or synthesized. |
| `structure combi, complex1 complex1 match` | `_combi.ts:277`: block-local `Normal implements IStructure`, passed to `sub` at 282 | Recovered by bounded lexical-class lowering. |

Assignment expressions already return their RHS, as JS requires. Returning the
config object instead of **400** would miscompile the original test. The two
LineLength registrations remain explicit active non-passes. The four pending
StructureErrors rows remain at their unchanged erased shape-narrowing boundary;
no pending callback is executed or harvested from Node.

`Normal` is **stateless and capture-free**: one ordinary instance getMatcher
method, no fields, constructor, superclass, static members or computed members.
That shape needs fresh lexical constructor identity but no captured factory
frame. HIR's **FreshClassOf** allocates a descriptor once at each declaration
evaluation; the lexical binding and existing callback ABI carry that descriptor.
Its factory attaches the same descriptor to each new concrete instance. `new`,
instance `.constructor`, equality and descriptor-based `instanceof` agree across
repeated evaluations. No global shared-class hoist is used. Analysis marks fresh
identity conservative, and ABAP explicitly refuses lexical descriptors.

[`local_class_test.go`](../tsfront/local_class_test.go) executes repeated factory
evaluations, constructor inequality, instance constructor identity/name,
positive/negative instanceof, direct lexical construction and same-named classes
at distinct lexical sites against Node. Captured outer bindings, self-reference,
fields, explicit constructors, static/computed members and inheritance remain
explicit lowering refusals. Richer captured local-class frames are not claimed.
Structures improve **138 → 139 active passes**, with one original pending skip.

## Genuine prototype mismatch — stopped and reported first

A new refusal-boundary oracle found **Node `"Object"` versus Go `"A"`** for:

```ts
interface A { a: string; }
interface B { b: string; }
function name(x: A | B) { return x.constructor.name; }
name({a: "a"});
```

Work stopped; the mismatch was reported before repairing it. The incomplete
798f986 all-shard attempt was terminated after **eight completed shards**, with
all reached upstream FAIL counts still zero. It is **not a 17-shard measurement
or a certificate**. Root: the constructor-union proof consulted `Class.Structural`
during body lowering, but that flag is assigned only afterward. Property-only
interfaces were consequently mistaken for native classes, exposing a synthetic
shape descriptor's name.

The correction consults the existing `shapeLike` registry and shape-name marker
at that earlier phase. These structural unions again explicitly refuse; no
Object descriptor/prototype view is fabricated. The exact original probe is
retained as a Node-oracle refusal regression, and all positive native descriptor,
erased dispatch, coercion and lexical-class oracles were rerun successfully.

A second independent oracle found **Node TypeError versus Go `"base"`** for an
erased call on a subclass whose getter shadows an inherited method. Getter slots
have synthetic `get.*` HIR names, so a lookup of the inherited virtual method
alone was not a proof of the actual JS property. Work stopped and the mismatch
was reported before repair; the incomplete **7d3a8fc** attempt was also terminated
after **eight completed shards**, all with upstream FAIL 0. The exact
[`native_method_shadow_test.go`](../tsfront/native_method_shadow_test.go) probe
remains. Accessor-bearing source prototype layouts now explicitly refuse, with
separate HIR metadata rather than the broader data-view-unsafety flag. A focused
prototype that reused the broader flag lost four target passes as explicit traps;
it was discarded, never certified. The final proof restores native-node dispatch
while retaining getter/setter and empty-catalogue refusals.

Before any further full run, the function oracle found **Node arrow-function
source versus Go `[object Object]`**. Work stopped and the mismatch was reported
before repair. Synthesized callable-storage classes are not ordinary JS class
instances: their existing callable-interface identities now exclude them from
both native capabilities. The exact
[`function_coercion_refusal_test.go`](../tsfront/function_coercion_refusal_test.go)
probe retains Node's actual function source and requires Go's explicit refusal.
Its additional binding oracle proves that a local class named Symbol can map
`Symbol.for("debug.description")` to a **toString string key**; the harmless
computed-debug exemption therefore also requires the checked global library
Symbol binding, not merely matching source text. Callable constructor unions
remain refused, not granted synthetic native metadata. Existing erased-void
value bridges are used explicitly; an unknown void return is not claimed to be
JS undefined.

Corrected-source gates, same-path default output and Go workloads were repeated.
Only the subsequent **completed** 17-shard run on **469fd1d** is certified. There
are **two interrupted eight-shard attempts and one completed 17-shard run**,
not three full measurements. No mismatch is accepted, and no failed assertion
is reclassified as a trap/pass.

## Stopped regression: pre-existing Node-contract wiring

The optional `tools/abaplint-tests-runtime-test.mjs` gate unexpectedly reported
**4 passes / 2 failures**, versus its expected 5/1. The extra failure was
`suite contain: chai array include intrinsic unavailable`. Work stopped and the
unexpected failure was reported before continuing. The **unchanged 8288338
script reproduces it**, and neither its runtime TS nor its contract source had
been changed by N. Root: its independent Node VM invoked a deliberately
Go-only `chaiArrayInclude` stub with no Node intrinsic installed.

The contract harness now installs **original pinned Chai's** inclusion operation,
returning false only for that package's AssertionError. All original scenarios
and expected messages are retained. A ninth oracle scenario adds strict object
identity, absent/negated inclusion and a genuine absent-inclusion assertion
failure. **All nine pass.** This does not change original abaplint tests or the
Go assertion adapter. No genuine TS-HG@Go/Node value mismatch was accepted, and
all focused shard FAIL counts remained zero. The separate real constructor-union,
getter-shadow and function-coercion mismatches and their stops/corrections are
disclosed above.

Other discarded development failures were explicit refusals/setup errors:
computed-hook diagnostics in a strict probe helper; an unavailable ABAP helper
spelling; a nil prototype declaration-symbol map; an untyped dynamic instanceof
operand; and raw reference-array slots incorrectly expected to be pre-boxed.
Their regressions now execute and pass. The first short gate also correctly
caught the expected effects-catalogue/golden update. A background build lost its
stdout pipe after the contract gate exited; that incomplete build/status was
discarded, then repeated with redirected logs. No failed setup is a certificate.

## Final certificate

[Final machine-readable evidence](evidence/traps-n-final.json) measures frozen
**469fd1d**, **17 shards / 625 files**. Fresh original pinned Node has **11,060
passes, FAIL 0, 32 pending**. Go has **11,027/11,060 active passes (99.70%), FAIL
0**, **31 traps in 20 groups**, zero async-pending.

**33 active non-passes = 31 traps + 2 unregistered + 0 skipped + 0 FAIL +
0 async-pending.** Not-run **34 = 28 original pending skips + six unregistered
(two active LineLength + four pending StructureErrors)**. Registration parity is
therefore **false only for these six missing titles**, not claimed repaired:
11,086 registered versus Node's 11,092 total. Two initialization actions remain.
Original whole-suite and owned title multisets reconcile with those explicit
missing occurrences; exclusions, wiring failures, unmatched executions, borrowed
FAILs and executed pending bodies are all zero.

The directly measured four-shard gain against **8288338** is **five passes**:
CDS +3, Help +1, structures +1; rules unchanged. The final full total is **78
above M's historical 10,949**, but **22 were already present on the actual base**
in the four measured shards. The other **51** lie outside that focused base
measurement and are **not attributed to N** without evidence. No additional
full base run was made for attribution, and no incomplete run is substituted
for the final certificate.

### All 17 shards

| Shard | Files | Pass | FAIL | Trap | Not-run | Registered | Node active + pending |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap` | 18 | 216 | 0 | 0 | 4 | 216 | 216 + 4 |
| `abap/expressions` | 11 | 107 | 0 | 0 | 0 | 107 | 107 + 0 |
| `abap/lexer` | 3 | 200 | 0 | 0 | 0 | 200 | 200 + 0 |
| `abap/nodes` | 1 | 2 | 0 | 0 | 0 | 2 | 2 + 0 |
| `abap/statements` | 286 | 4,962 | 0 | 0 | 0 | 4,962 | 4,962 + 0 |
| `abap/structures` | 30 | 139 | 0 | 0 | 1 | 140 | 139 + 1 |
| `abap/syntax` | 8 | 1,362 | 0 | 1 | 10 | 1,373 | 1,363 + 10 |
| `abap/types` | 6 | 28 | 0 | 1 | 0 | 29 | 29 + 0 |
| `root` | 12 | 91 | 0 | 3 | 0 | 94 | 94 + 0 |
| `cds` | 2 | 358 | 0 | 0 | 1 | 359 | 358 + 1 |
| `ddl` | 1 | 32 | 0 | 0 | 0 | 32 | 32 + 0 |
| `lsp` | 16 | 181 | 0 | 7 | 5 | 193 | 188 + 5 |
| `objects` | 26 | 134 | 0 | 10 | 1 | 145 | 144 + 1 |
| `objects/rename` | 9 | 29 | 0 | 2 | 0 | 31 | 31 + 0 |
| `pretty_printer` | 1 | 30 | 0 | 1 | 0 | 31 | 31 + 0 |
| `rules` | 192 | 3,141 | 0 | 6 | 12 | 3,157 | 3,149 + 10 |
| `utils` | 3 | 15 | 0 | 0 | 0 | 15 | 15 + 0 |
| **Total** | **625** | **11,027** | **0** | **31** | **34** | **11,086** | **11,060 + 32** |

### Complete remaining trap list — 31 bodies, 20 groups

The exact normalized messages and examples are retained below; generic
conversion-domain messages are not mislabeled as a newly diagnosed coercion
failure. Dell's absence/union, nonfinite, enumerable-key, sibling assertion and
config-shape domains remain explicit handoffs.

| Count | Shards | Remaining reached refusal (full example) |
| ---: | --- | --- |
| 3 | rules (1), lsp (1), abap-syntax (1) | `src/abap/5_syntax/structures/class_data.ts:12:3: value outside supported numeric, ordering or conversion domain` |
| 3 | lsp (3) | `src/lsp/rename.ts:104:3: lowering trap: src/lsp/rename.ts:104:3: HIR verification: node 168894 (src/lsp/rename.ts:109:52): assignment type mismatch` |
| 2 | objects (2) | `runtime/chai.ts:26:3: chai object enumerable keys are unsupported` |
| 2 | objects (2) | `src/objects/_abap_object.ts:126:3: src/objects/_abap_object.ts:151:82` |
| 2 | rules (2) | `src/objects/_dynpros.ts:30:1: value outside supported numeric, ordering or conversion domain` |
| 2 | rules (1), objects (1) | `src/objects/message_class.ts:65:3: value outside supported numeric, ordering or conversion domain` |
| 2 | objects-rename (2) | `src/objects/rename/renamer_helper.ts:107:3: value outside supported numeric, ordering or conversion domain` |
| 2 | root (2) | `src/rules/space_before_dot.ts:50:3: value outside supported numeric, ordering or conversion domain` |
| 2 | objects (2) | `test/objects/domain.ts:129:22: value outside supported numeric, ordering or conversion domain` |
| 1 | lsp (1) | `src/lsp/help.ts:93:3: config JSON own-property order is unproven for src/abap/4_file_information/_identifier.ts.Identifier` |
| 1 | lsp (1) | `src/lsp/semantic.ts:42:3: lowering trap: src/lsp/semantic.ts:42:3: HIR verification: node 169279 (src/lsp/semantic.ts:50:23): invalid narrowing classvalue to classref<shape.c3b70776d029edfc69fd75ab61b91f27fe8deaade7fdd314df00aa7ee5f66866>` |
| 1 | objects (1) | `src/objects/domain.ts:100:3: input is outside the supported strict JSON subset` |
| 1 | rules (1) | `src/objects/number_range.ts:36:3: src/objects/number_range.ts:51:33: parseFloat requires a finite result` |
| 1 | objects (1) | `src/objects/proxy_object.ts:70:3: lowering trap: src/objects/proxy_object.ts:70:3: HIR verification: node 176593 (src/objects/proxy_object.ts:81:5): assignment type mismatch` |
| 1 | rules (1) | `src/rules/line_only_punc.ts:46:3: value outside supported numeric, ordering or conversion domain` |
| 1 | abap-types (1) | `test/abap/types/types.ts:10:1: value outside supported numeric, ordering or conversion domain` |
| 1 | objects (1) | `test/objects/view.ts:97:43: value outside supported numeric, ordering or conversion domain` |
| 1 | lsp (1) | `src/lsp/help.ts:39:3: lowering trap: unsupported-expr src/lsp/help.ts:46:5: delete supports ordinary lowered records only` |
| 1 | pretty_printer (1) | `test/pretty_printer/pretty_printer.ts:179:31: lowering trap: unsupported-expr test/pretty_printer/pretty_printer.ts:183:7: delete supports ordinary lowered records only` |
| 1 | root (1) | `test/registry.ts:452:62: lowering trap: unsupported-expr test/registry.ts:460:5: delete supports ordinary lowered records only` |

### Gates, golden, output and workload parity

[Compact gate certificate](evidence/traps-n-gates.json), repeated on **469fd1d**:

```sh
GOCACHE=$HOME/.cache/go-build GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false
ABAPITI_GRACECHECK=1 GOMAXPROCS=4
nice -n 10 go test ./... -short -timeout 40m
nice -n 10 go test ./tsfront/... ./hir/... ./cmd/... -timeout 40m
nice -n 10 go vet ./...
node tools/abaplint-tests-report-test.mjs
node tools/abaplint-tests-all-test.mjs
node tools/abaplint-tests-runtime-test.mjs
```

All exit **0**. Lexer facts regenerated equal the updated golden. The only golden
changes are catalogue facts: aliases/allocates/may_raise/writes **133→134**,
reads **201→205**, conservative entries **21→23**. One `dynamic.call0` operation
and conservative global coercion/native-hook effects account for them. Lexer
analysis, method facts and all **61 Grace call sites** are unchanged.

Default output from clean **8288338** and frozen **469fd1d**, regenerated at the
**same absolute output path**, has **raw `diff -r` exit 0 / zero bytes / 6,312
files**. There are **no default-output changes to justify**. The source-tree
identities in the certificate verify that documentation does not mutate the
measured compiler.

Fresh emitted-Go generation/build exits **0**. Original pinned Node, Go and the
v0.1.1 kit are byte-identical on zabapgit **clean 0 / seeded 5**; final Go timings
are **8.558s / 8.899s**. The kit ZIP SHA-256 is
`371ef1aa8765b5d6b04d3a78f668195b189cc3c7ba2c79428b155b128592655bf`.
No genuine mismatch is accepted. Generated output trees, Go binaries, temporary
upstream closures, interrupted-run artifacts, ZIP/kit, facts and logs are removed;
only compact evidence and implementation/regression source remain. No push,
transpiler clone, source-site workaround or weakened/deleted original test.
