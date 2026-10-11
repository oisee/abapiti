# Integration train 2: config JSON merge

The merge combines train-2/train-1 HEAD `db2834c` with
`feat/fit-config-json-abap` at `89519d5`. Both CI conflicts are resolved as a
union: runtime debt, syntax, string repeat/match and RegExp.exec suites remain,
and all four config JSON cases are generated, linted and executed. Fixture
mode remains accepted, config JSON mode selects ABAP v702, and other modes
are rejected. The osgo jobs limit, known-gap handling, cleanup trap, inherited
GOFLAGS and slash-safe case report names remain in place.

The auto-merged ABAP emitter retains the train implementations and adds the
config writer. Integration tests exposed a stale backend capability entry:
`json.stringifyConfig` was still marked unsupported. It is now registered as
supported, and its effect description no longer says ABAP refuses it. No TS
lowering, reachability, or lexer golden changes were needed for this merge.

## Gates

All commands ran with Node on PATH via
`~/.local/opt/node-v26.9.0-linux-x64/bin`, `GOMAXPROCS=2`, and `GOFLAGS=-p=2`.

| Command | Result |
| --- | --- |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test -short -p 2 ./...` | PASS |
| `ABAPITI_GRACECHECK=1 go test -count=1 -run TestLexerFactsReport ./tsfront/` | PASS; golden unchanged |
| `go test -count=1 -run TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend ./cmd/abapiti/` | PASS |
| `.github/ci/hir-unit.sh /tmp/train2-merge-gates/runtime` | PASS; actually executed |

The first short run failed because of the stale capability entry. The focused
config/backend checks and the full final gate run passed after registration
was corrected. No `GRACE_FACTS_OUT` regeneration was necessary.

The existing runtime checkout was clean at the CI pin
`7720f6e29b41a6a12faee17ea65076ac6f046552`. The script verified and linked the
pinned transpiler/runtime `ee5c2cd1a45aaac6be503a46fc903ce95a946877`. A fresh
work directory can be prepared with
`.github/ci/osgo-unit.sh "$work" setup` before running `hir-unit.sh "$work"`;
setup does not execute the deprecated WASM tests.

| Suite | osgo | OSG-JS |
| --- | --- | --- |
| TestFixtures | 7/7 PASS | 7/7 PASS |
| TestRuntimeDebtFixtures | 475/475 PASS | 475/475 PASS |
| Test750Semantics | 1/1 PASS | 1/1 PASS |
| TestEmitSyntaxFeatures | 1 known gap | 1/1 PASS |
| TestEmitStringRepeatMatch | 1 known gap | 1/1 PASS |
| TestEmitRegExpExec | 1 known gap | 1/1 PASS |
| TestConfigJSONFixtures/case0 | 1/1 PASS | 1/1 PASS |
| TestConfigJSONFixtures/case1 | 1/1 PASS | 1/1 PASS |
| TestConfigJSONFixtures/case2 | 1/1 PASS | 1/1 PASS |
| TestConfigJSONFixtures/case3 | 1/1 PASS | 1/1 PASS |

The two match-related gaps are exactly `LOOP form: LOOP AT groups INTO group.`;
the exec gap is exactly `FIND PCRE`. Both diagnostics are already listed in
`osgo-known-gaps.txt`; there were no other non-success rows. Totals are 488
passes plus three known gaps on osgo, and 491 passes on OSG-JS. All generated
ABAP lint checks passed, including the dedicated config v702 check. The exit
trap removed the generated HIR directory.

The lexer gate also passed on both runtimes: 19 positive runs each, including
the 44-case/4,663-token baseline and number-fix regression. Each runtime's
three deliberate mutations (early return, skipped case, token count) failed
as expected. These are negative controls, not unexpected failures.

## Default output and full closure

Fresh binaries were built from `git archive 141ecaf`, `git archive origin/main`
(`23b3640`, whose tree is identical to `141ecaf`), and the resolved merge tree.
Each used `go build -o "$binary" ./cmd/abapiti`. All default runs used the same
working directory and output spelling, moving the output aside after each:

```sh
/tmp/train2-merge-gates/base-bin abaplint -o /tmp/train2-merge-gates/default --target all,go
mv /tmp/train2-merge-gates/default /tmp/train2-merge-gates/default-base
/tmp/train2-merge-gates/main-bin abaplint -o /tmp/train2-merge-gates/default --target all,go
mv /tmp/train2-merge-gates/default /tmp/train2-merge-gates/default-main
/tmp/train2-merge-gates/branch-bin abaplint -o /tmp/train2-merge-gates/default --target all,go
mv /tmp/train2-merge-gates/default /tmp/train2-merge-gates/default-branch
diff -r /tmp/train2-merge-gates/default-base /tmp/train2-merge-gates/default-branch
diff -r /tmp/train2-merge-gates/default-main /tmp/train2-merge-gates/default-branch
/tmp/train2-merge-gates/base-bin abaplint --full -o /tmp/train2-merge-gates/full-base
/tmp/train2-merge-gates/branch-bin abaplint --full -o /tmp/train2-merge-gates/full-branch
```

Both diffs are empty, including the generated Go executable. Both full runs
exit 0 in measurement mode, without the reachability manifest.

| Measure | Before: train-1 `141ecaf` | After: resolved train-2 |
| --- | ---: | ---: |
| Blocking diagnostics | 35 | 35 |
| Root blocking | 29 | 29 |
| Cascade blocking | 6 | 6 |
| HIR verification errors | 173 | 151 |
| Files with blocking/verification errors | 47 | 41 |
| Config source blockers (`src/config.ts`, `src/_config.ts`) | 0 | 0 |

The exact blocking-message histograms are identical. The 22 fewer HIR errors
come from earlier train-2 merges; this config merge fixes the ABAP config
writer refusal and its stale capability registration, with no frontend root
blockers removed. Remaining unpruned lowering and verification gaps prevent
full-core emission and the goal of all 11,060 compiled-core tests passing.
They belong to other workstreams and were left unchanged.

[Compact evidence](evidence/train-2-config-json-merge.json) records exact
commands, all blocking messages, summaries, runtime totals and known gaps.
