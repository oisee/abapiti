# Trap batch B roots and integration gates

Worktree/branch: `trapsb`, `feat/fit-traps-b`. Upstream is read-only at
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`. No push or transpiler clone.

## Integration and the stopped run

Local no-fast-forward merges: `30b3bd4` (CDS grouped-switch fix/all-shard runner)
and `3cfbda5` (batch A). Conflict resolution retains the train-2 **ABAP config
writer** as well as batch A's JSON graph, artifacts inventory, constructor
arguments and regexp grammar. Base tests and vet passed. A same-absolute-path
comparison of default output against train-2 changed **only** the paths listed
in [the CDS report](cds-lexer-miscompile.md).

The first full measurement stopped at a **genuine syntax assertion mismatch**;
see [the object-rest root cause](traps-b-object-rest.md). This was not relabeled
as a refusal. `a990d9b` repairs it before trap work. The repeated complete base
measurement is **8,142/11,060; FAIL 0**. All 625 files/17 shards reconcile to
11,060 active and 32 pending original-Node titles. Exact top-20 reached traps,
per-shard counts and an empty failure list are retained in
[`abaplint-tests-traps-b-base.json`](abaplint-tests-traps-b-base.json).

The runner now stops after the first shard containing a failed record and prints
its actual titles/messages. Typed runtime/lowering refusals stay trapped;
reports do not reclassify failures. Cleanup still runs when stopping.

## Implemented roots (one commit per root)

### Parsed XML/JSON array ABI — `68b479f`

The biggest syntax runtime blocker was `xmlToArray`: 20 syntax tests, 66 across
all shards. Parser arrays used `array[*dynamic]`, whereas native reference arrays
(including `any[]`) use `array[any]`. `dynRef` also rejected the array tag.
JSON/XML parsers now allocate the canonical reference-array storage. Native
views share the **same object**, not a converted snapshot. Reads, replacements,
pushes and repeated views stay live; boxing a native view retains parsed-array
identity. Typed data materializers still make their existing checked projections.

Node/Go regression: parsed JSON -> `xmlToArray`-shaped helper -> two aliases ->
replacement/push -> original data reads and serialization. An emitted XML
runtime regression checks repeated XML elements, alias identity and mutation.
Existing JSON/config/XML tests pass. ABAP's existing tagged-to-native array
boundary still refuses explicitly; no ABAP array execution is claimed.

Focused syntax: **1,244 passed, FAIL 0**, versus 1,243. Nineteen formerly blocked
cases reach the independent DDIC `Math.ceil` refusal, rather than count as passes.

### ASCII word classes and capture-safe groups — `7f7016b`

The initial biggest new-shard refusal was `/^@\w+$/`: **340 reached tests**,
324 CDS and 16 DDL. The certifying parser now supports ASCII `\w`, `\W`, `\d`,
`\D`, positive word/digit escapes in character classes, ASCII folded classes,
ordered alternation, noncapturing groups, and nonrepeated captures/optional
captures. Dot remains one UTF-16 unit excluding JS's four line terminators.
No Go Unicode case folding is substituted for non-u JavaScript semantics.

Repeated captures refuse: RE2 retains captures that JavaScript resets between
iterations. Backreferences, general lookarounds, Unicode/whitespace escapes,
unsupported flags, empty branches and lazy/bounded repeats still refuse.
Node/Go tests check match text/index and capture absence, ASCII/non-ASCII word
membership, astral input, line terminators, quoted identifiers, function-group
filenames and labels. ABAP retains its reviewed-pattern refusal for additions.

Focused CDS: **44 passed, FAIL 0**, versus 12. Focused DDL: **4 passed, FAIL 0**,
versus one. The next CDS barrier is a separately unsupported string language.

### Numeric `Math.ceil` — `20f4aab`

After XML arrays, DDIC conversion becomes syntax's largest blocker (27 reached
tests). Only the default-library Math binding with one numeric argument is an
intrinsic; shadowed Math calls retain their own behavior. Receiver-independent
argument effects run exactly once. Go uses IEEE-754 `math.Ceil`, including signed
zero, NaN and infinities. Input coercion/spread/other arities refuse. ABAP
explicitly rejects the new `number.ceil` HIR operation.

Focused syntax, with XML and the first regexp root: **1,252 passed, FAIL 0**.

### Related nominal class views — `a0bc4fb`

Many syntax/object/type tests assert the concrete runtime class, then use an
`as` assertion to access it. Valid related nominal views now keep the original
instance/identity. Unproven downcasts are admitted only in a checked Go domain:
a mismatched runtime type raises a typed refusal, **not a catchable JS error**.
ABAP explicitly refuses `object.checkedClassView`; it does not emit an unproven
`?=` cast. Pruned/default lowering and existing pinned proofs are unchanged.

Node/Go tests cover identity, live inherited fields, once-only evaluation and
refusal escaping source catch. The original critic fixture is retained: valid
Sub passes, a Base outside the domain refuses, ABAP refuses emission. Its old
whole-body-diagnostic expectation was replaced by these stronger boundary tests.

Focused syntax: **1,274 passed, FAIL 0**; types: **21 passed, FAIL 0**; objects:
**30 passed, FAIL 0**. These focused totals are not added to the final suite.

### Quote-delimited regexp language — `db05a5b`

The next CDS refusal affected **292** tests:
`/^'(?:[^'\\]|''|\\'|\\\\|\\(?!'))*'$/`.
The local lookahead can be eliminated for this **complete regular language**:
a nonempty backslash run can escape a quote or precede an ordinary unit; a run
of at least two can leave a following quote unconsumed. The certifier translates
these alternatives, without claiming general lookahead support.

An independent Node oracle exhaustively compares **349,525** strings of lengths
0..9 over quote/backslash/ordinary/newline. Emitted-Go exec tests additionally
cover Unicode/astral strings and line terminators. A simplistic pairwise escape
rewrite failed the oracle and was discarded; the final translation passes.
Focused CDS: **334 passed, FAIL 0**, versus 44 after the preceding grammar root.
ABAP still refuses this additional unreviewed pattern.

## Default output and full-workload follow-up

Final default ABAP output differs from the merged base only in
`z_src_abap_5_s_92c431cfd2b94f.clas.abap` (identically under `classes/`, `osg/`,
`native/lib/`) and the containing A4H zip. This is **InlineData's corrected
object-rest semantics**, not a test-only override or unrelated drift.

Because that output changes, the full zabapgit workload was rerun:
`tools/hir-go-check.mjs` freshly rebuilt original Node and compared clean
(**zero issues**) and seeded (**five issues**) to generated Go and the release
kit; `tools/go-target-check.py` independently compared both generated-Go outputs
byte-for-byte. All agree. The existing release ABAP-derived Go binary was also
checked on clean input; no execution of newly generated ABAP is claimed.

Downloaded kit SHA-256:
`2e3311517885ed72e651c97004d6cba064fa5093873c75f1104955891e965d7a`.
Release binary SHA-256:
`e9f9666e3a2b66c762b4731528425f6379fb552834897faeb2555e42cc6a6ac7`.
The kit, binaries, generated module and profiles were deleted after checking.

Runtime-effect catalogue goldens were regenerated with `GRACE_FACTS_OUT`; the
only changes are counts for the two new reviewed operations and one conservative
record. No lexer oracle or upstream source was edited. Final suite/gates and
remaining top traps are in [abaplint-tests.md](abaplint-tests.md).

Dell's registry assignment, EditHelper forEach, qualified-type panic, nested
unused-variable optionals and dynamic `sy_read_restriction` keys were not edited.
LSP's unproven shape own-key enumeration, chai deep equality, helper captures and
remaining registration losses are still explicit limitations, not completion
claims.
