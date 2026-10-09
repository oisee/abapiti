# Grace v2 — rewrite rules over abapiti's HIR (draft spec, dell, 2026-10-09)

Owner: dell (with executors). Customer and reviewer: abapiti. Home: package `hir/rewrite` in oisee/abapiti (branch, not main, until reviewed).

## Purpose

Replace hand-written, fingerprinted overrides and per-backend tricks with declarative facts and rewrites over the typed HIR, so every backend (ABAP, Go, later others) benefits from one rule, and every rule is checked by two independent oracles (ABAP vs Node, Go vs Node).

## Model

Two layers, one small language (S-expressions, like MinZ's Grace):

1. **Facts** (Datalog-like, recursive, tabled to a fixed point). Base facts are extracted from the HIR; derived facts come from rules with a base case and a recursive (tail) case. Evaluation is semi-naive with memoised tables; cycles in the call graph and the CFG terminate because facts only grow and the domain is finite.
2. **Rewrites** (match / where / action). A pattern over HIR nodes and CFG shape, guard predicates over facts, and actions from a small composable set. After every rewrite batch the program is re-verified with `hir.Verify`; tables are invalidated for the rewritten region.

## Base facts (extracted)

- `class(C)`, `extends(C, B)`, `implements(C, I)`, `method(C, M)`, `static(C, M)`, `final(C)`
- `calls(Caller, Callee, Site)`, `virtual_call(Caller, Recv, M, Site)`, `new(Site, C)`
- `reads_field(M, C, F)`, `writes_field(M, C, F)`, `reads_static(M, C, F)`, `writes_static(M, C, F)`
- `throws(M, Site)`, `trap(M)`, `try(M, Region)`
- `local(M, V, Type)`, `assign(M, V, Expr)`, `returns(M, Expr)`, `param(M, I, V)`
- `runtime_op(Site, Op)` with the catalogue's `Mutates` flag

## First derived facts

- `receivers(Site, C)`: the classes a virtual call can reach on the closed world (class hierarchy analysis, then narrowed by `new`/assignment flow).
- `may_throw(M)`: `throws(M, _)`; or `calls(M, N, _), may_throw(N)`; or `virtual_call(M, R, X, S), receivers(S, C), may_throw(C.X)`; runtime ops that may raise.
- `pure(M)`: no field/static writes, no mutating runtime ops on non-local objects, calls only pure methods.
- `escapes(M, V)`: V reaches a field, a static, a return, a throw, or an argument of a call whose parameter escapes (recursive).
- `writes_static_transitive(M, C, F)` (abapiti's caveat 1).
- `memo_store(M, C, F)`: the shape "if not has(k): v = pure(k); set(k, v); return get(k)".
- `lazy_init(C)`: guarded class_constructor or an initialised flag (abapiti's caveat 2).

## Actions (initial set)

`replace(node, expr)`, `inline(site)`, `devirtualise(site, C)`, `hoist(expr, region)`, `scalar_replace(alloc)`, `specialise_membership(set, table)`, `mark_parallel(loop, plan)`. Each action keeps evaluation order (no reordering of effects), stays within an expansion budget, and produces HIR that passes `hir.Verify`.

## Guards that are always on

- Evaluation order of effects is preserved.
- A rule that cannot prove its guard does nothing (no "probably").
- Expansion budget per method and per program.
- Byte-identical differential on the oracles after a rule set changes: lexer 44/44, registry oracles, zabapgit check (ABAP and Go vs Node).

## Milestones

1. **Engine + facts, no rewrites.** Extract base facts, compute `receivers`, `may_throw`, `pure`, `escapes`; print them; spot-check against hand reading of abaplint's lexer.
2. **First rewrite = abapiti's inliner.** Express `hir/inline.go` as Grace v2 rules; output HIR byte-identical to hers on the same inputs. This proves the language can carry a real pass.
3. **Devirtualisation** via `receivers` (generalises the Go emitter's "concrete pointers for leaf classes" into HIR, so ABAP gains too).
4. **may_throw → exception convention** facts exported to the Go emitter (keeps panic by default; switch per method only with profile evidence).
5. **Parallel map proof** (`mark_parallel`): iteration writes only fresh objects and its own result slot, reads shared data, writes statics only as `memo_store` (thread-safe compute-once cache in the Go runtime) or commutative counters (atomics), `lazy_init` warmed before the region. Join re-raises the lowest-index exception; results in source order.

## Not in scope now

E-graphs and per-target cost extraction (only when rules start to conflict); ISLE-style algebraic simplification (only if a profile asks for it); fuzzing (after the whole chain works end to end).

## Additions (from the design talk with Alice, 2026-10-09)

**Recursion with a base, a tail and a bound.** Fact rules are written as a base case plus a recursive (tail) case. Fact evaluation terminates by the fixed point (finite domain, facts only grow). Rewrites do not have that guarantee, so every recursive pattern or rewrite may carry an explicit bound, and the engine has global ones:
- `(bound depth N)` on a recursive pattern: path length / nesting depth it may follow (e.g. a chain of blocks, an inlining chain);
- rewrite rounds per program are bounded; a rule never fires on a node it produced in the same round; recursive inlining (a method calling itself, directly or through a cycle) stops at the bound instead of unrolling;
- the per-method and per-program expansion budgets above.

**Rule priorities** (as in MinZ Grace: `(grace name priority ...)`): higher first; ties by declaration order, so the result is deterministic.

**Shape matching on the structured HIR.** MinZ's Grace matched CFG blocks and edges; abapiti's HIR is structured (Block, If, While, ForEach, Try, Seq), so patterns match statement shapes instead: "a While whose condition reads stream position and whose body calls only small stream methods", "a Try around one call", "an early-return chain". Block/edge patterns are not needed while the HIR stays structured.

**The engine is small and portable on purpose.** It should itself be translatable by abapiti (TS/Go source -> HIR -> ABAP/Go), so the same rules can run inside any runtime later.

**Measured cost models, not guessed** (the MinZ lesson: without a trusted cost model, choosing between rewrites is pointless): ADT trace hit lists on A4H for ABAP, pprof for Go. Rules that trade one cost for another are enabled per target only when a measurement says so.

**Two oracles per rule:** every HIR -> HIR rule is checked by ABAP vs Node and Go vs Node; a rule set that breaks either is reverted.

**Later, only if needed:** e-graph with per-target extraction when rules start to conflict; ISLE-style local algebra if a profile asks for it; fuzzing once the whole chain works end to end.

## Milestone 1 implementation contract

This package is read-only: it verifies its input, extracts facts and evaluates
rules. No actions, invalidation, backend integration or oracle changes exist yet.
The design above describes later milestones as well as this one. All source and
rules here are fresh; MinZ's Grace, Datalog and ISLE supplied syntax ideas only.

The public entry points are `Parse`, `Evaluate`, `NewDB`, and (with extraction)
`Analyze` and `Report`. The package uses only HIR and the Go standard library.
A DB is a set of string tuples indexed by predicate and argument; reads return
lexicographically sorted copies. Duplicate tuples are memoised.

```lisp
(fact edge "A" "B")
(rule reachable 10
  (head (reachable ?a ?b))
  (base (edge ?a ?b))
  (tail (reachable ?a ?x) (edge ?x ?b))
  (bound depth 4))
```

Every base/tail section is an alternative conjunction. Variables start with `?`,
`_` is an anonymous wildcard, strings use Go quoting, and `;` starts a comment.
Heads and negated atoms must be range restricted by positive body atoms. Rules
cannot invent new values. `(not (predicate ...))` uses stratified negation:
negative dependencies finish before their consumers; negative cycles are errors.
Arity conflicts and malformed syntax are errors. Priorities are descending, with
stable declaration order for ties; they affect scheduling, never the fixed point.

Evaluation uses delta joins, including each positive body position as a pivot,
and batches additions per round. Proof depths are memoised as minimum heights:
base facts have height 0 and a rule proof has height 1 + maximum positive premise
height. `bound depth N` caps that proof height (including the base case), rather
than global rounds or number of joins. Shorter proofs can unlock bounded rules.
This is an explicit fact-layer interpretation of the draft's bound; future
recursive rewrite patterns will bound structural path length instead.
