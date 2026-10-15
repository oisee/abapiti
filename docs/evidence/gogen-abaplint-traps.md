# Gogen full-core lowering regression

The comparison uses `origin/selfhost` (`9b7aef5`) and
`fix/gogen-abaplint-traps`, each built with Go 1.26.0, then:

```sh
abapiti abaplint /home/alice/dev/abaplint -o OUT --target go --evidence
```

The core is pinned to abaplint `577f875e`. This is the default **unpruned,
binary64, trap-on-failure** build, not the reachability-pruned Grace workload.

## Causes and repair

* A6's resolved inherited-data classifier counted class property declarations.
  Interfaces such as `IMethodDefinition extends Identifier` therefore became
  data classes because `Identifier` has private/protected storage. Method calls
  then became function-slot reads, and references disagreed with existing
  interface annotations. Only inherited property signatures select the data-row
  path. Uninstantiated property annotations retain their existing ABI; mapped
  and instantiated members still use their resolved types.
* A9 canonicalizes anonymous property order. `shapeAliasOf` still compared
  names by position, so the checker-inferred `{type, object}` result of
  `DDIC.lookupDDLS` disagreed with the fingerprinted nominal `ILookupResult`
  return override. Locals in three callers acquired incompatible storage.
  Alias recognition now compares the same field-name set regardless of order.
  Canonical shape synthesis and generic alias resolution remain enabled.

## Trap baseline

The train emitted 14 `assignment type mismatch` occurrences; the repaired
compiler emits exactly 2, both `LanguageServer.prepareRename` at
`src/lsp/language_server.ts:83:3` (ordinary and profile variants). No new trapped
bodies remain: 163 on the train become the same 5 bodies as selfhost.

The grep count describes assignment failures, not every lowering failure.
Selfhost also traps `CodeActions.find` at `src/lsp/code_actions.ts:17:3`
(`missing expression`, emitted twice), and deliberately refuses the three
`debugDescription` bodies in `SpaghettiScopeNode`, `TypedIdentifier`, and
`ABAPObject`. The regression pins all five bodies and their exact reasons,
normalizing only HIR node IDs. It rejects additions, reason changes, and stale
allow-list entries. It runs under `ABAPITI_GRACE_FULL=1` alongside Grace in CI;
the ordinary shards skip it. Applying the test to `origin/feat/gogen-a9`
fails with 158 unexpected trapped bodies (85.72s); it passes on the repaired
branch (94.80s).

## Complete generated-source comparison

Comparison is before replacing `main.go` with the M1 driver. Exactly these
four Go source files differ from selfhost:

| File | Retained intended differences |
| --- | --- |
| `hir.go` | A6 resolved data/method-signature slots, A9 canonical anonymous shape layouts and deduplication, callable JSON omission, literal evaluation temporaries; resulting synthetic names, locals and node IDs change. |
| `hir_profile.go` | The same changes with profiling instrumentation and corresponding site IDs. |
| `literals.go` | Literal entries for changed synthetic shape hashes, closure IDs and runtime descriptor names. |
| `site_profile.go` | Profile-site metadata follows the changed synthetic declarations; redundant shapes disappear. |

`main.go`, `promise.go`, `runtime.go`, `site_profile_off.go`, and `go.mod` are
byte-identical. The closure changes from 2055 classes / 75 interfaces to
2052 classes / 77 interfaces: canonical shapes deduplicate and two callable
signatures become explicit interfaces. This representation change is retained
for the gogen features; it does not introduce additional trapped bodies.

## Validation

* Gogen oracle tests: passed (55.35s).
* `go vet ./tsfront/ ./hir/... ./cmd/abapiti/`: passed.
* `ABAPITI_GRACECHECK=1` lexer facts: passed (7.61s).
* `ABAPITI_GRACE_FULL=1` Grace full closure: passed (286.86s).
* `go test ./tsfront/ ./hir/... ./cmd/abapiti/ ./internal/abapfixture/ -count=1 -timeout 60m`: passed (tsfront 359.87s).
* `gofmt` and `git diff --check`: clean.
* M1: copied `/mnt/ram/abapiti/selfhost-m1/go-multifile/main.go`, built
  `zabaplint-mf`, ran it in `selfhost-m1/osg-check-kit` with `--file files.txt
  --config abaplint.json`; `cmp` against `expected-clean.txt` passed (81 lines,
  SHA-256 `95d37bc666403557ceecbb6f0e12e31867c18d3c2870e105e3ceafb5b9267428`).
