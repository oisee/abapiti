# Architecture decision records

One file per decision: context, decision, consequences. A record is never
rewritten after it is accepted; a later record supersedes it and says so.

| # | Decision | Date |
|---|---|---|
| [0001](0001-own-object-hir.md) | The TypeScript track uses its own typed object HIR, not MinZ's | 2026-10-07 |
| [0002](0002-tsgo-front-end.md) | The front end is tsgo, vendored into `internal/tsgo` | 2026-10-07 |
| [0003](0003-abap-750-for-hir.md) | The HIR emitter targets ABAP 7.50; the WebAssembly path stays 7.02 style | 2026-10-07 |
| [0004](0004-readable-750-style.md) | Plain, readable 7.50: narrow `FOR`, no `REDUCE`/`LET`/`FILTER` | 2026-10-07 |
| [0005](0005-number-semantics.md) | `number` stays binary64 unless an integer range is proven | 2026-10-07 |
| [0006](0006-monomorphisation-later.md) | Monomorphisation is an optimisation pass, not the HIR's model | 2026-10-07 |
| [0007](0007-verification-order.md) | Verification: static gate, differential corpus on osgo and OSG-JS, A4H after every phase | 2026-10-07 |
| [0008](0008-number-ranges.md) | Proven integer storage (I32/I64) during TS lowering; division and unproven values stay binary64 | 2026-10-08 |
| [0009](0009-reachability-pruning.md) | Proposed: source-pinned workload body pruning; declaration pruning needs a reference graph | 2026-10-08 |
