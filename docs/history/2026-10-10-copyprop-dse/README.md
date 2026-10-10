# Opt-in Grace copy propagation and dead-store elimination

Implementation through `c5633f7`, based on `0d49b50`. No PR opened.
`ABAPITI_COPYPROP=1` enables both rules before ABAP and Go emission; default off.

The adapter proves one definition/read, identical HIR types, and safe motion in
one statement list. Inert expressions move past inert stores that preserve their
local inputs. Calls, unknown summaries and heap/runtime writes are barriers.
Effectful or raising expressions only move into the immediately following whole
expression. Loop/try/finally crossings and lowered closure captures are rejected.
Changed RHS plans wait for a new verified round; shared mutable syntax is detached.
DSE rejects calls, identity allocation, numeric arithmetic that can raise,
nullable runtime receivers and unreviewed operations. Declarations remain; only
initializers are cleared. Dead assignments become empty blocks.

`store_next` contracts the CFG to the first same-binding read or kill on every
path, including exceptions/finally. Grace derives `not_read_after`. The full
input cross-check covers 1,970 inert candidates against ordinary liveness.
Methods without an inert store or possible immediate whole-expression copy are
skipped. Eligible methods retain their complete CFG. Changed methods refresh
between rounds; unfiltered full recomputation remains an oracle.
Every round verifies HIR within existing budgets. Verification reuses its class
index for inherited constructor lookup.

## Counts

Counts use the pinned 1,538-file closure (1,927 classes, 73 interfaces). ABAP's
baseline includes its existing singleton/inlining pipeline, so target counts differ.

| Stage | Go copy | Go DSE | ABAP copy | ABAP DSE |
| --- | ---: | ---: | ---: | ---: |
| lexer | 0 | 2 | 0 | 2 |
| statements | 101 | 15 | 102 | 16 |
| structures | 6 | 4 | 6 | 6 |
| syntax | 5 | 461 | 6 | 714 |
| rules | 2 | 34 | 2 | 34 |
| other | 2 | 139 | 2 | 189 |
| Total | 116 | 655 | 118 | 961 |

Combi is a subset of statements: Go [1, 6], ABAP [1, 7] (copy, DSE).
Both targets converge in three rounds. See [methods.csv](methods.csv).

Emitted ABAP statement terminators outside strings/comments:
568,019 → 564,916, removing 3,103
(0.546%).
The 58 requested hot methods total 7,258 → 7,228 (30 removed).
Changed methods follow; [hot-statements.csv](hot-statements.csv) includes unchanged rows.

| Method | Before | After | Removed |
| --- | ---: | ---: | ---: |
| Lexer::add | 1639 | 1633 | 6 |
| Expression::run | 157 | 154 | 3 |
| Expression::run_one | 153 | 150 | 3 |
| StarPriority::run | 159 | 156 | 3 |
| StopBefore1::run | 113 | 110 | 3 |
| StopBefore2::run | 207 | 204 | 3 |
| Result::constructor | 48 | 45 | 3 |
| Result::getNodes | 115 | 112 | 3 |
| Result::wrapConsumed | 129 | 126 | 3 |

## Go timing and build cost

The unchanged `tools/go-target-check.py` checks raw bytes against the zabapgit
release kit. All twelve runs are byte-identical: clean zero issues, seeded five.
Both modes use the same paths, caches, four workers and timing instrument.

| Workload | Flag off median (3) | Flag on median (3) | Change |
| --- | ---: | ---: | ---: |
| Clean | 10.404s | 10.745s | +3.28% |
| Seeded | 10.395s | 10.418s | +0.22% |

Samples are in [go-timings.json](go-timings.json). These timings do not
establish a repeatable runtime speedup. Single cached end-to-end builds measured
20.39s off and 20.94s on.
Three independent pass samples give Go median 1.006s,
ABAP median 1.356s. Go cost is
4.93% of the baseline Go build.
See [counts-and-cost.json](counts-and-cost.json).

## Validation

Passed `go vet ./...`, `go test -short ./...` with the flag off and on,
and `./.github/ci/lint.sh gate origin/main` (golangci-lint 2.13.2, zero new
issues, 5/5 canaries). `ABAPITI_GRACE_FULL=1 ABAPITI_COPYPROP=1` full closure
passed in 2,606.73 seconds, including independent comparisons of 435,383,414
and 443,152,814 premises, ordinary/contracted liveness agreement, unfiltered
recomputation, determinism, budgets, termination and idempotence. Executable
flag-on lexer (44/44 and mutation rejection), registry JSON/XML, arrays,
iterators, features and sorts tests passed.

Synthetic tests cover intervening local/alias writes, effect and exception order,
loop duplication, optional conversion, finally paths, closure capture, identity
allocation, unused raising RHS, arithmetic overflow, nullable receivers, shared
syntax, dependent plans, custom action safety, reference agreement, determinism
and idempotence. Four baseline emitter-shape tests explicitly disable this pass.

The repository's pinned OSGO runner was retried with the correct SHA-256 of
`0`, fresh flag-off/on ABAP output, and an 8 GB Node heap. Both attempts
returned `invalid unit result (exit 1)` with zero reported tests. This runner
has not established ABAP equality for the closure; equality is left for
abapiti's A4H/OSGO run. See [osgo-attempt.json](osgo-attempt.json).

[Draft PR description](pr-draft.md); no PR was opened.
