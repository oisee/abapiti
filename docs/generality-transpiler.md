# Generality probe: @abaplint/transpiler

| Transpiler-only measurement | Count |
|---|---:|
| Source files lowered | **322** |
| Blocking diagnostics | **139** |
| Root / cascade | **95 / 44** |
| HIR verification errors with transpiler locations | **341** |
| Files with blockers / blockers or HIR errors | **35 / 138** |
| Checker errors / observed version skew | **0 / 0** |
| Distinct normalized root messages | **29** |

The supplied transpiler-alone baseline was **6,116 blockers**. Resolving and
lowering core removes the dominant missing-declaration noise; it does **not**
prove successful translation. The combined HIR is still invalid.

Evidence: [summary, provenance and HIR histogram](evidence/generality-transpiler.json)
and [all 139 transpiler blockers](evidence/generality-transpiler-blocking.json)
(about 40 KB total; no source dump or core diagnostic list).

## Blockers by category

Counts include cascades, matching `abapiti abaplint --full` semantics.

| Category | Count | Category | Count |
|---|---:|---|---:|
| unsupported-expr | 56 | unsupported-regex | 35 |
| unsupported-call | 25 | unsupported-type | 5 |
| unsupported-assignment | 4 | unsupported-statement | 3 |
| unsupported-array | 2 | unsupported-new | 2 |
| unsupported-shortcircuit | 2 | assume-integer | 1 |
| skipped-cast-view | 1 | unsupported-conditional-join | 1 |
| unsupported-number | 1 | unsupported-static-init | 1 |

## Top normalized roots

All **29** groups are shown (there is no 30th); their counts sum to **95**.
Examples below are relative to `transpiler/src/`.

Classification: **a** = TS/JS language lowering; **b** = library/API boundary,
including ECMAScript built-ins, Node and npm; **c** = version skew;
**d** = abaplint-specific override assumption. Group classifications follow the
causal boundary, not merely the diagnostic wording. The evidence JSON gives
an explanation for every group. Counts: **a 23, b 72, c 0, d 0**.

| # | Count | Class | Normalized message | Example |
|---:|---:|:---:|---|---|
| 1 | 35 | b | `replace requires a literal replacement without dollar substitutions` | `db/populate_tables.ts:199:31` |
| 2 | 12 | b | `identifier X is not a lowered local or module value` | `chunk.ts:205:32` |
| 3 | 4 | b | `assignment to X is not lowered` | `expressions/sql_cond.ts:225:9` |
| 4 | 4 | b | `method X on string is not lowered` | `db/populate_tables.ts:37:61` |
| 5 | 4 | b | `method X on void is not lowered` | `traversal.ts:109:9` |
| 6 | 4 | a | `push spread needs a matching array (array<classref<X>> <- array<classref<X>>)` | `structures/class_implementation.ts:48:5` |
| 7 | 3 | a | `unresolved static method X` | `statements/data.ts:57:22` |
| 8 | 2 | a | `KindTypeAliasDeclaration is not lowered` | `expressions/corresponding_body.ts:21:5` |
| 9 | 2 | a | `call to KindIdentifier is not lowered` | `expressions/type_name_or_infer.ts:45:16` |
| 10 | 2 | b | `checker type X is not a lowered declaration` | `chunk.ts:204:11` |
| 11 | 2 | a | `method X of transpiler/src/traversal.ts.Traversal is not static` | `handlers/handle_type_pool.ts:29:26` |
| 12 | 2 | b | `new X is not a lowered class` | `traversal.ts:60:82` |
| 13 | 2 | a | `operands cannot preserve the result type interfaceref<X> (left optional<classref<X>>, right classref<X>)` | `expressions/parameter_s.ts:13:44` |
| 14 | 2 | b | `spread element needs an array` | `initialization.ts:136:13` |
| 15 | 1 | a | `KindInterfaceDeclaration is not lowered` | `expressions/value_body.ts:107:5` |
| 16 | 1 | b | `checker type X IReference[], { plain: Map<string, number[]>; virtual: Map<string, number[]>; }> is not a lowered declaration` | `traversal.ts:19:7` |
| 17 | 1 | a | `compare.getChildren().length / 3: division can produce a fraction; requires a fingerprinted integer exception (sha256=X)` | `statements/read_table.ts:66:21` |
| 18 | 1 | b | `method X on array is not lowered` | `statements/assign.ts:118:21` |
| 19 | 1 | a | `method X on classref is not lowered` | `index.ts:72:40` |
| 20 | 1 | a | `method X on interfaceref is not lowered` | `db/populate_tables.ts:121:27` |
| 21 | 1 | b | `method X on set is not lowered` | `traversal.ts:460:7` |
| 22 | 1 | a | `module initializer is not provably pure and order-independent` | `expressions/type_name_or_infer.ts:17:56` |
| 23 | 1 | a | `no exact conditional join for optional<interfaceref<X>> and optional<classref<X>> (context optional<classref<X>>, inferred optional<classref<X>>)` | `validation.ts:153:18` |
| 24 | 1 | a | `push spread needs a matching array (array<dynamic> <- nil)` | `db/populate_tables.ts:125:11` |
| 25 | 1 | a | `remainder needs a proven nonnegative integral dividend (negative zero must remain binary64)` | `expressions/string_template_source.ts:41:28` |
| 26 | 1 | b | `sort needs an inline or lowered module comparator` | `initialization.ts:136:16` |
| 27 | 1 | b | `type reference WeakMap is not a lowered declaration` | `traversal.ts:61:40` |
| 28 | 1 | b | `type reference WeakSet is not a lowered declaration` | `traversal.ts:60:42` |
| 29 | 1 | b | `view member X: union parameter is skipped` | `chunk.ts:212:18` |

**Main language gaps:** structural record-array coercion; static/instance
same-name method identity (`setValues`); block-local type/interface declarations;
function-valued record/property calls and optional joins; union-valued `||`;
dynamic dispatch through `any`; module initialization of function tables;
and integer division/remainder proofs.

**Main API gaps:** dollar/computed replacement strings (even a lone `$` is
refused), `WeakMap`/`WeakSet`, `source-map`, `padEnd`/`search`, `Number.parseInt`,
`console.dir`, default `sort`, array `toString`, and `Set.forEach`.

Root/cascade is the existing failed-local/identifier heuristic, **not a complete
causal graph**. Groups 2–5 and 14 include consequences of API failures still
labelled root; group 24 follows the dynamic-dispatch failure. Do not interpret
95 as independent missing features. Integer exceptions in group 17 are the
explicit general numeric policy, not an abaplint override mistakenly applied
here. No override targets a transpiler file; no class-d failure was identified.

## HIR and version skew

Of **341** located HIR errors, **255** concern the method-bearing structural
alias `SourceMapTraversal`: 241 `.append` argument mismatches, six
`.ensureStartMapping` mismatches and eight unresolved-class errors. This is a
major **class-a** gap not exposed as a blocking root message. Other errors
include missing expressions (25), fallthroughs (13), non-boolean conditions
(13), and a duplicate `setValues` member. Counts are raw verifier findings,
not deduplicated features; some follow failed lowering.

**Version skew: none observed.** The transpiler requests core **2.120.59**;
the measured source is **2.120.56**, pinned at `577f875e`. The checker returned
no transpiler diagnostics. Neither upstream tree was changed.

## Reproduction and scope

Frontend: `integ/train-13` plus panic fix `ba4f47d`; the new
[`lower --full` command](abaplint-cli.md#other-typescript-projects) shares
`LowerProjectWithOptions`, registry overrides, normalization and verification
with the abaplint full build. No pruning or failure-trap policy was enabled.

Scratch layout (copy, do not edit upstream):

- `src/`: all **1,541** core source files from `~/dev/abaplint/packages/core/src`.
  All **1,538** registry-closure hashes were verified. The three additional
  files are `cds/artifacts.ts`, `ddl/artifacts.ts`, `ddl/ddl_lexer.ts`.
- `transpiler/src/`: **322** files from
  `/mnt/ssd-510/osg/abapiti/transpiler-ee5c2cd1/packages/transpiler/src`.
- `harness/registry_run.ts`: the same registry harness as `abaplint --full`.
- `node_modules/`: pinned registry declaration packages from
  `tsfront/testdata/registrycorpus/node-packages`, plus the existing transpiler
  installation's `source-map`, `@types/node` and `undici-types`.
- `tsconfig.json`: exact configuration is in the evidence JSON. Paths map
  `@abaplint/core` to `./src/index.ts`; ES2022 matches the transpiler project,
  with the registry's strict-null/function and integer policies.

```sh
export GOCACHE=$HOME/.cache/go-build GOTOOLCHAIN=go1.26.0 GOFLAGS=-buildvcs=false
nice -n 10 go build -o .local/generality/abapiti ./cmd/abapiti
(cd .local/generality/project &&
  find src transpiler/src harness -type f -name '*.ts' | LC_ALL=C sort
) > .local/generality/files.txt  # 1,864 tsconfig-relative files
nice -n 10 .local/generality/abapiti lower --full \
  --tsconfig .local/generality/project/tsconfig.json --overrides registry \
  --scope transpiler/src -o .local/generality/result \
  $(< .local/generality/files.txt)
```

The complete run has **159 blockers (106 root / 53 cascade), 405 HIR errors**.
Excluded from the headline: **19 core blockers**, **one dependency blocker**,
**52 core-located HIR errors**, and **12 unlocated HIR errors** (not assigned to
either project). Scope filters diagnostics only, never the lowering closure.
No packages were fetched and no repository was cloned.

Gates **passed**: `nice -n 10 go test ./tsfront/ -short` and
`nice -n 10 go vet ./...`; the CLI scope/summary tests also passed.
The regression was also run without the guard and reproduced the SIGSEGV,
then passed with the guard restored.
