# DDIC local bindings on TS-HG@Go

The full 17-shard execution passes **9,576/11,060 active tests**, with
**FAIL 0**, **1,211 traps**, and **305 not-run** cases.
The original Node oracle passes all 11,060 active tests, with 32 pending. All 625
files are selected, no files are excluded, and the whole-suite title multiset
reconciles. These are actual generated-Go executions; refusals before assertions
remain traps. [Complete machine-readable evidence](ddic-locals.json).

The merged base is `fa19c4edc1495cee20a4dfa277ccade34f33e7cd`, with parents `648101e` and `2169c5d`.
The general frontend root fix is `30f95f33829491d5f82e976d2478ca2afbbc29ea`. Compiler source and
measurement revisions are recorded in the JSON. The full-suite compiler's SHA256
matches the frozen after compiler exactly. Upstream stays pinned and
read-only at `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

## Root fixed

`DDIC.textToType` at `src/ddic.ts:353` lowers the nine packed-type switch labels
into separate branches containing copies of the same source block. Its
`packedLength` and `decimals` declarations share checker symbols across those
copies. The frontend naming pass used before numeric range analysis previously
assigned each declaration a fresh name, then rebound every read to the last
copy's name. Earlier branches therefore
failed HIR verification at `src/ddic.ts:388`. The test harness then refused
the entire method, trapping callers requesting other DDIC types as well.

The pass now records the first name chosen for each checker binding and reuses
it for every later declaration and reference of that binding. Distinct shadowed
symbols remain distinct. There is no DDIC-specific override or backend patch.
Const, let, destructuring, and nested shadowing regressions verify HIR and both
emitters; the nested-shadowing execution matches original Node.

The clean merge also exposed a stale qualified-name refusal test: batch B now
supports its nominal downcast through a checked Go view. Its fixture now uses an
unrelated class cast, retaining the refusal and qualified `N.const` alias check.
The stale fixture failed with the baseline frontend restored through a Go source
overlay; the corrected fixture passes. Cast lowering itself is unchanged.

## Affected test shards

| Shard | Revision | Pass | FAIL | Trap | Not-run |
| --- | --- | ---: | ---: | ---: | ---: |
| abap/syntax | Merged base | 1,275 | 0 | 49 | 49 |
| abap/syntax | After | 1,302 | 0 | 22 | 49 |
| rules | Merged base | 2,301 | 0 | 836 | 22 |
| rules | After | 2,316 | 0 | 821 | 22 |

The exact formerly-DDIC-trapped titles and their after statuses are retained in
`ddicTestDelta.freedTests`. The merged baseline reaches this trap in 27 syntax
tests and 57 rules tests
(84 total); the rules count exceeds the task's earlier 14-test estimate. Every
newly reached refusal is measured, not inferred from static diagnostics. All 84
DDIC-local refusals disappear: 42 tests now pass and 42 reach another trap.

Next trap(s) reached by those tests:

| Source | Next trap | Tests |
| --- | --- | ---: |
| `src/rules/xml_consistency.ts:51` | Unreviewed JavaScript regexp | 20 |
| `src/rules/7bit_ascii.ts:38` | Value outside supported numeric, ordering or conversion domain | 19 |
| `src/rules/downport.ts:713` | Runtime argument expects `ExpressionNode`, receives `array<ExpressionNode>` | 2 |
| `src/rules/downport.ts:3174-3175` | `crypto` and `hash` identifiers are not lowered | 1 |

Exact messages are retained in `ddicTestDelta.nextTraps` in the JSON.

## Whole suite after the fix

| Shard | Pass | FAIL | Trap | Not-run |
| --- | ---: | ---: | ---: | ---: |
| abap | 55 | 0 | 26 | 139 |
| abap/expressions | 41 | 0 | 9 | 57 |
| abap/lexer | 200 | 0 | 0 | 0 |
| abap/nodes | 2 | 0 | 0 | 0 |
| abap/statements | 4,942 | 0 | 20 | 0 |
| abap/structures | 138 | 0 | 0 | 2 |
| abap/syntax | 1,302 | 0 | 22 | 49 |
| abap/types | 21 | 0 | 8 | 0 |
| root | 61 | 0 | 33 | 0 |
| cds | 334 | 0 | 24 | 1 |
| ddl | 4 | 0 | 28 | 0 |
| lsp | 74 | 0 | 114 | 5 |
| objects | 78 | 0 | 66 | 1 |
| objects/rename | 0 | 0 | 31 | 0 |
| pretty_printer | 0 | 0 | 2 | 29 |
| rules | 2,316 | 0 | 821 | 22 |
| utils | 8 | 0 | 7 | 0 |
| **Total** | **9,576** | **0** | **1,211** | **305** |

Async-pending is zero. Registration losses and pending/skipped tests remain
not-run; no failed record is reclassified. The 305 not-run cases include the
32 original pending tests; active not-run is 273. The before measurement selects only
syntax and rules. An earlier full-before attempt completed `abap` at 55/0/26/139
before it was stopped to focus on the requested shards; it is not presented as a
completed full-suite baseline.

## Full-closure gap

Both modes use `abaplint --full`, with no reachability manifest. Exact blocking
message histograms and HIR message histograms are in `fullClosure` in the JSON.

| Mode | Measure | Before | After |
| --- | --- | ---: | ---: |
| Binary64 | Lowering blockers | 32 | 32 |
| Binary64 | Total HIR errors | 100 | 84 |
| Binary64 | DDIC unresolved locals | 16 | 0 |
| Binary64 | All unresolved locals | 20 | 4 |
| Default integer | Lowering blockers | 50 | 50 |
| Default integer | Total HIR errors | 127 | 127 |

Default integer mode blocks DDIC earlier at the existing `Math.ceil` integral
proof refusal. Binary64 matches the test harness and exposes the binding category
directly. The four remaining unresolved locals refer to mutable `fix` inside
Downport's `addFix` closure (`src/rules/downport.ts:730-733`), a separate capture
problem. Other remaining lowering roots are in the exact histograms and reached
traps in the full-after summary; the excluded ownership categories were not changed.

## Gates and output

- `go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...`: PASS.
- `go test -short -p 2 -timeout 60m` on the same packages: PASS.
- `ABAPITI_GRACECHECK=1` lexer-facts golden: PASS, unchanged.
- Backend guard `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`: PASS
  in the normal short gate, together with the `hir/abap` runtime-op guard.
- `./.github/ci/lint.sh gate fa19c4edc1495cee20a4dfa277ccade34f33e7cd`: 0 new issues, canary 5/5.
- Harness selection, refusal, and full-suite reconciliation tests: PASS.
- Default `--target all,go` versus merged base: empty `diff -r`, all 6,348 files
  byte-identical, including Go executable and ABAP packages.

The original combined short/Grace attempt timed out at Go's default ten minutes
and exposed the stale cast fixture. The final clean short gate and explicit Grace
golden both pass; the failed attempt is not counted as a passing gate.

The first whole-suite Node oracle had three wall-clock timeouts (2000 ms,
250 ms, 2000 ms) on the shared host. Every shard's Node oracle passed. A fresh
whole-suite build and rerun, with assertions and timeout limits unchanged, passed
all 11,060 active tests and 32 pending. The retained Go records were unchanged;
`abaplint-tests-summary.mjs` then passed its exact whole-suite count and title
multiset guards. Initial timeout messages and rerun totals are in
`wholeSuiteOracleRetry`; no failed record was relabeled.

The additional origin/main snapshot `0b4cd7662cadd4ab385a8d0144da19eb43855579` comparison
has 29 differing paths inherited from the requested integration merge. Their
exact list is in `default.mainToBranch`; branch output equals merged-base output,
so this fix introduces none of those differences.

## Commands

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$HOME/.local/bin:$PATH"
export GOMAXPROCS=2 GOFLAGS='-p=2 -buildvcs=false'
# Freeze the merged-base compiler before applying the fix:
go build -o /tmp/ddic-evidence/base-abapiti ./cmd/abapiti
# On the fixed branch, use that immutable baseline compiler:
ABAPITI_TESTS_SHARDS='abap/syntax,rules' \
ABAPITI_TESTS_COMPILER=/tmp/ddic-evidence/base-abapiti \
ABAPITI_TESTS_COMPILER_COMMIT=fa19c4edc1495cee20a4dfa277ccade34f33e7cd \
node tools/abaplint-tests-all.mjs ~/.cache/kits/abaplint-577f875e /tmp/ddic-evidence/base-tests.json
node tools/abaplint-tests-all.mjs ~/.cache/kits/abaplint-577f875e /tmp/ddic-evidence/after-tests.json

go build -o /tmp/ddic-evidence/after-abapiti ./cmd/abapiti
/tmp/ddic-evidence/base-abapiti abaplint --full -o /tmp/ddic-evidence/full-base
/tmp/ddic-evidence/after-abapiti abaplint --full -o /tmp/ddic-evidence/full-after
/tmp/ddic-evidence/base-abapiti abaplint --full --assume-int=false -o /tmp/ddic-evidence/full-number-base
/tmp/ddic-evidence/after-abapiti abaplint --full --assume-int=false -o /tmp/ddic-evidence/full-number-after

# Each compiler writes the same path, then its output is moved aside:
/tmp/ddic-evidence/base-abapiti abaplint -o /tmp/ddic-evidence/default-out --target all,go
mv /tmp/ddic-evidence/default-out /tmp/ddic-evidence/default-base
/tmp/ddic-evidence/after-abapiti abaplint -o /tmp/ddic-evidence/default-out --target all,go
mv /tmp/ddic-evidence/default-out /tmp/ddic-evidence/default-after
diff -r /tmp/ddic-evidence/default-base /tmp/ddic-evidence/default-after
```

The completed runner retained its run after the first oracle reconciliation
failure. The fresh original whole-suite rerun replaced only `node-all.json`,
followed by strict reconciliation:

```sh
node --input-type=module <<'JS'
import {readFileSync, writeFileSync} from 'node:fs';
import {buildUpstream} from './tools/statements-upstream.mjs';
import {executeOracle} from './tools/abaplint-tests-oracle.mjs';
const run = '/tmp/abapiti-test-shards/all-R0ZJBi';
const upstream = process.env.HOME + '/.cache/kits/abaplint-577f875e';
const plan = JSON.parse(readFileSync(run + '/shards.json', 'utf8'));
const build = buildUpstream(upstream);
try {
  const result = executeOracle(build, upstream, plan.files);
  if (result.failed) throw new Error('whole-suite original Node failure');
  writeFileSync(run + '/node-all.json', JSON.stringify(result));
} finally { build.dispose(); }
JS
node tools/abaplint-tests-summary.mjs /tmp/abapiti-test-shards/all-R0ZJBi /tmp/ddic-evidence/after-tests.json
```

The actual full-after execution started with `GOMAXPROCS=1`; later compiler
invocations used `GOMAXPROCS=2` through a task-local launcher of the unchanged
compiler binary. Go retained `-p=2` throughout. The transition time and compiler
SHA256 are recorded in the JSON. The additional main executable was built from
an isolated source archive
of the recorded snapshot, then used the same output-path spelling before its
output was moved to `default-main` and compared with `default-after`.
