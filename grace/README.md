# Grace core

`github.com/oisee/abapiti/grace` is the IR-independent fact database, symbol
interner, S-expression parser, semi-naive stratified evaluator, indexed joins,
demand selection, regional invalidation and deterministic text report. It uses
only the Go standard library. `hir/rewrite` supplies the HIR extraction, effect
table, native actions and embedded rule files; another repository can supply
its own IR and facts without importing HIR.

## API

- `Tuple`, `DB`, `NewDB`: `Add`, `Has`, `Facts`, `Count`, `Predicates`.
  `Lookup` retrieves a constant prefix using indexes in insertion order.
- `Rules`, `ParseRules(text) (*DB, *Rules, error)`, `Evaluate(db, rules)`.
  Read `.grace` text with `os.ReadFile` or an embedded filesystem and pass it
  to `ParseRules`. The returned DB contains literal `(fact ...)` declarations;
  retain it when the source includes facts. Rules are immutable.
- `SelectDemand(sets, goals)` returns selected rules and their required
  relations, including transitive and negative dependencies. `DB.SetDemand`
  filters subsequent additions; `DB.Demands` lets extractors skip unused work.
  `Rules.Heads` lists clause heads in evaluation order, including duplicates.
- `DependentRegions(changed, reverse)`, `DB.SetRegionOwner`,
  `DB.InvalidateRegions(affected, rules)`, `DB.SelectRegions` and
  `RegionSelection{Column, Contains}` support adapter-defined regions.
- `RewriteRule`, `Rules.Rewrites`, `Rules.RewriteGoals`, `RewriteRule.Action`,
  `Bound`, `Kind`, `Validate`, `Matcher` and `Matcher.Match` expose opaque
  match/guard/action declarations and indexed action argument queries. The
  adapter implements native actions and enforces IR and growth constraints.
- `Report(db) string` preserves the existing analysis report's counts, method
  sets, receiver lists and static-write summary byte for byte. `DB` and `Tuple`
  are the report data types; there was no separate structured report type.
  This report expects the analysis relation schemas; for custom schemas use
  `Predicates`, `Count` and `Facts` directly.

An adapter verifies its IR, assigns stable identities, inserts ground facts and
checks every error before reading derived relations. The engine does not inspect
IR objects or decide what effects an unknown operation has. An adapter must
represent uncertainty conservatively (for example, `unknown_effect`) instead of
inferring safety from absent facts. Parsing/evaluation reject invalid syntax,
inconsistent arities, unsafe variables and negation cycles. Evaluation can
mutate the DB before an error; discard failed results and do not apply actions.

`Facts` and `Predicates` return sorted copies. Evaluation preserves rule priority,
source order on ties, and insertion order within indexed joins. Stable extraction
order and stable identifiers are required for deterministic action selection.
Do not mutate demand/selection maps during use. DBs and reusable matchers are
not safe for concurrent mutation.

Set the region owner before adding regional facts. Invalidate the reverse
closure of changed regions, re-extract their base facts, restrict evaluation to
those regions, and reevaluate. Shared derived facts are discarded conservatively,
including negative proofs; unrelated regional facts survive. Region pruning is
sound only when the adapter's ownership and dependency graph cover all rule
premises. Use full recomputation for arbitrary cross-region rules. Rebuild
matchers after invalidation and call `SelectRegions(nil, nil)` to reset pruning.

## Toy adapter (30 lines)

This example is executed by `ExampleEvaluate` in `example_test.go`. The toy IR
contains only functions, calls and a raising flag; it imports no HIR package.

```go
func ExampleEvaluate() {
    type function struct {
        name string
        calls []string
        throws bool
    }
    ir := []function{{"main", []string{"read"}, false}, {"read", nil, true}}
    db := grace.NewDB()
    for _, f := range ir {
        if err := db.Add("method", f.name); err != nil { panic(err) }
        for _, callee := range f.calls {
            if err := db.Add("calls", f.name, callee); err != nil { panic(err) }
        }
        if f.throws {
            if err := db.Add("throws", f.name); err != nil { panic(err) }
        }
    }
    _, rules, err := grace.ParseRules(`
    (rule throws 0 (head (may_throw ?m)) (base (throws ?m))
     (tail (calls ?m ?c) (may_throw ?c)))`)
    if err != nil { panic(err) }
    selected, _ := grace.SelectDemand([]*grace.Rules{rules}, []string{"may_throw"})
    if err := grace.Evaluate(db, selected); err != nil { panic(err) }
    for _, fact := range db.Facts("may_throw") {
        fmt.Println(fact[0])
    }
    // Output:
    // main
    // read
}
```

`adapter_test.go` also loads the real `hir/rewrite/rules/analysis.grace` text and
evaluates hand-made toy facts, including transitive throwing and negative purity
guards. `TestNoHIRDependency` checks the transitive production dependency graph
with `go list -deps`; importing HIR or any HIR subpackage fails the test.
