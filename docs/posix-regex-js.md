# JavaScript classes on the legacy ABAP POSIX regex path

Static literal translation and the runtime constructor now share the same
JavaScript (non-`u`) character languages:

- `.` excludes LF, CR, U+2028 and U+2029. With `s`, it remains the POSIX dot,
  which already matches line terminators.
- `\s` contains ECMAScript WhiteSpace and LineTerminator, including NBSP,
  U+1680, U+2000–U+200A, U+202F, U+205F, U+3000 and U+FEFF. It excludes
  U+0085 and historical U+180E. `\S` complements this class.
- Within a bracket class, `\s` inserts members rather than nested brackets.
  Class-contained `\S` is deliberately unsupported: literal emission fails
  with a diagnostic, and dynamic construction raises an exception.
- Escaped dots and dots inside bracket classes remain literals. The `i` flag
  does not change dot or whitespace membership.

Members are literal characters, not hex escapes. Control characters and Unicode
line separators/BOM are built with `cl_abap_conv_in_ce=>uccpi` in emitted ABAP.
The static translation is idempotent through the constructor's second pass.

## Word boundaries

POSIX word boundaries are locale-dependent. An unanchored literal without `g`/`y`
containing `\b` or `\B` selects the existing checked PCRE/RE2 path only when
`jsregex.TranslateSubset` certifies it and its translated pattern requires no
lookaround refused by the pinned host. This path uses ASCII word membership,
including for non-`u` case-insensitive patterns. Non-stateful `exec` searches the
original pattern directly and preserves its captures/offsets without a synthetic
cursor assertion.

Unknown dynamic patterns, uncertified literals, global and anchored boundary patterns
retain their legacy spelling and its locale-dependent gap. In particular, the
checked path translates absolute anchors and global/sticky cursor positioning
using lookaround, which pinned OSGO cannot execute; routing these patterns to it
would merely introduce a refusal.
`TestDynamicWordBoundaryLegacyGap` and `TestRegexJSRequirements` document this
limit instead of claiming full JavaScript boundary support.

## Regression fixture

`TestEmitPOSIXRegexJSClasses` obtains expectations directly from Node and exports
an ABAP Unit observation test plus a dynamic complement-refusal test. It covers
`test`, boolean `match`, materialized `match`, replacement, dynamic patterns,
all four dot line terminators, Unicode whitespace, ASCII boundaries around
non-ASCII letters, and `s`/`i` combinations. The suite is included in
`.github/ci/hir-unit.sh` with two required successful tests and no known-gap waiver.

## Verification at OSGO 16c024c8

Upstream abaplint: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.

| Run | Pass | FAIL | Trap/refusal | Unregistered |
| --- | ---: | ---: | ---: | ---: |
| Node-oracle ABAP Unit fixture | 2/2 | 0 | 0 | 0 |
| `test/rules/sequential_blank.ts` | 6/12 | 0 | 6 | 0 |
| `abap/lexer` | 200/200 | 0 | 0 | 0 |

All six focused-file traps are the unrelated existing Chai refusal
`array.length: hole-aware storage required`, not regex failures. Fixture lint
reported seven files and zero issues. `go vet ./...`, the standard
`go test -short -p 2 ./hir/... ./tsfront/ ./cmd/... -count=1` gate, and lint against
`origin/main` passed (zero new issues; canary 5/5).

The standard Go gate ran with `ABAPITI_OSG_HOME` unset, so its optional OSG host
probes did not run. With that variable exported, the broader suite also executes
`TestParsedNestedObjectABAPHost`, which crashes in the dynamic runtime with a nil
pointer at line 244. The same failure was reproduced on the unchanged base
`1458e21`; its reproduction contains no regex. This unrelated optional integration
failure is not waived or counted as a passing OSG test above.

All heavy runs used the shared flock, a 48 GiB scope and niceness 10, with load
admission below 12. The pinned OSG checkout remained unmodified; its ABAP Unit
staging directory was a temporary writable mount over an otherwise read-only tree.
