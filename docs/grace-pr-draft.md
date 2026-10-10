# Draft PR: Grace facts and bounded HIR inlining

Grace adds a small S-expression rule language over typed HIR: finite,
stratified fact evaluation for hierarchy, dispatch, effects and escape analysis,
plus bounded, verified rewrites. Its first real rewrite expresses the existing
small-instance-method inline selection as rules, with a native action for
ordered bindings, scoped copies and return-template lowering.

This PR contains the rule engine and adapter, analysis/inline rules, independent
reference evaluators and synthetic/property/closure checks. It rebases onto
abapiti v0.2.0 (`e8e8c35`) and compares independent deep copies directly against
main's `hir.InlineStats`; the old pinned implementation is removed. The
70394ff uninitialized-declaration guard applies in Grace, including Seq bodies,
and the TypeScript loop regression produces `[5,-1]` and remains a call.

The adapter has explicit effect records for all 87 runtime catalogue operations
and six SpecialOps, with coverage tests and nine marked conservative records.
`may_throw` uses MayRaise rather than the former whitelist; on the full closure
it changes from 3,776 to 3,767 methods. The oracle produces byte-identical HIR
and identical counters for 1,487 call sites across 190 callees. Emitter-derived
`ensure_init(Method,Site,Class)` facts account for constant fields, active
initializers, static writes, class allocations, and static-method/constructor
entry. Static/field identities keep owner and name as separate columns; constant
tracking uses a structured pair key.

Evidence and measured runtimes are recorded in [grace-full-closure.md](grace-full-closure.md).
The full closure covers 1,538 source files, 1,927 classes, 73 interfaces, and
6,257 defined methods. Tests compare HIR bytes and total/per-callee counters,
verify rewritten HIR, compare the fact engine with independent reference joins,
and check reevaluation, monotonicity, determinism, budgets, termination and
idempotence. The source loop oracle is executed in Node. The full test passed in 13m 1.60s
and the short suite in 32.30s; lowering had zero blocking diagnostics and zero
HIR verification errors.

Deliberately deferred: production backend integration or replacing main's
inliner, new HIR nodes, emitter/runtime semantic changes, devirtualization,
exception-convention changes, parallel execution, memo-cache synchronization,
e-graphs and target cost models. These checks establish fact/rewrite correctness;
this PR does not claim a new end-to-end ABAP or backend performance result.
