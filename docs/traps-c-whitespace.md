# Batch C: reviewed ECMAScript whitespace regexp

`/\s+/g` now uses the shared reviewed-pattern table with the exact ECMAScript
WhiteSpace + LineTerminator code points. It deliberately does not use RE2's
ASCII `\s` or Unicode `White_Space`: U+0085 is excluded and U+FEFF included.
The existing global match/replace machinery remains unchanged. Both Go and ABAP
consume the shared translation; ABAP's existing PCRE syntax check passes.

`TestWhitespaceRegexNodeOracle` independently checks replacement, match text,
UTF-16 match index, and `lastIndex` against Node for every non-surrogate BMP code
point, consecutive whitespace, empty/no-match inputs, and an astral prefix:
**63,492 inputs**. The reviewed-pattern Node oracle also passes.

Focused original DDL shard: **17/32 passed, FAIL 0, 15 trapped**, versus the
batch-B 4/32 baseline. Remaining refusals are the separate chai.deep root. This
is a focused measurement, not a completed whole-suite batch-C total.

Commands (Grace enabled, Go 1.26.0, buildvcs disabled):

- `go test ./hir/golang -run 'TestWhitespaceRegexNodeOracle|TestReviewedRegexNodeOracle' -count=1`
- `go test ./hir/jsregex ./hir/abap -short`
- Original `test/ddl/tabl.ts` closure via `abaplint-tests-closure.mjs` and
  `abapiti abaplint-tests`: 32 registered, no failures or initialization losses.

Generated closure and build were deleted. No assertion mismatch was observed.

## Shared conversion triage (not fixed here)

The config setters' erased bridge performs a nominal `castRef` on a plain object,
for example `SevenBitAscii.setConfig` casts `dynRef` to `SevenBitAsciiConf`.
A snapshot/materialization is not a general JS fix: identity and live writes
must be preserved. No nominal type/ABI code was changed for this triage.

## Incomplete integration

Another process was actively editing this same worktree (`tsfront/lower_expr.go`
and `tsfront/receiver_context_test.go`) during this work. Those edits were left
untouched and are not part of this root. Full-suite, default-output, zabapgit,
and repository-wide final gates have not been certified for this mixed working
tree. The batch-B whole-suite evidence remains the last completed measurement.
