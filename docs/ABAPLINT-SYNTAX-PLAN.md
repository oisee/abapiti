# Phase 3b: syntax closure and acceptance plan

Measured 2026-10-08. Translation/oracle source pin: `577f875ebec44cfaf64841cfe71c8ab8dc32622e` (@abaplint/core 2.120.56). No syntax or rule implementation was added in this phase.

Evidence filenames below are relative to the phase3 handoff directory.

## Scope and measured closure

The executable target is a PROG plus abaplint/deps, with abapGit's `ci/abaplint.json`: v702, its error namespace, global constants/macros, and exactly check_syntax, parser_error, unknown_types, implement_methods, superclass_final and allowed_object_naming. Preserve filenames, object names/types, dependency status, release/language and config defaults. The original standalone input is 5,185,150 bytes. The provided bench-ci loader adds 392 dependency files recursively, including repository metadata; the Registry ignores non-object inputs. Production loading must honor the configured `/src/**/*.*` filter rather than implicitly treating every filesystem entry as an object.

The measurement follows relative import and re-export edges from Registry, Config, MemoryFile and the six rule modules, using the locked upstream TypeScript parser. Lines count physical lines; classes count ClassDeclarations, excluding synthesized HIR shapes. Inventory evidence is `inventory.json`; the complete candidate file list is its `files` property.

| Selection | Files | Source classes | Lines |
| --- | ---: | ---: | ---: |
| Full static import closure | 1,538 | 1,728 | 81,976 |
| Candidate retaining only the six rule modules and rule support modules | 1,347 | 1,343 | 56,155 |
| Candidate files newly selected relative to phases 1–3 | 605 | 585 | 36,745 |

The six-rule candidate is a planning bound, not a compilable reduced closure: ArtifactsRules still references the discarded rule classes. Reachability analysis or a fingerprinted closed rule registry must precede that reduction. The full static closure is the honest starting point for unmodified module imports. Neither count proves that all methods in a selected module execute on this PROG.

The candidate includes 225 files under `abap/5_syntax`, 64 under `abap/types`, six under `abap/4_file_information`, and 184 under `objects`. It also drags in 49 CDS, 17 DDL, 19 LSP and four pretty-printer files via broad object/export registries. The syntax closure therefore includes more than 5_syntax itself. These unrelated paths are candidates for explicit exclusion with traps, never fabricated successful returns.

Registry builds object indexes by uppercased name/type, marks dependencies separately, parses ABAP objects/includes/macros, and supplies global definitions and DDIC/MSAG/macro references. Program, Class and Interface must be supported for this actual dependency set; retain the dependency loader's DDIC object handlers as required by files, not by guessing from PROG alone. DDIC resolves names, domains/data elements/table and table-type components into BasicTypes and typed identifiers. File-information analysis records classes, interfaces, methods, inheritance and implementations. SyntaxLogic builds and traverses its scope graph, resolves local/global identifiers, expressions, statements and OO references, and caches the result consumed by check_syntax and unknown_types. ImplementMethods and SuperclassFinal also use file information and related include/dependency lookup; AllowedObjectNaming needs object-specific naming metadata.

## TypeScript and external boundaries

The conservative closure imports `fast-xml-parser`, `json5`, Node `crypto` and `vscode-languageserver-types`. Object XML uses fast-xml-parser through XMLUtils, while Config parses JSON5 and then constructs/defaults rule configuration. Crypto is used by utility hashing; LSP types/factories enter edit/quick-fix metadata and broad exports. Do not assume these dependencies are types-only. Provide exact ABAP adapters or exclude unreachable methods with pinned diagnostics. No network retrieval belongs inside the translated checker: pass the config, files and pre-fetched dependency set as explicit inputs.

The new candidate selection contains 115 arrow functions, 658 indexed/dynamic accesses, 73 spread elements, seven Map constructions, 29 Set constructions, ten dynamic RegExp constructions, four object spreads, eight sorts, 25 maps, 16 filters, 43 finds, 18 some calls, four every calls, three findIndex calls and one await expression. These are AST occurrences, not all proven runtime hot paths. The await path is Registry's optional asynchronous API; the benchmark uses synchronous parse/findIssues.

Generic declarations are already present in the prior closure (AbstractNode<T>); the measured additional selection introduces no new generic declaration syntax. The harder new work is recursive/aliased semantic type graphs, larger unions, typed identifier/scope/reference interfaces, shared caches, callback capture/escape, callback-driven sort/search, nested records and XML's dynamic shapes. Preserve object identity and mutation: copying a structural view into a new object can corrupt scope caches. Map/Set need ordered iteration, object-key identity, deletion/update semantics and exact missing-value behavior. Record indexes can produce undefined even where TypeScript annotates a total index signature. Short-circuit value expressions must return the selected operand and run its side effects once.

## Full-check CPU profile

`node --cpu-prof` runs the supplied `zabapgit/bench-ci.mjs` against a freshly rebuilt, verified original pin, using a temporary package shim pointing at that build. Config and inputs are the provided abapGit benchmark files. Node v26.9.0; core 2.120.56; 392 dependency files; zero issues. Parse: 10,861 ms; findIssues: 1,329 ms; total: 12,190 ms; RSS: 1,201 MB. Profile: `profile/pinned.cpuprofile`; derived self-time table: `profile/pinned-top20.json`. Self time sums sampled timeDeltas, approximately 1 ms sampling; source locations below refer to compiled JS.

| Function | File:line | Self ms |
| --- | --- | ---: |
| SubStatement.run | abap/3_structures/structures/_combi.js:283 | 7168.569 |
| (garbage collector) | (V8):0 | 1480.295 |
| Sequence.run | abap/2_statements/combi.js:517 | 344.425 |
| Expression.run | abap/2_statements/combi.js:594 | 191.410 |
| AlternativePriority.run | abap/2_statements/combi.js:808 | 165.651 |
| ExpressionNode.findFirstExpression | abap/nodes/expression_node.js:259 | 123.238 |
| wrapSafe | node:internal/modules/cjs/loader:1846 | 111.745 |
| Lexer.process | abap/1_lexer/lexer.js:262 | 80.483 |
| Alternative.run | abap/3_structures/structures/_combi.js:97 | 79.788 |
| Star.run | abap/3_structures/structures/_combi.js:180 | 78.504 |
| Word.run | abap/2_statements/combi.js:104 | 59.824 |
| SyntaxLogic.traverse | abap/5_syntax/syntax.js:419 | 52.353 |
| CurrentScope.addNamedIdentifier | abap/5_syntax/_current_scope.js:135 | 51.826 |
| Result.wrapConsumed | abap/2_statements/result.js:30 | 51.372 |
| Lexer.add | abap/1_lexer/lexer.js:61 | 50.243 |
| Regex.run | abap/2_statements/combi.js:71 | 48.631 |
| OptionalPriority.run | abap/2_statements/combi.js:292 | 45.756 |
| Lexer.run | abap/1_lexer/lexer.js:54 | 39.088 |
| StructureNode.findAllStructures | abap/nodes/structure_node.js:201 | 39.067 |
| SyntaxLogic.updateScopeStatement | abap/5_syntax/syntax.js:510 | 37.845 |

SubStatement.run alone accounts for about 7.17 s: the original consumes the head with `statements.splice(1)`, copying almost the entire tail repeatedly. GC is another 1.48 s. Translation performance must measure these allocations on A4H; merely translating more syntax handlers will not fix this cost. Any array/view optimization must preserve splice's input mutation, retained aliases, partial matches, failed alternatives, issue selection and child trees. Scope traversal and named-identifier lookup are the visible syntax hot paths after parsing. The scope/type result cache must prevent the six rules from repeatedly rebuilding the same semantic graph.

## Proposed sub-phases and acceptance

1. **Registry/config/object input boundary.** Lower synchronous Registry, the explicit six-rule config selection, PROG/CLAS/INTF and the actual dependency object handlers. Add XML/JSON5/hash adapters as required. Acceptance: differential object/file inventories, dependency flags, normalized metadata and config; case sensitivity, duplicate names, missing files, malformed XML/config, INCLUDE resolution and dependency-only objects; original and translated lexer/statement/structure dumps for main and all ABAP dependencies.
2. **File information, semantic types and global/DDIC definitions.** Lower class/interface/method information, BasicTypes, TypedIdentifier, DDIC and global definition lookup. Acceptance: canonical graph dumps preserving shared-reference IDs, recursive types, inherited attributes/interfaces, method signatures, XML-defined dependency types, unknown types and namespace behavior. Include deliberately missing and cyclic definitions; diagnose every unlowered feature.
3. **Scope and reference machinery.** Lower CurrentScope, spaghetti scope, variable/type/OO reference resolution and cache lifecycle. Complete callback and collection semantics required by actual source. Acceptance: differential scopes/reference targets, captures with mutation, ordered Map/Set iteration and object keys, missing map values, nested classes/forms/methods, macro-expanded/include positions, cache invalidation after editing input, and repeated checks returning the same issues without duplicate work.
4. **Expression and statement syntax handlers.** Expand all reachable 5_syntax handlers through the normal tsgo → HIR → ABAP pipeline. Zero blocking lowering/HIR diagnostics and stale-override failures remain mandatory. Acceptance: existing upstream syntax tests, v702 release checks, focused invalid typing/OO/SQL cases, exact issue key/severity/filename/start/end/message, mutation of a resolution/compatibility branch, newest-core ABAP syntax check and both pinned runtimes. Profile each stage and budget memory/allocations before the large run.
5. **Phase 4 rule integration and final oracle.** Implement the six requested rules only after the syntax/type/cache infrastructure passes. Acceptance: baseline zero issues, every negative variant below, canonical equality of complete issue sets and deterministic repeated runs. Run the real A4H background report with dependencies/config, require all issue fields equal to Node and total checking time ≤300 seconds. Retain smaller stage timing/checksum reports to localize failures. New runtime gaps get minimal ABAP cases; emitter behavior must remain kernel-valid.

## Negative variants of the actual standalone input

Each variant below was executed independently on the full original source with the same deps/config and freshly compiled original pin. Each produced exactly one issue, of severity Error. Full edits and issue tuples are in `negative-issues.json`. Edits append after the last existing line unless a filename change is specified; these locations depend on the exact benchmark bytes above.

### check_syntax

Append:

```abap
FORM phase3_syntax.
WRITE lv_phase3_missing.
ENDFORM.
```

Node issue: `check_syntax`, `zabapgit_standalone.prog.abap`, 159474:7–159474:24, Error: `"lv_phase3_missing" not found, findTop`.

### unknown_types

Append:

```abap
CLASS lcl_phase3_unknown DEFINITION.
PUBLIC SECTION.
DATA foo TYPE zphase3_missing_type.
ENDCLASS.
CLASS lcl_phase3_unknown IMPLEMENTATION.
ENDCLASS.
```

Node issue: `unknown_types`, `zabapgit_standalone.prog.abap`, 159475:6–159475:9, Error: `Variable "FOO" contains unknown: ZPHASE3_MISSING_TYPE not found, lookup`.

### implement_methods

Append:

```abap
CLASS lcl_phase3_methods DEFINITION.
PUBLIC SECTION.
METHODS missing.
ENDCLASS.
CLASS lcl_phase3_methods IMPLEMENTATION.
ENDCLASS.
```

Node issue: `implement_methods`, `zabapgit_standalone.prog.abap`, 159477:7–159477:25, Error: `Implement method "missing"`.

### superclass_final

Append:

```abap
CLASS lcl_phase3_final DEFINITION FINAL.
ENDCLASS.
CLASS lcl_phase3_child DEFINITION INHERITING FROM lcl_phase3_final.
ENDCLASS.
CLASS lcl_phase3_final IMPLEMENTATION.
ENDCLASS.
CLASS lcl_phase3_child IMPLEMENTATION.
ENDCLASS.
```

Node issue: `superclass_final`, `zabapgit_standalone.prog.abap`, 159475:7–159475:23, Error: `Superclasses cannot be FINAL`.

### parser_error

Append:

```abap
THIS IS NOT AN ABAP STATEMENT.
```

Node issue: `parser_error`, `zabapgit_standalone.prog.abap`, 159473:1–159473:31, Error: `Statement does not exist in the configured ABAP version(or a parser error), "THIS"`.

### allowed_object_naming

Rename the input file to `zabapgit-bad.prog.abap`; retain its contents. This deliberately introduces a disallowed hyphen in the PROG object's name.

Node issue: `allowed_object_naming`, `zabapgit-bad.prog.abap`, 1:1–1:42, Error: `Name not allowed`.
