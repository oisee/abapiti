# Train 12 verification

Production integration: `4594997` (main `f78b973` plus `feat/fit-typeof-guards`, including last-absence and delete-records, and `feat/fit-traps-n`). Test-only merge repair: `f679289`. Original upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`, locked dependencies and TypeScript 6.0.3. All 625 files ran in 17 disjoint shards, with two workers and Go package parallelism at most four.

**11,051 / 11,060 pass, FAIL 0: +19 versus main (11,032).** The Node-active tail is **9 = 7 traps + 2 unregistered + 0 skipped**; async-pending 0. Node has 11,060 active passes and 32 pending cases, 11,092 total. Full-suite and shard title multisets reconcile with no exclusions or infrastructure failures.

Every remaining Node-active title, exact trap message, and registration source site is in [the final tail](train-12-remaining.md). [Shard evidence](evidence/train-12-shards.json) and [tail evidence](evidence/train-12-remaining.json) retain the measured accounting.

| Shard | Pass | FAIL | Trap | Not-run, all | Not-run, active | Node-active |
|---|---:|---:|---:|---:|---:|---:|
| abap | 216 | 0 | 0 | 4 | 0 | 216 |
| abap/expressions | 107 | 0 | 0 | 0 | 0 | 107 |
| abap/lexer | 200 | 0 | 0 | 0 | 0 | 200 |
| abap/nodes | 2 | 0 | 0 | 0 | 0 | 2 |
| abap/statements | 4962 | 0 | 0 | 0 | 0 | 4962 |
| abap/structures | 139 | 0 | 0 | 1 | 0 | 139 |
| abap/syntax | 1363 | 0 | 0 | 10 | 0 | 1363 |
| abap/types | 29 | 0 | 0 | 0 | 0 | 29 |
| root | 94 | 0 | 0 | 0 | 0 | 94 |
| cds | 358 | 0 | 0 | 1 | 0 | 358 |
| ddl | 32 | 0 | 0 | 0 | 0 | 32 |
| lsp | 182 | 0 | 6 | 5 | 0 | 188 |
| objects | 143 | 0 | 1 | 1 | 0 | 144 |
| objects/rename | 31 | 0 | 0 | 0 | 0 | 31 |
| pretty_printer | 31 | 0 | 0 | 0 | 0 | 31 |
| rules | 3147 | 0 | 0 | 12 | 2 | 3149 |
| utils | 15 | 0 | 0 | 0 | 0 | 15 |
| **TOTAL** | **11051** | **0** | **7** | **34** | **2** | **11060** |

All not-run cases include the Node-pending cases: 28 skipped and 4 unregistered. Those 32 cases are outside the 11,060 denominator.

## Merge repair and gates

- `go build -p 4 ./...` and `go vet -p 4 ./...`: pass, including the final test-only repair.
- Both merge commits (`4c87045`, `4594997`) retain exactly the union of their parents’ map keys in Go supported ops, ABAP unsupported ops, runtime specs, and Grace effects. The conservative global effects for native `toString` hooks and `dynamic.call0` remain intact.
- The only repair is the expected refusal message in `TestObjectStringCoercionNodeOracle`: the merged record-hook guard rejects the own-hook case before N’s native coercion path. Its exact message is now expected; the original Node `TypeError` oracle and all positive comparisons remain. The focused test and final `go test -short -p 4 -timeout 40m ./...` pass. No production code changed.
- `ABAPITI_GRACECHECK=1 GRACE_FACTS_OUT=<scratch-file> go test -p 4 -count=1 -run TestLexerFactsReport ./tsfront/`: exported once; byte-identical to the combined checked-in golden, so no golden edit. Compared with main, `op_aliases`, `op_allocates`, `op_may_raise`, and `op_writes` rise **137 → 140** for `dynamic.delete`, `dynamic.typeofValue`, and `dynamic.call0`. `op_reads` rises **205 → 212**: two delete reads, one typeof read, three call reads, and one added global read for `dynamic.toString`. `op_conservative` rises **21 → 23** for native `toString` hooks and `dynamic.call0`. Lexer facts and all 61 call sites remain unchanged.
- `ABAPITI_GRACE_FULL=1 go test -p 4 -count=1 -timeout 55m -run TestGraceFullRegistryClosure -v ./tsfront/`: pass in 366 seconds, including reference/incremental equality, monotonicity, determinism, budgets, termination and idempotence. **1,927 classes and 64 interfaces** remain correct; no expected count update. Zero blocking diagnostics and verification errors.
- Explicit `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`: pass without skip.
- `./.github/ci/lint.sh gate origin/main`: **0 new issues, canary 5/5**, golangci-lint 2.13.2; repeated after the test repair.
- Default `--target all,go` output from main and integration, emitted at the same scratch path: **6,321 files each**. File-level differences: **`go/hir.go`, `go/hir_profile.go`, `go/runtime.go`, `go/zabaplint-go`, `sites.json`**. Each HIR file adds 1,927 `nativeString` methods and changes seven erased-array reads from `castRef[*dynamic]` to `box`, all inherited from N (`798f986`); removing those additions and undoing those seven substitutions makes each file identical to main. Runtime changes are N’s bounded array/object coercion and dynamic class-descriptor unwrapping. The binary reflects the source changes. `sites.json` matches after removing only generated locations. The other **6,316 files**, including ABAP sources and archives, match byte for byte. The expected empty diff was not observed; these are branch feature changes, so no feature behavior was removed to hide them.
- SHA-256-verified v0.1.1 kit (`2e3311517885ed72e651c97004d6cba064fa5093873c75f1104955891e965d7a`): fresh original pinned Node and emitted Go match kit bytes and each other exactly, clean **0 issues**, seeded **5 issues**. `go-target-check.py` passes both variants.

## Top 15 trap groups

Standard report normalization combines coordinates and node IDs; the tail document keeps exact messages. There are 5 normalized groups, so this list contains the first 5.

1. **3** — `src/lsp/rename.ts: HIR verification: assignment type mismatch`
2. **1** — `src/lsp/help.ts: config JSON own-property order is unproven for src/abap/1_lexer/tokens/identifier.ts.Identifier`
3. **1** — `src/lsp/help.ts: config JSON own-property order is unproven for src/abap/4_file_information/_identifier.ts.Identifier`
4. **1** — `src/lsp/semantic.ts: HIR verification: invalid narrowing classvalue to classref<shape.c3b70776d029edfc69fd75ab61b91f27fe8deaade7fdd314df00aa7ee5f66866>`
5. **1** — `src/objects/domain.ts: input is outside the supported strict JSON subset`

## Tail comparison with main

Compared with main’s measured train 11 tail, 19 cases now pass and 0 additional cases are non-passing, for a net gain of 19. Exact messages for all current cases are in the final tail.

Recovered titles:

- **abap/structures** — structure combi, complex1 complex1 match
- **abap/types** — Types, TYPES TYPES structure should sometimes inherit qualified name
- **cds** — CDS Parser define table entity with association/composition wrapped in CDSTableField
- **cds** — CDS Parser define table entity — field annotations sit under CDSTableField
- **cds** — CDS Parser — quoted identifiers FROM-clause alias '"=A0"' token value preserved in CDSAs → CDSName
- **lsp** — LSP, help first character = DO
- **objects** — Domain, parse main xml parser error
- **objects** — Domain, parse main xml parser error, valid xml
- **objects** — View, parse XML database view with 2 append views
- **objects** — malformed object XML ignores malformed inner object metadata
- **objects** — malformed object XML ignores structured values where text is expected
- **pretty_printer** — Remove sequential blanks, no config REPORT zfoo.
- **root** — exclude list will exclude issues based on the local exclude patterns
- **root** — exclude list will not crash with an undefined global exclude list
- **root** — severity an issue will have the severity specified in the config
- **rules** — Message rule no number
- **rules** — Rule: dynpro_checks Element SDFSDF length longer than 132
- **rules** — Rule: dynpro_checks overlapping dynpro fields
- **rules** — test line_only_punc rule "." should have 1 issue(s)

Only a test expectation and verification/progress documentation changed. All commits remain local; nothing was pushed.
