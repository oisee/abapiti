# Rules FAIL triage: refusals were counted as wrong answers

Base: `origin/feat/fit-parsetype` at `d539061`. Local harness:
`origin/feat/abaplint-tests-harness` at `43bb2ba`. Neither the harness merge nor
its local verification cache belongs to this branch. Upstream remains
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

All 104 historical FAILs are `unknown_types` test executions (102 distinct
titles; two titles occur twice). The ledger retains title-occurrence ordinals. None reaches the issue-comparison Chai
assertion or returns a registry issue list. Every one raises
`rangeFault` with this message:

```
src/rules/7bit_ascii.ts:38:3: value outside supported numeric, ordering or conversion domain
```

The generated bridge for `SevenBitAscii.setConfig` performs
`castRef[SevenBitAsciiConf](dynRef(ruleConfig))`. `Config.getDefault` has
serialized and parsed its config; that rule config is a JSON object with tag 7.
`dynRef` accepts only native references with tag 2, so it refuses before the
cast and before any regex executes. This is an unsupported structural config
adapter, not number formatting, issue ordering, or a regex returning a default.
The existing guard already fails closed and translated TS catches rethrow it.

The harness counted only the distinct `trap` panic type as TRAP. Domain guards
use `rangeFault`, which fell into its FAIL bucket. The fix covers both module
initialization and individual test recovery boundaries. Assertions and
unexpected Go panics remain FAIL; no message-text heuristic is used.

| Cluster | Tests | Class | Fixed? |
| --- | ---: | --- | --- |
| JSON rule-config reference refusal counted as FAIL | 104 | (c) harness classification | Yes: both recovery boundaries recognize `rangeFault` |
| Returned wrong issues / messages | 0 | (a) semantic bug | No such execution in this cohort |
| Unsupported operation returning a silent default | 0 | (b) missing trap | No such execution in this cohort; the config conversion already refuses |

The underlying structural config adapter remains unsupported. Implementing it
requires preserving plain-object identity, fields and inherited config defaults
across the dynamic/nominal ABI. That intersects the assigned object work and is
outside this triage. It must continue refusing until an adapter is implemented.
Reclassifying these tests adds no passing tests and claims no translation gain.

## Two minimal reproductions

The same runtime guard can be isolated without the rest of abaplint:

```ts
new SevenBitAscii().setConfig(JSON.parse('{"severity":"Error"}'));
new SevenBitAscii().setConfig(JSON.parse('{"severity":"Warning","exclude":[]}'));
```

Original JS accepts either structural config. In the generated bridge,
`dynRef(parseJSON(...))` raises `rangeFault` for either JSON object. Node's
corresponding plain-object config assignment does not coerce it to a class.
These two examples exercise the config guard, not the unsupported rule body.

Two small actual upstream tests also reproduce it:

| Test | Input | Node's asserted `unknown_types` issues | TS-HG@Go |
| --- | --- | --- | --- |
| `unknown_types Rule TABL, error` | `REPORT zfoobar.\nDATA ls_tadir TYPE ztadir.` | 1: `Variable "LS_TADIR" contains unknown: ZTADIR not found, lookup`, row 3 / col 6 in the original fixture | Refusal above; no issue array |
| `unknown_types Rule SELECT-OPTIONS, this becomes a void reference` | `SELECT-OPTIONS foo FOR structure-field.` | 0 | Same refusal; no issue array |

Both use `test/rules/_utils.ts:16` (`runMulti`). The config call is
`src/config.ts:84`, under `Config.getEnabledRules` (`:72`). The bridge's source
is `src/rules/7bit_ascii.ts:38`; the implementation's rule body starts at `:42`.
The ledger retains all registry issues, including other rules, and the filtered
`unknown_types` issues for every failing title. A missing actual issue list is
recorded as `null`, never as an empty list.

## Reproduction and local integration

Use a clean pinned upstream checkout with its locked dependencies. The existing
checkout need not be modified. Set `UPSTREAM` to that checkout.

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"
export GOFLAGS=-p=2 GOMAXPROCS=1
RUN=/tmp/fail-triage-evidence
HARNESS=/tmp/abapiti-fail-harness
UPSTREAM=/tmp/config-json-upstream

git worktree add --detach "$HARNESS" feat/fit-fail-triage
git -C "$HARNESS" merge --no-commit --no-ff origin/feat/abaplint-tests-harness
```

Resolve local conflicts retaining both conditional config JSON and repeat
runtime snippets, the harness property-thunk/Mocha hook and the feature's
argument adaptation. Keep the base's lexer facts golden. For timely measurement,
cache declaration maps for `VerifyBody` locally after signature repair; do not
change body checks or error order. This optimization stays in the disposable
worktree. Do not commit or push this combination.

```sh
cd "$HARNESS"
go build -o /tmp/abapiti-fail-baseline ./cmd/abapiti
files=$(rg --files "$UPSTREAM/packages/core/test/rules" -g '*.ts' | sort |
  sed "s@$UPSTREAM/packages/core/@@")
node tools/abaplint-tests-closure.mjs "$RUN/rules" "$UPSTREAM" $files
node tools/abaplint-tests-oracle.mjs "$RUN/node-rules.json" "$UPSTREAM" $files
node tools/abaplint-tests-triage-node.mjs "$RUN/node-issues-final.json" "$UPSTREAM" $files
/tmp/abapiti-fail-baseline abaplint-tests "$RUN/rules" -o "$RUN/before" \
  > "$RUN/go-before.json" 2> "$RUN/go-before.log"
```

That last command exits 1 because unsupported tests refuse. Apply the isolated
source patch when combining the harness for future builds:

```sh
git apply tools/abaplint-tests-refusals.patch
```

For an already generated module, the equivalent guarded post-generation fix
avoids lowering the identical closure again:

```sh
cp -a "$RUN/before/go" "$RUN/after-go"
node tools/abaplint-tests-refusals.mjs "$RUN/after-go"
go -C "$RUN/after-go" build -o abaplint-tests-go .
"$RUN/after-go/abaplint-tests-go" > "$RUN/go-after.json" 2> "$RUN/go-after.log"
node tools/abaplint-tests-triage-report.mjs "$RUN/node-rules.json" \
  "$RUN/go-before.json" "$RUN/go-after.json" "$RUN/node-issues-final.json" \
  "$RUN/triage.json"
node tools/abaplint-tests-refusals-test.mjs
```

The post-generation tool requires exactly the two known recovery boundaries and
both runtime refusal types; unknown shapes fail without changing the file. Its
regression executes Go to ensure domain guards trap while assertion payloads
and ordinary Go panics remain failures. The source patch and post-generation
fix produce the same recovery code. No compiler/runtime implementation or
reserved ownership area is changed.

## Gates and full closure

```sh
go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 GRACE_FACTS_OUT="$RUN/lexer.facts.actual" \
  go test -short -p 2 ./tsfront -run '^TestLexerFactsReport$' -count=1
cmp tsfront/testdata/lexer.facts.golden "$RUN/lexer.facts.actual"

/tmp/fail-triage-branch-bin abaplint --full -o "$RUN/full-before"
/tmp/fail-triage-branch-bin abaplint --full -o "$RUN/full-after"
```

Both full runs have **347 blocking diagnostics** (136 roots, 211 cascades),
**262 HIR errors**, and 56 affected files. Exact blocking-message histograms
are identical. The runtime JSON-to-native-config guard has **0 static blocking
diagnostics before and after**: it refuses at execution, not compilation. No
compiler root was fixed by changing its status in the harness.
[evidence/fail-triage-gates.json](evidence/fail-triage-gates.json) contains both
full summaries and every exact blocking-message count, not only normalized roots.

Build the default reference from `origin/main` (`7d6e5d9`) and the feature
compiler from the task branch. Use identical output spelling; READMEs embed it.

```sh
/tmp/fail-triage-main-bin abaplint -o "$RUN/default" --target all,go
mv "$RUN/default" "$RUN/default-main"
/tmp/fail-triage-branch-bin abaplint -o "$RUN/default" --target all,go
diff -r "$RUN/default-main" "$RUN/default" > "$RUN/default.diff"
```

The diff is empty (0 bytes), including all ABAP targets, Go sources, site maps,
archives and the executable. Vet and the required short package suite pass.
The Grace golden now counts the inherited `json.stringifyConfig` operation:
`op_aliases`, `op_allocates`, `op_may_raise`, `op_writes` 96→97;
`op_conservative` 9→10; `op_reads` 141→143. No lexer or method facts change.
A freshly exported report equals that golden. The local-only verifier cache
also passes comparison with fresh verification after field removal and trap
replacement; no cache implementation is included in this branch.

## Final rules shard

| Run | Passed | Failed | Trapped | Not-run | Registered | Node total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Before | 15 | 104 | 1,487 | 1,553 | 3,074 | 3,159 |
| After | 15 | 0 | 1,591 | 1,553 | 3,074 | 3,159 |

Original and instrumented Node both report 3,149 passes, 10 pending, zero
failures. Reconciliation includes duplicate titles as occurrences and 85
unregistered oracle tests in not-run; raw Go not-run is 1,468. The before run
is fresh, not reused from the historical parseType report. The after run
executes a rebuilt Go module with identical `hir.go`, runtime and promise
sources. Only the two host recovery boundaries in `main.go` differ.

[evidence/fail-triage-tests.json](evidence/fail-triage-tests.json) records all
104 executions, their complete Node registry messages/positions, filtered
`unknown_types` issues, and the unchanged Go refusal before/after. Node expects
one `unknown_types` issue in 21 executions and zero in 83. Both duplicate titles
(`mi_ixml should be void`, `intf, returning parameter cannot be generic`) retain
their separate zero-based occurrence numbers. The reporter requires the whole
104-execution cohort and rejects changed messages, registrations or unrelated
statuses.

Fixed roots: two harness recovery boundaries. Compiler roots fixed: zero.
No semantic wrong answer or silent-default path is observed among these 104.
This result establishes accurate failure classification, not additional passes.
The 104 config conversions remain refused because structural config reference
adapters are missing. Other reached traps remain, chiefly the shared
`test/rules/_utils.ts:47` dense/stable foreach proof (1,321 executions). Async
callbacks, skipped tests and registration gaps remain not-run; they are not
counted as passes. Regex, object and other independently owned features remain
outside this change.

All raw runs and generated modules are retained under `/tmp/fail-triage-evidence`.
The pushed branch contains the isolated fix/tools, compact gate evidence,
complete 104-test ledger, documentation and the corrected facts golden. It
contains no harness merge and opens no PR.
