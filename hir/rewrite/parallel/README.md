Grace milestone 5, step 1: conditional parallel-map facts

`Analyze` is read-only and opt-in. `mark_parallel(Loop)` is a fact, never an
executed rewrite action. This package does not participate in `rewrite.Inline`,
`rewrite.Analyze` or the default build. `Source()` exposes the actual rules to
`internal/gracecheck`'s independent naive evaluator.

Inputs are **reviewed effect certificates**, not an automatic HIR ownership
extractor. The existing `confined` relation rejects returns as escapes and cannot
prove a graph returned into a map's private result slot. A contextual certificate
can prove this graph without weakening `escapes`, `pure` or the runtime Effects
table. Certificates must cover all calls, constructor effects, virtual receiver
alternatives, field aliases, runtime effects, and conditional initialization.
`p_complete(M)` attests that coverage. A missing certificate or unknown effect
blocks the proof. Certificates are trusted premises, like an adapter Effects
table; they must be reviewed and tied to exact source bytes. They must not be
created by treating an absent write as evidence of safety.

The input contract is:

- `p_loop(L,M)` selects an iteration entry; `p_method`, `p_complete` identify
  reviewed contextual summaries. `p_call(M,N,Site)` and `p_virtual(M,Site)` /
  `p_target(Site,N)` enumerate every reachable method. Virtual sites also require
  `p_dispatch_complete(Site)`, including inherited implementations and all possible
  receiver classes. Unknown external/runtime operations use `p_unknown(M)`.
  Aggregated summaries may conservatively cover every method of a receiver class.
- `p_write(M,Region,Kind)` enumerates every write, including writes through aliases.
  Kinds are `store`, `append`, `memo`, `counter`. `p_read(M,Region)` enumerates all
  reads of mutable state (including array length and returned counter values).
  A counter's internal read for its exact increment is not an observed read.
  Region identities cover the reachable mutable subgraph, not just its root.
  Shared immutable leaves are separate read-only regions; a returned reference
  to an immutable input does not confer ownership of that input.
- `p_isolated(L)` certifies fresh/quarantined iteration graphs, no inter-iteration
  mutable aliases, no concurrent outside writers, and publication only at join.
  `p_owned(L,Region)` certifies an iteration-private graph or its unique result
  slot. Returning such a graph does not disqualify it; publishing it during the
  iteration does. Objects already visible through a registry cannot be called
  quarantined merely because their array indices differ.
- `p_slot_plan(L)` requires private result slots published in source order.
  `p_ordered_plan(L,Region)` permits only append-only contributions buffered until
  join and concatenated in iteration order. Any iteration reading that shared
  array, even just its length, blocks this exception.
- `p_memo(M,Region,Compute)`, `p_pure(Compute)`, `p_key_complete(M,Region)`,
  `p_value_semantic(M,Region)`, `p_no_invalidation(L,Region)` jointly certify
  deterministic key-to-value computation with all dependencies in the key or
  immutable for the loop, no invalidation/eviction, no visible hit/miss telemetry,
  no observable value identity or mutable cached graph. Every cache reader must
  be marked `p_memo_read(M,Region)`; unrelated readers block the exception.
  The output requires a coherent, thread-safe memo implementation before runtime
  parallelism. The old `memo_store` fact alone supplies none of these guarantees.
- `p_exact_counter(M,Region)` proves a commutative increment (exact integer or
  modular integer arithmetic, no overflow trap), not a floating-point `+=`.
  `p_counter_unobserved(L,Region)` proves no iteration observes intermediate
  totals. The output requires atomic increments. No existing counter is promoted
  just from the old `counter_store` shape.
- `p_init(M,Class)` covers every lazy or EnsureInit guard reachable from M.
  `p_warmed(L,Class)` requires dominance of the actual initializer completion
  before the loop; calling a constructor does not prove nested caches warm.
  `p_warm_plan(L,Class)` instead adds a mandatory precondition. These facts allow
  exclusion of the *guarded initializer body*, never unconditional calls or stores.
  All nested lazy initialization must be listed independently.
- `p_throw(M)` includes adapter MayRaise, explicit throw, trap and transitive
  callees; `p_exception_plan(L)` requires join to rethrow the lowest-index failure.
  `p_visible(M)` records externally visible effects. Any such effect anywhere in
  a throwing iteration blocks proof conservatively; exact before-throw ordering
  is not inferred. Counter updates count as visible. Private slots and ordered
  appends remain unpublished on failure; memo stores are semantically invisible
  under their stronger contract. Timing, I/O and progress callbacks are visible.

All positive and near-miss cases compare full fact sets with both independent
reference modes and reverse insertion order. No source classifier or evaluator
shares the engine's joins. `cmd/grace-parallel` checks the pinned archive hash and
reports the actual audited candidates. The source audit is intentionally distinct
from automatically extracted HIR facts; unresolved ownership remains an analysis
gap for candidates without a complete certificate.
