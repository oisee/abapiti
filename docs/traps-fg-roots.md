# Reconciliation of trap batches F and G

Parents: `origin/feat/fit-traps-g@dd140d6` and
`origin/feat/fit-traps-f@6a20ee4`; shared base `7c2d33f`.
Upstream remains `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.
The measured semantic source is merge `ac868c0` (final amendment adds only
certification documentation/evidence). F and G measurements remain historical
in [abaplint-tests.md](abaplint-tests.md) and their original evidence files.

## One implementation per feature

* **Split:** retain G's `regexSplitRuntimeSource`, emitter `regexSplit` gate,
  two-argument `string.splitRegex(RegExp, Number)` signature and lowering.
  Discard F's narrower runtime source/emitter gate, duplicate capability-map
  entries, switch arm and effect record. There is exactly one operation and
  one implementation. G covers JS empty-match suppression, captures, UTF-16,
  ToUint32 limits, evaluation order, original anchor context and lastIndex;
  F supported only nonempty, capture-free delimiters. ABAP still explicitly
  refuses this Go-only operation. All original G tests remain, with F's cases
  in `tsfront/regex_split_f_test.go` and a new cross-feature bounded-split oracle.
* **Construction/grammar:** one shared `TranslateSubset`, used for literals
  and dynamic constructors; G's emission gate remains. Retain F's greedy,
  capture-safe bounded repetitions and checked erased-string arguments, plus
  G's `$^` empty-only certification and exact naming reviews. Whitespace and
  ASCII boundary implementations are shared, not duplicated. F's class
  whitespace membership flag deduplicates equivalent repeated `\s` fragments.
  Unsupported coercion, syntax and nullable string-array escapes still refuse.
* **Non-overlap:** retain all F registration-helper/static-capture work,
  missing-title reporting, multi-array concat evaluation order, dynamic JSON
  array fallback/for-of, Chai `instanceOf`, and shard-order helper. Preserve all
  G/base live-array/own-key work and the Chai live-length guard. The existing
  ABAP `dynamic.classView` catalogue entry was already present on G.

## Contradictory domain tests decided by Node

F's three split probes (`capture`, `boundary`, `empty`) asserted typed refusals
because its implementation domain was narrower. Node returns `["a"," ","b"]`,
`["ab"]`, and `[]`, respectively. Their exact source cases are retained and
now compared with fresh Node results, not dropped or made no-ops. The original
F ABAP split-specific refusal check also remains on its original source.

G's old `a{2}` negative represented a previously uncertified domain, not a JS
syntax error. F certifies it against Node; retain that case as an explicit
positive translator test and live Node split differential. All other distinct
negative regex cases from both parents remain, including repeated captures,
oversized/malformed/lazy bounds, unsupported flags, class ranges and Unicode
properties. F's existing qualified-alias sibling negative remains; G already
had the nominal-ancestry positive fixture, so no test is weakened here.

The first F test adaptation tried to call usage-gated JSON helpers from its Go
main without a corresponding HIR operation, then its added JSON operation
masked the original ABAP split refusal. These were **test harness errors**, not
Go/Node value mismatches: serialization now runs through TypeScript/HIR, while
the ABAP check uses the unchanged original F source. Both checks pass. Early
gate executions before final conflict/test resolution were discarded and
rerun; no partial run contributes to the certificate. No genuine assertion
mismatch has been accepted or hidden.

## Goldens and default output

`ABAPITI_GRACECHECK=1 GRACE_FACTS_OUT=<scratch>/lexer.facts go test ./tsfront
-run '^TestLexerFactsReport$' -count=1` exports facts **byte-identical to G's
committed golden**. No golden was hand-edited or regenerated from a different
path. There are no golden diffs: G already has the single split record and
live-class/own-key catalogue facts. F's historical smaller catalogue counts
must not replace those facts. Full Grace-enabled tests also check the other
goldens; no oracle changes are needed.

Default `abaplint -o <same absolute path> --quiet` from an archived G compiler
and the reconciled compiler has an empty raw `diff -r` (exit 0), including zip
output. Thus there are **no default-output changes to justify**. Separate
`--target go` emission is exercised on clean/seeded zabapgit against fresh
original pinned Node and the release kit. Full counts and gates are recorded
in [the certificate](abaplint-tests.md) and compact FG evidence. Generated
builds, source archives, closures, logs, kit copies and binaries are removed
at the end. No push or transpiler clone.
