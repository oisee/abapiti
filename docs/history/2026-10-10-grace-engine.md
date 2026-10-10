# Grace engine performance

Branch `perf/grace-engine`, starting at `2ed0636`. Measurements use the
production 1,538-file closure and preserve shared HIR pointers. Pass measurements
reuse the capture and alias restoration described in
[the flag gates](2026-10-10-grace-inline-flag.md). Three independent processes,
median wall time and median maximum RSS; pass RSS includes resident input HIR.
Whole build time excludes binary compilation. Heavy processes hold
`flock /tmp/abapiti-heavy.lock`; Go uses `$HOME/.cache/abapiti-go-cache`.
Evidence for this run is in `/tmp/grace-perf`.

| Step | Pass seconds | Pass RSS MiB | Whole build seconds | Build RSS MiB |
|---|---:|---:|---:|---:|
| Baseline (flag report) | 24.234552 | 1305.55 | 35.30 | 1338.45 |
| 1: demanded relations | 6.090076 | 577.47 | 17.08 | 597.47 |

## Step 1

Rewrite selects the transitive dependency closure of its match and guards,
including negated relations, from analysis and supplied rules. The ordinary
`Analyze` API still evaluates its complete analysis. Extraction filters additions
and avoids effect/escape/receiver traversal when only syntax is demanded.

Inline retains these relations: `node`, `dispatch`, `defined`, `static`,
`inline_dispatch`, `inline_arity`, `inline_forbidden`, `inline_candidate`,
`inline_overridden`, `inline_edge`, `inline_path`, `inline_allowed`,
`inline_call`, `inline_arguments`, `inline_parameters`, `inline_abstract`,
`inline_variadic`, `inline_name`, `inline_stmt`, `inline_expr`,
`inline_seq_return`, `inline_uninitialized`, `inline_virtual`, `inline_block`,
`inline_size`, `inline_ancestor`, `inline_decl`, `inline_variant`, `inline_owner`.
No derived analysis relation is required. Generic user guards still pull in
analysis dependencies, including negative ones.

Validation: rewrite tests including reference evaluator and dependency selection;
`go test -short ./...`; pinned 2.13.2 lint gate against origin/main, 0 new issues
and canary 5/5. Three whole Grace builds match every one of the default build's
6,336 regular output files, including the zip, and the emitted stats. Pass stats
remain 1,487 sites / 190 callees. The expensive copied binding maps and relation
joins remain for step 2.

Raw pass seconds / KiB: 6.073294 / 584440, 6.263578 / 591328,
6.090076 / 592788. Raw build seconds / KiB: 17.39 / 611808,
17.08 / 617420, 15.98 / 609140.

## Step 2

The interpreter interns symbols into integer IDs, keys ordinary tuples with fixed
arrays, and uses composite hash indexes selected from rule-body bindings. Wide
user tuples retain exact keys through a packed overflow representation. Each
semi-naive pivot consumes its delta first; subsequent joins prefer bound-column
selectivity and relation cardinality. One reusable binding array and undo stack
replace per-candidate map copies. The initial round evaluates each conjunction
once rather than once per pivot. Minimum proof depths and negation strata remain.

The HIR adapter retains only matched nodes for inline actions, omits paths during
shape walks, avoids constructing paths for nil children, and indexes method names
for variant detection. The unrestricted diagnostic extraction API retains all
its facts. Unused template conversion is skipped under demand selection.

| Step | Pass seconds | Pass RSS MiB | Whole build seconds | Build RSS MiB |
|---|---:|---:|---:|---:|
| 2: indexed interpreter | 0.273982 | 169.17 | 9.87 | 380.69 |

Raw pass seconds / KiB: 0.272817 / 173228, 0.273982 / 173788,
0.277580 / 173004. Raw build seconds / KiB: 10.10 / 390608,
9.87 / 383200, 9.64 / 389824. The target is reached without a rule compiler.
The pass is about 7.7 times the historical hir.Inline median (stretch target 5).

Validation: short suite and pinned lint gate (0 issues, 5/5); synthetic reference
checks include repeated bindings and ten-column tuples; all three builds match
all 6,336 output files and emitted stats. Pass stats are 1,487 / 190.
A preliminary CPU profile before eliminating redundant initial pivots showed
joins, verification, and GC as the remaining costs; final profiling follows the
incremental round implementation.

## Step 3

Multiple rounds of the fixed inline rules retain immutable declarations/hierarchy
and unaffected method proofs. Changed methods are marked by successful actions;
the adapter supplies their transitive reverse-call dependency closure and the
ownership of method/site relations. Only these methods' node and syntax facts
are extracted again. The generic engine invalidates their base and derived rows,
rebuilds affected indexes, and prunes joins as soon as the regional head binding
is known to be unaffected. Shared `inline_overridden` proofs are conservatively
recomputed. Neither the indexed interpreter nor generic region invalidation
imports HIR. The existing adapter entry points are unchanged.

`ABAPITI_GRACE_RECOMPUTE=full` forces full recomputation between rounds. Custom
rule sets retain full recomputation because they can relate arbitrary regions.
The production Inline API still uses one round, so this step adds multi-round
support rather than changing its rewrite policy.

| Step | Pass seconds | Pass RSS MiB | Whole build seconds | Build RSS MiB |
|---|---:|---:|---:|---:|
| 3: incremental rounds | 0.302252 | 170.95 | 10.59 | 377.62 |

Raw pass seconds / KiB: 0.309882 / 175052, 0.301496 / 171768,
0.302252 / 175356. Raw build seconds / KiB: 10.59 / 391628,
10.78 / 380304, 10.48 / 386684. Three full CLI builds again match all 6,336
regular files, including emitted ABAP and the zip, and emitted stats.

Current hir.Inline baseline (same capture, three processes): 0.031429 / 91736,
0.035928 / 91592, 0.032095 / 91688. The final Grace pass is approximately
80 times faster than the original Grace baseline, and 9.4 times this current
hir.Inline median. The time/RSS targets are met; the 5-times stretch is not.
Step 4 is therefore not needed: no generated rule compiler was added, and
all rule execution continues through the interpreter.

On the closure with four rounds allowed (two actually verified), incremental
refresh takes median 0.449842 s / 214.98 MiB versus full recomputation at
0.513026 s / 216.26 MiB. Raw incremental seconds / KiB:
0.443155 / 220244, 0.449842 / 213112, 0.488321 / 220136.
Raw full seconds / KiB: 0.513026 / 223060, 0.521826 / 221156,
0.491697 / 221452. This measurement includes both verified rounds and is separate
from the production single-round pass target.

Tests compare per-round relations, HIR and stats for a fixture which becomes
eligible in round two; they verify an unrelated proof retains its identity,
negative proofs are invalidated, and cyclic reverse dependencies terminate.
All 32 seeded graphs compare full/incremental HIR and stats. The full closure
also compares the two paths: both terminate after two rounds with 1,487 sites.
Short tests and pinned lint gate pass (0 issues; 5/5).

Final CPU profile (a short 0.304-second sample, cumulative costs overlap):
verification about 100 ms, fact insertion and structural walking about 70 ms
each, native inline facts about 60 ms, evaluation about 50 ms, and background GC
about 70 ms. Verification, adapter walks/fact storage, and GC are the remaining
hot spots; relation joins are no longer dominant. Profile and raw logs are in
`/tmp/grace-perf`.

Final gate: `ABAPITI_GRACE_FULL=1 go test ./tsfront -run
'^TestGraceFullRegistryClosure$' -count=1 -v -timeout=60m` passed in 188.97 s
under the heavy lock. Analysis and inline reference equality, reevaluation,
phased preparation, positive monotonicity, declaration determinism, depth/growth
budgets, verified termination, idempotence, and incremental/full equality are
all green. Logs: `/tmp/grace-perf/full-final-test.log`.
