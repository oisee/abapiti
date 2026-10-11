# String repeat/match lowering

`repeat` takes a numeric count, truncates toward zero (including -0.9 -> 0),
and raises the existing target range fault for negative/infinite counts or
oversized allocations. Empty receivers avoid allocation. ABAP's existing
Number domain excludes NaN/Infinity; arithmetic producing them already raises.
The Go runtime also implements NaN -> 0 and explicitly rejects infinities.

`match` accepts the forms found in the core source scan: regex literals,
RegExp locals/parameters/constructors, and string patterns (implicitly
constructed as RegExp, notably the abapdoc strings). The global cases are
`/&&/g` and `/&/g` in message.ts. Non-global consumers read full matches,
participating captures, length, index, truthiness, and null checks. HIR uses
Optional<Array<String>> and a match-index operation. Null/truth-only direct
calls reuse regexp.match_test without materializing captures. Receiver effects
precede pattern construction. Global matching resets the regex's lastIndex.

Existing target regex translation and Go's reviewed-pattern restrictions remain
in force; this is not a new arbitrary-JS-regex implementation. Literal match
flags other than i/g get a not-lowered diagnostic. Numbered captures that do
not participate cannot be represented in Array<String> and raise instead of
silently returning the empty string; core capture-array consumers use mandatory
captures. Global index/named groups/input properties are not supported uses.

## Validation

- `go test ./tsfront/... ./hir/...`: PASS.
- Node-generated oracle cases cover 0/1/n/empty/fractional repeat, null, captures,
  index/length, global matches, and implicit string patterns. Go executes them;
  `.github/ci/hir-unit.sh` now runs the dedicated TestEmitStringRepeatMatch fixture
  on both osgo and OSG-JS and requires its one test to succeed.
- Local osgo/OSG-JS execution was unavailable: the runtime checkout lacks its
  installed @abaplint/core dependency. No runtime checkout was changed.
- Unpruned a4h evidence (`--assume-int=false --evidence`, temporary
  GAP_NO_PRUNE=1 coverage=nil hook): **644 -> 383 blocking diagnostics (-261)**;
  HIR verification errors **392 -> 261**. Repeat diagnostics **40 -> 0**;
  match diagnostics **31 -> 0**. Remaining identifiers named `match` come from
  unrelated unsupported RegExp.exec declarations/assignments, not String.match.
  The temporary hook was removed before committing.

## Default a4h output vs origin/main

The initial HEAD was exactly origin/main (69c79ee). The default pruned build
already has source-pinned repeat/match adapters, which remain in place. No new
repeat/match runtime or translated method body appears in its output.

`diff -r` changes only these artifacts:

- `classes/z_closure_1063_d771406cea51bd.clas.abap` is renamed to
  `classes/z_closure_1063_659efc9f101dad.clas.abap` (identical body apart from name).
- `classes/z_runtime_desc_61ac7bd03e15db.clas.abap`: descriptor name/reference.
- `classes/z_src_abap_5_s_7035f4eed88c9b.clas.abap`: two closure-type references.
- `names.json`: corresponding closure and member origin IDs.
- `a4h/abaplint-577f875e-a4h.zip`: contains those changed names/references.

Direct RegExpMatchArray type mapping avoids the old speculative structural-type
lowering, changing the shared lowerer serial by five: closure.106339 becomes
closure.106334 at perform.ts:79:25. As a control, temporarily disabling only
the two new MatchArray type mappings restores the entire default output
**byte-for-byte (`diff -r`: empty)**. Thus this is generated-name drift, not a
reachable string-method behavior change.
