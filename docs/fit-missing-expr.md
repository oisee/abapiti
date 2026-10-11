# Concat calls and record property keys

Base: `origin/integ/yolo-5`, `392aecaa1967cdaacec393a061154aadaafc9286`.
Branch: `feat/fit-missing-expr`. Upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

The base report assigns 39 traps to
`test/abap/syntax/syntax.ts: HIR verification: missing expression` and 18
to `src/objects/_abap_object.ts: HIR verification: call argument type mismatch: string <- dynamic in runtime .`.

The missing expression is an argument of a lowered **CallExpression**, not
an unimplemented AST expression kind. The local `runFixtures` helper calls
`fixtures.concat([{filename, contents}], extra ?? [])`. Array concat previously
used the one-argument helper, which returns nil for two arguments, and inserted
that nil into HIR without a diagnostic. Concat now handles zero arguments,
multiple arrays, and scalar items. The receiver and arguments are evaluated
before any copying; arguments run once, in source order. Compatible subtype
arrays follow the existing HIR covariance rules. Copies retain element identity.
The established single-array path is preserved.

The dynamic argument mismatch is an **ElementAccessExpression**: reads and
writes of `this.texts[id]` use a tagged key, but the record runtime requires a
string. Ordinary string-keyed record access now applies JavaScript primitive
ToPropertyKey conversion, including tagged strings, numbers, booleans, null,
and undefined. This also fixes the corresponding errors in
`src/rules/remove_descriptions.ts`. Map method arguments keep their existing ABI.
Object ToPrimitive and Symbol keys remain explicit refusals. No object-literal,
typeof, RegExp, reserved builtin, void ABI, or WASM lowering was changed.

Node differential tests cover argument order, mutation before copying, empty
concat, scalar concat, subtype arrays, shallow identity, and primitive record
keys. Their HIR verifies and emits through both Go and ABAP. Tagged concat
arguments with unknown array shape and argument spreads still refuse explicitly.

Full source closure command, run with the base and branch compilers:

```sh
GOMAXPROCS=2 "$compiler" abaplint --full -o "$evidence_dir"
```

`--full` is present in origin/main. It measures the 1,538-file source closure
without the reachability manifest; it does not include upstream test files.
[Exact diagnostic histograms](fit-missing-expr-full.json) record the commands and
branch compiler source hashes. Blocking diagnostics remain **44 → 44** (31 root,
13 cascade); HIR errors fall **103 → 98**. `string <- dynamic` argument errors
fall **5 → 0**. Source-closure `missing expression` errors remain **12 → 12**,
in `lsp/code_actions.ts`, `lsp/dump_scope.ts`, `lsp/help.ts`,
`objects/rename/renamer.ts`, `rules/downport.ts`, and `rules/object_naming.ts`.
These are separate roots outside this task, including reserved LSP/regexp work.

Focused upstream measurement command, run for the base and branch binaries:

```sh
GOMAXPROCS=2 ABAPITI_TESTS_SHARDS=abap/syntax,objects,rules \
  ABAPITI_TESTS_WORKERS=1 ABAPITI_TESTS_COMPILER="$compiler" \
  ABAPITI_TESTS_COMPILER_COMMIT=392aeca \
  node tools/abaplint-tests-all.mjs "$upstream" "$report"
```

The harness revision field names the common base; the after compiler is built
from the branch working tree. Only these three shards are measured. The full
17-shard suite remains a train-assembly check.

[Paired focused reports](fit-missing-expr-tests.json):

| Shard | Pass before → after | Trap before → after | FAIL |
| --- | ---: | ---: | ---: |
| abap/syntax | 1,310 → 1,349 | 53 → 14 | 0 → 0 |
| objects | 105 → 108 | 39 → 36 | 0 → 0 |
| rules | 2,752 → 2,765 | 395 → 382 | 0 → 0 |
| Total | 4,167 → 4,222 | 487 → 432 | 0 → 0 |

All 39 requested syntax traps now pass. The 18 `_abap_object.ts` dynamic-key
verifier traps fall to zero: 16 pass and two expose an existing runtime refusal
at `_abap_object.ts:151:82`, `parseInt(t.LENGTH, 10)`. The function-group
`findTextFile, Z case` and `findTextFile, namespace case` fixtures omit LENGTH;
JavaScript returns NaN, while the current frontend explicitly traps a missing
parseInt result. Numeric/NaN representation is a separate root; this change
preserves that refusal. No assertion mismatch is hidden as a trap.

The focused Node oracle has 4,656 active cases and 21 pending cases. Not-run
stays 23, including those 21 pending cases and two existing unregistered rule
cases. Unmatched cases and infrastructure failures are zero. The remaining
432 traps belong to other roots, including regexp, numeric-domain, LSP and
callback lowering; the original whole-suite report is preserved.

Validation commands:

```sh
GOMAXPROCS=2 GOFLAGS=-p=2 go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...
GOMAXPROCS=2 go test -short -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
GOMAXPROCS=2 ABAPITI_GRACECHECK=1 go test -short -p 2 ./tsfront -run '^TestLexerFactsReport$' -count=1
GOMAXPROCS=2 go test -short -p 2 ./tsfront -run '^TestConcatAndRecordKeysNodeOracle$' -count=1
GOMAXPROCS=2 go test -short -p 2 ./cmd/abapiti -run '^TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend$' -count=1
GOMAXPROCS=2 GOFLAGS=-p=2 ./.github/ci/lint.sh gate origin/integ/yolo-5
```

All pass. The Grace golden is unchanged. Lint uses CI's checksum-verified
golangci-lint 2.13.2 and reports zero new issues, canary 5/5.

Default output is byte-identical to both origin/main (`a55a2f8`) and the
integration base (`392aeca`). Each compiler is built with `go build -p 2`.
All three generate into the same absolute output directory, which is copied
to a separate snapshot after each invocation. This keeps path-bearing README
text and Go executable debug paths equal, without normalizing generated files:

```sh
GOMAXPROCS=2 "$compiler" abaplint -o "$common_output" --target all,go
cp -a "$common_output" "$snapshot"
# Remove common_output before the next compiler's invocation.
diff -r "$main_snapshot" "$branch_snapshot"
diff -r "$base_snapshot" "$branch_snapshot"
```

Both diffs are empty (exit 0).
