# Transitive settledness

The TS→HIR producer proof now follows source calls and virtual dispatch, including
implementations loaded from compiled tests. Each `Lower` creates a new proof
graph and dispatch inventory. Promise annotations and interface declarations
remain insufficient evidence.

A settled-returning body must certify every await and every promise adoption.
Its source calls must reach no pending source. Synchronous helpers and invoked
callback arguments participate in that check, so a timer hidden behind a void
helper invalidates its async caller. Promise constructors, pending Promise
operations, unproven promise producers, timers, and arbitrary thenables refuse
certification. External non-promise intrinsics retain the compiler's existing
synchronous ABI; this proof does not add external runtime implementations.

For a virtual/interface call, the inventory includes concrete classes, inherited
methods, structural implementations, static providers and existing object
providers in the loaded Program. Every dispatched body must be translated and
settled-returning. Provider-returning interface/abstract calls are resolved to their concrete
source implementations too; a bodyless signature alone does not imply an
external factory. Inventory queries are separated from world audits to avoid
recursive audit expansion. A failing implementation is named with its relative source
path. Provider mutation, unsafe casts/erasure, and external providers invalidate
the proof. Nullable types are reduced before the audit, and `never`, absent
values and types without the dispatched method are excluded: assignability
alone would mistake `never` for a provider. Transitive proof notes use the
producer's source file rather than the caller's file. The specialized optional `IProgress.tick` proof remains authoritative,
including its recursive checks of provider carriers and input erasure. Generic
dispatch cannot replace a refusal from that stronger proof.

Immutable aliases of `new X()` pin the receiver's dynamic class. A fluent method
preserves that class when its source returns `this` on every accepted return,
with no ordinary fallthrough. A same-class annotation alone cannot establish
identity: it can return a different subclass. Pinned dispatch still audits
provider mutation and erasure. Object lowering, builtins, and the void ABI are
unchanged.

## Cycles

The call graph uses the **greatest fixpoint**. Each body starts provisionally
safe. Every dependency is checked; a pending or unprovable leaf turns false and
invalidates all predecessors, including members of an already visited strongly
connected component. A previously visited producer can therefore become unsafe
before the root query completes. Circular value aliases remain unprovable.
Separate graph nodes track settled returns and the absence of pending sources
inside synchronous helpers.

This proves that no pending source is reachable, rather than proving termination.
A recursive computation can still diverge synchronously, just as a synchronous
loop can. Recursion with a reachable pending leaf is refused. Existing restrictions
on unawaited producers with await sites still prevent observable microtask races;
no scheduler or microtask queue is introduced.

## Validation and measurements

The focused probes execute interface dispatch, transitive awaits, recursion and
fluent receivers on TS-HG@Go. Refusal probes cover one pending implementer,
pending SCC invalidation of an earlier member, missing explicit implementers,
inherited pending initializers, hidden pending helpers, mutation,
erasure, and loaded-but-untranslated implementations. The complete existing
progress probes are also retained.

Commands, exact blocking-message histograms, full-closure counts, harness counts,
and gate outcomes are recorded in `fit-settled-transitive.json`. The harness
combination is local only, based on `origin/feat/harness-async` at `29d913e`.
The published branch is based exclusively on `origin/main` at `d01e6e8`.

The full rules shard has the following reconciled results (3,159 original Mocha
titles, matched by title and occurrence ordinal):

| Result | Before | After |
| --- | ---: | ---: |
| Passed | 5 | 5 |
| Failed | 0 | 0 |
| Trapped | 1,610 | 2,744 |
| Async-pending | 1,134 | 0 |
| Not run | 410 | 410 |
| Registered | 2,759 | 2,759 |

All **1,058 targeted async-pending callbacks now reach traps**, with none lost
from registration. Among the original 1,382 async candidates, the other 308
remain trapped and 16 remain unregistered. The broader shard also drains 76
additional pending callbacks. This removes the settlement barrier; it does not
make the trapped tests pass. The largest remaining traps are `src/config.ts:69`
(1,348), the `forEach` dense/stable receiver proof (1,069), and downport
`JSON`/`conf702` lowering (296). The 410 not-run titles comprise 400 unregistered
and 10 skipped/focused tests, unchanged before and after.

Full-core `abaplint --full` blocking stays **372 → 372** (154 roots and 218
cascades), with **0 → 0 async blockers** and 291 HIR verification errors. All
84 exact blocking-message counts are unchanged. These remaining compiler
categories belong to other workstreams.

The focused execution/refusal probes, `go vet`, and all requested short tests
passed with Node on PATH and Go parallelism capped at two. Main and branch
`abaplint --target all,go` default outputs have an empty recursive diff,
including the generated Go executable. No harness combination was pushed.
