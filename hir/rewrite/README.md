# Grace — rules over abapiti's HIR

Adapted from the Grace design draft (2026-10-09). Reviewer: abapiti.
Home: package `hir/rewrite` in oisee/abapiti, branch `proto/grace`.
Milestones 1 and 2 implement the fact layer and the bounded rewrite layer,
including the Grace inliner. Backend integration remains separate.

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
- `runtime_op(Site, Op)` and the adapter's `op_reads`, `op_writes`,
  `op_allocates`, `op_may_raise`, `op_aliases` records
- `ensure_init(Method, Site, Class)` for emitter-derived lazy initialization guards

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
2. **First rewrite = abapiti's inliner.** Express `hir/inline.go` as Grace rules; output HIR byte-identical to hers on the same inputs. This proves the language can carry a real pass.
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

`Analyze`, `Extract`, `Evaluate` and `Report` remain read-only. Milestone 2 adds
mutation through `Rewrite` and `Inline`, as described below. All engine source
and rules here are fresh; MinZ's Grace, Datalog and ISLE supplied syntax ideas
only. The test oracle invokes main's HIR inliner on an independent deep copy.

The public entry points are `Parse`, `Evaluate`, `NewDB`, and (with extraction)
`Analyze` and `Report`. The HIR adapter imports the [IR-independent Grace core](../../grace/README.md).
`DB`, `Rules`, and `Tuple` alias core types; `Parse`, `Evaluate`, `NewDB`, and
`Report` retain their existing signatures as compatibility wrappers. The core
uses only the Go standard library and has no transitive HIR dependency.
A DB exposes string tuples and sorted-copy reads. Internally it interns symbols
into integer IDs, memoises packed tuple keys, and builds composite hash indexes
from the bound columns of rule bodies. The interpreter reuses binding arrays
and undo stacks instead of allocating a map for each candidate tuple.

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

### HIR extraction and analysis precision

`Analyze` calls `hir.Verify` first; `Extract` assumes verified input. Method keys
are `Class::member` (constructors use `constructor`). Sites and local bindings
use method-qualified structural paths, so missing/duplicate Node IDs are safe,
shadowed locals remain distinct, and output is reproducible. Assignment/return
expressions are expression site keys. `param` indices are zero based, with `this`
for the receiver. `new` records class allocations. Interfaces enter through
`implements` and virtual receiver types; abstract method bodies are not analysed.
`final` means a closed-world leaf, since HIR has no final/sealed declaration flag.

Support relations (`concrete`, `defined`, `dispatch`, `interface_subtype`, `site_type`, `narrowed`,
`exact_receiver`, `expr`, `flow`, `alias`, `argument`, `sink`, `fresh`, `mutation`,
`static_origin`, `field_origin`, `site_method`, `raises`,
`unknown_effect`, `ensure_init`, `implicit_init`, `flag_init`, `memo_shape`, `counter_shape`,
`noncounter_write`) describe HIR identities, shapes and effects. The seven
requested derived relations are evaluated from the embedded `.grace` rules.
`flow(M,destination,source)` tracks possible value dependence for escape/alias
analysis. It deliberately overapproximates dependencies of expressions.

Receivers include structural interface-to-interface compatibility, as in Verify,
concrete descendants/implementors and nearest inherited method
implementations. Immutable locals initialised from `new`, immutable aliases,
checked views and conditional unions narrow the candidate classes. Any local
name assigned elsewhere in the method disables that narrowing (including loops
and Seq); mutable locals and parameter/field/return flow fall back to CHA. This
is deliberately conservative and not an interprocedural points-to analysis.
Constructor calls and emitter-derived `ensure_init(Method,Site,Class)` guards
enter the call graph. Guards cover non-constant static reads, static writes,
class allocations, and static-method/constructor entry (excluding the initializer
itself). Constant-only or empty initializers need no guard. The adapter mirrors
`hir/abap/constants.go`; static/field facts retain separate owner and name columns,
and constant tracking uses a `FieldKey{Owner,Name}` key. Empty virtual dispatch is unknown.

`may_throw` includes explicit throws, traps, transitive calls, checked casts,
checked conversions, potential Number arithmetic failures and runtime operations
selected by the explicit adapter effects table in `effects.go`. Every catalogue
operation and SpecialOp has Reads (Receiver/Args/Global), Writes
(None/Receiver/Arg(i)), Allocates, MayRaise (None/Trap/Catchable), and Aliases
(None/Receiver). Base relations are `op_reads(Op,Source)`, `op_writes(Op,Target)`,
`op_allocates(Op,Bool)`, `op_may_raise(Op,Kind)`, and `op_aliases(Op,Source)`.
`op_conservative(Op,Reason)` marks unresolved target behavior. Allocates alone
does not prove confinement (Optional boxes may contain existing references).
Invoked constructors in classvalue.new/materialize retain unknown effects until
separate call-target analysis resolves them. `may_throw`
uses `op_may_raise`, independently of the HIR catalogue's calling-convention
`Mutates` bit. Only clock.telemetry reads Global. Catch regions are recorded but do not suppress may-throw:
this milestone does not prove handler coverage. Allocation failure is excluded.

Purity means no field/static writes, no external or unknown effects, and calls
only to methods meeting that same definition. Local collection mutation is
allowed only on a proven fresh receiver with no escaping alias. The rules derive
impurity first and then its complement among defined methods, so pure recursive
cycles are supported. Throwing and divergence are independent of effect purity;
`pure` is not a claim that memoisation or parallel execution is safe. Constructors
writing their own fields are conservatively impure in this milestone.

Reference binding aliases share escape status in both directions.
Escapes propagate backwards through assignments/value dependencies and across
arguments to escaping callee parameters, including `this`. Stores, returns,
throws and mutating/unknown runtime arguments are conservative sinks. Reads of
static/instance fields retain origins across local aliases: mutating a static
map/array counts as a static write. Assignment lvalues are also recorded as reads.
`writes_static_transitive` follows all resolved call targets, including implicit
initialisation. No facts change the input HIR.

Memo recognition is the exact two-statement shape: `if (!staticMap.has(k))`
containing only `v = staticPureMethod(k)` and `staticMap.set(k,v)`, then
`return staticMap.get(k)`. Keys must be the same primitive local/literal; no else
or additional statements are accepted. Shape extraction supplies `memo_shape`;
the rule requires `pure(compute)`. This records shape only, without proving
cache ownership, key completeness, invalidation, or thread safety.

`lazy_init` records implicit HIR class constructors (ABAP's guarded first-use
semantics), or a method guarded by `!staticBool` whose last statement sets that
same bool true. This does not prove backend-independent ordering or atomicity.
`counter_store` records only `static = static + literal` with no other assignment
to the same field in that method; it is a syntactic classification, not a proof
of floating-point commutativity or atomicity. Everything else is `other`.

### Reproducing the lexer report

```sh
go test ./hir/... ./tsfront/...
GRACE_FACTS_OUT=/tmp/lexer.facts go test ./tsfront -run '^TestLexerFactsReport$' -v
```

The helper shares the existing lexer closure lowering and checks the 44-case
oracle corpus. Its golden is `tsfront/testdata/lexer.facts.golden`; tests never
rewrite it. The report includes base/derived counts (including zeros) and nonempty support relation counts, may-throw/pure sets,
receiver classes for every virtual site (including empty sets), and static writes
by method and class. Write totals count distinct `(method,class,field)` tuples,
not dynamic events or individual store sites. A field may therefore have more
than one classification across methods. The original lexer differential and
backend goldens remain unchanged. The milestone 2 test also compares independent
HIR copies with main's reference inliner before reporting the original facts.

### Open HIR interface questions for abapiti

- Can RuntimeSpec expose semantic MayRaise and external/read/write effects, including SpecialOps?
- Should HIR distinguish declared final/sealed classes from closed-world leaves?
- Can HIR expose stable scoped local/site identities and resolved inherited declaration owners?
- Should implicit class-constructor edges and initialisation order be represented in HIR?
- What purity contract should fresh-object construction, static reads, throws and divergence use?

## Milestone 2 rewrite contract

`Rewrite(program, rules, Limits)` mutates verified HIR and returns `Stats` plus
an error. `Inline(program)` parses the embedded `rules/inline.grace` and invokes
that runner. `Stats.CallSites` and `Stats.Callees` match the reference's total
and `Class.method` counters; `Stats.Rounds` also records verified rounds.

```lisp
(grace small-instance-method 10
  (match (node ?site virtual))
  (where (inline_allowed ?site ?callee))
  (action (inline ?site))
  (bound depth 64))
```

A match is `(node site kind)` over structural expression sites. Statement kinds
are also available as `node` facts for guards, and the inline shape facts cover
structured bodies, nested blocks/Ifs and Seq statements. `where` is a conjunction
of positive or negated facts. Guards and action operands are range restricted.
`le`, `neq` and `contains` are reserved, two-argument comparisons, evaluated after
positive joins; unknown/non-numeric operands cannot prove a numeric guard.
Priorities descend; declaration order breaks ties. A node takes the first
successful action, and replacements are never traversed again in that round.

`inline(site)` supports statically class-typed virtual calls, and void calls in
expression-statement position. Its primitive resolves the existing `dispatch`
fact, builds a return template, binds receiver then arguments once, and copies
with fresh scoped names. It substitutes matching literal arguments only for
never-assigned parameters, and substitutes `this` only for the declaring class.
It does not impose purity. Selection policy is in Grace: virtual instance body,
12-statement limit, excluded statement/expression kinds, variadic/constructor/
checker-variant exclusions, exact arity, closed-world override tests, and
candidate-call-cycle exclusion. This deliberately uses static-class dispatch;
`receivers` narrowing and `pure` would change the reference's criteria.

`replace(node, expr)` takes an expression site as its second operand. Currently
both expressions must be same-typed literals, locals or `this`; effects cannot
be dropped, duplicated or moved. Richer expression constructors and effect
proofs can extend this primitive later. No backend changes are made.

Round inputs are snapshotted. Operands are visited X/Y/Z/Args/Seq, statements
X/Y/Body/Else/List, and methods in program order (constructors first). When an
inline action needs a template, the callee's original nodes are visited first.
Each method is visited once per round. This reproduces the reference's naming
and call-site order without visiting generated nodes. Rule depth bounds count
callee-chain edges: depth 0 prevents rewriting; 64 is the embedded rule's bound.
Candidate cycles are rejected rather than unrolled, exactly as in the oracle.

Zero-valued `Limits` choose one round, 100,000 added nodes per method and
1,000,000 per program. Growth counts statements plus expressions, charging only
positive growth, across all rounds. A budget-rejected expansion consumes no
serial number or call-site counter. User limits must be nonnegative. Every round
runs `hir.Verify`. Rewrite evaluates only the transitive relations required by
its match and guards, including negative dependencies. Inline requires no derived
analysis relations: its analysis inputs are `dispatch`, `defined`, and `static`.
The full `Analyze` API and diagnostic extraction retain their complete facts.

For multiple rounds of the embedded inline rules, changed methods and their
transitive callers are refreshed. Method-local base and derived facts are
invalidated together; unaffected proofs and immutable hierarchy/declaration facts
survive. The shared `inline_overridden` relation is conservatively recomputed.
Region ownership and the reverse call graph are supplied by the HIR adapter;
the tuple evaluator and region invalidation have no HIR dependency. Custom rules
can connect arbitrary regions, so they use full recomputation. Set
`ABAPITI_GRACE_RECOMPUTE=full` to force the reference recomputation path for the
embedded rules too. Per-round fact equality, final HIR and stats equality, and
stale negative proofs are tested. A round without actions terminates early.
These safety limits can intentionally differ from the unbounded reference on
larger inputs; the checked fixtures do not reach them.

### Oracle and reproducibility

`internal/inlineoracle` calls main's real `hir.InlineStats`. Only tests import
it; production rewrite code has no oracle dependency. Each comparison makes two
independent type-preserving deep copies, compares `hir.Dump` byte for byte and
compares total/per-callee counters. Failures print the first differing line or
stats. The 70394ff selection guard rejects any callee with an uninitialized
VarDecl, including inside Seq. A synthetic TypeScript loop executes in Node
with result `[5,-1]` and remains a call under both inliners.

The lexer fact-report fixture performs this comparison, and all seven registry
emission fixtures perform it when `ABAPITI_GRACECHECK=1`. JSON/XML do it before their existing external-corpus emission
skips; singleton sources are pinned locally with hashes so the default test run
covers their HIR too. The original ABAP/Go differential tests and goldens are
unchanged. Focused oracle cases cover recursive and mutually recursive calls,
checker variants, descendant overrides, size limits, guarded returns, void
bodies, shadowing, Seq scopes, parameter assignment and fresh-name collisions.
Runner tests cover budgets, depth, priority/ties and same-round exclusion.

```sh
go test ./hir/... ./tsfront/...
ABAPITI_GRACECHECK=1 go test ./tsfront -run 'TestLexerFactsReport|TestEmitRegistry' -v
go test ./hir/rewrite -run 'TestInline|TestReplaceRoundSnapshot' -v
```

The fixtures are byte-identical, with identical total and per-callee counters:
lexer 61; registry arrays 2, features 0, iterators 7, JSON 0, singletons 0,
sorts 10, XML 1. The lexer corpus still contains 44 cases.

### Size and language boundary

The rewrite runner, native inline action/return lowering, and fact extractor
are Go. Selection and derived analysis live in the small Grace rule files.
Runtime effects and emitter initialization guards live in adapter Go files.
The oracle uses the main HIR inliner directly; no implementation is vendored.

No reference selection criterion is omitted. Return-template feasibility is a
native shape fact (`inline_template`), and template lowering, effect bindings,
hygienic naming and scoped copying are implemented by the native `inline`
action. The tuple language cannot itself construct AST lists, lower early
returns to Conditional/Seq, or carry lexical renaming environments. Expressing
those operations entirely in Grace would require AST constructors and recursive
sequence/scoping patterns beyond this milestone's action primitive.

## Engine regression tests

`internal/gracecheck` is test support only. Its independent parser and naive
stratified fixed-point evaluator scan full relations and every rule on every
round. They use no engine joins, argument indexes, deltas or cached joins.
Ordinary fact-set membership and minimum proof heights implement set semantics
and bounded proofs. The evaluator caps premise examinations at one billion;
the engine comparison also has a 30-second deadline. Both `analysis.grace` and
`inline.grace` are checked, including the phased preparation used by `Rewrite`.
`ExtractRewriteFacts` exposes a verified, read-only native syntax snapshot for
that comparison; it makes no selection decisions.

The common checker runs on 64 seeded programs (32 cyclic call graphs and 32
independent-literal statement permutations, seeds 0..31) and 15 hand-written
programs: empty/single methods, throws across calls and loop nesting at depths
0/1/3/4/5/63/64/65, a structural interface diamond, and array/map aliases with
and without optionals. Separate tests exercise cycles of lengths 1..4 against
five depth bounds, six callee-chain depth boundaries, and the minimal Seq budget
regression. HIR supports single class inheritance; the diamond's two arms are
represented through structural interfaces.

The lexer fixture calls the same checker before any external emission-oracle
skip. The seven registry emission tests run it only with
`ABAPITI_GRACECHECK=1`, keeping the expensive checks out of the shared race-test
shards. A separate advisory CI job enables them without `-race`. The checker
checks reference fact equality, reevaluation,
declaration permutations, positive-rule monotonicity, one-pass inline
idempotence, verified rounds, zero depth, independently counted per-method and
program growth, and main's inliner dump/counter comparison. Synthetic graph
cycles are virtual calls; embedded rules reject recursive candidates rather
than unrolling them. The Seq budget regression fixes an omitted enclosing block
in the native node counter; default fixture oracle results are unchanged.

Two qualifications are deliberate. Relational negation is not monotone under
arbitrary input growth: adding a write can remove `pure`. Monotonicity therefore
uses the positive alternatives of the actual embedded rules; the complete
stratified rules are covered by the independent evaluator. Executable statement
order is semantic, and structural fact site IDs encode that order. Statement
permutation tests use independent literals. Declaration permutation facts must
be exactly equal, while rewritten dumps are compared after sorting declarations
and alpha-normalising generated local names. The main oracle numbers those
names by traversal order. `TestInlineOracleNamesFollowDeclarationOrder` keeps a
minimal demonstration that literal dump invariance under method permutations
would conflict with byte-identical oracle compatibility.

```sh
go test -short ./hir/... ./tsfront/... ./internal/gracecheck -count=1
ABAPITI_GRACECHECK=1 go test ./tsfront -run 'TestLexerFactsReport|TestEmitRegistry' -v
ABAPITI_GRACE_FULL=1 go test ./tsfront -run '^TestGraceFullRegistryClosure$' -v -timeout 60m
```

The full test skips under `-short`. It uses `REGISTRY_CLOSURE` when provided,
or materialises the embedded pinned archive and declaration packages, verifies
all 1,538 source hashes, and lowers with the production CLI's fingerprinted
registry overrides, embedded reachability, `RegistryRun` harness and assume-int
contract. Lowering errors fail the test. The expected closure has 1,927 classes
and 73 interfaces. No Go HIR backend is needed.

`CheckFull` runs the same checks as `Check`, with independent hash projections
in the reference evaluator and a five-minute engine deadline per equality check.
The reference still reparses rules independently, rescans every rule over the
complete snapshot each round, and shares no engine join or delta code. Ordinary
fixtures compare this mode with the Cartesian reference as well as the engine.
Both reference modes retain the one-billion-premise work cap. The Cartesian
reference exhausted that cap on the full closure after 108.3 seconds.

The test logs the facts summary, inline oracle counters, per-check runtimes and
verified rewrite rounds. Set `GRACE_FULL_FACTS_OUT` to export the complete sorted
facts report, including method sets, virtual receivers and static writes by
class. See [the full-closure results](../../docs/grace-full-closure.md).

Measured engine changes and the retained relation list are recorded in
[the performance report](../../docs/history/2026-10-10-grace-engine.md).

## Flow and kernel opportunity analysis (2026-10-10)

`Extract` additionally records `loop(Site,Iterated,RowVar)` for ForEach and
`in_body(NodeSite,LoopSite)` for all transitively enclosed statement/expression
sites (including nested loops and Seq). `def(Local,Site)` describes parameter entry, declaration,
assignment, loop-row and catch definitions; `use(Local,Site)` records actual
local/this expression reads, excluding a bare assignment LHS. `local_ref` retains
binding identities even for that LHS. `use_count(Local,Count)` counts syntactic
read occurrences, including zero for unused bindings; it is not a dynamic count.

The analysis-only `next` graph preserves left-to-right operand evaluation,
statement sequence, both If/conditional/short-circuit branches, While back-edges,
ForEach row definitions, break/continue, early return, Try handlers and Finally.
ForEach evaluates its input once; its iteration body has a back-edge. Seq gets
its own lexical scope and its contained control-flow graph. Evaluation/control points with no local read or definition are contracted
into path edges; every read and definition remains a distinct point, including
operand order and exception alternatives. This avoids forcing the independent
whole-rule evaluator through thousands of empty transfer steps. The embedded rules
compute backward `live_in`/`live_out`, killing the old binding value at `def`.
`not_read_after(Local,Site)` means that a value read at that expression completion
site is not live on any successor path; it is emitted at actual read sites,
rather than materializing every possible local/site pair. A later assignment
kills the old value; reads of the new value do not prevent a last-use claim.
Exceptions conservatively connect every nonliteral/nonlocal expression to its
handler. Finally joins normal and exceptional exits, possibly admitting paths
that do not execute; this can lose opportunities, never prove an unsafe last use.
No branch constant folding or path feasibility proof is attempted.

`writes(Method,none|own_fields|any)`, `allocates(Method)`, `may_raise(Method)` and
`may_diverge(Method)` summarize the closed-world call graph, including all Grace
virtual targets and ensure_init edges. `writes_value`, `writes_param` and
`site_writes` propagate reference-alias writes and map callee parameter indices
(including `this`) back to actual arguments. Immutable declaration aliases supply separate `must_alias` ownership proofs;
possible aliases never prove own-field ownership. Direct writes to fields of `this`
are own-fields; statics, unknown effects and writes to other unconfined storage
are any. Proven confined local mutation is compatible with none. Calls to an
any-writing method conservatively keep any even if a more precise ownership
proof could confine that call. Allocations include runtime-table allocation
flags; unknown effects may allocate/raise/diverge. Loops and recursive call
cycles may diverge, even when a termination proof might exist. Raising uses the
existing Effects adapter and may_throw rules; catches do not suppress it.

Reference identity aliases are flow-insensitive and include declaration/assignment
bindings, checked views, adapter receiver aliases, conditional results and Seq reference results.
They are not an interprocedural heap points-to analysis. Value-dependence `flow`
remains separate: an escaping element can conservatively mark its container as
escaping. Kernel candidate reports treat uncertainty as a blocker. The default
inliner still demands only its existing syntax relations and evaluates none of
these new facts; no rewrite or emitter rule consumes them.

`recursive_call(Method)` supplies closed-world SCC cycle membership without an
all-pairs reachability table. An independent DFS in test support checks these
seeds against Grace's resolved call relation, including virtual/initializer edges.

The full independent reference evaluates the four liveness rules separately for
each method's disjoint CFG, then evaluates the remaining global rules with those
independently derived facts. It verifies ownership of every CFG edge and local
before partitioning and falls back to global evaluation for cross-method edges,
custom liveness rules, derived transfer inputs or external consumers of these
facts. Every component still uses repeated whole-rule snapshot scans; there are
no production-engine joins or deltas. Ordinary fixtures compare this result with
the unchanged Cartesian evaluator. This avoids repeating all unrelated methods
for each step along the longest method during the full registry gate.

## Copy propagation and dead stores (opt in)

`ABAPITI_COPYPROP=1` enables the shared `BeforeEmission` pass in both HIR
backends. The default is off. `CopyProp` can also be called directly. Policy is
in `rules/stores.grace`; the adapter adds `substitute-use` and
`remove-statement`, rechecks their safety, and verifies HIR after each bounded
round. Statistics include copy/store totals and per-method counts.

Copy propagation requires one initialized definition, one read, exact HIR
types, and a use later in the same statement list. Inert expressions can move
past inert stores that do not redefine their dependencies. Calls and heap or
runtime writes are barriers, so alias writes and unknown method summaries
cannot justify motion. An effectful or raising expression only moves into the
whole expression of the immediately following statement. Loop, branch,
try/finally crossings and lowered closure captures are rejected. Changed RHS
plans are deferred to a new round.

DSE accepts only positively reviewed inert expressions: no calls, identity
allocation, checked arithmetic, casts or unknown runtime operations. The CFG
includes exception/finally paths. `store_next` contracts that CFG to the first
read or definition of the candidate binding on each path, and Grace derives
`not_read_after` from those boundaries. Declarations remain to bind emitter
locals; only their initializer is removed. A dead assignment becomes an empty
block. Later rounds refresh changed methods; setting
`ABAPITI_GRACE_RECOMPUTE=full` retains full recomputation for comparison.

`GRACE_COUNTS_MEASURE_STORES=1 go run ./cmd/grace-counts -output DIR` reports
three pass samples on independent copies of the pinned full closure, stages,
and whole/hot-method emitted ABAP statement counts in `stores.json`. The ABAP
baseline includes the existing singleton/inlining pipeline; Go starts at the
original lowered input. `GRACE_COUNTS_PROFILE=PATH` optionally profiles the
first Go pass sample.
