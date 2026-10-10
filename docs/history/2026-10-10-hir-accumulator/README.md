# HIR accumulator evidence

Opt-in `ABAPITI_ACCUMULATOR=1` specializes fresh append-only array producers
and whole-array append consumers before both Go and ABAP emission. Default is
OFF. Array arguments retain reference identity. Variant families use name and
signature, include direct and dispatching default implementations, and never
mark the added interface method Abstract. Calls are rewritten before defaults
are appended. `hir.Verify` runs after the pass.

Measured source: bundled abaplint commit `577f875e`, core 2.120.56.
Default comparison: `origin/main` = `f9d5f8c`, generated into the identical output
path with the same toolchain/environment before taking a baseline copy.
`diff -r baseline off` is empty across Go sources/binary, native ABAP classes,
interfaces and maps. Profiling binaries are separate measurement artifacts.

## Static transformation coverage

16 consumers transformed, 21 direct producer clones, 146 default variants.
The site CSV records every original structural HIR path and exception proof.
Stage and per-class combinator CSVs include zero rows.

| Stage | Sites | Transitively nonthrowing | Private output dead on exception |
|---|---:|---:|---:|
| Lexer | 0 | 0 | 0 |
| Statements | 4 | 0 | 4 |
| Structures | 0 | 0 | 0 |
| Syntax | 0 | 0 | 0 |
| Rules | 2 | 0 | 2 |
| Other | 10 | 1 | 9 |
| Total | 16 | 1 | 15 |

Statement consumers: Alternative 1, Optional 1, Permutation 2. Other combinator
classes have zero sites. Sequence's reassignment consumer is out of scope.
AlternativePriority/OptionalPriority and other priority/repetition consumers
inspect or choose child results; their general iteration is not transformed.
A method can still have a producer variant without having a transformed consumer
inside its own body. This explains allocation savings in additional classes.

## Validation

All green on the final implementation:

- `go vet ./...`
- `go test -short ./...`
- `./.github/ci/lint.sh gate origin/main`: zero new issues, canary 5/5;
  `~/.local/bin/golangci-lint` 2.13.2.
- `ABAPITI_ACCUMULATOR=1 go test -short ./hir/...`
- `ABAPITI_ACCUMULATOR=1 go test ./tsfront -run '^(TestGoLexerDifferential|TestLowerLexerClosure)$' -count=1 -v`:
  44/44 lexer cases agree, token mutation rejected; emitted closure differential
  driver has 44 cases and 17,057 lines.
- Full flag-ON Go and native ABAP generation verifies HIR and succeeds. Both
  backends report the same 16 original sites. Native emits 2,036 classes and
  75 interfaces. A4H/OSGO execution belongs to abapiti and was not run here.
- Synthetic guards include result reads/escape, output alias/read/extra use,
  live/dead output on throws, transitive throws, handlers, polymorphic
  direct/default dispatch, signature separation, binding collisions, shared
  expression occurrences, frontend initializer and spread shapes. Used Grace
  facts agree with the independent reference evaluator. Generated mixed
  receiver Go tests execute both flag states and preserve element order.

## Reproduction

Use `GOCACHE=$HOME/.cache/abapiti-go-cache`, `GOFLAGS=-buildvcs=false`,
`GOMAXPROCS=4`. Every heavy build/test/measurement uses
`flock /tmp/abapiti-heavy.lock`. Temporary files live under the clone.

Generate OFF and ON with the same freshly built CLI:

```sh
ABAPITI_ACCUMULATOR=0 abapiti abaplint --target go,native -o .local/accumulator/off
ABAPITI_ACCUMULATOR=1 ABAPITI_ACCUMULATOR_STATS=1 abapiti abaplint --target go -o .local/accumulator/on
ABAPITI_ACCUMULATOR=1 ABAPITI_ACCUMULATOR_STATS=1 abapiti abaplint --target native -o .local/accumulator/abap-on
flock /tmp/abapiti-heavy.lock python3 tools/accumulator-measure.py \
  .local/accumulator "$HOME/dev/dell-work/kits/zabapgit-check-kit" \
  "$HOME/.cache/abapiti-target-go/upstream/abaplint/packages/core"
```

The runner performs three OFF/ON pairs per input, alternating pair order, before
building either profile target. Each output must equal the kit's expected bytes.
`--metrics` measures check time; CPU profiles supply sampled GC share (GC worker,
assist, sweep/scavenge frames), not wall-time fraction or MemStats GCCPUFraction.
The median of three is reported separately for each flag/input. GOGC is default.
Profile builds use `go build -tags profile_sites`. Four profile certificates
must have matching input SHA-256 and schema `site-profile/1`; raw certificates
and logs remain under `.local/accumulator` (not committed).

Array-only CSVs map each original `new` site to its pinned TypeScript location
and retain array literals/new Array. Counts are allocation executions, not bytes.
Cloned producers keep their original site identity; `_combi.ts` structure sites
are included and unchanged. These allocation reductions are exact for this kit,
while the timing samples are noisy and do not establish a general speedup.

## Measured results

| Input | OFF seconds | ON seconds | OFF sampled GC | ON sampled GC |
|---|---:|---:|---:|---:|
| clean | 10.858 | 10.121 | 40.88% | 39.30% |
| seeded | 11.493 | 10.903 | 40.63% | 40.32% |

| Input | OFF combinator array allocations | ON | Delta |
|---|---:|---:|---:|
| clean | 33,779,740 | 33,346,820 | -432,920 |
| seeded | 33,781,982 | 33,349,019 | -432,963 |
