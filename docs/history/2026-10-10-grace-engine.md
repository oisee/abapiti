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
