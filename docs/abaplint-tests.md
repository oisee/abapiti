# abaplint's own tests on TS-HG@Go

## First N/total (trap-on-failure build)

Pinned upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e` (@abaplint/core
2.120.56). These are **actual original-mocha Node runs and generated Go binary
runs**, not estimated coverage. Every selected `.ts` file, including helpers,
was supplied in sorted path order to both hosts.

| Shard | Files selected | Passed | Failed | Trapped | Not-run | Node total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `rules/7bit_ascii.ts` (smoke; included in rules below) | 1 | 0 | 0 | 7 | 0 | 7 |
| All `test/rules/` | 192 | 0 | 0 | 1,338 | 1,821 | 3,159 |
| `test/abap/lexer/` | 3 | 4 | 17 | 0 | 179 | 200 |
| `test/abap/statements/` | 286 | 0 | 0 | 0 | 4,962 | 4,962 |
| `test/abap/structures/` | 30 | 23 | 1 | 0 | 116 | 140 |
| **ABAP group subtotal (three separate binaries)** | **319** | **27** | **18** | **0** | **5,257** | **5,302** |
| **Unique tests, rules + ABAP group** | **511** | **27** | **18** | **1,338** | **7,078** | **8,461** |

The pinned test directories are named `lexer`, `statements`, and `structures`;
the corresponding source directories are `1_lexer`, `2_statements`, and
`3_structures`. They are built **per directory**, not combined into an enormous
Go binary. Rules are one shard as requested. This is **27/8,461**, not 27 out of
abaplint's entire test suite. The smoke run is not counted twice.

Node passed all selected non-pending tests: rules 3,149 passed / 10 pending;
lexer 200 passed; statements 4,962 passed; structures 139 passed / 1 pending. Go registered
2,730 rule tests: 1,338 trapped, 1,382 async callbacks not run, and 10 skipped.
The other 429 rule tests did not register. Lexer registered 21 tests,
statements zero, structures 24. Registration/module-initialization failures
remain an important limitation; **unregistered tests are not counted as failed
or trapped tests**.

The 17 lexer failures report `runtime/chai.ts:28:3: value outside supported
numeric, ordering or conversion domain`; the structure failure reports the
same runtime refusal at `test/abap/structures/_combi.ts:37:20`. They are host
non-trap failures, not necessarily incorrect upstream assertions. Do not treat
this first table as semantic certification of the assertion shim.

### Top 20 traps, ranked by failing tests (only three distinct messages)

Counts below are reached test traps, not compiler diagnostic occurrences.
Creation locations are removed when grouping; JSON reports retain an actual
example with its TS location. No other shard had a reached trap.

| Rank | Rules tests | Trap message / location | Next fix |
| --- | ---: | --- | --- |
| 1 | 1,070 | `test/rules/_utils.ts:47:11: forEach needs a source-pinned dense/stable receiver proof` | The issue-fix loop in the test callback; conservatively traps the whole callback even when that branch would not execute |
| 2 | 250 | `src/config.ts:69` (via `src/config.ts:11:3`) | Existing pinned `Config.getDefault` override traps its default-config JSON round trip |
| 3 | 18 | `test/rules/_utils.ts:13:21: identifier JSON is not a lowered local or module value` | `fullErrorNamespace` JSON serialization |

**7bit_ascii: exact failure reason:** all seven original titles registered and
trapped in the `_utils.ts:41:24` callback because of its `_utils.ts:47:11`
`forEach`. It has not reached the rule's two `RegExp.exec` calls yet. There are
no seven passes claimed here.

Major **not-run** registration blockers are distinct from that ranking:

- lexer `count.ts:94:3` and `type.ts:19:3`: registration `forEach` needs a
  dense/stable proof;
- statements `_utils.ts:131:5`: registration `forEach` on `dynamic`; some
  version-case arrays also have incompatible anonymous shapes;
- structures `_utils.ts:105:5`: registration `forEach` proof; `_combi.ts:277:3`
  also has an unsupported local class declaration.

## Reproduce

Run inside this worktree. Use a clean pinned upstream checkout with installed
npm dependencies; no test sources are embedded in the executable. The
materializer refuses an existing closure directory. Use a fresh output
directory as well.

```sh
export GOCACHE="$HOME/.cache/go-build"
export GOTOOLCHAIN=go1.26.0
export GOFLAGS=-buildvcs=false
UPSTREAM=/home/alice/dev/abaplint
RUN=$(mktemp -d)
go build -o "$RUN/abapiti" ./cmd/abapiti

# Smoke first.
node tools/abaplint-tests-closure.mjs "$RUN/seven" "$UPSTREAM" \
  test/rules/7bit_ascii.ts
node tools/abaplint-tests-oracle.mjs "$RUN/node-seven.json" "$UPSTREAM" \
  test/rules/7bit_ascii.ts
"$RUN/abapiti" abaplint-tests "$RUN/seven" -o "$RUN/seven-out" \
  >"$RUN/go-seven.json" 2>"$RUN/go-seven.log" || test "$?" = 1
node tools/abaplint-tests-report.mjs "$RUN/node-seven.json" \
  "$RUN/go-seven.json" "$RUN/report-seven.json"

# Rules shard, followed by the ABAP group's per-directory binaries.
for spec in rules abap/lexer abap/statements abap/structures; do
  name=${spec##*/}
  # Intentional shell splitting: these upstream paths contain no spaces.
  files=$(find "$UPSTREAM/packages/core/test/$spec" -name '*.ts' | \
    sort | perl -pe "s@^$UPSTREAM/packages/core/@@")
  node tools/abaplint-tests-closure.mjs "$RUN/$name" "$UPSTREAM" $files
  node tools/abaplint-tests-oracle.mjs "$RUN/node-$name.json" "$UPSTREAM" $files
  "$RUN/abapiti" abaplint-tests "$RUN/$name" -o "$RUN/$name-out" \
    >"$RUN/go-$name.json" 2>"$RUN/go-$name.log" || test "$?" = 1
  node tools/abaplint-tests-report.mjs "$RUN/node-$name.json" \
    "$RUN/go-$name.json" "$RUN/report-$name.json"
done
```

The first-run artifacts are retained in `/tmp/tests-first/`: `node-*.json`,
`go-*.json`, `go-*.log`, `report-*.json`, generated modules under `*-out/go/`,
and lowering inventories under `*-out/evidence/`. The smoke closure used the
previous verified `scratchpad/tests/closure3`; the other closures were freshly
materialized. `docs/abaplint-tests-first.json` records the compact first table.

The reporter reconciles full titles against Node as a **multiset**, including
duplicate titles. Passed + failed + trapped + not-run must equal Node total.
Not-run includes explicit async/done callbacks, skipped/focus-excluded tests,
and oracle tests that never registered. Unexpected Go titles are emitted as
`unmatched` evidence rather than silently counted; these runs had none.
The Node oracle has a 128 MiB output limit so large mocha JSON runs are not
truncated at Node's default 1 MiB subprocess limit.

## Policy and runtime

`LowerOptions.TrapOnFailure` is opt-in; `LowerAbaplintTests` enables it with
**nil coverage** and explicit binary64 number semantics. `ABAPITI_ASSUME_INT`
does not change this test build. The normal abaplint path, including `--full`
measurement mode, does not enable it.

- Function, method, constructor and escaping-thunk diagnostic boundaries
  replace failed bodies with `hir.Trap`. Nested callback failures do not poison
  the enclosing suite's registration body.
- HIR verifies each body against the complete declaration table. Verification
  failures trap that body, even when synthetic operands have node ID zero.
- Invalid declaration members/signatures are removed rather than guessed;
  retained uses fail verification and trap. Failed static initialization cannot
  expose fabricated zero values: its fields are removed and consumers verified
  again. Unused unsupported declarations (e.g. external namespaces) are dropped
  with evidence. Remaining structural errors still refuse the build.
- Go-target-specific emission failures with attributable HIR nodes (notably
  unsupported regexp patterns) also replace the owning body with a trap.
  Evidence is refreshed after emission to include those traps and constructors.
  `evidence/test-diagnostics.json` also preserves policy notes and dropped-
  declaration diagnostics; `registry-blocking.json` alone is not the inventory
  of failures converted to traps.
- Diagnostic text and original TS locations are carried in trap reasons. This
  is the same uncatchable-by-TS-payload Trap kind used by coverage pruning.
  **Only the Go test boundary** recovers it, counts it, and proceeds to the next
  test; ordinary TypeScript `try/catch` still cannot swallow it.

The Go host invokes registered thunks directly, reporting `{title,passed,
message,status}` to stderr and one JSON summary to stdout. It distinguishes
ordinary panic/assertion failures from typed traps. Exit codes: 0 for no
reached failures, 1 for failed/trapped tests, 2 for a host refusal. A zero-test
summary is not success evidence: the reconciliation report marks the oracle
count not-run.

Tests-only shim integration supports merged `it.only/skip` and
`describe.only/skip`, resolves actual shim symbols, binds `function()` mocha
callbacks to `TestContext`, and lowers the shim's context call. `timeout()` is
a no-op. The pinned `_utils.ts` test-row registration arrays have a narrowly
scoped dense/stable iteration allowance; arbitrary core `forEach` calls are
**not** unconditionally allowed. Optional synthetic-shape subsets and defaulted
constructor parameter-property reads also have tests-only handling. Stored
callbacks capture each loop iteration's value; a Go-host regression exercises
that distinction.

The owned chai shim keeps `equal`/`equals` as strict equality, never substitutes
deep equality. Boxed values use the existing tagged dynamic ABI. Deep chains,
hooks, arbitrary chai methods, async execution/Promise draining, real timers,
and the full missing-property/undefined ABI remain unsupported or uncertified.
Unsupported constructs trap; they are not no-op assertions. Sources in the
upstream test closure are copied unchanged, hashed and verified. There are no
SevenBitAscii, Registry, or upstream assertion-body rewrites.

## Gates

```sh
go test ./tsfront/... ./hir/... ./cmd/...
go vet ./...
node tools/abaplint-tests-report-test.mjs
node tools/abaplint-tests-runtime-test.mjs /home/alice/dev/abaplint
```

Both passed. Rebased the spike onto origin/main `f834c16`. Origin/main then
advanced independently to `7659ba6`; the final comparison was repeated against
that revision as well. Built origin/main executables from `git archive`,
generated the **default all-target build** from
both executables with the same `-o` spelling (driver READMEs embed that path),
snapshotted the first, removed the output directory, and generated the second
fresh. `diff -r` was empty, including the A4H zip. Also compared explicit A4H
builds: empty diff. Evidence: `/tmp/tests-first/default-common.diff` and
`default-final2.diff` (both zero bytes against `f834c16`), and
`default-latest.diff` (zero bytes against `7659ba6`).

Regression gates exercise opt-in vs strict lowering, constructor/function/
nested-closure diagnostics, TS locations, verifier attribution with ID-zero
operands, failed static initialization at use, trap recovery, ordinary test
failure recovery, async/done not-run counting, and fresh loop captures.
