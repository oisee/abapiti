# TS-HA@OSGO numeric emission

This branch adds guarded ABAP emission for dynamic `string.parseIntNumber`
results, `number.ceil`, and `i64` dynamic boxing. The ABAP Number ABI remains
`f`, and the dynamic numeric payload remains `f`; there is no NaN, Infinity,
or portable signed-zero extension to that ABI. Accepted values preserve
JavaScript/Go results. Unproven values explicitly refuse rather than returning
an initial/default value.

Base: `origin/integ/o1` at `63b91b734f69bfe5ebf3922d8d406ee2478613ef`.
PR #134 merged during validation. The implementation was rebased onto main
at merge commit `428a8d5`; its source tree is identical to the O1 base.
No abaplint suite harness, lowering policy, Go backend, or upstream suite source
was changed. The HIR CI test kit now includes the numeric fixtures.

## Static closure measurement

The existing `abaplint-tests --target osg --gap-only` counter from
[osgo-suite-gap.md](osgo-suite-gap.md) is used unchanged, with
`ABAPITI_INLINE=classic`. The same pinned upstream
`577f875ebec44cfaf64841cfe71c8ab8dc32622e` supplies 625 roots, 17 shards,
and 2,166 source files, yielding 7,052 HIR classes and 85 interfaces. Runtime operations are counted before ABAP passes;
emitter refusals are deduplicated after ABAP passes, with source/body context.
These are different inventories and are not summed into one site census.

The completed O1 baseline and final census report:

| Assigned gap | Inventory | Before | After |
| --- | --- | ---: | ---: |
| Dynamic/nonfinite `parseInt` Number input | Post-pass emitter refusals | 66 | 0 |
| `number.ceil` | Pre-pass unsupported runtime sites | 9 | 0 |
| Dynamic boxing of `i64` | Post-pass emitter refusals | 4 | 0 |

The unsupported catalogue shrinks from 24 to 23 operations; occurring
unsupported runtime groups shrink from 22 to 21, with sites **113 → 104**.
Post-pass emitter-refusal groups shrink from three to one, with sites
**72 → 2**. The remaining two sites are erased structural `object.has` in
`src/objects/rename/renamer.ts` and `test/lsp/_apply_edit.ts`; their erased
structural membership ABI remains unimplemented and is outside this numeric
change. All other unsupported runtime groups and their counts are unchanged.
Class-of-root-object and fresh lexical-descriptor refusals from M0 were already
removed by O1, independently of this change.

Both censuses cover 2,166 files with zero blocking diagnostics, HIR errors,
and coverage traps. Surviving nonfinite literal sites remain zero. The final
census completed successfully using `GOMEMLIMIT=20GiB`; an earlier attempt was
restarted after competing census processes caused severe memory paging.

A zero static refusal count means the owning body can emit guarded ABAP. It
does not mean every runtime value succeeds, nor does it certify 11,060 ABAP-host
tests. Whole-suite execution and attribution remain owned by the existing
suite harness.

## Accepted values and explicit remaining refusals

- `string.parseIntNumber` has a string operand and a Number result. Dynamic
  inputs now skip ECMAScript leading whitespace, consume one optional sign,
  and parse decimal digits until the first non-digit. An `int8` accumulator
  checks the safe-integer bound **before** multiplication/addition. Leading
  zeroes do not trigger a length-based rejection, and state resets on each
  loop visit. No digits, a negative sign with a zero result, or magnitude above
  `9007199254740991` raises `cx_sy_range_out_of_bounds`. This includes NaN,
  Infinity, negative-zero results, and unproven binary64 rounding outside the
  safe-integer subset. Statically invalid literals retain emitter refusals.
- A Number argument still goes through the existing Number/dynamic `ToString`
  boundary **before parsing**. Safe integers render as decimal; Number `-0`
  renders as `"0"`, so parsing returns positive zero. Fractional/unsafe Number
  inputs retain that boundary's explicit runtime refusal. For example, Node
  parses Number `1e21` or `1e-7` as `1` because their strings use exponents;
  ABAP refuses those inputs instead of substituting numeric truncation. String
  `"1e21"` parses as `1`, and string `"1.9"` parses as `1`.
- `number.ceil` evaluates its input once, checks the finite binary64 bound,
  and computes the ceiling in `f`. At magnitude at least `2^52`, every finite
  binary64 input is already integral, so it returns the input directly.
  Smaller values use truncation and an increment only when needed. Dynamic
  inputs `-1 < x <= 0` raise because the ABI cannot portably preserve the
  negative-zero result or distinguish the sign of an input zero. A proven
  positive-zero literal succeeds; negative-zero and nonfinite literals
  explicitly refuse. Both largest finite binary64 inputs succeed unchanged.
- `i64` dynamic boxing checks the original integer against the inclusive
  safe-integer bounds before assigning to the dynamic box's `f` payload.
  The box has numeric tag/type and exact value, rather than string or object
  semantics. Larger integers explicitly raise, including both `int8` extrema;
  no narrowing, rounding, or fallback zero is accepted. Optional `i64` follows
  the same guard when present and preserves absence when absent.

Other numeric producers and the unrelated Chai/JSON/string/XML/storage gaps
retain their existing refusals. This change does not broaden general Number
string formatting, nonfinite arithmetic, or the integer-lowering contract.

## Executed ABAP tests and gates

`TestNumericFixtures` materializes **68 ABAP Unit cases**, wired into the
existing `.github/ci/hir-unit.sh` kit. Node supplies parse/ceil expected values;
cases cover whitespace, signs, decimal prefixes, arbitrarily long digits,
safe-integer edges, fractions, exponent strings, Number-to-string conversion,
negative zero, subnormals, the `2^52` boundary, largest finite Numbers, boxing
tags/values, parser state reuse, and optional integer presence/absence in a loop. Refusal tests catch the precise
range exception and fail if a default value is returned. Additional emitter
tests require nonfinite/negative-zero ceil literals to refuse.

On the O1 OSG pin `f557e12f4217addcec7834b0607234aecf9b723a`, the emitted
ABAP passes syntax checking (33 files, zero issues) and compiles/runs through gogen
and the OSG Go kernel: **68/68 SUCCESS, failure 0, NOT_COMPILED 0, error 0**.
The same 68 cases also pass on OSG-JS with zero failures/errors.
The installed open-abap-core is the locked
`d5aea88bdb3c8abfe3d57d35b686e933a2bdcfae`; Node is v26.9.0 and Go is 1.26.0.

The resumed final gate sequence also passed in full: the `tsfront` short
package completed in 572.426 seconds, Grace in 7.571 seconds, and the backend
catalogue gate in 21.634 seconds. Lint reported zero new issues and canary 5/5
with golangci-lint 2.13.2. The Grace golden remained unchanged.

Passed:

- `go vet -p 2 ./...`.
- `go test -short -p 2 -timeout 30m ./...`. The uncached `tsfront` package
  completed in 451.225 seconds during implementation validation. Initial runs
  expired under shared load, and one run saw lint's deliberate canary; the
  subsequent clean full gate passed. The resumed gates run sequentially to
  keep the lint canary out of test runs.
- `ABAPITI_GRACECHECK=1 go test -p 2 -count=1 -run TestLexerFactsReport ./tsfront/`.
  The Grace golden did not change.
- `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend` with `-p 2 -count=1`.
- `./.github/ci/lint.sh gate origin/integ/o1`: **0 new issues, canary 5/5**.

The default zabapgit Go emission (`abaplint --target go`, default workload and
integer policy) was generated at the same output path before and after:
**raw `diff -r` exit 0, zero bytes**, including eight Go-module files (the built
executable included) and `sites.json`. This was repeated with the final
committed implementation and remained empty. Go emission code was unchanged.

## Reproduction

Use disposable directories and the exact upstream/OSG pins above. `UPSTREAM`
points to the clean upstream checkout, `OSG_HOME` to the O1 host checkout, and
`WORK` to a fresh disposable directory. Dependencies must already be installed. The final static audit uses a 20 GiB
Go heap budget; this changes resource use, not the counting or emission policy.

```sh
export GOFLAGS=-p=2
export GOMEMLIMIT=20GiB
node --input-type=module - "$WORK/closure" "$UPSTREAM" <<'JS'
import {resolve} from 'node:path';
import {discoverShards} from './tools/abaplint-tests-all.mjs';
import {materializeRegistryClosure} from './tools/registry-closure.mjs';
const [out, upstream] = process.argv.slice(2);
materializeRegistryClosure(out, upstream, {
  roots: discoverShards(upstream).files,
  runtime: resolve('tsfront/testdata/abaplint-tests/runtime'),
});
JS
go build -p 2 -o "$WORK/abapiti" ./cmd/abapiti
ABAPITI_INLINE=classic "$WORK/abapiti" abaplint-tests "$WORK/closure" \
  --target osg --gap-only -o "$WORK/gap"
ABAPITI_TEST_OUT="$WORK/fixtures" go test -p 2 ./hir/abap \
  -run '^TestNumericFixtures$' -count=1
node .github/ci/hir-lint.mjs "$WORK/fixtures/TestNumericFixtures" \
  "$OSG_HOME" --fixture
(cd "$OSG_HOME" && npm run -s osgo:unit -- \
  "$WORK/fixtures/TestNumericFixtures" --json --jobs 2)
```

Run the same closure counter using an O1 binary for the before inventory. No
harness edits, test pruning, or changed refusal deduplication are needed.
