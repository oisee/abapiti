# Live config row and string-array views

Frozen semantic source: `4242fc67ad701daec81d3f0e390888b1c5414da7`.
Base: `origin/main` at `ab026e7`, merged with `origin/feat/fit-traps-i`
at `01a15a7` using `--no-ff`; merged base `8e75bb8`. The merge was clean.
The initial ownership audit is [traps-I](traps-i-roots.md).
Upstream is unchanged abaplint `577f875ebec44cfaf64841cfe71c8ab8dc32622e`;
original Node 26.9.0 / TypeScript 6.0.3 oracles are rebuilt independently.
Compiler builds use Go 1.26.9; the existing shard runner pins Go 1.26.0.

Live config layouts now discover nested structural record/interface rows,
including optional fields and arrays, and seal their prototype boundary before
inlining. Go field readers and writers use the original row storage. Compatible
native references retain their identity. Boxing a proxy returns its backing
record, so shared rows, extra fields, own-key order and writes through either
alias stay observable. An absent optional `from` stays absent; assigning
undefined retains a present own property. Missing config arrays stay undefined,
including partial configs inheriting `BasicRuleConfig`.

Erased array writes retain parsed record/array tags instead of exposing their
raw map/table as a nominal reference. Typed reads through a native row-array
alias also use the certified row converter after a proxy writes a parsed row.
The ordinary nominal assertion boundary remains separate.

Unpruned erased string-array handoffs lower to `dynamic.arrayView`, including
erased virtual argument slots. Go uses the existing live array storage with a
string check on each element read, including after caller mutations. A
non-string or explicit undefined element raises `live string array requires a
string element`; holes retain separate membership. ABAP explicitly refuses this
operation and parsed live-array field conversions. The existing Chai live-array
length guard is retained and directly exercised by the presence regression.

Node/Go regressions cover record aliases and interfaces, plain and JSON-parsed
rows, native row-array aliases, inherited config fields, partial configs, optional
absence versus explicit undefined, extra properties, identity, element
replacement, push/splice, caller writes and checked non-string reads. HIR tests
check the string-only operation signature and preserve nested row prototype
refusals across inlining. No upstream test is edited, excluded or weakened.

Only the five shards reached by these roots were measured. The same registered
title sequence is present before and after, including duplicates; original Node
title multisets reconcile, and borrowed executions are checked for FAIL before
ownership subtraction. There is no whole-suite extrapolation.

| Shard | Passed before → after | Trapped before → after | FAIL | Not-run |
| --- | ---: | ---: | ---: | ---: |
| abap | 199 → 215 | 17 → 1 | 0 | 4 |
| root | 74 → 88 | 20 → 6 | 0 | 0 |
| lsp | 147 → 169 | 41 → 19 | 0 | 5 |
| objects/rename | 0 → 0 | 31 → 31 | 0 | 0 |
| rules | 2,963 → 3,111 | 184 → 36 | 0 | 12 |
| Total | **3,383 → 3,583** | **293 → 93** | **0** | **21** |

Selected Node: 3,678 active tests and 19 pending; generated Go registers 3,691
of 3,697 total titles. Not-run remains 15 skipped/focused plus six unregistered
(four of those are Node pending). Async-pending, wiring failures, unmatched
titles and genuine FAILs are zero.

Both first roots disappear: Recommendations **195 → 0**, RulesRunner
**18 → 0**. The fresh train-7 merged base has nine RulesRunner refusals in rules,
rather than the historical six, explaining **213**, rather than 210, owned
executions. Exactly 200 of these executions now pass. The remaining 13 reach
later roots: primitive `find` callback ABI (three), LSP rename parameter types
(two), Renamer's `in` storage boundary (six), and
`renameDDICTTYPReferences`' DDIC-object conversion domain (two). These are outside
the two config-array handoffs. All other registered test statuses are unchanged.
The 93 remaining refusals include 80 existing unrelated refusals; their complete
message counts and the exact owned-root transitions are in the
[certificate](evidence/live-rows-final.json).

Full-closure lowering is measured with the existing `abaplint --full` command:
**31 → 31 blocking diagnostics**, comprising 21 root and ten cascade diagnostics;
**86 → 86 HIR errors** in 22 files. Both config-array roots are runtime boundaries
and contribute zero static blocking diagnostics. Exact blocking message counts
before and after are identical and retained in the certificate.

Commands below use `SCRATCH` for the disposable measurement directory and
`UPSTREAM` for the clean pinned checkout. Baseline binaries were built from
`git archive 8e75bb8`; the final binary records the frozen semantic SHA above.

```sh
export GOFLAGS='-p=2' GOMAXPROCS=2
"$SCRATCH/merged-abapiti" abaplint --full -o "$SCRATCH/full-before"
"$SCRATCH/branch-abapiti" abaplint --full -o "$SCRATCH/full-after"

ABAPITI_TESTS_SHARDS=rules,root \
ABAPITI_TESTS_COMPILER="$SCRATCH/merged-abapiti" \
ABAPITI_TESTS_COMPILER_COMMIT=8e75bb8 \
node tools/abaplint-tests-all.mjs "$UPSTREAM" "$SCRATCH/shards-before.json"
ABAPITI_TESTS_SHARDS=lsp,abap,objects/rename \
ABAPITI_TESTS_COMPILER="$SCRATCH/merged-abapiti" \
ABAPITI_TESTS_COMPILER_COMMIT=8e75bb8 \
node tools/abaplint-tests-all.mjs "$UPSTREAM" "$SCRATCH/shards-before-extra.json"
ABAPITI_TESTS_SHARDS=rules,root,lsp,abap,objects/rename \
ABAPITI_TESTS_COMPILER="$SCRATCH/branch-abapiti" \
ABAPITI_TESTS_COMPILER_COMMIT=4242fc6 \
node tools/abaplint-tests-all.mjs "$UPSTREAM" "$SCRATCH/shards-after.json"

go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 go test -short -p 2 ./tsfront -run TestLexerFactsReport -count=1
go test -short -p 2 ./cmd/abapiti \
  -run '^TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend$' -count=1
./.github/ci/lint.sh gate origin/main
```

All gates pass. Grace's regenerated golden changes only the six catalogue
counts introduced by the new operation; lexer facts and call sites are unchanged.
Lint reports zero new issues and canary 5/5 with golangci-lint 2.13.2.

Default `abapiti abaplint -o "$SCRATCH/default-output" --target all,go` was run
with compilers built from origin/main, the merged base, and the final source.
The same output path was used each time, moving previous outputs aside.
Raw `diff -r` between merged base and final is empty over **6,321 files**,
including Go binary and ABAP zip. Origin/main already differs from the required
merged base in `go/runtime.go` (the inherited traps-H/I regexp runtime/table)
and `go/zabaplint-go`; that inherited difference is retained without editing the
separately owned RegExp implementation.

Only compact Markdown/JSON evidence is retained. Generated closures, binaries,
raw results, oracle builds and scratch checkouts are disposable. Commits stay
on `feat/fit-live-rows`; no push or PR was performed.
