# Qualified type assertion compiler panic

Branch `feat/fit-qualified-type-panic` starts at `6d72bfa` from
`feat/fit-registry-assign` (integration base `ce2f789`). Upstream is pinned to
`577f875ebec44cfaf64841cfe71c8ab8dc32622e` (@abaplint/core 2.120.56).

The baseline seven-root harness build reproduces:

```text
panic: Unhandled case in Node.Text: *ast.QualifiedName
  tsfront.typeRefName (lower_types.go:1017)
  (*lowerer).asExpression (lower_expr.go:2364)
```

`asExpression` checks whether an assertion is `as const` before mapping its
checker type. `TypeReference.TypeName` is an AST entity name, which can be an
identifier or a recursively qualified name. `Node.Text` supports identifiers;
it panics on a qualified name. This affected namespace-qualified assertions
before their operands or asserted types could be lowered. For example,
`test/objects/table_type.ts:34` asserts
`reg.getFirstObject()! as Objects.TableType`; line 38 uses `type as Types.TableType`.

`typeRefName` now uses the AST's `EntityNameToString` utility. This also fixes
its other caller, the unresolved type-reference diagnostic. Full qualification
is preserved: `N.const` does not match the special `const` assertion. The
checker and the existing assertion lowering/refusal rules remain responsible
for the asserted type. No exception recovery, type-system relaxation, source
fingerprint, LSP shape change, backend change, or ABI change is involved.

Regressions execute generated Go for a qualified numeric alias and a qualified
class assertion, including object identity, and retain a passing `as const`
case. An unproven qualified downcast produces `unsupported-assertion` in strict
mode and a verified HIR lowering trap in trap mode. The refusal regression uses
the exported alias `N.const`, verifying that only bare `const` skips ordinary
assertion checks.

## Reproduction

Use Node from `~/.local/opt/node-v26.9.0-linux-x64/bin`:

```sh
export PATH="$HOME/.local/opt/node-v26.9.0-linux-x64/bin:$PATH"
export GOMAXPROCS=2 GOFLAGS='-p=2 -buildvcs=false'
```

Build ordinary executables from archives of `6d72bfa`, the branch, and
`origin/main` (`23b3640`). Full-closure and default output gates use those
ordinary executables. The full flag is present in origin/main.

```sh
"$BASE_ABAPITI" abaplint --full -o "$RUN/full-base"
"$AFTER_ABAPITI" abaplint --full -o "$RUN/full-after"

# Use the same working directory and identical output spelling for each build;
# move completed output aside before the next invocation.
"$ABAPITI" abaplint -o out --target all,go
mv out "$RUN/default-$stage"
diff -r "$RUN/default-base" "$RUN/default-after"
diff -r "$RUN/default-main" "$RUN/default-after"
```

Harness builds use the declaration-index-only overlay documented in
`docs/evidence/train-2-verifier-index.patch`, identically before and after.
It reuses declaration lookup maps; body verification and refusal semantics
are unchanged. The overlay is absent from production builds and all gates.
Supply every selected root in sorted path order to both closure materialization
and the fresh original upstream Mocha/chai oracle:

```sh
go build -overlay "$OVERLAY" -o "$TEST_ABAPITI" ./cmd/abapiti
node tools/abaplint-tests-closure.mjs "$CLOSURE" "$UPSTREAM" $files
node tools/abaplint-tests-oracle.mjs "$NODE_JSON" "$UPSTREAM" $files
"$TEST_ABAPITI" abaplint-tests "$CLOSURE" -o "$OUT" > "$GO_JSON" 2> "$LOG"
node tools/abaplint-tests-report.mjs "$NODE_JSON" "$GO_JSON" "$REPORT_JSON"
```

Selections: the complete rules shard (192 roots), complete LSP shard (16 roots),
complete objects shard (35 roots), and the seven previously isolated roots:

- `test/lsp/references.ts`
- `test/lsp/rename_global_class.ts`
- `test/lsp/rename_method.ts`
- `test/lsp/rename_variable.ts`
- `test/lsp/symbols.ts`
- `test/objects/domain.ts`
- `test/objects/table_type.ts`

The seven-root selection overlaps the complete LSP and objects shards and must
not be added to their unique total. Build-level panics have zero executed tests;
all oracle titles are not-run, rather than invented reached runtime traps.
The reporter reconciles full titles as multisets, including duplicates.

## The formerly blocked 62 tests

| Selection | Pass before → after | FAIL | Trap before → after | Not-run before → after | Node total |
| --- | ---: | ---: | ---: | ---: | ---: |
| Seven qualified-type roots | 0 → 10 | 0 → 0 | 0 → 50 | 62 → 2 | 62 |

All 62 titles register and reconcile with the fresh Node oracle (60 passed,
two pending, zero failed). The two remaining not-run tests are original skipped
tests. There are no initialization failures or async-pending results. All 60
originally runnable tests now execute; none remains build-level not-run.
The ten passes are two global-class prepare-rename cases, three variable-rename
cases, and five symbols cases. The JSON companion lists their original titles.

The complete next-trap histogram follows; exact messages and locations are in
the JSON companion.

| Tests | Next trap / reason |
| ---: | --- |
| 15 | `src/artifacts_objects.ts:8`: the existing factory only authorizes CLAS/INTF/PROG/TYPE/XSLT; other object types reach its coverage trap |
| 12 | `src/lsp/references.ts`: a property-access callback is not an inline arrow callback |
| 12 | `test/objects/domain.ts` / `table_type.ts`: unproven nominal downcasts remain `unsupported-assertion` refusals |
| 4 | `src/lsp/rename.ts`: `IRenameParams` does not match the lowered `ITextDocumentPositionParams` argument ABI |
| 4 | `src/rules/7bit_ascii.ts`: numeric, ordering or conversion domain refusal |
| 2 | `test/objects/table_type.ts`: HIR assignment type mismatch |
| 1 | `test/lsp/rename_global_class.ts`: chai method `throw` is not lowered |

These are actual reached runtime traps, including the existing explicit cast
refusals. The helper does not pretend that qualified assertions establish a
nominal narrowing proof. Factory coverage, callbacks, LSP typing, conversions,
assignment representations, and additional chai APIs remain separate work.

## Rules, full-core gap and default output

The complete rules shard is unchanged: **1,587 passed, zero failed, 1,550
trapped, 22 not-run**, out of 3,159. Registration is 3,147 before and after.
Every ordered title and per-test status matches, including duplicate titles;
there are zero passing-test regressions. Generated Go files and executable are
byte-identical as well. The rules builds use the same output path, moving the
completed baseline output aside before the after build.

The fresh rules Node oracle initially hit the first `7bit_ascii` case's
original 250 ms timeout in two cold runs under machine load (3,148 passed,
one timeout, ten pending). A separate fresh original-source Mocha/chai build
with **`--retries 2`** passes 3,149 tests, with ten pending and zero failures.
Its title multiset is identical to both cold runs. The scratch copy of
`tools/abaplint-tests-oracle.mjs` adds only `--retries 2` to the Mocha command;
no upstream source, assertion, timeout value, or test selection is changed.
The seven-root, LSP, and objects Node oracles use the ordinary command without
retries and have zero failures.

| Full-core measurement | Before | After |
| --- | ---: | ---: |
| Blocking diagnostics | 35 | 35 |
| Root / cascade blocking | 29 / 6 | 29 / 6 |
| HIR verification errors | 150 | 150 |
| Qualified-name compiler panics | 0 | 0 |
| `unsupported-assertion` / `unsupported-type` blockers | 0 / 0 | 0 / 0 |

Both full-core commands complete successfully. The qualified-assertion panic is reproduced by the test closure rather than
by the full-core measurement.
The exact blocking-message histogram and all gap-summary fields are unchanged;
the JSON companion records the complete message histogram. Existing full-core
barriers concern unsupported calls/expressions, callbacks, arrays, LSP APIs,
coercion/integer proofs, and other HIR representation mismatches.

Default `abaplint -o out --target all,go` output is unchanged against `6d72bfa`:
`diff -r` is empty for all **6,348 files**, including the Go executable and A4H
zip. The separate comparison against origin/main `23b3640` is also empty for all
6,348 files. The JSON companion records the common file-manifest digest.

## Complete affected shards

| Shard | Roots | Pass before → after | FAIL | Trap before → after | Not-run before → after | Node total |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Rules | 192 | 1,587 → 1,587 | 0 → 0 | 1,550 → 1,550 | 22 → 22 | 3,159 |
| LSP | 16 | 0 → 63 | 0 → 0 | 0 → 131 | 199 → 5 | 199 |
| Objects | 35 | 0 → 3 | 0 → 0 | 0 → 172 | 176 → 1 | 176 |
| **Unique total** | **243** | **1,587 → 1,653** | **0 → 0** | **1,550 → 1,853** | **397 → 28** | **3,534** |

Both complete affected shards reproduce the same qualified-name compiler
panic on the baseline, before any Go registration or execution. Both after
builds lower and emit successfully with zero blocking diagnostics and zero
HIR errors. LSP registers all 199 titles; objects registers all 176. There
are no initialization failures, unmatched titles, or async-pending results.
All remaining not-run tests in these two shards are original skips.

Thus **369 formerly blocked runnable tests execute**, producing **66 additional
passes and 303 later traps**, with zero FAIL. The 62-test subset is included
in these shard totals exactly once, and all 62 statuses agree with its separate
run. No previously passing rules test regresses. The remaining 28 not-run tests
are 16 original skips and the same 12 existing unregistered rules tests.
This measures 3,534 selected original tests, rather than the whole 11,060 suite.

The larger shards expose additional existing barriers: 53 LSP tests trap in
`src/lsp/_lookup.ts` on Object/JSON operations; the factory coverage trap occurs
in 22 LSP tests and 71 objects tests. Unproven object downcasts, unsupported
callbacks, coercion/domain proofs, collection/assignment typing, closure captures,
and chai API gaps account for the other reached refusals. Complete ranked messages
are in the JSON companion. These existing features remain outside this panic
fix; no LSP types, object literals, small builtins, regexp, Math, typeof, or void
ABI implementations were changed.

Compiler source revision: `0ddd95d`; subsequent evidence updates do not change
the measured compiler or regression tests. All transient checkouts, closures,
generated programs, executables and raw logs are removed after retaining these
compact summaries.

## Gates

```sh
go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 go test -short -p 2 -timeout 30m \
  ./hir/... ./tsfront/... ./internal/... ./cmd/...
```

Both gate commands passed on the final source and regression tests.
The short gate includes the mandatory
`TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend` guard and the Grace
fixture/golden checks. Golden and oracle sources are unchanged.
