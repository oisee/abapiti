# Live rule-config class views

Implementation base: `b89c039` (the supplied train-3/traps-B branch plus the
no-fast-forward traps-C merge). Upstream oracle: clean abaplint
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`, TypeScript 6.0.3, Node 26.9.0.
The historical 173/72/22/17 buckets in `traps-c-roots.md` are not a current
before/after measurement.

## Storage and boundary

Unpruned TS lowering emits `dynamic.classView` for an erased dynamic parameter
whose implementation expects a named class. Both emitters first retain a
compatible native reference. Otherwise an eligible nominal layout becomes a
view over the original structural record. Inherited field accesses use proxy
reads/writes; boxing a view returns its backing record. Equality therefore
compares the original storage, including two separate rules viewing one object.
Plain and parsed sources do not acquire the nominal prototype or constructor
initializers. Optional absence and extra properties remain in source storage.

Eligibility is shared HIR policy: public data ancestry, known acyclic bases,
no accessors, private fields, or prototype-dependent behavior. Unused simple
field-reader declarations may coexist with a view; a closed program that
actually dispatches a prototype method refuses conversion before inlining.
Native instances of behavior-bearing classes remain valid. Unsupported field
payload conversions still raise typed refusals; parsed arrays do not silently
become snapshots of native typed arrays. Explicit nominal assertions retain
their separate checked boundary.

The ABI and structural adapters are usage-gated. Default reachability builds
retain the original lowering and emitted storage. Two prerequisite integration
repairs accompany this change: the missing SHA-1 runtime effect catalog entry
and a stale qualified-alias test, which now executes the existing checked
nominal assertion and requires an invalid native instance to refuse.

## Oracle and gates

`TestLiveClassViewNodeOracle` executes fresh Node and generated Go for native,
plain and JSON-parsed sources; identity; writes through caller and rule;
two rules sharing storage; inherited fields; optional reads and writes;
missing fields without defaults; extra properties; constructor counts; and
native/structural `instanceof`; and native array sharing, element mutation and
reference replacement through an existing field. Its exported ABAP unit test uses the same Node
answer. `TestLiveClassViewRefusesPrototypeStorage` retains native identity and
requires typed Go refusals for behavior, getter, private and hash-private cases.
Existing checked nominal-view tests remain separate.

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$HOME/.local/bin:$PATH"
export GOMAXPROCS=1
export GOFLAGS='-p=2 -buildvcs=false'
go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -p 2 -timeout 30m ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 go test -p 1 ./tsfront ./hir ./hir/abap \
  -run '^(TestLiveClassView.*|TestDataClassViewBoundary|TestGoCheckedNominalClassViewNodeOracle|TestLexerFactsReport|TestRuntimeOpsSupportedByEveryBackend|TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend)$' -count=1
./.github/ci/lint.sh gate b89c039
ABAPITI_TEST_OUT=/tmp/live-class-view/abap-oracle go test -p 1 ./tsfront \
  -run '^TestLiveClassViewNodeOracle$' -count=1
node .github/ci/hir-lint.mjs /tmp/live-class-view/abap-oracle /tmp/train3-ci/open-steamgate --fixture
# In the open-steamgate checkout:
npm run -s osgjs:unit -- /tmp/live-class-view/abap-oracle --json
```

## Default build comparison

Both compilers ran `abapiti abaplint /tmp/config-json-upstream --target all,go`
with `-o /tmp/live-class-view/default-check-base/output`. The baseline output
was preserved at `/tmp/live-class-view/default-snapshot` before the final
branch compiler regenerated that exact output directory. This avoids different
absolute paths in generated READMEs and Go debug metadata; no artifacts were
normalized or substituted. `diff -r` between the snapshot and branch output
returned 0 with an empty diff, including binaries and abapGit packages.

The separately built `origin/main` compiler at `b5a191b` already differs from
`b89c039` in inherited train-3/traps-B/traps-C output. The empty gate here is
against the task's supplied merged base; this change does not claim to erase
those prerequisite differences from origin/main.

## Full closure

`abaplint --full` is present in origin/main. Both runs lower the entire pinned
closure without the reachability manifest. Blocking diagnostics stay **48 → 48**
(**35 roots + 13 cascades**); HIR errors stay **123 → 123**, in 33 files. This
runtime class-conversion root contributes **0 → 0 lowering diagnostics**.
Every root diagnostic message/count is unchanged; see
[live-class-view-full-gap.json](live-class-view-full-gap.json) for commands and
counts by message. Fractional numeric evidence, LSP calls, JSON stringify
arguments, array callback absence, coercions, and unrelated expression roots
remain outside this change.

The final expanded oracle passes in Go and in the ABAP runtime (1 composed
unit test, 0 failures, 0 compile errors), with 15 ABAP files and 0 lint issues.
Runtime/transpiler provenance is 2.13.93 with Node 26.9.0. Native reference
payloads write the original structural slot; unsupported known field types
refuse rather than shadowing that slot in the extra-property bag.

## Final upstream measurement

| Shard | Pass before → after | Trap before → after | Not-run before → after | FAIL |
| --- | ---: | ---: | ---: | ---: |
| rules (3,159 Node cases) | 2,649 → 2,659 | 488 → 488 | 22 → 12 | 0 → 0 |
| root (94 Node cases) | 61 → 63 | 33 → 31 | 0 → 0 | 0 → 0 |

Async-pending is 0; both final runs have no infrastructure failures or unmatched
registrations. Rules registrations rise from 3,147 to 3,157: the two
keyword_case initialization refusals disappear, registering ten passing tests.
The line_length initialization refusal remains. Config improves from 2 passes
and 5 traps to 4 passes and 3 traps. Obsolete_statement/avoid_use ancestry gains
already belong to the prerequisite base; no new gain is attributed to them.

The SevenBitAscii erased-setter refusal falls **142 → 0** in rules and
**15 → 0** in root. ShortCase's root setter refusal also disappears. These
class conversions now retain the original object and live data storage.
Most newly reached global-config cases still refuse at
`ClearExportingParameters.setConfig`: **142 rules + 14 root cases**.
Its `skipNames?: string[]` read encounters a parsed dynamic-element array,
which cannot be cast to native typed-array storage while retaining shared
identity and writes. That needs a separate live typed-array storage bridge;
materializing a copy would break the contract. The class view deliberately
keeps that payload refusal. This is not 157 new passing tests.

Other leading rules traps remain data_definition conditional verification
(71), xml_consistency regexp (59), downport unresolved fix (26), domain empty
arrays (22), object_naming dynamic regexp (18), and selection_screen assignment
verification (17). Root still has registry chai.throw/captured getConfig,
Config applyUnspecifiedRules assignment and JSON.stringify arguments, and
severity captured-getConfig refusals. Those roots were not widened or relabeled.

See [live-class-view-shards.json](live-class-view-shards.json) for exact commands,
counts, initialization records, and top trap messages. Measurement used temporary
selected-shard copies of `tools/abaplint-tests-all.mjs`: before rules from the
archived base, after root + rules from this branch. They retained fresh original
Node builds/oracles, all source roots in each selected shard, compiler/runtime
fixtures, ownership reconciliation and genuine-FAIL checks. Only the plan
filter and selected-report aggregation/retention changed; no source or emitted
Go overlays were used. The before root run used the same materializer and CLI,
reconciled against the independent root Node oracle. Temporary runner copies
were removed after recording results.
