# Integration train 3

Branch `integ/train-3` starts at `236da867641c41f9f48c03f08ef1401d1c6e4ba3`. The production compiler measured here is `f0555d2131d720cf5613e8996946fd7cf75dfd85`; the final fixture/report commit adds no production compiler changes. TS→HIR is the only compiler path in scope.

The original test suite passes **9,021/11,060 active tests** on generated Go, with **1 assertion failure**. Node independently passes 11,060 tests with 32 pending. The raw full-closure blocker count rises from 35 to 50 because shared labels repeat existing refusals; unique blockers fall from 35 to 33, and HIR errors fall from 151 to 127.

## Ordered merges

| Order | Source branch | Source | Merge |
| ---: | --- | --- | --- |
| 1 | `fix/cds-lexer-miscompile` | `083bb37` | `f6de590` |
| 2 | `feat/fit-traps-a` | `3602b5b` | `698ff3a` |
| 3 | `feat/fit-registry-assign` | `6d72bfa` | `dc203f1` |
| 4 | `feat/fit-edit-helper-foreach` | `e29f805` | `7ee83ed` |
| 5 | `feat/fit-qualified-type-panic` | `4ea9446` | `5c15907` |
| 6 | `feat/fit-nested-optionals` | `96ff692` | `f0555d2` |

All six merges use `git merge --no-ff` in the requested order. Both optional branches were pushed and had green gate evidence at step 5. The qualified-type tip advanced to `4ea9446` before resolution; that published tip is included. No PR was created and main was not pushed.

The summary conflict retains async-pending accounting and the full 17-shard partition/title reconciliation. Batch A retains train-2’s ABAP config writer, adds the JSON-subset and constructor-argument catalogue/effect entries, and records its separate four-shard history in `fit-traps-a.{md,json}`. Its initial catalogue guard exposed a contradictory unsupported config entry; the unpushed merge was amended to retain the existing implementation. The qualified-name helper uses the AST utility and keeps both regression sets. Other merges were clean.

## Switch fixture

`TestEmitSwitchGroupedLabels` lowers the original TS fixture, checks generated Go against the pinned Node oracle, and emits an ABAP unit consumed by the real HIR script on osgo and OSG-JS. It observes both whitespace labels, `a`, grouped `b`/`c`, an unmatched default in the middle, and exactly six side effects for six discriminants. The observation is `whitespace/whitespace/a/bc/bc/fallback/SSSSSS`. Nonempty fallthrough is checked in the same CI generation test: strict lowering reports `unsupported-statement`, trap lowering verifies and emits a whole-body trap, and generated Go reaches that trap.

Oracles were regenerated once using the five `tools/{syntax-features,registry-features,string-repeat-match,regexp-exec,switch-grouped}-oracle.mjs` commands against `~/.cache/kits/abaplint-577f875e`. Existing oracles are unchanged. The single lexer-facts export exactly matches the merged golden. Against the base, only catalogue counts change: aliases/allocates/may-raise/writes 109→111, conservative 10→12, reads 164→167; lexer method and call-site facts are unchanged.

## Gates

| Gate | Result |
| --- | --- |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `ABAPITI_GRACECHECK=1 go test -short -p 2 ./...` | PASS |
| `Grace lexer facts export and golden comparison` | PASS |
| `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend` | PASS |
| `Real .github/ci/hir-unit.sh` | PASS |
| `All-shard discovery and report contract` | PASS |
| `Fresh Node versus generated Go zabapgit clean and seeded` | PASS |

```sh
go build ./...
go vet ./...
ABAPITI_GRACECHECK=1 GRACE_FACTS_OUT=/tmp/train3-evidence/lexer.facts go test -short -p 2 ./...
go test -p 2 ./cmd/abapiti -run '^TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend$' -count=1 -v
.github/ci/hir-unit.sh /tmp/train3-ci
node tools/abaplint-tests-all-test.mjs
node tools/abaplint-tests-report-test.mjs
```

Node is on PATH via `~/.local/opt/node-v26.9.0-linux-x64/bin`. Go parallelism never exceeds `-p 2`; reference/measurement work also uses one worker where practical. Each requested numbered merge step was pushed after its local green gate. Final short tests include Grace checks; the backend guard was also executed explicitly with verbose output, rather than skipped.

The disposable runtime is clean at `7720f6e29b41a6a12faee17ea65076ac6f046552`, with transpiler/runtime `ee5c2cd1a45aaac6be503a46fc903ce95a946877` and the exact locked library refs. The shared dirty Open Steamgate checkout was untouched. All ABAP suites lint with zero issues. The real script accepts only its existing allowlisted osgo gaps: one syntax fixture gap and the two narrowly scoped pinned PCRE lookbehind gaps. The new switch fixture passes on both runtimes. Lexer baseline and all mutation/refusal probes pass the script’s expected-verdict checks; injected faults are expected failures and are not original abaplint-test failures.

Totals: **osgo 488 successes and 3 allowlisted gaps / 491 tests; OSG-JS 491 / 491 successes**.

| Runtime suite | Success | Tests | Known gaps |
| --- | ---: | ---: | ---: |
| `hir-osgjs-Test750Semantics` | 1 | 1 | 0 |
| `hir-osgjs-TestConfigJSONFixtures-case0` | 1 | 1 | 0 |
| `hir-osgjs-TestConfigJSONFixtures-case1` | 1 | 1 | 0 |
| `hir-osgjs-TestConfigJSONFixtures-case2` | 1 | 1 | 0 |
| `hir-osgjs-TestConfigJSONFixtures-case3` | 1 | 1 | 0 |
| `hir-osgjs-TestEmitRegExpExec` | 1 | 1 | 0 |
| `hir-osgjs-TestEmitStringRepeatMatch` | 1 | 1 | 0 |
| `hir-osgjs-TestEmitSwitchGroupedLabels` | 1 | 1 | 0 |
| `hir-osgjs-TestEmitSyntaxFeatures` | 1 | 1 | 0 |
| `hir-osgjs-TestFixtures` | 7 | 7 | 0 |
| `hir-osgjs-TestRuntimeDebtFixtures` | 475 | 475 | 0 |
| `hir-osgo-Test750Semantics` | 1 | 1 | 0 |
| `hir-osgo-TestConfigJSONFixtures-case0` | 1 | 1 | 0 |
| `hir-osgo-TestConfigJSONFixtures-case1` | 1 | 1 | 0 |
| `hir-osgo-TestConfigJSONFixtures-case2` | 1 | 1 | 0 |
| `hir-osgo-TestConfigJSONFixtures-case3` | 1 | 1 | 0 |
| `hir-osgo-TestEmitRegExpExec` | 0 | 1 | 1 |
| `hir-osgo-TestEmitStringRepeatMatch` | 0 | 1 | 1 |
| `hir-osgo-TestEmitSwitchGroupedLabels` | 1 | 1 | 0 |
| `hir-osgo-TestEmitSyntaxFeatures` | 0 | 1 | 1 |
| `hir-osgo-TestFixtures` | 7 | 7 | 0 |
| `hir-osgo-TestRuntimeDebtFixtures` | 475 | 475 | 0 |

## Default output

Built exact source archives for `236da86` and origin/main snapshot `23b3640`, and the integrated compiler. The main snapshot was built in the same disposable directory after the base to reuse unchanged compiler dependencies. All executables use the same working directory and output spelling:

```sh
"$ABAPITI" abaplint -o out --target all,go
# Move each completed out directory to its revision snapshot.
diff -r "$BASE_DEFAULT" "$AFTER_DEFAULT"
diff -r "$MAIN_DEFAULT" "$AFTER_DEFAULT"
```

Each output has 6,348 files. Base and main are byte-identical. Integrated output legitimately differs due to the CDS switch fix: CDSL lexer shared bodies, CurrentScope discriminant snapshots, and Perform closure `106334`→`106336` with its generated name/references. The generic fix also covers DDIC datatype families, source-expression prefixes and LSP token labels; frozen DDIC override and unchanged bare-break behavior mean those do not add distinct default ABAP changes in this measurement. All changed paths follow; the A4H zip packages the same class changes, while Go/profile/site metadata records the corrected code and node identities.

| Path | Change |
| --- | --- |
| `a4h/abaplint-577f875e-a4h.zip` | changed |
| `classes/z_closure_1063_659efc9f101dad.clas.abap` | removed |
| `classes/z_closure_1063_fd9340a501a5f1.clas.abap` | added |
| `classes/z_runtime_desc_61ac7bd03e15db.clas.abap` | changed |
| `classes/z_src_abap_5_s_0548eb8d704afe.clas.abap` | changed |
| `classes/z_src_abap_5_s_7035f4eed88c9b.clas.abap` | changed |
| `classes/z_src_cds_cds__9ff42c48675695.clas.abap` | changed |
| `go/hir.go` | changed |
| `go/hir_profile.go` | changed |
| `go/site_profile.go` | changed |
| `go/zabaplint-go` | changed |
| `names.json` | changed |
| `native/lib/z_closure_1063_659efc9f101dad.clas.abap` | removed |
| `native/lib/z_closure_1063_fd9340a501a5f1.clas.abap` | added |
| `native/lib/z_runtime_desc_61ac7bd03e15db.clas.abap` | changed |
| `native/lib/z_src_abap_5_s_0548eb8d704afe.clas.abap` | changed |
| `native/lib/z_src_abap_5_s_7035f4eed88c9b.clas.abap` | changed |
| `native/lib/z_src_cds_cds__9ff42c48675695.clas.abap` | changed |
| `osg/z_closure_1063_659efc9f101dad.clas.abap` | removed |
| `osg/z_closure_1063_fd9340a501a5f1.clas.abap` | added |
| `osg/z_runtime_desc_61ac7bd03e15db.clas.abap` | changed |
| `osg/z_src_abap_5_s_0548eb8d704afe.clas.abap` | changed |
| `osg/z_src_abap_5_s_7035f4eed88c9b.clas.abap` | changed |
| `osg/z_src_cds_cds__9ff42c48675695.clas.abap` | changed |
| `sites.json` | changed |

The changed output was validated with the real release check kit and a fresh original pinned Node build:

```sh
node tools/hir-go-check.mjs "$KIT" "$AFTER_DEFAULT/go/zabaplint-go" "$RUN/kit-check" '' "$UPSTREAM"
```

Node and Go outputs are byte-identical to the kit: clean **0 issues**, SHA-256 `9a271f2a916b0b6ee6cecb2426f0b3206ef074578be55d9bc94f6f3fe3ab86aa`; seeded **5 issues**, SHA-256 `0a08fa85133e89faa08a841a01572b228428191c50844b62e0c8bb9965145e50`.

## Full closure

```sh
"$BASE_ABAPITI" abaplint --full -o /tmp/train3-evidence/full-base
"$AFTER_ABAPITI" abaplint --full -o /tmp/train3-evidence/full-after
```

Both commands exit 0 in measurement mode; neither claims full-core emission succeeds. `--full` uses no reachability manifest. Exact message histograms and source-attributed diagnostics are in `train-3.json`.

| Measure | Base | Integrated |
| --- | ---: | ---: |
| Raw blockers | 35 | 50 |
| Raw root blockers | 29 | 36 |
| Raw cascades | 6 | 14 |
| Unique blockers | 35 | 33 |
| Unique roots | 29 | 27 |
| Unique cascades | 6 | 6 |
| HIR errors | 151 | 127 |
| Files with blockers or HIR errors | 41 | 33 |

| Blocking category (raw diagnostics) | Base | Integrated |
| --- | ---: | ---: |
| `assume-integer` | 2 | 10 |
| `unsupported-array` | 2 | 2 |
| `unsupported-call` | 3 | 4 |
| `unsupported-callback` | 1 | 1 |
| `unsupported-expr` | 19 | 23 |
| `unsupported-json` | 0 | 2 |
| `unsupported-lsp-call` | 5 | 5 |
| `unsupported-new` | 1 | 1 |
| `unsupported-regex` | 1 | 1 |
| `unsupported-statement` | 1 | 1 |

Uniqueness uses source location, category, exact message and cascade status. Correct grouped labels lower the DDIC shared body nine times: its existing Math.ceil refusal and packedLength cascade each occur 9 times instead of 1. The existing LSP `getStr` refusal occurs twice instead of once. These 17 duplicate rows explain the raw increase. Four old `identifier JSON` diagnostics disappear; two become explicit unsupported JSON formatting-argument refusals, and two are removed. No Math or LSP type support was changed.

| Verifier source root | Before | After |
| --- | ---: | ---: |
| `(no source)` | 18 | 17 |
| `src/ddic_references.ts` | 1 | 0 |
| `src/edit_helper.ts` | 1 | 0 |
| `src/lsp/_lookup.ts` | 4 | 2 |
| `src/lsp/help.ts` | 5 | 3 |
| `src/rules/cyclic_oo.ts` | 1 | 0 |
| `src/rules/definitions_top.ts` | 3 | 1 |
| `src/rules/empty_structure.ts` | 1 | 0 |
| `src/rules/obsolete_statement.ts` | 1 | 0 |
| `src/rules/sy_read_restriction.ts` | 7 | 0 |
| `src/rules/uncaught_exception.ts` | 4 | 0 |
| `src/rules/unused_variables.ts` | 1 | 0 |

## Original test results

```sh
GOMAXPROCS=1 GOFLAGS="-p=2 -buildvcs=false" \
ABAPITI_TESTS_SCRATCH=/tmp/train3-evidence/shards \
node tools/abaplint-tests-all.mjs "$UPSTREAM" /tmp/train3-evidence/all-tests.json
```

The runner executes all 17 direct-directory shards and an independent whole-suite Node oracle. All 625 test files form a disjoint exhaustive partition, with borrowed registrations removed and the complete title multiset reconciled. It uses the ordinary compiler: no verifier overlay, source editing, registration synthesis or skipped test roots. Active denominator is 11,060; table totals include Node’s 32 pending tests and therefore sum to 11,092. Refusals are trapped; FAIL is an observable assertion/test mismatch. The census exits 0, with no infrastructure failures, unmatched titles or excluded roots.

| Shard | Pass | FAIL | Trapped | Async pending | Not run | Total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| abap | 55 | 0 | 26 | 0 | 139 | 220 |
| abap/expressions | 41 | 0 | 9 | 0 | 57 | 107 |
| abap/lexer | 200 | 0 | 0 | 0 | 0 | 200 |
| abap/nodes | 2 | 0 | 0 | 0 | 0 | 2 |
| abap/statements | 4942 | 0 | 20 | 0 | 0 | 4962 |
| abap/structures | 138 | 0 | 0 | 0 | 2 | 140 |
| abap/syntax | 1243 | 1 | 80 | 0 | 49 | 1373 |
| abap/types | 4 | 0 | 25 | 0 | 0 | 29 |
| root | 61 | 0 | 33 | 0 | 0 | 94 |
| cds | 12 | 0 | 346 | 0 | 1 | 359 |
| ddl | 1 | 0 | 31 | 0 | 0 | 32 |
| lsp | 64 | 0 | 124 | 0 | 5 | 193 |
| objects | 19 | 0 | 125 | 0 | 1 | 145 |
| objects/rename | 0 | 0 | 31 | 0 | 0 | 31 |
| pretty_printer | 0 | 0 | 2 | 0 | 29 | 31 |
| rules | 2231 | 0 | 906 | 0 | 22 | 3159 |
| utils | 8 | 0 | 7 | 0 | 0 | 15 |
| TOTAL | 9021 | 1 | 1765 | 0 | 305 | 11092 |

Not-run accounting: 0 async/done registration cases, 26 skipped/focused cases, and 279 unregistered tests. Initialization refusals that prevent registration are retained per shard in the JSON; they are not reached test traps or assertion failures. Unresolved asynchronous executions have their own async-pending column.

## Assertion failures

| Shard | Test | Expected | Actual |
| --- | --- | --- | --- |
| abap/syntax | syntax.ts, Check Variables inline character literal passed to string method parameter is not compatible | 1 issue: Method parameter type not compatible, IV_REPOSITORY_ID | 0 issues |

The inline character-to-string parameter mismatch is at pinned `test/abap/syntax/syntax.ts:6053`. The source declares a string parameter, infers a character variable from a literal, and calls the method with that variable. Original Node reports the incompatibility; generated Go accepts the call. Its compiler root cause remains a follow-up outside the requested integration fixture. A targeted run with the exact 236da86 compiler and the same eight syntax roots panics before producing test JSON: `Unhandled case in Node.Text: *ast.QualifiedName` in `typeRefName`. The integrated run reaches the tests after that panic is fixed. This prevents a before/after assertion comparison against the base; the mismatch is not claimed to be newly introduced.

## Top 15 reached traps

Counts are affected original test executions, excluding initialization refusals. Exact representative source messages and shard counts are retained in the JSON. Repeated diagnostic segments are compressed within a table row; test counts are unchanged.

| Rank | Tests | Trap |
| ---: | ---: | --- |
| 1 | 340 | `not supported in the Go prototype: JavaScript regexp /^@\w+$/` |
| 2 | 224 | `src/rules/downport.ts: HIR verification: call argument type mismatch: classref<src/abap/nodes/expression_node.ts.ExpressionNode> <- array<classref<src/abap/nodes/expression_node.ts.ExpressionNode>> in runtime .` |
| 3 | 176 | `src/rules/7bit_ascii.ts: value outside supported numeric, ordering or conversion domain` |
| 4 | 104 | `src/xml_utils.ts: value outside supported numeric, ordering or conversion domain` |
| 5 | 88 | `not supported in the Go prototype: JavaScript regexp /@EndUserText\.label: '([\w,.:-=#%&() ]+)'/` |
| 6 | 79 | `src/rules/obsolete_statement.ts: value outside supported numeric, ordering or conversion domain` |
| 7 | 61 | `unsupported-expr src/ddic.ts: identifier Math is not a lowered local or module value [×9]; unsupported-expr src/ddic.ts: identifier packedLength is not a lowered local or module value [×9]` |
| 8 | 53 | `unsupported-expr src/lsp/_lookup.ts: identifier Object is not a lowered local or module value` |
| 9 | 38 | `not supported in the Go prototype: JavaScript regexp /^v\d\.\d\.\d$/` |
| 10 | 34 | `unsupported-array src/objects/domain.ts: empty array literal outside a statement context [×2]` |
| 11 | 29 | `unsupported-expr src/rules/downport.ts: identifier crypto is not a lowered local or module value; unsupported-expr src/rules/downport.ts: identifier hash is not a lowered local or module value` |
| 12 | 22 | `src/config.ts: value outside supported numeric, ordering or conversion domain` |
| 13 | 20 | `src/rules/avoid_use.ts: value outside supported numeric, ordering or conversion domain` |
| 14 | 19 | `unsupported-assertion test/objects/class.ts: type assertion requires checker-proven narrowing of its operand; unsupported-expr test/objects/class.ts: identifier clas is not a lowered local or module value; unsupported-expr test/objects/class.ts: identifier s is not a lowered local or module value; unsupported-expr test/objects/class.ts: identifier scope is not a lowered local or module value` |
| 15 | 18 | `not supported in the Go prototype: JavaScript regexp /^\)?\. *(\".*)?$/` |

## Remaining work

Core translation remains partial. Remaining reached traps above identify the next execution barriers; other blockers include unsupported config/object conversions, callbacks and lost registrations, regex constructs outside the reviewed grammar, primitive-array absence handling, ToPrimitive/integer proofs, dynamic APIs and LSP/vscode representations. They remain explicit refusals in their owners’ workstreams. No unsupported result was fabricated or assertion bypassed to increase N.

Compact evidence is in [train-3.json](train-3.json); the latest all-shard report is [abaplint-tests-latest.json](abaplint-tests-latest.json). Logs and disposable outputs are under `/tmp/train3-evidence` and `/tmp/train3-ci`.
