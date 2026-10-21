# RegExp.exec, match-array declarations, and runtime patterns

`RegExpExecArray` and `RegExpMatchArray` from the TypeScript library share the
string-array storage ABI. Runtime `regexp.exec` and `string.match` use an absent
reference for non-match internally. The explicit, non-mutating `regexp.boxMatch`
boundary consumes exactly `Optional<Array<String>>`: an absent result becomes
tagged JavaScript null, while a present result boxes the same array storage.
Both Go and ABAP implement this boundary before a source result escapes to
a local, conditional/coalesce, return, any or unknown value. Nullable match-array
checker types therefore use the ordinary Dynamic union mapping, which also
preserves explicit undefined. Ordinary Optional absence and ordinary nil boxing remain undefined;
this operation does not convert arbitrary missing arrays to null. `exec` returns the full match,
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
before compilation; invalid syntax is a trap. On Go, literal and dynamic constructors also use `TranslateSubset` from
`hir/jsregex/subset.go`: ordered alternation, capture-safe groups, ASCII
word/digit classes, JS whitespace (`\s`/`\S`, including `\s` inside classes),
ASCII word boundaries (`\b`/`\B`), capture-safe greedy bounded repetitions
(up to RE2's 1000 bound; repeated captures, malformed/oversized bounds and lazy
repeats refuse), non-u ASCII case folding and non-m absolute anchors. `$^` matches only empty input. Dynamic constructors activate this same
translator even when no non-table literal occurs in the program. Unknown or
uncertified patterns/flag sequences produce
`unreviewed JavaScript regexp /pattern/flags`; this is not arbitrary RE2 fallback.
Static patterns outside Go's reviewed language fail emission. Unknown/duplicate static flags
fail lowering; `m` is restricted to reviewed translations. Backreferences,
arbitrary lookaround, Unicode mode, and `.input` are
not added by this change. A RegExp object passed as the constructor pattern is
refused; regex *literals* as constructor patterns preserve their original flags
unless overridden.

## Go String.split with a RegExp

`string.splitRegex` is a separate typed operation; a RegExp never enters the
string-separator ABI. It follows non-u JS @@split's private-sticky-clone semantics, using equivalent
full-subject leftmost-first enumeration over the certified regular language:
receiver/separator/limit evaluation order, UTF-16 unit advancement, original
anchor context, suppressed leading/trailing empty matches, numbered capture
splicing, and numeric ToUint32 limit (including truncation, wraparound and
nonfinite values). The original RegExp's lastIndex is ignored and preserved.
Enumeration suppresses empty matches at the preceding separator end and does
not attempt a match at the final input boundary. Exec now retains one preceding
UTF-16 unit for anchor/boundary context instead of a bounded-repeat prefix, so
both exec and split cursors above 1000 are supported.
Empty input returns `[]` if the separator matches empty, otherwise `[""]`.
Leading/trailing **nonempty** separators preserve empty result elements.

Both abaplint regexp split sites (`catch_and_raise` and `cds_field_order`) use
this operation. Captured patterns must certify in the capture-safe subset;
older table reviews of repeated exec captures are not extended to split.
An actually unmatched capture that would be appended refuses with a typed
`nullable string-element ABI` trap: the current string[] ABI cannot preserve
undefined through all slice/iteration/escape operations. It is not emitted as
an empty string or hole. Captures not reached because of the limit need no ABI
extension. Non-number limits, undefined separators, custom species/@@split and
Unicode-mode regexps are not added. ABAP explicitly refuses this Go-only op;
its legacy string split runtime is unchanged. Live Node differentials in
`regex_split_test.go` and `regex_split_edges_test.go` cover the supported domain
and assert the typed refusal for unmatched/repeated capture domains.

String.replace accepts RegExp objects and string pattern/replacement variables.
A variable string pattern replaces the first literal occurrence, including the
empty needle. String replacements implement GetSubstitution over the original
UTF-16 input, including literal dollars, whole matches, prefix/suffix text,
numbered captures and admitted named captures.
A variable string pattern replaces the first literal occurrence, including the
empty needle. String replacements implement GetSubstitution over the original
UTF-16 input, including literal dollars, whole matches, prefix/suffix text,
numbered captures and admitted named captures. replaceAll accepts string
patterns or literal global RegExp patterns; callbacks and coercing replacements
retain located refusals. See [the step-4 evidence](../docs/evidence/dell-types.md).

ABAP reuses the shared translations with **PCRE, requiring ABAP 7.55+**,
only in the separately emitted `runtime.regexp_js` subclass. Legacy FIND REGEX
is longest-match, implicitly multiline/dotall, and rejects lazy quantifiers;
it cannot stand in for materialized JS exec. Materialized exec converts a
legacy receiver on demand, carrying lastIndex back to the original object.
Sticky/multiline, lookaround outside the legacy exclusion subset, and lazy
literal syntax select the JS subclass at construction. Observed exec/lastIndex
state also selects it for global constructors. Unknown runtime flags use the
checked JS constructor. The usage scan includes Seq preludes and sees only
bodies remaining after pruning/lowering, not regex type declarations or traps.

Ordinary String.match does **not** activate JS exec globally. Existing dynamic
configuration/name patterns with fixed i/g flags keep their pre-branch POSIX
contract unless state requires the JS path; arbitrary dynamic JS regex syntax
is not claimed by that legacy contract. Checked constructors and the shared
translation/validation table live exclusively in the JS subclass. A mixed
program still constructs ordinary literals with the legacy runtime.

The general corpus retains its v750 lint gate; regex feature suites select
v755 when PCRE is present. The default pruned abaplint build remains 7.50 and
contains no PCRE instructions. A4H output reports the newer minimum only when
needed (the project's A4H is 7.58).

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
* The initial exec commit changed 88 default class files. That was a regression,
  not an intentional default-runtime upgrade. The [OSGO compatibility follow-up](../docs/regexp-osgo-compatibility.md)
  removes it and records the complete remaining output diff and fresh full-build
  measurements.

The Node-generated fixture oracle is committed. Go/tsfront/ABAP unit tests and
`go vet ./tsfront/... ./hir/... ./cmd/...` pass. The feature ABAP corpus also has
zero v755 parser/check_syntax issues against the pinned open-steamgate
open-abap-core dependencies. The JS-runtime result-table types and FIND FIRST result forms were corrected
while bringing up this gate; the legacy default-runtime ABI is unchanged.
Both regex fixture OSG-JS tests pass against Node. CI records the pinned osgo
negative-lookbehind refusal as a narrowly checked known gap (NOT_COMPILED,
not a passing native test); any other failure remains fatal.
