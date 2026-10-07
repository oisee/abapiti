# 0006. Monomorphisation is an optimisation pass

Status: accepted, 2026-10-07 (Alice).

## Context

MinZ monomorphises interfaces as its only dispatch mechanism. abaplint needs
real polymorphism (virtual calls, `instanceof`, classes as values), but much of
it can be specialised when the whole program is known.

## Decision

The HIR keeps polymorphic semantics. A later pass specialises where it is
proven: lambdas passed to `map`/`filter`/`some`/`find` inlined into loops,
`Map`/`Set` per key and value type as typed internal tables, generics
instantiated (one generic declaration; 88 explicit and 150 inferred
instantiations), calls devirtualised and leaf classes made `FINAL` from the
class hierarchy, `instanceof` as a class-id or ancestor-table test, object
literals per shape.

## Consequences

- Correctness first; every specialisation is checked by the corpus comparison.
- Lambda inlining is designed together with closures in phase 2.
