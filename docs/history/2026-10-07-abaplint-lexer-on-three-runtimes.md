# 2026-10-07: The abaplint lexer on three runtimes

The original abaplint lexer, compiled from TypeScript to ABAP and compared
with Node on osgo, OSG-JS and a real SAP kernel. The goal and plan are in
`docs/TS-HIR-PLAN.md` (PR #31) and ADR 0001–0007 (PR #34).

## Input

56 TypeScript files from the oisee/abaplint fork at `577f875`
(`@abaplint/core` 2.120.56): `src/abap/1_lexer/**`, `position.ts`,
`virtual_position.ts` and `files/_ifile.ts`. Copied verbatim, byte-identical
to upstream, under MIT with `LICENSE.abaplint` alongside. No hand edits and
no abaplint-specific generator code: checked for abaplint names in
`tsfront/lower*.go` and `hir/`.

The pipeline is TypeScript → tsgo (`microsoft/typescript-go` at `2bd066d`,
vendored in `internal/tsgo`, PR #35) → abapiti object HIR (`hir/`, PR #33) →
ABAP (`hir/abap`). PR #36 emits 7.50-style ABAP; this run used the
7.02-style phase-1 branch output, with empty `PROTECTED SECTION` and
`PRIVATE SECTION` added by the fix that became PR #40.

## Output

59 lowered classes, one interface, runtime classes and a differential
harness: 113–115 ABAP files, about 8.8K lines of lexer code, 25.6K lines for
the whole set including the embedded oracle. Longest line: 254 characters.

`tools/lexer-oracle.mjs` runs the original lexer in Node and dumps
`Type|str|row|col`. The comparison covers 44 cases: 41 snippets and three
real files (`zwasm_compiler.prog.abap`, `zcl_abapiti_bench_mem.clas.abap`
and abapGit's `zcl_abapgit_http_agent.clas.abap`), totaling 4,663 tokens.

## osgo/OSG-JS

At open-steamgate SHA `ad3d1e87cd3c3545b32dd4ba2ddeceb25708f05f`, clean
osgo (Go runtime) and OSG-JS (JavaScript runtime) each matched Node on all
44/44 cases. Mutating the emitted ABAP to swap a token type or add one to
a column makes the test fail.

## A4H

On SAP kernel 7.58, 2026-10-07, the first activation attempt failed for
105 of 113 classes: non-final global classes require a `PROTECTED SECTION`
or `PRIVATE SECTION`. osgo, OSG-JS and abaplint accept classes without
them; reported as oisee/open-steamgate#649 and abaplint/abaplint#4393.

After adding the sections, object-by-object activation failed on mutual
references. An abapGit zip import with mass activation worked: 114 objects,
ABAP Unit 1/1 PASS, all 44 cases equal to Node. The A4H test on 2026-10-07
took 3.14 s including the comparison; this is not a speed measurement.
The throwaway packages were deleted, with no residue.

## What the kernel taught us

On A4H, 2026-10-07, `strlen` of `😀A` is 3 and `s+2(1)` is `A`: ABAP
counts UTF-16 code units, like JavaScript. OSG-JS at
`ad3d1e87cd3c3545b32dd4ba2ddeceb25708f05f` agrees; osgo at that SHA counts
runes. Case 026 has a patch at open-steamgate.

Critic rounds 1–2 after the first green run found a harness that could pass
after an early `RETURN`; it now checks the completion count. They also
found silent miscompiles in field initializers, `super()` order, parameter
defaults, Optional narrowing, unproven casts, regex anchors, `toString`
radix, top-level statements, static initialization order and lone
surrogates. Each is now exact or produces a blocking diagnostic. Numbers
lower to ABAP `f` (binary64), because OSG-JS does not trap i32 overflow.

## Corpus

The build machine has 454,670 `.abap` files with 17,784 unique contents.
The original abaplint lexer lexed all of them with zero errors: 1.06 billion
tokens, 54 s with content deduplication and 16 workers. This corpus result
is for the original lexer.

## Not yet

Speed comparison is in progress in `tools/allbackends`. Phase 2, the
statement parser (674 files), is also in progress.
