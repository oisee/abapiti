# Grace core extraction (E0)

Baseline: `origin/main` at `619cfb42ef7835267580f3e780471f4c1093c59a`.
Branch: `refactor/grace-core`. Go environment:
`GOCACHE=$HOME/.cache/abapiti-go-cache`, `GOFLAGS=-buildvcs=false`;
temporary files and artifacts are under this checkout's ignored `.e0/`.

The seven original core files had no HIR imports. Their private interfaces
still coupled demand selection and rewrite matching to the adapter. The new
`github.com/oisee/abapiti/grace` package owns relational storage, interning,
parsing, evaluation, joins, demand closure, regional invalidation and report
formatting. The HIR effect table, extraction, native actions, shape analysis,
verification and rewrite runner remain in `hir/rewrite`. Existing adapter
entry points retain their signatures through type aliases and wrappers.

The parser's match/guard/action declarations and compiled matchers are opaque;
syntax trees, clauses, interned IDs, tables and join plans remain private.
The adapter caches declarations once per runner, avoiding a copy per node.

## Exported API

| Area | Types/functions/methods |
|---|---|
| Facts | `Tuple`, `DB`, `NewDB`, `DB.Add`, `Has`, `Facts`, `Count`, `Predicates`, `Lookup` |
| Rules/evaluation | `Rules`, `ParseRules`, `Evaluate`, `Rules.Heads` |
| Demand | `SelectDemand`, `Rules.RewriteGoals`, `DB.SetDemand`, `Demands` |
| Regions | `RegionSelection{Column, Contains}`, `DependentRegions`, `DB.SetRegionOwner`, `InvalidateRegions`, `SelectRegions` |
| Adapter queries | `RewriteRule`, `Rules.Rewrites`, `RewriteRule.Action`, `Bound`, `Kind`, `Validate`, `RewriteRule.Matcher`, `Matcher`, `Matcher.Match` |
| Report | `Report(*DB) string`; the existing report data types are `DB`/`Tuple` |

The README's 30-line toy adapter has the same tokens as the executed example
test. A separate external-package toy adapter reads the real `analysis.grace`
text and checks transitive throwing and negative purity guards using hand-made
facts. The transitive `go list -deps` test rejects HIR and all HIR subpackages.
Core tests retain internal proof-identity checks; HIR tests compare round facts,
program dumps and stats against full recomputation and the independent oracle.

## Validation

- `go vet ./...`: passed.
- `go test -short ./...`: passed.
- `go test ./... -timeout 30m`: passed.
- `./.github/ci/lint.sh gate origin/main`: passed with golangci-lint 2.13.2,
  zero new issues and canary 5/5. Run after the test suites because lint writes
  a temporary canary package that intentionally fails vet.
- `ABAPITI_GRACE_FULL=1 go test ./tsfront -run
  '^TestGraceFullRegistryClosure$' -count=1 -v -timeout 60m`: passed in
  365.85 seconds. The pinned 1,538-file closure lowered to 1,927 classes and
  73 interfaces; reference equality, reevaluation, phased preparation,
  incremental/full equality, positive monotonicity, declaration determinism,
  depth/growth budgets, verified termination and idempotence passed.
- The full-closure inline oracle retained 1,487 sites and 190 callees;
  incremental and full recomputation both terminated after two rounds.
- `TestNoHIRDependency` passed; production dependencies of `grace` consist
  solely of the standard library and the package itself.

Built baseline and candidate with `go build -o <binary> ./cmd/abapiti`.
Each binary ran `abapiti abaplint -o <checkout>/.e0/out --quiet`, using the
same absolute output path and moving completed trees aside before the next
run. The final candidate was rebuilt after the last implementation change.
`diff -r .e0/out-main .e0/out-final` exited 0 with **empty output**.
All 6,336 regular files match, including the reproducible ZIP and READMEs.
Both sorted SHA-256 manifests have digest
`772c1d67d3ba00f7ec775fd7020154abd1b4a88ffaafc9ca405debfb377547a6`.

## Grace pass timings

Go 1.26.0, Linux amd64, same host. Three independent processes per revision,
alternating baseline and candidate. Both timing modules use the same helper
source and replace `github.com/oisee/abapiti` with either the `origin/main`
archive or the current checkout. The helper uses the full-closure test's
production lowering inputs, verifies closure dimensions and diagnostics,
applies `hir.Singleton`, and runs GC before starting the timer. Only
`rewrite.Inline` is timed; lowering, singleton preprocessing, GC, compilation
and output emission are excluded. The measured pass retains its internal
verification. Heavy runs are serialized with a lock inside `.e0/`.

Every run reports **1,509 sites / 198 callees**. These counts include production
singleton preprocessing; the full-closure oracle above measures the original
lowered program before that preprocessing.

| Revision | Run 1 (s) | Run 2 (s) | Run 3 (s) | Median (s) |
|---|---:|---:|---:|---:|
| origin/main | 0.474490504 | 0.433753610 | 0.438136114 | 0.438136114 |
| Grace core extraction | 0.420279093 | 0.419560242 | 0.442785034 | 0.420279093 |

The candidate median is 4.1% lower; the ranges overlap. No pass-time regression
was observed, and this refactor makes no performance improvement claim.

Local evidence is retained under the checkout's ignored `.e0/`: `vet.log`,
`short-final.log`, `full-final.log`, `lint-final.log`, `full-closure.log`,
`output-final.diff`, `out-main.sha256`, `out-final.sha256`, `grace-deps.txt`,
`pass-{base,current}-{1,2,3}.log` and the two standalone timing modules.

## Draft PR description

Title: Extract the IR-independent Grace core for external adapters (E0)

Move Grace's relational database, parser, evaluator, indexed joins, demand
selection, regional invalidation and report into `github.com/oisee/abapiti/grace`,
so external IR adapters can use them without importing HIR. Preserve the existing
`hir/rewrite` API and keep HIR extraction, effects and native rewrite actions in
the adapter. Document the core API with a tested 30-line toy adapter and enforce
its transitive dependency boundary.

Validation: vet, short/full Go suites, lint gate (0 new issues, canary 5/5), and
full-closure reference checks pass. All 6,336 CLI output files are byte identical
to origin/main. Grace pass medians are 0.4381 s before and 0.4203 s after, with
identical counters and no observed slowdown.
