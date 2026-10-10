# Value-object screening — 2026-10-10

**Result is not a value object under the requested conditions.** Its first
witnessed post-constructor field write is
`src/abap/2_statements/result.ts:30`: `fromChain` assigns `ret.nodes` after
`new Result` has returned. Line 31 assigns `ret.nodeCount`. Moving these two
assignments into the constructor would not establish immutability:
`wrapConsumed` writes both fields at lines 62–63, `popNode` writes them at
72–73, and `setNodes` changes them too. These are writes to the existing Result
instance, including an alias (`ret`), rather than changes to unrelated node
objects. The Grace witness is
`Result::fromChain/body/s1` in the class-qualified source identity.

Replacing existing Result references with copied table rows is therefore not
justified by the immutable-value-object argument. An ownership or explicit
state-threading design would need a different proof. This work changes no
emitter and performs no rewrite.

## Inputs and scope

The baseline is `c56cd3d148d28a398fafe293bc04a8be5e192d63`, the clone's merge of
abapiti main and `analysis/grace-flow-facts`. The command verifies all **1,538
pinned TypeScript files plus RegistryRun**, npm-package hashes, embedded
reachability and fingerprinted overrides, with assume-int. Lowering has **1,927
classes, 73 interfaces, no blocking diagnostics and no HIR verification errors**.
Grace supplies 6,257 method bodies and 42,766 receiver facts. Analysis takes
place before inlining, using the same input contracts as production abaplint.

The report covers the requested Result, Position, VirtualPosition, TokenNode,
ExpressionNode, StatementNode, Identifier, every lexer token class and every
combi class. It also includes the twenty largest conditional parser allocation
opportunities in the whole lowered world, including generated records/tuples.
The command's complete output has 1,858 non-module class rows and 2,803 static
class allocation sites. The committed CSV has **92 candidates and 727 corresponding allocation sites**.
It is a candidate selection from that
full-closure analysis, not a separately lowered subset.

## Conditions and evidence

The [Grace rules](../../../hir/rewrite/rules/value_objects.grace) derive
`vo_immutable`, `vo_identity_free`, `vo_closed`, `vo_absent_flag`, `vo_pass` and
`value_object` from source witnesses. Extraction is analysis-only and uses Grace
hierarchy, lexical binding identities, aliases, and method/site write summaries.
The value rules are deliberately excluded from the optimization pipeline.

1. Field writes are checked throughout the closure, including aliases,
   inherited fields, constructor helpers and writes from other classes. Only
   writes to the instance under construction in its constructor/base constructor
   are exempt; shared static field writes outside the constructor/class initializer
   also block condition 1. Static fields do not enter the instance row layout.
   Constructor proof requires a definite initialization prefix before
   publishing/observing `this`; helper/branch-dependent initialization can remain
   unproven. Direct publication before initialization has a leak witness.
2. Reference equality includes both HIR Binary comparisons and erased
   `dynamic.strictEquals`, reference membership via `array.includes/indexOf`,
   Map keys, Set membership/construction, `instanceof`, checked reference
   downcasts, dynamic class tests, and `object.classOf` (`x.constructor`).
   Primitive field comparisons and absent tests do not observe object identity.
   Uses through interfaces/supertypes screen all possible classes. Unsupported
   WeakMap constructs would be rejected by the lowering/diagnostic gate, rather
   than silently qualifying.
3. Subclasses are witnessed at their declarations. Mixed interface/supertype
   storage is conservatively screened. A type admitting several classes is a
   **potential** mixed storage barrier until object-specific flow establishes
   which non-C values actually reach that location. A broader declared type alone
   is not reported as a witnessed mixed store.
4. Optional references need an explicit absent flag. This is informational and
   does not reject a representation that supports absence. The CSV prioritizes
   direct Optional<C> sites, then nullable supertypes, then erased object-root or
   dynamic uncertainty. Nullable fields are also listed in the field layout.
5. The copy-cost screen uses a declared threshold of 64 inline bytes and four
   reference slots. Bool/I32/I64/number estimates are 1/4/8/8 bytes; strings use a
   16-byte descriptor/one reference slot, other references an 8-byte handle, and
   Optional adds 8 bytes for presence/alignment. These are **bytes-ish estimates,
   not measured ABAP layouts**. Referenced payloads are excluded; object/container
   fields retain their allocations. No recursive field flattening is proved.

`fail` means there is a witnessed blocker. `fail:unproven` means qualification is
blocked by conservative uncertainty; it is **not a proof of a semantic violation**.
Both prevent `value_object`. The CSV contains the first proof barrier and a
separate first witnessed violation for each substantive condition. “First” uses
TS file order, then numeric line/column and stable HIR path; every category keeps
its first witness. Generated override sites retain their source provenance.

No candidate is admitted by this conservative full-world screen. This is not a
proof that every class in the project fundamentally needs identity. In
particular, erased Dynamic/object-root values and coarse alias write summaries
prevent positive proofs for otherwise simple classes. The independent reference
evaluator agrees on the extracted facts and rules; it does not independently
validate the HIR extractor or resolve those uncertainties.

Position has VirtualPosition as a subclass. VirtualPosition is class-tested and
is assigned to Position. TokenNode has TokenNodeRegex as a subclass and is tested
through node unions. ExpressionNode and StatementNode inherit mutable children
from AbstractNode. Token classes preserve their runtime classes through lexer
and parser class tests, including constructor comparison in combi.Token.
Identifier has Alias/TypedIdentifier subclasses and class tests through its base
reference type. These provide independent witnessed reasons to reject the
requested classes even where another condition remains unproven.

## Result layout and allocation interpretation

Result has four fields: `tokens` (array reference), `tokenIndex` (I32), `nodes`
(Optional<ResultNode>), and `nodeCount` (I64): **36 estimated inline bytes and two
reference slots**. The token array, linked ResultNode records and the nodes they
refer to still need their existing allocations; removing Result alone would not
remove those. The optional `nodes` field needs presence representation, separately
from Optional<Result> uses at caller sites.

There are **two static `new Result` sites**, both lexically outside loops: one in
Result.fromChain (statements) and one in combi.Combi (its own stage row). The
factory is called from parser loops, so lexical “outside loop” does not imply
cold execution. Loop depth measures the allocation's enclosing HIR loops, not
transitive call frequency.

[Per-class table](tables.md) and [candidates.csv](candidates.csv) rank first by
**proven parser allocation opportunities removed**, then by the conditional
opportunity if a class qualified, then stable class identity. All proven scores
are **zero**, so that score is a tie; the secondary ordering is an investigation
order. “Parser-hot” here means lexer + statements (combi separated by caller
class) + structures. Syntax, rules and other are counted separately in CSV.
This is a static stage proxy, not a per-class measured A4H frequency.

The reported **22–32 seconds of A4H object creation are motivation only**. Static
sites are not allocation executions and cannot apportion or predict those
seconds. `dynamic_new_potential` lists compatible dynamic constructor sites
separately; they have no statically proven concrete `new C` identity and are
excluded from savings. This matters for token factories.

[ABAP class names](names.json) use the exact `hir.Names` identity/hash algorithm
used by `abapiti abaplint -o out` and its `names.json`; each entry maps back to the
qualified class and TS source. This creates a class-only mapping without invoking
an emitter. No A4H profile/names file was supplied for execution-weighted joining.
The existing profile #5 combi hot classes are marked in [tables.md](tables.md).

## Artifacts and validation

* [candidates.csv](candidates.csv): conditions 1–5, both kinds of first site,
  optional uses, full fields/layout, references, static allocations by stage and
  loop, dynamic construction uncertainty and ranking scores.
* [allocations.csv](allocations.csv): every static New site for the selected
  candidates, including file:line, Grace path, caller/combi class and loop depth.
* [witnesses.json](witnesses.json): candidate facts, first witnesses per blocker
  category/nullable type, field layouts and every allocation site.
* [tables.md](tables.md): printable per-class table and conditional investigation
  ranking. U means unproven, F a witnessed failure, P pass, A absent flag needed.
* [names.json](names.json): ABAP trace names for all selected classes.

The synthetic suite has a qualifying primitive-only class and rejects mutation
after construction, mutation through an alias, reference equality, instanceof,
subclassing, Map keys, leaked this, erased dynamic equality and array reference
membership and shared static-field mutation. Primitive equality and Optional
references remain positive; an Optional array does not make each row Optional. Both
Cartesian and independently hashed reference evaluation agree on these fixtures;
extraction leaves HIR unchanged. The opt-in full reference check compares the
value-object rules over all 1,927 classes.

Passed: **`go vet ./...`**, **`go test -short ./...`**, and the full-closure
`TestValueObjectsFullClosure` with independent reference equality. The final command facts also match the independently checked run byte for byte.
Exact full-run premise counts and timing are in [validation.txt](validation.txt).

```sh
mkdir -p "$HOME/.cache/abapiti-go-cache" "$HOME/.cache/grace-tmp"
export GOCACHE="$HOME/.cache/abapiti-go-cache" GOFLAGS=-buildvcs=false
export GOTMPDIR="$HOME/.cache/grace-tmp" TMPDIR="$HOME/.cache/grace-tmp"
go vet ./...
go test -short ./...
go run ./cmd/grace-counts -value-objects -output "$HOME/.cache/value-objects"
ABAPITI_VALUE_OBJECTS_FULL=1 go test ./cmd/grace-counts \
  -run '^TestValueObjectsFullClosure$' -count=1 -v -timeout=30m
```

Each green implementation step and the final evidence commit are pushed to
`origin/analysis/value-objects`, using the requested Alice V. commit identity.
