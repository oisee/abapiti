# Conversion-domain roots

Base: `origin/integ/yolo-5@392aecaa1967cdaacec393a061154aadaafc9286`.
Upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e` (unmodified source,
fresh Node oracle, TypeScript 6.0.3).

The baseline report's exact examples are:

| Root | Count | Example |
| --- | ---: | --- |
| nesting | 31 | `test/abap/nesting.ts:253:72: value outside supported numeric, ordering or conversion domain` |
| Domain | 23 | `src/objects/domain.ts:100:3: value outside supported numeric, ordering or conversion domain` |
| Config | 22 | `src/config.ts:11:3: value outside supported numeric, ordering or conversion domain` |
| pretty printer | 15 | `src/pretty_printer/pretty_printer.ts:16:3: value outside supported numeric, ordering or conversion domain` |

These are distinct conversion roots behind one runtime refusal message.

## Live data conversion

`Config.getDefault` receives a plain release/language object through its erased
version union. Checked narrowing used `dynamic.asRef`, which requires native
reference storage. PrettyPrinter similarly assigns `readByRule`'s parsed
indentation object to a typed configuration local. That assignment also used
nominal unboxing; absent configurations trapped before optional field reads.

Unpruned direct coercion, checker narrowing and assertions now share the
existing `dynamic.classView` boundary used by erased interface arguments.
Native instances return unchanged. Certified data layouts retain live source
storage without running constructors. Absent references remain absent, so
optional chains can short-circuit. Private/accessor/prototype-dependent layouts
and incompatible payloads retain explicit refusals. Go and ABAP both implement
the absence rule; the default reachability-pruned path keeps its previous ABI.

## Inherited-method assertion

Nesting obtains a Program through `IObject`, asserts Class, then calls the
inherited `ABAPObject.getMainABAPFile`. JavaScript erases that assertion. A
nominal Class check refuses the correct Program before the shared method call.

An unpruned HIR pass recognizes a uniquely declared, unwritten, nonescaping
assertion local used only through inherited methods and identity-neutral
reference predicates. For class operands it retains the original ancestor
reference. For interface operands it checks the asserted class's immediate
ancestor layout, accepting actual sibling instances through the existing native
class-view boundary. Each called method must have the same declaring owner on
both layouts. Subclass overrides, subclass fields, escaping aliases, arguments,
captures and mutable locals keep their previous checked boundary. No Class
instance, prototype or method is invented.

Node/Go probes cover Child and sibling virtual dispatch, one evaluation,
unchanged instanceof behavior, absence, and incompatible objects refusing.
ABAP emission is checked too; ABAP runtime execution was not performed here.

## Primitive method conversion

Domain's AFF JSON path calls `parsed.format?.length?.toString()` and the same
operation on decimals. The dynamic method fallback treated every receiver as a
string, emitting `dynString` for a numeric JSON value. An unpruned zero-argument
primitive `toString` call now uses the existing primitive conversion operation,
with a separate nullish method-receiver refusal. Optional chains still skip
both conversion and its prelude for absent/null receivers. String, boolean and
supported numeric values are compared with Node; receivers run once. Radix
arguments, object/prototype conversion and numbers outside the existing numeric
serialization domain remain refused.

## Boundaries left to their owners

Domain XML fixed values can omit `DOMVALUE_H` or another field that DomainValue
declares as a required string. Generated construction then calls
`dynString(undefined)`. Preserving that value and its own-property presence
requires changing structural/object-literal storage. Converting it to an empty
string would be wrong. This task explicitly excludes object-literal changes,
so that root remains refused.

Pretty-printer tests that write/delete fields through Config's private native
graph retain their lowering refusals. Own-property write/delete support is
separate from the constructor's live read view and was excluded from this fix.
Malformed JSON/XML adapters and other unsupported operations remain explicit
traps as before. No assertion result or test registration is manufactured.

## Reproduction

Use a clean pinned upstream checkout with `npm ci` in both the repository root
and `packages/core`. Scratch paths below use `TMPDIR`, defaulting to `/tmp`.

```sh
export GOMAXPROCS=2 GOTOOLCHAIN=local
UPSTREAM=${TMPDIR:-/tmp}/abaplint-upstream
SCRATCH=${TMPDIR:-/tmp}/conversion-domain
mkdir -p "$SCRATCH"
git worktree add "$SCRATCH/base" 392aeca
go -C "$SCRATCH/base" build -p 2 -o "$SCRATCH/base-abapiti" ./cmd/abapiti
go build -p 2 -o "$SCRATCH/after-abapiti" ./cmd/abapiti

ABAPITI_TESTS_COMPILER="$SCRATCH/base-abapiti" \
ABAPITI_TESTS_COMPILER_COMMIT=392aeca \
ABAPITI_TESTS_SHARDS=abap,abap/expressions,abap/statements,objects,rules,lsp,pretty_printer \
ABAPITI_TESTS_WORKERS=1 \
node tools/abaplint-tests-all.mjs "$UPSTREAM" "$SCRATCH/base-shards.json"

ABAPITI_TESTS_COMPILER="$SCRATCH/after-abapiti" \
ABAPITI_TESTS_COMPILER_COMMIT=HEAD \
ABAPITI_TESTS_SHARDS=abap,abap/expressions,abap/statements,objects,rules,lsp,pretty_printer \
ABAPITI_TESTS_WORKERS=1 \
node tools/abaplint-tests-all.mjs "$UPSTREAM" "$SCRATCH/after-shards.json"

"$SCRATCH/base-abapiti" abaplint --full "$UPSTREAM" -o "$SCRATCH/full-base"
"$SCRATCH/after-abapiti" abaplint --full "$UPSTREAM" -o "$SCRATCH/full-after"

# Use the same emission path so generated README paths and Go binary debug
# paths agree. Compare every file, including the executable and ABAP archives.
"$SCRATCH/base-abapiti" abaplint "$UPSTREAM" -o "$SCRATCH/default" --target all,go
mv "$SCRATCH/default" "$SCRATCH/default-reference"
"$SCRATCH/after-abapiti" abaplint "$UPSTREAM" -o "$SCRATCH/default" --target all,go
diff -r "$SCRATCH/default-reference" "$SCRATCH/default"

go vet -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
go test -short -timeout 30m -p 2 ./hir/... ./tsfront/... ./internal/... ./cmd/...
ABAPITI_GRACECHECK=1 go test -short -p 2 ./tsfront \
  -run 'Test(DirectDataConversion|Common.*Assertion|DynamicPrimitiveMethodConversion|LexerFactsReport|EmitRegistry)'
go test -short -p 2 ./cmd/abapiti -run TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend
./.github/ci/lint.sh gate origin/integ/yolo-5
```

`abaplint --full` is present in origin/main. Full-closure blocking findings stay
**44 (31 root, 13 cascade)**, with **103 HIR errors in 28 files**. Both complete
gap summaries, including counts by normalized message, are recorded in
`evidence/conversion-domain-full-{base,after}.json` and are identical. The exact
runtime-domain refusal is not a lowering diagnostic: its full-closure blocking
count is zero before and after. Runtime recovery is measured by the selected
shards, separately from this static gap.

## Selected-shard result

Only the seven affected shards were measured; the complete 17-shard suite was
not run. Registrations, Node oracle results and not-run counts are unchanged.

| Shard | Pass before | Pass after | Trap before | Trap after | Fail after |
| --- | ---: | ---: | ---: | ---: | ---: |
| abap | 168 | 199 | 48 | 17 | 0 |
| abap/expressions | 42 | 44 | 8 | 6 | 0 |
| abap/statements | 4942 | 4962 | 20 | 0 | 0 |
| lsp | 97 | 101 | 91 | 87 | 0 |
| objects | 105 | 109 | 39 | 35 | 0 |
| pretty_printer | 10 | 21 | 21 | 10 | 0 |
| rules | 2752 | 2755 | 395 | 392 | 0 |
| Total | 8116 | 8191 | 622 | 547 | 0 |

There are 79 not-run cases in both measurements. The four owned roots decrease
from **91 to 19 refusals**: nesting 31→0, Config 22→0, pretty printer 15→0,
Domain 23→19. The 19 Domain XML cases need the excluded object-literal/absence
storage change described above. Three additional rules cases recover in the
aggregate, giving **75 total recoveries**. The baseline rules per-case artifact
was not retained before scratch cleanup, so those three prior messages and
titles are not attributed. All 72 owned recoveries have exact matched titles
and baseline refusal messages in the evidence.

Evidence:

- [Baseline selected report](evidence/conversion-domain-base.json) and
  [final selected report](evidence/conversion-domain-after.json).
- [Owned root counts and 72 matched recoveries](evidence/conversion-domain-recovered.json).
- [Commands, compiler source blobs and passing gates](evidence/conversion-domain-verification.json).
- [Exact blocking messages before](evidence/conversion-domain-blocking-base.json)
  and [after](evidence/conversion-domain-blocking-after.json).

Vet, short tests, Grace checks, the default backend-operation coverage test,
and lint against origin/integ/yolo-5 passed. Grace goldens needed no update.
The default zabapgit output diff is empty, including Go executables and ABAP
archives. The default comparison uses the task-specific integration base.
