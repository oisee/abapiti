# Trap batch I — conversion-domain ownership and Chai actions

Base: `933e68fc1ebf8d73a73d29c227631b7d6525fd70` (train 6 + H).
Original upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.
No upstream test is changed, deleted, weakened or excluded.

## Focused baseline: exact reached sites, not numeric guesses

A fresh `rules,lsp,root` run reports **3,090 passed, FAIL 0, 339 traps,
17 not-run**. It reproduces 173 Recommendations and 12 RulesRunner refusals
in these three shards (the H whole-suite counts were 195 and 15).
The runtime source locator names the enclosing method declaration, not the
individual expression. Inspection of the actual emitted Go identifies the
conversions below; neither is a numeric/release-comparison failure.

### Recommendations — STOP: Dell's live config/class-view ABI

Reached location: `src/rules/function_module_recommendations.ts:125:3`,
`FunctionModuleRecommendations.runParsed`. The first guard reads
`this.conf.recommendations` through the generated live field reader. Its
physical code is equivalent to:

```go
d := self.viewSource.get(str("recommendations"))
return liveArrayView[any](d, func(d *dynamic) any {
    return castRef[RecommendationsRef](dynRef(d))
})
```

`Config.getDefault` JSON round-trips the rule config; `setConfig` installs that
parsed object through the data-only `FunctionModuleRecommendationsConf` view
(a `BasicRuleConfig` subclass). Its recommendations array contains parsed
ordinary records `{name, replace}` (or `{name, replace, from}`), not nominal
`Recommendations` instances. `liveArrayView` eagerly checks present rows before
publishing the view. The nominal row converter rejects the tagged records;
absent array fields are also outside its current conversion domain. Thus the
first guard can refuse before the FM traversal or the findIndex callback.

Needed: presence/absence-preserving **live row views** over the original config
array, including optional `from`, identity and mutations through aliases, with
BasicRuleConfig inheritance and partial `setConfig` input preserved. Do not copy
rows into new nominal objects or manufacture absent fields/default values.
Dell owns this in train 7 (`feat/fit-conversion-domain`). No repair is made.
`resolveRelease`'s `Release[from]`, its catch, `versionToABAPRelease`,
`releaseAtLeast`, and the default literal are **not the reached first root**;
no speculative enum-read or release rewrite is credited.

### RulesRunner — STOP: erased live configuration array

Reached location: `src/rules_runner.ts:133:3`, `RulesRunner.excludeIssues`.
The concrete boundary is line 150:

```ts
const ruleExclude: string[] =
  this.reg.getConfig().readByKey(key, "exclude") ?? [];
```

The actual emitted conversion is
`castRef[*array[jsString]](dynRef(value))` **after** the nullish fallback.
`Config.readByKey` reads `this.config["rules"][rule][key]`; present parsed
`exclude` arrays are erased/tagged array storage, not native string slices.
Missing/nullish values do use the ordinary native empty fallback, so fixing
`??`, regex construction, or JSON text parsing would not repair this handoff.

Needed: a checked live primitive-array view retaining original storage,
membership, undefined-vs-missing, identity, alias mutation and string element
domain. This is a config-object read through erased storage, explicitly covered
by the STOP rule. No conversion, cloning, cast relaxation or config override is
added here; hand off to Dell.

## Chai — owned harness fix

Reached baseline location: `runtime/chai.ts:46:3`, `Assertion.throw`.
Three Registry duplicate-filename tests pass **value-returning** actions such as
`() => registry.addFile(file2)` or `addDependency(file2)`. The stored value is a
certified `closure.function.*` object with a reference result, not the
`closure.thunk` void interface. Casting it to `() => void` fails before invocation.
The fourth whole-suite site, `malformed object XML ignores malformed DDIC
field metadata`, has value-returning Table/View `parseType(reg)` actions in
`test/objects/malformed_xml.ts:111,117` and asserts `.not.throw()`.

Chai invokes its synchronous action exactly once and ignores its return value.
The shim now passes its original boxed action to `chai.throwMatch`. The Go
harness dispatches only compiler-created closure classes implementing the
existing thunk/callable ABI with zero parameters, calls the original method and
discards its synchronous result. This is not a general erased-callable or
class/config-view adaptation. Arbitrary objects with `call` methods, parameterized
callables, constructors, unknown storage and unsupported callback bodies still
refuse. Value-returning Promise callables are not added; the existing normalized
thunk mode is retained, not a new general async-Chai contract.

The existing operation moves from the single-receiver catalogue to SpecialOps
so its verifier accepts the original thunk or a boxed action. It still requires
one string matcher and a boolean result; ABAP still explicitly refuses it.
Callback effects remain unknown/global, not pure. Existing payload matching is
unchanged: only translated string/native Error payloads match; lowering/runtime
refusals and host faults escape, never count as expected throws.

`tsfront/chai_throw_test.go` retains the original Node oracle, genuine assertion
failure and refused callback. It additionally compares reference, number,
string and boolean results, named/aliased functions, effect counts and substring
matching in both ordinary and normalized thunk modes. New negative rows retain
parameterized-callable and fake-call-object refusals, with original Chai's false
outcomes independently measured on Node. `hir/chai_throw_test.go` checks the
receiver/argument verifier boundary. The existing runtime fault/payload and
unknown-effect regressions remain unchanged.

## Follow-up and certification

The post-fix `rules,lsp,root` run is **3,093 passed, FAIL 0, 336 traps,
17 not-run**: root **71 → 74**, LSP **110 → 110**, rules **2,909 → 2,909**.
Borrowed SQL-value-conversion executions are still checked for FAIL before
ownership subtraction. Registration and missing titles are unchanged.

The new focused top 20 contains **no clearly owned harness/regex/string/JSON
root**. Its newly visible tail is LSP Semantic's ClassValue-to-structural-class
narrowing, not string processing. Other entries are the two delegated config
roots above; original LSP own-key storage; ABAPObject and RemoveDescriptions
Dynamic map keys; Domain/DataElement; Downport's erased getParameters/importing;
primitive find's absence-preserving callback ABI; LSP rename parameter and
optional Location views; PrettyPrinter, SpaceBeforeDot, class_data and dynpros
conversion domains; optional TypedIdentifier; applyUnspecifiedRules live config
assignment; UnusedVariablesConf inherited skipAbstract; and Help's optional
native delete/own-property storage. H's ownership audit already delegates these.
No speculative regex/numeric/config repair is added to inflate the gain.

### Final certificate

Frozen semantic source: **dcd1a1a2b8ed06b67d61ee3e843cbbb079828379**; the final
amendment adds documentation/evidence only. Exactly **one final 17-shard /
625-file run** reports **10,478/11,060 active tests passed (94.74%), FAIL 0,
522 typed/domain traps, zero async-pending and 92 not-run**. Not-run is **0
async/done + 28 skipped/focused + 64 unregistered** (60 active + four Node
pending). Fresh original Node has 11,060 passed, zero failed and 32 pending.
No exclusions, infrastructure/wiring failures, unmatched titles or borrowed
FAILs; complete title multisets reconcile, including duplicates.

The actual gain versus H is **four**: three root duplicate-filename tests and
the objects malformed-DDIC-metadata test. Root is **74**, objects **107**;
every other shard retains its prior pass count. All four former Chai conversion
refusals are gone. Recommendations **195** and RulesRunner **15** remain
unchanged and explicitly delegated. The full top 20 likewise contains no
clearly owned harness/regex/string/JSON root; the newly visible class_data
conversion bucket is a type/storage boundary, not a string or parser repair.

Passed: `go test ./... -short -timeout 40m`, Grace-enabled
`go test ./tsfront/... ./hir/... ./cmd/... -timeout 40m`, `go vet ./...`,
reporter/partition checks and the expanded fresh Chai Node oracle. The lexer
facts export is byte-identical to the committed golden (**no golden edits**),
despite moving the existing Chai operation into polymorphic SpecialOps typing.
Callback effect facts remain unchanged and conservative.

Default `abaplint` output versus an archived **933e68f** compiler, using the
same absolute output path, has **raw `diff -r` exit 0 over all 6,312 files**,
including zip. No output changes need justification. Fresh emitted Go and fresh
original pinned Node are directly byte-identical on zabapgit **clean 0 /
seeded 5**, independently matching the release kit.

See [focused measurements](evidence/traps-i-focused.json),
[all 17 shards, exact missing multisets and final top 20](evidence/traps-i-final.json),
and [gate/output/workload certificate](evidence/traps-i-gates.json).
Two scratch setup errors (output attempted before the archived compiler existed;
a Node scratch import with an extra parent segment) were corrected before
successful gates. Neither produced a comparison or a genuine value/assertion
mismatch. No genuine mismatch was encountered or accepted.

Only compact Markdown/JSON is retained. Generated source closures, Go, binaries,
archived baseline, kit, raw records, facts and logs are deleted. Clean commit,
no transpiler clone and no push.
