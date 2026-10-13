# String and XML operations on TS-HA@OSGO

**733 ABAP Unit assertions: SUCCESS. Statements 429/4,962; root 8/94;
lexer 200/200. FAIL 0 in all three completed shards, with 4,533 / 86 / 0
traps respectively. Pass counts are unchanged from `fa6efed`.**

| Completed shard | `fa6efed` pass / active | Final pass / active | FAIL | Traps | Unregistered | Saved baseline host phases (s) | Final host phases (s) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `abap/statements` | 429 / 4,962 | 429 / 4,962 | 0 | 4,533 | 0 | 545.614 | 716.465 |
| `root` | 8 / 94 | 8 / 94 | 0 | 86 | 0 | 306.240 | 396.859 |
| `abap/lexer` | 200 / 200 | 200 / 200 | 0 | 0 | 0 | 243.393 | 273.872 |

Final compiler: **`5fc340b`**; OSG: **`fd07c7928`**; original Node oracle:
**`577f875e`**. Async-pending, Node-pending and registration/title mismatches
are zero in these completed measurements. The host-phase sum is frontend +
emission + Go build + execution; TS/HIR lowering is outside it. Baseline times
are the saved [JSON5 measurement](evidence/osgo-json5-subset-shards.json),
not a fresh serial baseline run. Final runs used two workers, GOMAXPROCS 4,
GOGC 50 and GOMEMLIMIT 32GiB amid other workloads. These timings do **not**
establish a performance regression or improvement under controlled load.

[Compact shard evidence](evidence/osgo-string-xml-shards.json) includes phase
breakdowns and the interrupted broader experiment. Statements still has 4,528
native cast traps and five native exceptions, including the known sparse-array
boundaries. Root still reaches the independently owned JSON serialization and
indentation refusals. None is credited as a pass. All 429 statements and eight
root passing titles from the initial `a368ee4` run survive the final declaration
repair, checked as title multisets; lexer passes all 200 original titles twice.
This is not a whole-suite certificate or permission to run A4H.

## Broader shard attempts: explicitly unmeasured

| Attempted shard | Original Node-active titles not run | Node-pending outside denominator | FAIL / traps | Boundary |
| --- | ---: | ---: | --- | --- |
| `objects` | 144 | 1 | unmeasured | Go compiler killed under severe memory pressure |
| `rules` | 3,149 | 10 | unmeasured | own frontend suspended, then terminated to relieve memory pressure |
| `abap/syntax` | 1,363 | 10 | unmeasured | own frontend terminated after prolonged resource-heavy generation |

The broader six-shard command exited **1**. Objects generated approximately
198 MiB of Go and its compiler exceeded 30 GiB RSS while the machine exhausted
swap. Its recorded frontend/emit/build-to-interruption times are 237.770 /
34.412 / 1,878.195 s; **no test execution report was returned**. Rules and syntax
also returned no runner records. These are infrastructure interruptions, **not
traps, zero-FAIL certifications, or executed registration failures**. No roots,
assertions, or oracles were removed to produce the completed three-shard run.
The complete minimum three shards were rerun successfully after the final fix.
Broader coverage remains blocked and must be remeasured on an adequately
resourced host; the unit result must not be extrapolated to these shards.

The [full-closure site inventory](osgo-suite-gap.md#runtime-operation-inventory-and-measured-shard-impact)
motivated these additional attempts:

| Operation | Sites | Owning source / motivated shard |
| --- | ---: | --- |
| `string.splitRegex` | 2 | CatchAndRaise, CDSFieldOrder / `rules` |
| `string.padStart` | 3 | basic_variables database-table builder / `abap/syntax` |
| `string.parseFloatNumber` | 3 | NumberRange, LineOnlyPunc, RulesRunner / `objects`, `rules` |
| `string.sha1Hex` | 1 | Downport.uniqueName / `rules` |
| `xml.parseWithErrors` | 3 | AbstractObject, FunctionGroup, RemoveDescriptions / `objects`, `rules` |

A site in the shared closure is not evidence that a particular test executed
that operation. The directly executed unit fixtures provide that evidence.

## Implemented, checked observation boundary

All five requested operations now have ABAP emission. The unsupported catalogue
is **17 → 12**, without changing any Go implementation or the independently
owned JSON, array-storage, numeric, dynamic-call, LSP or class-view operations.

- **splitRegex:** a UTF-16 scanner implements the two actual site grammars,
  `\s+` and `[\s.,:(]+`, using ECMAScript whitespace, not host `\s`. It retains
  leading/trailing empty segments, coalesces separator runs, handles empty input
  and ToUint32 limits within the finite safe-integer truncation domain, and
  does not modify the supplied RegExp. Plain/global flags are executed in the
  fixture. Unchecked grammars, captures, zero-width languages, other flags and
  limits outside the proved domain raise an explicit StringOpRefusal; this is
  not general RegExp split support.
- **padStart:** ToLength truncation, UTF-16 fill repetition/truncation, preserved
  blanks and no-op behavior for already-long strings or empty fill. The Go
  allocation ceiling of 1,048,576 units is retained. A partial surrogate pair
  is retained exactly, rather than replaced with U+FFFD. Nonfinite Number
  production remains outside the ABAP Number ABI.
- **parseFloatNumber:** trims the exact JS whitespace set, finds the decimal
  prefix including incomplete-exponent fallback, and converts finite binary64
  values. Nonfinite/invalid prefixes, conversion failures and negative zero
  explicitly refuse. It never invents a finite substitute for NaN or Infinity.
- **sha1Hex:** the native UTF-8 SHA1 pipeline returns lowercase hex for checked
  well-formed UTF-16, including supplementary pairs, NUL and non-ASCII text.
  Lone surrogates explicitly refuse: no unproved replacement encoding is
  silently hashed. Go's broader replacement-encoding implementation is unchanged.
- **xml.parseWithErrors:** reuses the reviewed abapGit parser language but mirrors
  the Go/fast-xml-parser EOF and mismatched-close behavior. Unterminated ordinary
  opening tags throw the translated native Error payload with the exact position
  and 50-unit context message. Other XML languages remain coverage refusals and
  cannot be swallowed as successful source catches. The strict parseSubset
  runtime is unchanged. A case-sensitive literal declaration-prefix check avoids
  ABAP CP/NP's case folding. No Error class in the HIR means explicit refusal,
  just as in Go.

The new string runtime and extra RegExp flags storage are usage-gated. Helpers
use the existing UTF-16LE conversion technique for code-unit observations above
ASCII; native `uccpi` alone is not a safe surrogate observer on the pinned host.

## Differential, compile and regression evidence

[Unit evidence](evidence/osgo-string-xml-unit.json): **12** Node pad answers,
**16** decimal-prefix answers, **9** SHA1 answers, **264** split cases, **11** XML
value answers and **5** XML Error messages. XML uses real fast-xml-parser
**5.10.1**, with the pinned options, not a shim. Additional assertions cover plain
RegExp flags/state, unsupported languages, allocation limits, negative zero,
nonfinite prefixes and SHA1 lone-surrogate refusals.

The fixture emits 18 ABAP files: syntax lint reports **0 issues**, and gogen
compiles and executes the unit owner with **733 assertions, SUCCESS**. Final
unit total is **11.890 s** (emit 0.424, Go build 2.519, run 0.017 s). The existing
generic writer's `->* of a data` alert remains visible in the evidence; none of
the five requested paths is accepted by hiding a compilation refusal.

Validation stopped at the first pad fixture mismatch. Node had produced a lone
surrogate, but decoding its oracle JSON into a Go string replaced it with U+FFFD.
Expected pad strings now remain **raw JSON** until ABAP decoding. A subsequent
raw-unit check demonstrated native `uccpi`'s surrogate-observation limitation;
the regression now checks the exact UTF-16LE bytes `3DD8`. SHA1 validation uses
that same checked conversion boundary. No expected Node code unit was discarded.

Review also found that CP/NP could admit uppercase `?XML` declarations outside
Go's reviewed language. The new literal check refuses them. Both the Go coverage
regression and executed ABAP refusal pass. Restoring case-insensitive matching
in a scratch fixture causes the new assertion to **FAIL**, as intended; this
intentional mutation is not an upstream FAIL or a passing workload. It was never
applied to production sources or the pinned host.

## Gates, host setup and cleanup

Passed `go vet ./...`, `go test -short -p 2 ./...`, uncached Grace
`TestLexerFactsReport`, and lint against `origin/main`: **0 new issues, canary
5/5**, golangci-lint 2.13.2. Updated capability tests now require checked emission
and runtime refusals; their Go/Node comparisons remain intact. No oracle,
assertion or upstream test was weakened.

Fresh default Go generation from `git archive fa6efed` and the final compiler,
at the same output path, gives raw `diff -r` **exit 0, 0 changed bytes**, for all
**9 files including the executable**. No Go backend production source changed.

OSG PR **#719** remains required and unmerged. The documented `osg-fdpos` branch
had advanced to its additional test commit, so a temporary detached worktree
inside this worktree was created at the exact pin **fd07c7928**. It reused existing
read-only node_modules and library checkouts; no clone, install, host source edit
or shared-checkout switch occurred. The original shared OSG checkout remains
clean at **f557e12f4**.

Reproduce with that exact clean pin, the environment in
[osgo-json5-subset.md](osgo-json5-subset.md), and `nice -n 10`:

```sh
ABAPITI_TEST_OUT="$TMPDIR/string-xml-fixtures" \
  nice -n 10 go test ./hir/abap -run '^TestStringXML' -count=1
OSG_HOME="$ABAPITI_OSG_HOME" nice -n 10 node \
  "$ABAPITI_OSG_HOME/tools/gogen/unit.mjs" \
  --input "$TMPDIR/string-xml-fixtures/TestStringXMLFixtures" \
  --class Z_STRINGXMLPRO_434DF3D9466396 --jobs 1 --no-cache --out "$TMPDIR/string-xml-unit"
ABAPITI_TESTS_TARGET=osg ABAPITI_TESTS_SHARDS=abap/statements,root,abap/lexer \
  ABAPITI_TESTS_WORKERS=2 GOMAXPROCS=4 GOGC=50 GOMEMLIMIT=32GiB \
  nice -n 10 node tools/abaplint-tests-all.mjs "$ABAPLINT_CHECKOUT" result.json
```

Generated fixtures, Go modules/binaries, archives, raw reports and invocation
scratch are removed after recording compact evidence. The temporary OSG worktree
and its gogen cache are removed; shared caches belonging to other runs are not
modified. No push and no clone. Implementation commits: **a368ee4**, **5fc340b**.
