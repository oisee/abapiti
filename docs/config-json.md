# Config.getDefault JSON round trip

Without a reachability manifest, this change replaces the source-pinned `src/config.ts:69` trap with
`json.stringifyConfig(config)`. It writes strict JSON and calls the original
`new Config(...)` path. The existing constructor adapter parses that JSON into
a recursive tagged graph, materializes the typed `IConfig` fields, and retains
all graph fields as backing metadata. The parsed `syntax.version` union projects
object values to native `VersionNew` data while retaining their source graph;
string and absent values keep their original tags. This makes the existing
narrowed release/language reads consume the typed JSON result. Constructor defaults, deduplication, and
version checks continue to run. This is an actual stringify/parse round trip,
not a replacement clone or a constructor bypass.

The writer is deliberately a package adapter for the pinned upstream
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. HIR data-shape field order does not
retain JavaScript object-literal insertion order. The adapter supplies the
orders proved by the pinned default-config literals: global, dependencies,
syntax, rules; their nested records; version release/language; recommendation
name/replace/from; and pragma pseudo/pragma rows. Rule configuration classes
write base declarations before derived declarations, with a redeclared field
replacing its value at the original position. Records sort canonical array-index
keys numerically and retain insertion order for other strings. Undefined object
properties are omitted; undefined array entries become null. Strings preserve
UTF-16 units, escape controls and isolated surrogates, and leave paired
surrogates, HTML characters, and U+2028 unescaped as V8 does. Numbers use
JavaScript JSON's fixed/exponent thresholds and normalize negative zero.

The default graph has 194 rule configurations, including the recommendation
and pragma arrays. A fresh original upstream build supplies the byte oracle.
All 194 default configuration classes have no explicit constructors; their
base-to-derived initialized declaration keys match V8 own-key order exactly.
`tools/config-json-oracle.mjs` checks this against original source and runtime
objects before writing its evidence. Unit differentials also cover fractional/exponent numbers, non-finite values,
undefined, key ordering, controls, surrogate pairs and isolated surrogates,
inherited fields, and mutation independence after parsing.

A supplied native `VersionNew` input without retained JSON backing is refused
when no language override rebuilds the version record: its literal key order
was erased upstream. Tagged JSON inputs retain their original key order.
Unsupported native shapes, `toJSON`, cycles, out-of-domain assumed integers,
and fields outside a pinned default literal refuse explicitly. General
`JSON.stringify` call sites are outside this adapter. The frozen, manifest-driven zabapgit build retains its original observation-only
trap to preserve every default artifact. A validated override variant enables
the writer only for lowering without that manifest, including full-closure
measurement and the Go test harness. The Go and ABAP writers are emitted only when this operation is present.
ABAP now implements the same config round trip; its native default graph,
typed projection, nesting, numeric formatting, escaping, undefined values,
and cycle refusal are checked against Node on osgo and OSG-JS. See
[the ABAP implementation and gates](config-json-abap.md).

## Reproduce

Use Node on PATH and cap Go builds/tests at two packages:

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"
export GOFLAGS=-p=2 GOMAXPROCS=2
go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
```

Full-closure commands, with executables built from the recorded baseline and
this branch, are:

```sh
/tmp/config-json-main-bin abaplint --full -o /tmp/config-json-before
/tmp/config-json-branch-bin abaplint --full -o /tmp/config-json-after
```

The default-output comparison uses the same output spelling for both builds,
since generated READMEs include it:

```sh
/tmp/config-json-main-bin abaplint -o /tmp/config-json-default --target all,go
cp -a /tmp/config-json-default /tmp/config-json-default-before
rm -rf /tmp/config-json-default
/tmp/config-json-branch-bin abaplint -o /tmp/config-json-default --target all,go
diff -r /tmp/config-json-default-before /tmp/config-json-default
```

Harness measurements use a disposable copy of
`origin/feat/abaplint-tests-harness` at `41c7a9a`, merged with the recorded main
baseline. The optional-type merge conflict takes main's implementation. Build
and run that copy before applying this task's patch; then apply only this task's
patch to the disposable copy and build/run again. No harness commits or sources
are added to this branch. Use the harness's `abaplint-tests-closure.mjs`,
`abaplint-tests-oracle.mjs`, and `abaplint-tests-report.mjs` as documented in its
`docs/abaplint-tests.md`, with all sorted `test/rules/*.ts` paths, including
helpers. The measured host commands are:

```sh
/tmp/config-json-measure-before abaplint-tests /tmp/config-json-rules \
  -o /tmp/config-json-rules-before
/tmp/config-json-measure-final3 abaplint-tests /tmp/config-json-rules \
  -o /tmp/config-json-rules-final3
```

Full-closure blocking diagnostics remain 373 (155 root, 218 cascade), with
291 HIR errors, before and after. Config has zero compile-time blocking
diagnostics in both runs: the pinned trap was a runtime limitation. The four
remaining general JSON identifier diagnostics belong to other call sites.
The default `all,go` output comparison is empty; vet and the requested short
package tests pass with `-p 2`.

The current integrated harness baseline reaches Config's trap in 211 tests.
The historical harness evidence reported 250, but registered tests changed
from 2730 to 2671 when integrating this main baseline. All counts, diagnostic
message histograms, and remaining reached traps are recorded in
[evidence/config-json-report.json](evidence/config-json-report.json).

The complete default-config byte oracle was produced from a fresh original TS
build (not existing upstream build output):

```sh
node tools/config-json-oracle.mjs /tmp/config-json-upstream /tmp/config-json-oracle
```

## Rules shard result

| Result | Before | After |
| --- | ---: | ---: |
| Passed | 0 | 5 |
| Failed | 0 | 0 |
| Trapped | 1295 | 1290 |
| Not run | 1864 | 1864 |
| Config JSON trap | 211 | 0 |

All 211 Config JSON traps are removed. Five tests pass, 196 now reach an
existing HIR call-arity mismatch at `find_global_definitions.ts:39`, and ten
reach the existing `artifacts_objects.ts:8` trap. The other reached traps are
1066 `_utils.ts` forEach receiver-proof traps and 18 unrelated JSON calls.
These are outside this task. The synchronous host excludes 1366 async/done
tests, ten are skipped, and 488 are unregistered. The independent Node oracle
passes 3149 tests with ten pending.

The final compiled-module probe checks the complete default JSON against the
fresh V8 oracle, mutation independence, `getRelease().name == "Newest"`, and
`getLanguageVersion() == "Normal"`. It uses the generated rules module and
runs `go test -p 2 -run TestCompiledConfigDefaultRoundTrip -v`.
