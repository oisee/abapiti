The initial probe projects are copied from critic-r1 verbatim (input.ts and
its tsconfig.json); generated output and critic runtime logs stay in scratchpad.
Additional probes exercise the fixes, including a const-compatible version of
the critic's pre-super module-counter probe (`super_preserved`).

`TestCriticR1Diagnostics` requires blocking file:line:col diagnostics.
`TestCriticR1Accepted` verifies and emits independent ABAP runtime assertions.
`.github/ci/lexer-unit.mjs` runs them on the clean pin and checks the 44-case
lexer corpus, including per-case token counts and teardown completion.

`i32-contract/` contains the raw ABAP overflow experiment. All four checks
(assignment, arithmetic into a field, return, comparison) trap on pinned osgo;
all four fail to trap on pinned OSG-JS. This rules out i32 as an enforced
bounded domain; Number/binary64 is used instead. The `overflow` and
`number_semantics` probes require 2147483648 to survive arithmetic, field
storage, return and comparison on both runtimes.

`unicode_upper` covers full BMP uppercase expansions. `supplementary` requires
length 2 and the exact JavaScript surrogate unit for `"😀"`; the pinned rune-based
osgo has an explicitly expected assertion failure (runtime gap 026), while
OSG-JS must pass. The
lexer harness checks virtual positions separately from the 44-case corpus.
`constructor_timing` checks static declaration order and one-time initialization
on both local runtimes. SAP kernel constructor timing remains unverified.

`unicode_codes` and `unicode_trim` check dynamic UTF-16 decoding independently
of dump equality, avoiding the pinned library's high-byte * 255 bug.
