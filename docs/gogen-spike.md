# gogen lowering spike

## Verdict

**Not yet a usable lowering of gogen.** tsfront can load/check the four typed
modules and represent substantial portions of their data interfaces. With
trap-on-failure enabled, the four-file outputs verify and the generated Go
builds. However, the exported executable functions are trapped or omitted,
and many essential IR fields are dropped. Build success here measures the
fail-closed skeleton, not a working generator.

Including the actual JavaScript dependency closure produces verifier-clean
partial HIR and 495 ABAP files, but Go emission fails on a regexp. There is no
reachability manifest, integer assumption, source override, or frontend/backend
feature change in this spike.

Input: read-only `/mnt/ssd-510/osg/abapiti/gogen-pilot`, open-steamgate
`68a6a3012`; roots are `ir.ts`, `symbols.ts`, `emit-constructors.ts`, and
`frontend-find-results.ts` under `tools/gogen/`.

## Reproduction and measurement policy

```sh
bash tools/gogen-spike.sh /mnt/ssd-510/osg/abapiti/gogen-pilot
```

The script retains a new `/tmp/gogen-lowering.*` directory containing copied
sources, npm dependencies, the command binary, and all outputs. It never writes
to the input. It installs `@abaplint/core@2.120.59` and
`@types/node@20.19.0`, disables npm scripts, and uses a scratch npm cache.
Environment: `GOCACHE=$HOME/.cache/go-build`, `GOTOOLCHAIN=go1.26.0`,
`GOFLAGS=-buildvcs=false`.

`cmd/gogen-spike/main.go` uses `tsfront.Load`, checker diagnostics,
`LowerWithOptions(..., TrapOnFailure: true)`, `hir.Verify`,
`golang.EmitPackage(..., "spike")`, and `abap.Emit`. Each emitter is attempted
independently. Go files are built with `go build ./...` in a fresh standalone
module. Using a library package avoids attributing a missing `main` to lowering.
No generated code is edited to make it build. Output directories must be fresh.

The original compiler options and four root includes are preserved, except for
adding `compilerOptions.types = ["node"]` in the scratch config. Without that
addition, tsfront reported TS2591 at `symbols.ts:3:28`, `4:56`, `5:29` (Node
module names not found), and TS2339 at `16:73` (`ImportMeta.dirname` absent),
despite installed Node declarations. With explicit ambient types, **all three
main runs have zero checker diagnostics**. This config-resolution issue is
separate from HIR lowering limitations.

Three main runs distinguish checker dependency loading from runtime lowering:

* **A0, actual JS loaded:** `allowJs` resolves actual `.mjs` imports, but only
  the four `.ts` roots are passed to Lower. This isolates the typed modules;
  their imported runtime implementations are deliberately outside the lowering.
* **A, actual JS lowered:** same config, but all 43 resolved project `.ts`/JS
  source files are passed to Lower, excluding declaration files and
  `node_modules`. This includes the transitive `.mjs` closure, not merely the
  two direct imports. External package declarations inform the checker but are
  not a lowered implementation of abaplint or Node.
* **B, declaration boundaries:** the same four TS files, with only the imported
  `omittedFactoryCall`, `Unsupported`, and `emitGo` names declared in handwritten
  stubs. NodeNext uses `.d.mts`, the ESM declaration-file counterpart of `.d.ts`,
  for the unchanged `.mjs` import specifiers. These are **declarations, not
  runtime overrides**. The stub signatures are included in the script.

Trap mode rewrites blockers into `note-trap-failure` and
`note-dropped-declaration`, repairs signatures, and may trap additional bodies
following HIR verification. Counting only categories not beginning `note-`
would therefore give the misleading answer “zero blockers.” The command also
runs untrapped lowering to preserve original categories and verifier evidence.
That exploratory run panics for the full JS closure; the trap run still
completes and collects its diagnostics. For A, categories in the four-file
evidence were recovered from the trap strings and dropped-declaration notes.

Each run writes complete `result.json`, `hir.txt`, and emitted files. The
checked-in [gogen-spike-results.json](gogen-spike-results.json) retains every
blocking diagnostic for the four requested files, all untrapped verification
errors, source lists, and outcome counts, with scratch prefixes normalized.
The much larger dependency-closure diagnostic stream remains reproducible in
the scratch results rather than being checked into the repository.

## Outcomes

| Measurement | A0: JS loaded, four TS lowered | A: JS closure lowered | B: declaration stubs |
|---|---:|---:|---:|
| Files passed to Lower | 4 | 43 | 4 |
| Checker errors | 0 | 0 | 0 |
| Blocking findings in the four TS files | 72 | 68 | 71 |
| Trap-mode diagnostic records, including ordinary notes | 1,136 | 14,211 | 1,132 |
| HIR classes / interfaces after repair | 65 / 1 | 445 / 4 | 64 / 1 |
| Final HIR verification errors | 0 | 0 | 0 |
| Untrapped HIR verification errors | 27 | run panics | 27 |
| Go emitted files | 3 | 0 | 3 |
| `go build ./...` | passes | not attempted: emission failed | passes |
| ABAP emitted files | 98 | 495 | 97 |
| ABAP emitter error | none | none | none |

A's Go emitter error is at `tools/gogen/frontend-session.mjs:6:25`, node 25846:
`not supported in the Go prototype: JavaScript regexp /=>|->|~|\b(?:TYPES|CONSTANTS|INTERFACES|INHERITING|NEW|CATCH|RAISE|ASSIGN|SELECT|INSERT|DELETE|UPDATE|MODIFY|APPEND|COLLECT|SUBMIT|DESCRIBE)\b|\b(?:CALL\s+FUNCTION|IS\s+(?:NOT\s+)?SUPPLIED|SET\s+HANDLER|GET\s+REFERENCE)\b/i`.
The untrapped full-closure run reports `panic: runtime error: invalid memory
address or nil pointer dereference`; it returns no HIR or diagnostic slice.
This is not a checker failure, nor evidence that only the regexp is missing.

ABAP file counts are files, not supported methods or classes. No ABAP compiler
or execution test was performed. Likewise, Go compilation is not an execution
or differential correctness test.

### Per-file blocking categories

Counts include repeated findings and cascades, not unique root causes.

| File / category | A0 | A | B |
|---|---:|---:|---:|
| `ir.ts`: skipped-interface-member | 31 | 31 | 31 |
| `symbols.ts`: skipped-interface-member | 2 | 2 | 2 |
| `symbols.ts`: unsupported-type | 3 | 3 | 3 |
| `symbols.ts`: unsupported-expr | 8 | 8 | 8 |
| `symbols.ts`: unsupported-call | 6 | 6 | 6 |
| `symbols.ts`: unsupported-new | 2 | 2 | 2 |
| `symbols.ts`: unsupported-array | 2 | 2 | 2 |
| `symbols.ts`: unsupported-closure | 1 | 1 | 1 |
| `emit-constructors.ts`: unsupported-new | 1 | 1 | 1 |
| `emit-constructors.ts`: unsupported-expr | 5 | 2 | 5 |
| `emit-constructors.ts`: unsupported-shortcircuit | 1 | 1 | 1 |
| `emit-constructors.ts`: unsupported-call | 1 | 0 | 1 |
| `emit-constructors.ts`: unsupported-array | 1 | 1 | 0 |
| `frontend-find-results.ts`: skipped-interface-member | 3 | 3 | 3 |
| `frontend-find-results.ts`: unsupported-type | 5 | 5 | 5 |

Examples below use locations relative to `tools/gogen/`; messages are the
original lowering messages, not invented explanations.

* **IR fields:** `ir.ts:7:49`: “data interface tools/gogen/ir.ts.Field:
  member type has unlowered types”; `9:158`: Argument's `value` has unlowered
  types. The 31 dropped members include Type/Expr/Stmt-bearing fields and
  external abaplint types; ordinary names, positions, booleans and many
  container fields survive.
* **Function-valued declarations:** `symbols.ts:8:14`: “checker type
  (s: string) => string is not a lowered declaration” (`typeName`);
  `12:14` similarly rejects `funcName`.
* **Parameterized closure:** `symbols.ts:21:10`: “only parameterless thunks
  are lowered as closures” (the collector callback).
* **Runtime imports:** `symbols.ts:16:12`, `37:3`, `46:3`: “call to
  KindIdentifier is not lowered” (process/filesystem boundaries).
* **Cast operand:** `symbols.ts:44:16`: “identifier emitGo is not a lowered
  local or module value”; `45:18`: “call to KindIdentifier is not lowered”.
* **Containers with initialization:** `symbols.ts:54:46` and
  `emit-constructors.ts:9:19`: “new Map is not a lowered class”;
  `symbols.ts:74:23`: “new Set is not a lowered class”.
* **Assignment/spread:** `symbols.ts:56:26`: “binary operator
  KindQuestionQuestionEqualsToken is not lowered”; `59:3`: “??= needs an
  optional local or this field”; `74:32` and `74:36`: “spread element needs
  an array”. `symbols.ts:94:14` also rejects checker type `Dirent<string>`.
* **Nullable truthiness:** `emit-constructors.ts:17:43`: “mixed tagged operands
  require value-preserving truthiness”. `21:96` in A0/A reports “empty array
  literal outside a statement context”. Several subsequent identifier failures
  are consequences of these earlier failed locals.
* **External node signatures:** `frontend-find-results.ts:5:45`: “type
  reference StructureNode is not a lowered declaration”; `30:40` and
  `30:142` reject StatementNode/ExpressionNode; `30:203`: “type reference
  Extract is not a lowered declaration”. `5:136` rejects Type.
  `71:25`, `82:3`, `83:3` drop `reg`, `XS`, and `S` interface members.

### HIR verification and retained executable code

The **27 untrapped errors** in A0/B include:

* 13 “void type argument” errors, e.g. `ir.ts:262:47` (`structs`),
  `265:3` (`consts`), and `284:108` (`fromP`), including synthetic node-zero
  and constructor duplicates;
* `emit-constructors.ts:16:22`: “field on non-class” and “unresolved field
  params”; `22:9`: ConstructorFactory call-argument type mismatch with void;
  `24:9`: “condition must be bool”; two missing-expression findings;
* invalid body/node/resultNode parameters and missing method bodies at
  `frontend-find-results.ts:5:1` and `30:1`;
* “method may fall through without return” at `symbols.ts:14:1`, `20:1`,
  `42:1`.

Final trap-mode verification has no errors because repair removes invalid
fields/signatures and replaces bodies with traps. In B, `typeName`/`funcName`
are absent, both exported find-results functions are absent, and
`constructorFactories` plus all six exported symbols functions have trapping
bodies. `symbolCollector` also receives a late verification trap. The surprising
`ArithmeticBranches` class used as its result in the dump is not a valid
SymbolCollector ABI. This reinforces that shape synthesis plus repair is not
successful callable-type lowering.

## Requested construct analysis

### `T | null` fields

Null is represented as HIR `Dynamic`, not stripped or treated as undefined.
In the actual IR, `Method.returning` becomes `optional<dynamic>`, as do
`Class.constructor` and `Class.super`: the outer Optional comes from `?`,
and null plus the non-null type becomes Dynamic. `Catch.into: string | null`
becomes `dynamic`. `TableColumn.type: Type | null` is **dropped**, because
Type itself cannot be lowered; null handling does not rescue it.

The control fixture in `cmd/gogen-spike/testdata/probe.ts` confirms
`Cell | null` and `string | null` fields become `dynamic`, while `optional?:
Cell` becomes `optional<classref<...Cell>>`. It emits and builds. Thus nullable
fields are not universally unsupported, but lose the useful non-null static
ABI through boxing. Mixed-value short circuiting in the real constructor leaf
then blocks.

### Discriminated unions `k` / `e` / `s`

The 19/70/107 alternatives are not materialized as a faithful sum-type IR.
Type and Expr are metadata intersections with recursive unions; Stmt is a
large recursive structural union. Their aliases do not become usable nominal
HIR declarations. Interface members and signatures referring to them are
skipped/rejected, including `Extract<Stmt, {s: "find_results"}>` and
`Extract<Type, {k: "struct"}>`.

A nonrecursive two-alternative control (`Tag` with `k: 'i'` or `k: 'c'`)
lowers its signature to the object root with `note-union-root`, then rejects
`x.k` with “property k is not lowered.” Its function is trapped. So the
problem is not merely union size or recursion; common discriminants,
variant payloads, and checker-proven tag narrowing need an ABI.

### Map / Set

Container types themselves work: actual `Program.routineSymbols` is
`optional<map<string,classref<...RoutineSymbol>>>`, `convertedClasses` is
`optional<set<string>>`, and Class.signatures maps to an ordered map.
ReadonlyMap/ReadonlySet use the same ordered container kinds.

The actual `new Map(program.classes.map(...))`, `new Map(Object.entries(...))`,
and nested/spread Set initialization fail, whereas the explicitly typed
empty result map in `constructorFactories` survives. The control's
`new Map<string, Cell>()`, `set`, `new Set<string>()`, `add`, `has`, and returns
lower and build. This is an iterable/tuple/callback initialization gap, not
“Map and Set unsupported.”

### `as unknown as` at emitGo

`symbols.ts:44` is not rejected with an unsupported-assertion diagnostic.
Instead, the source operand `emitGo` is not a lowered callable value, in both
stub and actual-JS runs; invoking the local cast result also fails. Declaration
stubs make the checker happy but cannot supply a runtime callable ABI.

The control `return x as unknown as string` lowers to a Dynamic-to-string
unbox and builds. Double assertion syntax is therefore not intrinsically the
blocker. It also does not establish that casting an imported function to a
five-argument callable is supported. Giving emitGo an honest typed contract
would remove the workaround, but not implement the missing runtime boundary.

The control run has zero checker errors, one original lowering blocker (tag
property access), one raw verification error (fallthrough), zero repaired
verification errors, two Go files that build, and 13 ABAP files without emitter
error. Its cast/map functions remain executable HIR; its tag function traps.

## Highest-value changes

### (i) Changes to gogen TypeScript style / boundaries

1. Type the actual emitGo boundary (or a small explicit adapter) instead of
   relying on `as unknown as`. Define a minimal frontend adapter contract too;
   declarations alone are not executable implementations.
2. Use named `function` declarations for exported helpers instead of const
   arrows. Where practical, replace returned multi-argument callbacks with a
   small object/class with a named method. This avoids current callable-value
   and closure gaps, though it may require changes to callers.
3. Prefer explicitly typed empty Map/Set construction plus simple insertion
   loops over `.map`/Object.entries/tuple/spread construction. Spell out optional
   property initialization instead of property `??=` and split mixed-value
   `||`/`&&` expressions into branches.
4. Isolate Node process/filesystem and abaplint implementation access behind
   small supported host adapters. Avoid passing entire `typeof Expressions` /
   `typeof Statements` namespaces if a few specific operations suffice.
5. If semantically acceptable, consistently use optional fields for absence
   where null is unnecessary; several current fields actually combine `?` and
   `| null` despite the header comment. This can preserve typed Optional values.
   **Do not mechanically replace null with undefined**: gogen may distinguish
   them. Nor should gogen flatten its useful discriminated IR just to fit this
   prototype—the sum types are the principal frontend-support requirement.

### (ii) Changes abapiti should support

1. A real structural discriminated-union ABI, including recursive aliases,
   metadata intersections, shared discriminants, variant narrowing,
   `Extract`, and composed/indexed utility types. This unblocks the IR contract
   before leaf-body polish matters.
2. Typed callable values, imported functions, module const functions, and
   parameterized closures with captures. Validate callback signatures rather
   than synthesizing unrelated data shapes.
3. Nullable typed references/fields and value-preserving truthiness for mixed
   boxed values; broader property `??=` and optional member access.
4. Map/Set iterable constructors with contextual tuple typing, iterators and
   array spreads; retain existing ordered-container semantics.
5. Explicit host/import contracts for Node and abaplint. A declaration file
   should not imply an implemented runtime class, function, or namespace.
6. Harden untrapped full-closure lowering against the observed panic; preserve
   original blocker categories in trap-mode diagnostics; improve unsupported
   signatures so raw HIR does not contain void container arguments or malformed
   bodies. Fix ambient Node type discovery independently.
7. Extend Go regexp handling (or provide a safe refusal representation) for
   the first emitted blocker, then remeasure subsequent backend blockers.
   Successful ABAP emission still needs compilation/runtime validation.

Validation performed: the full reproduction script, `go test
./cmd/gogen-spike`, and shell syntax validation. No input modifications, no
push, and no claim of generated gogen behavior being correct.
