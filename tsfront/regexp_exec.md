# RegExp.exec, match-array declarations, and runtime patterns

`RegExpExecArray` and `RegExpMatchArray` from the TypeScript library share the
existing nullable string-array reference ABI. `exec` returns the full match,
numbered captures, and a UTF-16 `.index`, or null. Absent optional captures carry
absence metadata rather than an empty string; indexed match-array reads preserve
that absence. The array length still includes absent captures.

Non-global execution ignores and preserves lastIndex. Global/sticky execution
starts at lastIndex, updates it to the match end, and resets it on failure. Sticky
execution cannot search ahead. An empty match does **not** advance lastIndex.
The start-position search retains the original input's anchor/boundary context.
`lastIndex` reads/writes are lowered, with a checked finite signed-32-bit integer
assignment contract (fractional, infinite, NaN, and larger assignments are refused).
This is not the complete JS arbitrary-value lastIndex/ToLength ABI.

## Checked regex language

Runtime `new RegExp(string[, string])` and implicit String.match construction
cannot bypass regex review. `hir/jsregex/patterns.go` is the shared translation
table: each pattern/flag sequence has a compiled matching language and, when
needed, a separate whole-input exclusion. It includes all five newly blocked
exec sites, BDEF comment stripping, CDS label unescaping, and the two computed
Search annotation patterns. Go compiles UTF-16 unit text, expands JS whitespace,
keeps word/digit classes ASCII, and expresses non-Unicode ignore-case folding
without Go's broader Unicode case folding.

The additional runtime families are `&[1-9][0-9]*` with `g`, and patterns matching
`^\^[A-Za-z0-9_|()?]+\$?$` with empty flags or `i`. These families are checked
before compilation; invalid syntax is a trap. Other dynamic patterns/flag
sequences produce `unreviewed JavaScript regexp /pattern/flags`. Static patterns
outside Go's reviewed language fail emission. Unknown/duplicate static flags
fail lowering; `m` is restricted to reviewed translations. Backreferences,
arbitrary lookaround, Unicode mode, named captures, `.groups`, and `.input` are
not added by this change. A RegExp object passed as the constructor pattern is
refused; regex *literals* as constructor patterns preserve their original flags
unless overridden.

String.replace accepts RegExp objects and string pattern/replacement variables.
A variable string pattern replaces the first literal occurrence, including the
empty needle. Replacement callbacks and dollar substitutions remain unsupported;
dynamic replacement strings are checked at execution.

ABAP reuses the shared translations with **PCRE, requiring ABAP 7.55+**.
Legacy FIND REGEX is longest-match, implicitly multiline/dotall, and rejects
lazy quantifiers; it cannot stand in for JS exec. The general corpus retains
its v750 lint gate; regex feature suites select v755 when PCRE is present.
Generated A4H output reports the minimum release (the project's A4H is 7.58).

The regex subject is a BMP unit view: supplementary/lone surrogate units become
one U+FFFD each, like Go's unitText. Offsets still select captures from the
original string. Compiled BMP escapes are expressed in syntax shared by PCRE
and the pinned JS ABAP runtime; absolute start/end assertions prevent that
runtime's implicit multiline flag from changing JS non-m anchors. BDEF's
initial word boundary explicitly tests ASCII word characters. Literal
replacement uses match offsets and original substrings, not native replacement
escape/substitution syntax. Result/submatch types are declared locally rather
than relying on missing runtime DDIC definitions.

The dedicated fixture is wired into `.github/ci/hir-unit.sh` for the osgo/OSG-JS
differential. The pinned osgo PCRE adapter currently refuses lookbehind and
subjects with line ends; those are runtime gaps, not substitutions of POSIX
semantics. Native SAP execution is not claimed by a parser or Go-only gate.

## Measurements on feat/string-repeat-match base

There is no `--full` option on this base. A temporary, uncommitted coverage=nil
build was used with `--assume-int=false`: the integer pass on the unpruned base
otherwise panics in `assume_integer.go:298`.

* Blocking diagnostics: **383 -> 336**; HIR errors: **261 -> 249**.
* Exec method: 5 -> 0; ExecArray declaration: 5 -> 0; MatchArray declaration:
  already 0 on this base; assignment to match: 2 -> 0; non-literal replacement:
  4 -> 0; unsupported new RegExp: 1 -> 0.
* ObjectNaming's pattern local still cascades from unsupported classref config
  indexing at line 120. This change does not claim to fix that separate root.
* Default a4h lowering remains 0 blockers/0 HIR errors. The source diff is **85
  source classes plus 3 runtime classes** (one new exception class): reviewed
  constructor translations/state setup, truthiness-only exec becoming the common
  exec ABI, absence-preserving match-array reads and associated temporaries,
  array absence metadata, control-character-safe literals, and runtime
  pattern/replacement guards. The a4h README reports the ABAP 7.55 minimum;
  the deterministic source archive changes accordingly. These are intentional;
  byte identity is not claimed.

The Node-generated fixture oracle is committed. Go/tsfront/ABAP unit tests and
`go vet ./tsfront/... ./hir/... ./cmd/...` pass. The feature ABAP corpus also has
zero v755 parser/check_syntax issues against the pinned open-steamgate
open-abap-core dependencies. The old String.match fixture's untyped result-table
and FIND FIRST into-table forms were corrected while bringing up this gate.
Both regex fixture OSG-JS tests pass against Node. CI records the pinned osgo
negative-lookbehind refusal as a narrowly checked known gap (NOT_COMPILED,
not a passing native test); any other failure remains fatal.
