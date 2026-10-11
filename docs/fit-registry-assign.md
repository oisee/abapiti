# Registry async result covariance

Based on `ce2f789`, on `feat/fit-registry-assign`. Upstream is pinned to
`577f875ebec44cfaf64841cfe71c8ab8dc32622e` (@abaplint/core 2.120.56).

## Exact failure and root

There is no TypeScript assignment at `src/registry.ts:370`: that line starts
`Registry.parseAsync(input?: IRunInput)`. `findIssues` starts at line 338.
The previously reported location is the whole-body refusal location, not the
source of an assignment. The failing HIR statement is the synthetic `return`
in the erased interface bridge:

```text
IRegistry.parseAsync: Promise<IRegistry>        // src/_iregistry.ts:17
Registry.parseAsync: Promise<Registry>         // inferred from return this
bridge return: Promise<IRegistry> <- Promise<Registry>
```

A measurement-only verifier diagnostic identified that return precisely:
`promise<interfaceref<src/_iregistry.ts.IRegistry>> <-
promise<classref<src/registry.ts.Registry>>`, in `Registry.parseAsync`, with
statement kind `return`, expression kind `virtual`, and synthetic node ID zero.
`eraseGenericOverrides` correctly preserves the interface slot and the concrete
implementation's signature, but its forwarding return did not adapt the promise
payload. HIR promises have invariant physical payload types; relaxing the
verifier would leave incompatible Go generic instantiations and ABAP promise
classes.

## General frontend fix

After interface/union signatures stabilize, async covariance bridges adopt the
source result using existing HIR `Async`, `PromiseAwait`, `Narrow`, and
`PromiseFulfill` operations. The source executes once; the fulfilled reference
is viewed through the compatible slot type, and a rejection remains a rejection
with its original payload. Inherited slots, interface slots, and compatible chains of
covariant bridges use the same rule. No source fingerprint, Registry override, verifier
relaxation, runtime operation, backend change, or void ABI change is involved.

This adoption is restricted to source async implementations and compatible
reference payloads. Async calls already produce fresh promises; their internal
implementation promise is not a separately exposed TS result. A synchronous
method returning a shared promise cannot use this adaptation: creating a new
wrapper would change observable identity. Such covariance remains refused until
an identity-preserving promise view representation exists.

Regressions execute generated Go and emit ABAP for interface covariance and
inherited covariance. They check one source execution per invocation, distinct
async promises, original fulfilled-object identity, original rejection identity,
and preserved virtual dispatch. A separate test keeps shared synchronous
promise covariance refused.

## Measurements and reproduction

Use Node from `~/.local/opt/node-v26.9.0-linux-x64/bin`, `GOMAXPROCS=2`, and
`GOFLAGS='-p=2 -buildvcs=false'`. Build source executables from `ce2f789`,
`origin/main` (`23b3640`), and this branch. Full/default gates use those ordinary
executables, without measurement overlays.

Full-closure commands (the flag is present in origin/main):

```sh
"$BASE_ABAPITI" abaplint --full -o "$RUN/full-base"
"$AFTER_ABAPITI" abaplint --full -o "$RUN/full-after"
```

| Measure | Before | After |
| --- | ---: | ---: |
| Blocking diagnostics | 35 | 35 |
| Root / cascade blocking | 29 / 6 | 29 / 6 |
| HIR verification errors | 151 | 150 |
| Assignment type mismatches | 17 | 16 |
| Covariant Registry promise bridge mismatches | 1 | 0 |

The exact blocking-message histogram is unchanged. The compact JSON companion
records both histograms and the HIR-message counts. Remaining full-core errors
belong to existing unsupported operations and representations, including LSP,
JSON/Object helpers, coercions, numeric proofs, collection/shape typing, and
other assignment/call mismatches.

Harness runs use the unchanged pinned upstream sources, sorted root paths,
fresh original Mocha/chai Node oracles, and the existing reporter's title
multiset reconciliation. The test-only declaration-index overlay from
`docs/evidence/train-2-verifier-index.patch` is reused for both harness compiler
builds. It caches declarations only; body verification and refusal semantics
remain intact. No production gate uses it.

```sh
go build -overlay "$BASE_OVERLAY" -o "$BASE_TEST_ABAPITI" ./cmd/abapiti
go build -overlay "$AFTER_OVERLAY" -o "$AFTER_TEST_ABAPITI" ./cmd/abapiti
node tools/abaplint-tests-closure.mjs "$CLOSURE" "$UPSTREAM" $files
node tools/abaplint-tests-oracle.mjs "$NODE_JSON" "$UPSTREAM" $files
"$TEST_ABAPITI" abaplint-tests "$CLOSURE" -o "$OUT" > "$GO_JSON" 2> "$LOG"
node tools/abaplint-tests-report.mjs "$NODE_JSON" "$GO_JSON" "$REPORT_JSON"
```

The after test executable was staged at `$RUN/diagnostic` by the scratch runner;
its SHA-256 matches `$RUN/after-fast`. Its name does not indicate verifier
instrumentation: the detailed verifier build was used only for the separate
root-cause full-closure run, before being replaced with the fixed executable.

| Shard / selection | Pass before → after | FAIL | Trap before → after | Not-run | Node total |
| --- | ---: | ---: | ---: | ---: | ---: |
| Complete rules shard, 192 files | 1,235 → 1,587 | 0 → 0 | 1,902 → 1,550 | 22 → 22 | 3,159 |
| Other runnable parseAsync callers, 29 roots | 12 → 29 | 0 → 0 | 149 → 132 | 30 → 30 | 191 |
| Qualified-assertion build refusals, seven roots | 0 → 0 | 0 → 0 | 0 → 0 | 62 → 62 | 62 |
| **Unique selected total** | **1,247 → 1,616** | **0 → 0** | **2,051 → 1,682** | **114 → 114** | **3,412** |

Rules registration remains 3,147; supplemental runnable registration remains
162. Every reconciled title matches the fresh original Node oracle, including
duplicates, and the ordered Go title sequences are identical before/after.
No previously passing test regresses. No async-pending tests remain in these
runs. The 369 additional passes are original abaplint tests; compiler regressions
are not added to this count. This is a selected subset of the 11,060-test suite.

Of the **736 targeted rules tests**, **352 now PASS** and **384 reach a next
trap**; none reaches the old Registry mismatch. Of the **19 supplemental tests**
that reached the same bridge, **17 now PASS** and **two reach the existing
7bit_ascii conversion refusal**. Thus all **755 reached Registry mismatches**
are removed, with **369 passes and 386 later traps**, and **zero FAIL**.

The supplemental selection includes all 36 non-rules test roots containing
`.parseAsync(`. Its combined production build fails before test execution,
both before and after, with `Unhandled case in Node.Text: *ast.QualifiedName`
from `typeRefName` in `asExpression`. The panic occurs while detecting `as const`
on namespace-qualified assertions. Seven roots are isolated to allow the other
29 roots to execute: five LSP roots (`references`, `rename_global_class`,
`rename_method`, `rename_variable`, `symbols`) and objects `domain` / `table_type`.
Their **62 tests are build-level not-run**, not invented reached runtime traps.
The full combined Node oracle is 250 passed / three pending / zero failed;
the runnable partition is 190 passed / one pending / zero failed. The title
multisets prove that partitioning accounts for all 253 selected tests.
No namespace/LSP typing changes were made to evade this unrelated panic.

The complete next-trap histogram for the 384 targeted rules tests follows.
The JSON companion retains exact messages and representative locations.

| Targeted tests | Next trap |
| ---: | --- |
| 147 | `src/rules/unused_variables.ts: HIR verification: nested optional` |
| 48 | `src/rules/sy_read_restriction.ts: HIR verification: unsupported collection key dynamic` |
| 34 | `not supported in the Go prototype: JavaScript regexp /^.?cx_.*$/i` |
| 18 | `not supported in the Go prototype: JavaScript regexp /^v\d\.\d\.\d$/` |
| 16 | `src/xml_utils.ts: value outside supported numeric, ordering or conversion domain` |
| 15 | `src/rules/no_prefixes.ts: unreviewed JavaScript regexp /^[LGM].?_/i` |
| 13 | `not supported in the Go prototype: JavaScript regexp / (abap_true\|abap_false)\s*\./i` |
| 10 | `src/rules/local_variable_names.ts: unreviewed JavaScript regexp /^[a-zA-Z]{1,3}_.*$/i` |
| 9 | `not supported in the Go prototype: JavaScript regexp /\.intf\.abap$/` |
| 9 | `src/rules/catch_and_raise.ts: HIR verification: call argument type mismatch: string <- regexp in runtime .` |
| 9 | `unsupported-call src/rules/identical_conditions.ts: find on primitive arrays needs an absence-preserving callback ABI` |
| 8 | `src/rules/unnecessary_return.ts: value outside supported numeric, ordering or conversion domain` |
| 7 | `src/rules/modify_only_own_db_tables.ts: unreviewed JavaScript regexp /^[yz]/i` |
| 5 | `unsupported-regex src/rules/object_naming.ts: new RegExp requires a lowered string pattern; unsupported-expr src/rules/object_naming.ts: identifier regex is not a lowered local or module value` |
| 4 | `not supported in the Go prototype: JavaScript regexp /DATA/i` |
| 4 | `src/rules/cyclic_oo.ts: HIR verification: foreach requires matching array` |
| 4 | `src/rules/obsolete_statement.ts: HIR verification: call argument type mismatch: optional<map<string,array<classref<src/edit_helper.ts.ITextEdit>>>> <- optional<dynamic> in call src/issue.ts.Issue.atStatement` |
| 3 | `src/rules/class_attribute_names.ts: unreviewed JavaScript regexp /^M._.+$/i` |
| 3 | `src/rules/method_parameter_names.ts: unreviewed JavaScript regexp /^[a-zA-Z]{1,3}_.*$/i` |
| 3 | `unsupported-expr src/rules/remove_descriptions.ts: identifier XMLParser is not a lowered local or module value; unsupported-new src/rules/remove_descriptions.ts: new XMLParser is not a lowered class` |
| 2 | `src/edit_helper.ts: HIR verification: foreach requires matching array` |
| 2 | `src/rules/class_attribute_names.ts: unreviewed JavaScript regexp /C_.+$/i` |
| 2 | `src/rules/local_variable_names.ts: unreviewed JavaScript regexp /^<[a-zA-Z]{1,3}_.*>$/i` |
| 2 | `src/rules/local_variable_names.ts: unreviewed JavaScript regexp /^<L._.+>$/i` |
| 2 | `src/rules/local_variable_names.ts: unreviewed JavaScript regexp /^L._.+$/i` |
| 1 | `src/abap/5_syntax/structures/class_data.ts: value outside supported numeric, ordering or conversion domain` |
| 1 | `src/objects/_abap_object.ts: HIR verification: call argument type mismatch: string <- dynamic in runtime .` |
| 1 | `src/rules/7bit_ascii.ts: value outside supported numeric, ordering or conversion domain` |
| 1 | `src/rules/abapdoc.ts: unreviewed JavaScript regexp /^\"! .*<[^/>]*><\//` |
| 1 | `src/rules/method_parameter_names.ts: unreviewed JavaScript regexp /^I._.+$/i` |

These later refusals remain explicit. They concern optional/collection typing,
reviewed regexp coverage, numeric/config/XML conversions, primitive-array
absence, and other callable or field representations. They belong to other
roots and workstreams; assertion mismatches were not reclassified as traps.


## Gates

Passed:

```sh
go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 go test -short -p 2 -timeout 30m \
  ./hir/... ./tsfront/... ./internal/... ./cmd/...
```

The short gate executes the backend-coverage guard
`TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`; it has no skip path.
Grace fixture/golden checks pass, and no golden or oracle file changes.

All three ordinary executables use the same working directory and `-o out`
spelling, moving each completed output aside before the next invocation:

```sh
"$ABAPITI" abaplint -o out --target all,go
diff -r "$RUN/default-base" "$RUN/default-after"
diff -r "$RUN/default-main" "$RUN/default-after"
```

Both diffs are empty: all **6,348 files**, including Go executable and A4H zip,
are byte-identical against both `ce2f789` and origin/main `23b3640`.
