# Proposals for abaplint upstream

A living list of small changes to abaplint's TypeScript that would let it map more directly onto abapiti's typed IR (HIR), translate to simpler ABAP, and run faster on every host (Node too). Each entry says what to change, why, and what we observed. Nothing here is required: abapiti compiles abaplint as it is. These are suggestions for Lars; the first batch went to him on 2026-10-11 as one comment on his morph2 PR ([#4402](https://github.com/abaplint/abaplint/pull/4402#issuecomment-6109734632)).

Status: **idea** (here only), **issue** (filed, link), **PR** (draft PR, link), **done** (merged upstream).

Add new entries at the end and keep them short. Builders and reviewers working on the fit-gap should append what they run into.

## Performance

### P1. `splice(1)` copies the tail of the array on every call — **done** ([#4399](https://github.com/abaplint/abaplint/issues/4399), merged upstream as [#4400](https://github.com/abaplint/abaplint/pull/4400))
Structure matching shifted the token array with `splice`, which made that stage quadratic. Fixed upstream; abapiti's runtime hand-over (v0.2) is no longer needed once the pin moves past it.

### P2. Combinators allocate a fresh result array per call
`Sequence.run`, `Expression.run`, `Token.run`, `Word.run` etc. start with `const result: Result[] = []` and `Sequence` wraps its input as `[input]`. A site profile of zabapgit shows ~49 M allocations, mostly these arrays. Most calls match nothing and return the empty array. Returning a shared frozen empty array when nothing matched, and passing a single input without wrapping it, would cut allocations a lot on every host, Node included. Needs care: callers must not mutate the returned array (true today as far as we checked).

## Shape and types (makes HIR exact, removes refusals)

### S1. Same method, different arity across union members: `parseType`
`Domain.parseType(reg, dataElement?, description?)` takes three parameters, while `Table.parseType(reg)` and `DataElement.parseType(reg)` take one; `FindGlobalDefinitions` calls `o.parseType(this.reg)` through the union. TypeScript accepts it, but an object IR needs one ABI per method name. abapiti now widens the union ABI, which took a fixed-point pass and blocked ~6,000 tests until then. Giving all `parseType` implementations the same optional parameters (or one shared interface method) would make it a plain virtual call.

### S2. `Config.getDefault()` round-trips through JSON
`getDefault()` builds an object literal and returns `new Config(JSON.stringify(config))`, and the constructor parses it again. A JSON round trip has to be modelled exactly (key order, `undefined` dropping, number formatting), which blocked ~6,000 tests until abapiti implemented a V8-exact version. A typed constructor taking the configuration object (with a deep copy where needed) would be simpler and faster on Node too.

### S3. Object rest to drop a flag: `const {derivedFromConstant, ...rest} = data`
`InlineData.stripDerivedFromConstant` removes one field with object rest. That is fine JavaScript, but it is the construct where abapiti had a silent miscompile (now fixed). Constructing the new type data explicitly, e.g. `{...data, derivedFromConstant: false}` or a small `withoutDerivedFromConstant()` helper, states the intent and maps 1:1 onto a typed copy.

### S4. Rules build `RegExp` objects from configuration strings
14 rules build `RegExp` from their configured patterns on every check (23 `new RegExp(` sites in 18 rule files; some use the global errorNamespace or no configuration). Dynamic regular expressions can't be translated ahead of time; abapiti has to translate them at runtime and refuse the parts that ABAP regex can't express. Compiling each configured pattern once in `setConfig` and keeping it would at least keep the dynamic part in one place per rule and make it cheaper on Node.

## Correctness-neutral style (smaller and clearer generated code)

### C1. Rule idea: inline a single-use CAST — **issue** [#4423](https://github.com/abaplint/abaplint/issues/4423)
Not about abaplint's own TypeScript but a new rule with a quick fix that would clean up generated ABAP like ours (and hand-written code).

## How to add an entry

Write: the site (file and function, upstream commit 577f875e), what to change, why (HIR / ABAP / speed), and the evidence (numbers, tests unlocked, a profile). Link the abapiti PR or doc where we hit it.
