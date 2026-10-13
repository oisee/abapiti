# TS-HA@OSGO: CDS and DDL

## Numbers first — measured at `11c0787`

| Shard | Node-active N | Pass | FAIL | Trap | Unregistered |
| --- | ---: | ---: | ---: | ---: | ---: |
| CDS | 358 | 13 | 0 | 345 | 0 |
| DDL | 32 | 4 | 0 | 28 | 0 |
| **Total** | **390** | **17** | **0** | **373** | **0** |

CDS also has **one pending title**, registered but not run, outside N. There
are 391 registrations overall, no initialization failures, no unmatched titles,
no active skipped titles, and no async-pending outcomes. Traps remain traps.

| Final first-executed root | CDS | DDL | Total | Owner / next action |
| --- | ---: | ---: | ---: | --- |
| Positive-length `new Array(n)` requires hole-aware storage | 344 | 28 | **372** | **Dell: array storage** |
| Erased native zero-argument method dispatch, `dynamic.call0` | 1 | 0 | **1** | **Dell class-view ABI / ABAP native-dispatch integration** |
| Owned harness, regex, normalization, JSON/XML runtime roots | 0 | 0 | **0** | Alice: cleared as first-executed roots, not a claim about hidden downstream work |

### Measurement history

| Stage | CDS pass / FAIL / trap / unregistered | DDL pass / FAIL / trap / unregistered |
| --- | --- | --- |
| First trustworthy reports, `7b6e2fa` | 12 / 0 / 346 / 0 | 4 / 0 / 28 / 0 |
| Native absolute assertions, `a1d3b82` | 12 / 0 / 346 / 0 | 4 / 0 / 28 / 0 |
| Shared certified literal grammar, `11c0787` | **13 / 0 / 345 / 0** | **4 / 0 / 28 / 0** |

First-report roots were 313 native PCRE validation refusals (308 CDS, 5 DDL),
60 initially opaque Chai refusals (37 CDS, 23 DDL), and one owning-body dispatch
refusal. Fixing absolute assertions exposed the actual **313 unreviewed CDS
quoted-string grammar refusals**, rather than passing those tests. Certifying
the literal grammar then exposed the storage boundary. The old 60 Chai refusals
were also identified as that same positive-length array allocation boundary.
The newly passing title is `CDS Parser extension, parsing error`.

## Evidence and counting

[Machine-readable evidence](evidence/osgo-cds-ddl.json) contains all three
measurement summaries, complete per-title Node and native outcomes, native
refusal/host evidence, normalized paths, and raw native-report SHA-256 values.
Its final `literal-subset` measurement is authoritative. Counts are by outcome
and title ownership, **not** static gap-site multiplicity. In particular the
repeated `dynamic.call0` diagnostic still accounts for only one title.

* Upstream: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`; freshly compiled original
  tests with TypeScript **6.0.3**. An independent combined Node run produced
  **390 pass, 0 FAIL, 1 pending**. Each measurement's shard-title multiset was
  reconciled with this combined oracle, including the pending title.
* OSG: clean at `368baab28111a072c3f5779a0e07d259305f836f` throughout. No shared
  source changes, clone, or push. Worktree: `feat/osgo-cds-ddl`.
* Full CDS and DDL closures fit without file splitting. Focused-file selection
  support was added in `755906f`; it retains oracle and ownership validation.
* Every live OSGO run acquired the shared flock, checked load **<12** and IO
  `avg10` **<30 after acquisition**, then ran in a systemd scope with
  `MemoryMax=48G`, `nice -n 10`, and one heavy job at a time.

| Final native host metric | CDS | DDL |
| --- | ---: | ---: |
| ABAP objects/files | 2,674 | 2,346 |
| Maximum line bytes / overlong lines | 255 / 0 | 255 / 0 |
| Maximum object-name bytes | 29 | 29 |
| Frontend ms | 179,634 | 108,893 |
| Go emission ms | 29,321 | 16,352 |
| Go build ms | 246,397 | 97,224 |
| Execution ms | 1,109 | 69 |
| Native-only absolute-assertion literal sites | 139 | 139 |

The final host exits **1** because tests are trapped, not because reporting
failed. Both shard reports are complete. Adapter-site counts describe emitted
literals, not tests or runtime executions.

## Owned repairs

1. **Harness boundaries (`7b6e2fa`).** Recover only native `NOT_COMPILED` at
   generated driver boundaries, after unwinding source execution. Other panics
   rethrow; source TRY/CATCH cannot swallow host refusals. Separate initialization
   actions preserve their identities and real missing registrations. Report
   exception-class/HIR identity; retain opt-in artifacts and native refusal
   evidence. No fabricated registrations or synthetic passes.
2. **Normalization (`7b6e2fa`).** Spell certified PCRE hex characters literally
   with correct metacharacter and extended-mode protection. Specialize only fresh
   singleton `/\u00a0/g` and `/\u000b/g` replacements with literal dollar-free
   replacements into exact substring replacement. Stateful receivers, other
   patterns/flags, and dynamic or dollar-bearing replacements are excluded.
   Multiline, repeated normalization, supplementary characters, and non-whitespace
   controls have collecting-emitter and Node-oracle regressions.
3. **Native anchor bridge (`a1d3b82`).** The exact compiler-internal absolute
   assertions `(?<![\s\S])` and `(?![\s\S])` become RE2 `\A` and `\z` only in
   invocation-local OSGO IR, scoped by SourceMap to `runtime.regexp_js`.
   Constant concatenations cover literal chunking. Unrelated classes and all
   other lookarounds remain untouched. **Emitted ABAP retains the JS-compatible
   assertions:** this is not the unsafe cross-host source-anchor substitution.
   The Node/RE2 oracle covers implicit multiline mode, line endings, ordered
   alternation, supplementary characters, and source isolation.
4. **Literal grammar (`11c0787`).** Go already admits the exact CDS delimited
   string language through `jsregex.TranslateSubset`; the ABAP checked runtime
   previously used only `Reviewed`. Its invocation-local table now additionally
   certifies literal RegExp constructors using that **same parser**, retaining
   reviewed precedence. PCRE rendering converts translated absolute escapes into
   the JS-compatible assertions above. Unseen dynamic patterns still explicitly
   refuse; this does not claim a general dynamic-regex parser. The shared
   `Reviewed` table is copied, never mutated. No shared lowering, `hir/golang`,
   or shared `jsregex` changes; default Go emission is unchanged.
5. **Attribution only.** Chai observation refusals retain their exception type
   and control flow, with method/reason text. Positive-length allocation reports
   `array.length: hole-aware storage required`. No storage workaround, dummy
   array, forced zero length, test removal, or assertion weakening was used.

No genuine upstream FAIL surfaced. The initial infrastructure escape was a
native PCRE refusal, not an assertion mismatch. The live synthetic regression
still contains two deliberate assertion failures; those remain FAIL, while
body and initialization refusals remain traps. Its ten registrations are
4 pass / 2 deliberate FAIL / 4 trap, with two additional distinct initialization
refusals; the regression itself is green.

## Dell handoffs — preserve Go/Node semantics

These are dependency handoffs, not additional additive outcome counts: downstream
roots are hidden behind the measured first refusal unless explicitly listed.

### Array storage — 372 measured titles

`src/abap/2_statements/result.ts:78`, `Result.getNodes()`, allocates
`new Array<ExpressionNode | TokenNode>(this.nodeCount)` and fills indices in
reverse. `hir/abap/emit.go` deliberately refuses positive allocation lengths
when the Chai closure is present. Current native reason text proves that boundary
for all 372 outcomes; `rg` finds this as the core source's positive-length
`new Array` allocator. Node passes these titles.

Need length distinct from present slots, holes distinct from explicit undefined,
correct indexed writes/reads/deletion and enumeration, and original reference
identity. Preserve backing storage through aliases and views. Do **not** replace
allocation with `[]`, initialize every hole as undefined, edit `getNodes()`, or
credit unreachable assertions as passed. Start with the live `array storage
refusal` fixture and rerun both full shards after storage support lands.

### Class views / erased native calls — 1 measured title

`test/cds/cds_parser.ts:3852:17`:
`nameNodes[0].getFirstToken().getStr()` in the quoted FROM-alias `"=A0"` case.
Its `findAll(n:any, className:string):any[]` helper uses optional `get()` and
`getChildren()` and inspects constructor names. The owning test body currently
refuses `dynamic.call0` at lowering; its assertions do not execute.

Go's `hir/golang/native_call.go` is the semantic reference: closed concrete-class
catalogue, actual virtual zero-argument methods/results, no function-storage
capability, no field-shadowed slots, no unsafe coercion/prototype-accessor layouts,
and a real value bridge for erased void results. ABAP needs the same certified
native dispatch plus class-view/storage ABI; do not cast the erased results to
one guessed constituent or synthesize a token/string. Keep method/prototype
identity, inheritance and constructor-name observations faithful.

### Live views

Preserve reference identity, write-through aliasing and reads from original
backing sources through materialization, array elements and structural fields.
`hir/abap/live_class_view.go`, `hir/abap/chai.go`, and `hir/abap/chai_box.go`
currently refuse observations they cannot certify; do not relax those checks
by boxing stale snapshots. **No separately measured first-root count** here:
array allocation prevents most parser assertions from observing these values.

### `dynamic.asRef`

Keep conversion tied to the live storage/nominal descriptor contract, preserving
null/undefined and actual receiver identity. Wrong-shape conversions must refuse
or throw as the Go contract specifies, not return a fresh dummy object. This is
needed for erased parser-node/class operations. **No separate first-root count.**

### Numeric operations

Preserve binary64 behavior, finite/range checks, truncation, NaN/infinities,
signed zero, UTF-16 offsets and numeric index conversion. Chai deep zero comparison
must not guess unproven zero-sign metadata. Inspect `hir/abap/chai.go`'s `equal`
boundary and numeric gap sites only after allocation is unblocked. **No numeric
first-root count was measured; static sites are not executed outcomes.**

### `dynamic.keys`

Enumerate actual own enumerable keys in JS order (integer-index keys first,
then insertion-ordered strings), omit holes, retain explicit undefined-valued
keys, and respect live structural/class views. Do not return an empty or
snapshot-derived key set to satisfy deep equality. **No separate first-root
count** before the array allocation blocker is removed.

## Gates, commits and cleanup

All required gates passed on each green repair step:

```sh
go vet ./...
go test -short -p 2 ./...
ABAPITI_GRACECHECK=1 go test -count=1 -run TestLexerFactsReport ./tsfront/
./.github/ci/lint.sh gate origin/main
```

Lint: **0 new**, canary **5/5**, golangci-lint **2.13.2**. Collecting helpers are
used for emitter regressions. Native live boundary/normalization/anchor/quoted
string/storage-refusal regression passed after each relevant step; the final
run took **8.552 s**. Selector, all-shard accounting, driver-boundary, and
Node/RE2 adaptation tests also passed.

Green commits, all by `Alice V. <ooisee@gmail.com>`: `755906f`, `7b6e2fa`,
`a1d3b82`, `11c0787`. Task TMPDIR and the shared `tools/gogen/.out/go-cache`
are removed after evidence publication, under the shared lock; the pinned OSG
checkout remains clean. No absolute home or task-temporary paths are published.
