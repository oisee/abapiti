# Grace singleton argument analysis — 2026-10-10

Analysis only; no rewrites. `cmd/grace-counts` uses the same production inputs as
`TestGraceFullRegistryClosure` and `abapiti abaplint`: 1,538 pinned source files
plus RegistryRun, verified source/package hashes, embedded reachability,
fingerprinted registry overrides and assume-int. The measured HIR has 1,927
classes, 73 interfaces, 6,257 defined methods and 42,766 Grace receiver facts;
zero blocking diagnostics and verification errors. Lowering took 8.23 seconds;
Grace finished at 9.56 seconds from process start. Counting follows analysis.

## Result

There are **21 S1 call/argument sites: 4 qualify for all receivers, 5 partially
qualify, 12 qualify for none**. Sequence has one additional chained row, excluded
from the S1 denominator: only its first iteration receives the local singleton;
later iterations receive the preceding callee's result.

| Statements combi caller | Loop | All | Some | None | Chained |
| --- | ---: | ---: | ---: | ---: | ---: |
| Expression (hot) | yes | 0 | 1 | 0 | 0 |
| Sequence (hot, first call) | yes | 0 | 1 | 0 | 1 |
| Optional (hot) | yes | 0 | 1 | 0 | 0 |
| OptionalPriority | yes | 0 | 1 | 0 | 0 |
| Combi | no | 0 | 1 | 0 | 0 |

All five interface sites have **277 receiver classes, 267 eligible (96.4%)**.
Inherited expression receivers share the eligible Expression.run implementation.
The ten ineligible implementations are Alternative, AlternativePriority,
LangVersNot, Permutation, Plus, PlusPriority, Star, StarPriority, Vers and
WordSequence. Star and StarPriority alias the input in a variable initializer;
the others first forward it to another virtual call. Parameter forwarding is
explicitly outside the requested only-iterated criterion, even when a forwarded
callee itself qualifies.

Profile #5 hot classes with eligible run parameters are Expression, Sequence,
Token, Word and Optional. Vers, Star, Alternative, Plus, Permutation and
WordSequence fail the criterion. FailCombinator and the reachability-pruned
LangVers implementation do not read their parameter and qualify vacuously.

**Verdict: 0/3 hot-class S1 sites (0%) are fully covered; 3/3 (100%) are partially
covered.** A virtual `m_one(e)` implementation for eligible receivers and default
`m_one(e) = m([e])` for the ten others covers all three sites conditionally on
receiver dispatch. Sequence still needs its ordinary array call for subsequent
chain iterations. The profile's array-creation ~22.9s and LOOP AT ~55.8s net are
motivation, not predicted savings: these static counts contain no execution
frequencies and do not establish the runtime share of eligible receivers.

## Evidence and reproduction

* [sites.csv](sites.csv): 22 rows, including the explicitly not-singleton chained
  row; stable Grace site paths, normalized TypeScript locations, parameter index,
  loop depth, source form, receiver classes, dispatch targets and verdicts.
* [methods.csv](methods.csv): 32 unique method/parameter rows, each with eligibility,
  foreach uses, first blocking HIR path, and Grace escape/alias/flow evidence.
* [tables.txt](tables.txt): complete caller-stage/hot-class and receiver-method
  tables printed by the command. Zero rows retain lexer, rules and hot classes
  without singleton sites.
* [Command documentation](../../../cmd/grace-counts/README.md) describes conservative
  recognition and the interpretation of element-flow escape evidence.

```sh
GOCACHE="$HOME/.cache/abapiti-go-cache" GOTMPDIR="$HOME/.cache/grace-tmp" \
  flock /tmp/abapiti-heavy.lock go run ./cmd/grace-counts \
  -output "$HOME/.cache/grace-counts"
```

Passed: `go vet ./...` and `go test -short ./cmd/... ./hir/rewrite/...`, under
`flock /tmp/abapiti-heavy.lock`. Focused regressions cover mixed receiver sets,
production Seq-wrapped literals, first/chained calls, multi-element arrays,
prior container calls, lexical shadowing, blocking use kinds and source literal
provenance. A CSV audit reconciled every eligible-receiver count with methods.csv,
checked unique call/argument/form rows and absence of absolute temporary paths.
