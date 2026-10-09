The HIR owns semantic types and resolved object declarations. It has no dependency
on TypeScript, MinZ, or target types. Tagged expression and statement nodes keep
traversal small; each expression has a Type, and each node can carry an ID and
source location. Frontends should supply those identities for useful diagnostics.
Verify checks lexical scopes, class ancestry, calls, assignments, interface
conformance, overrides, and abstract implementations before emission. Dump starts
with `hir v1`; its golden files are read-only during tests.

`abap.Emit` returns global classes, interfaces, and the runtime dependencies they
use. Names are case-safe qualified identities with deterministic hash suffixes,
independent of discovery order. A detected hash collision fails rather than
silently aliasing declarations. Virtual members share an emitted identity across
an inheritance hierarchy. IMPORTING values are copied into writable method locals.
Expression evaluation creates temporaries in source order, with lazy conditional
and boolean branches; loop conditions execute on every iteration.

Number maps to binary64 `f`, and only explicit I32/I64 types map to `i`/`int8`.
Strings use ABAP strings and escaped string templates preserve blanks. Length uses
native strlen( ), and indexing/slicing use native sections: both count UTF-16
code units on the SAP kernel (verified on 7.58), OSG-JS and pinned osgo, like
JavaScript. The osgo pin fixes runtime gap 026 for supplementary input.
Primitive optionals use specialized immutable boxes with value and has members;
reference optionals use an initial reference. Primitive optional equality compares
presence and, when present, the contained value. There is no null type in this phase.
Arrays, maps, and sets are reference objects, preserving aliasing. Maps and sets
use linear lookup in an insertion-ordered table; updating a key preserves its
position, and reference keys compare identity. `map.keys` and `set.values` return
snapshots for ordinary Array ForEach. Collection mutation during snapshot
iteration does not alter that snapshot. IndexGet is an unchecked element access;
`array.get` returns Optional for callers needing an out-of-range test.

The emitter targets ABAP 7.50. InstanceOf uses IS INSTANCE OF and returns a
boolean without exposing a narrowed reference. Narrow is a typed
view the front end's checker proved (flow narrowing, a dominating instanceof,
an assertion whose operand is already flow narrowed). Optional references are
unwrapped first; equal underlying types use `=`, and downcasts use checked `?=`.
The TypeScript frontend rejects assertions that lack checker-proven narrowing. A method named class_constructor (static, no
parameters, void) is emitted as ABAP's CLASS-METHODS class_constructor and
runs implicitly; the verifier rejects explicit calls to it. Typed exceptions
inherit cx_no_check and carry a payload; Try's Type selects the payload wrapper
caught by its handler. Interfaces forward to the ordinary virtual member so
inherited implementations still dispatch through the derived class.

Target limits are explicit: class descriptors/ClassValue execution is deferred;
non-finite Number literals are rejected with node/source diagnostics because ABAP
f cannot represent them. Number division requires a non-zero literal divisor;
other divisors fail with node/source diagnostics. Number +, -, and * remain
supported. Results beyond the binary64 range (|x| > ~1.8e308), including division
overflow, raise in ABAP instead of giving Infinity. This is an accepted divergence
until a later phase implements exceptional-number handling.
Integer division truncates toward zero and remainder keeps the dividend sign.
General Number remainder is rejected; the Number remainder-by-two runtime
operation implements the lexer's literal `% 2` case.
The string operations work on dynamic receivers: length counts UTF-16 code units
with strlen( ), substring/charAt clamp like JavaScript, substr keeps its
legacy negative-start semantics, trim strips exactly the ECMAScript white space
set, and replaceAll replaces every occurrence of a literal needle (the front end
maps the lexer's /\r/g to it and reports the mapping). i32.toString renders like
JavaScript String(int32) through a string template. string.charCodeAt returns the
UTF-16 code unit as i32 and raises cx_sy_range_out_of_bounds when out of range,
where JavaScript yields NaN — a documented divergence, like Number division by
zero. No operation converts the whole receiver merely to measure its length
or guard a section. Dynamic charCodeAt converts only its one-unit section to
UTF-16LE. The supplementary probe checks the exact JavaScript surrogate unit;
it must pass on both pinned runtimes.
Full BMP uppercasing applies the 102 full Unicode mapping
differences from the existing x/text dependency before target simple uppercase,
including sharp s, ligatures and Greek expansions. String literals are chunked
to stay within the ABAP line limit. The catalogue is the sole supported operation
list; unknown operations fail verification.
Closures, regular expressions, generators, and null are outside Phase 0.

Six hand-built programs have dump goldens and one ABAP Unit method per fixture.
`ABAPITI_TEST_OUT=/tmp/hir go test ./hir/abap -run TestFixtures -count=1` exports
all sources to `/tmp/hir/TestFixtures`. The existing runtime CI job runs the HIR
corpus through the v750 syntax gate and both osgo and OSG-JS, requiring all six
rows to pass. The default corpus must contain inline DATA and IS INSTANCE OF.
Both runtimes use the same default ABAP 7.50 output, including scalar VALUE
initializers and IS BOUND guards before IS INSTANCE OF. The osgo CI pin is
`73351d09c77968ebf4f14f83c0df9cd9772f9288` (it adds decode_base64 #680,
string→int8 #693, round caches #694, `where` positions #695 and the
APPEND LINES/DELETE fix #697 to the earlier
`7e7294323fd380859a71552b18b39ddffde93c69`), which supports these constructs
and mode keywords inside literals, with fixes for per-session statics (029),
UTF-16 length/sections and their memoization (026/031), the core read_int4 pin,
and IS INSTANCE OF dependency closure (034).
CI also runs a semantic regression for template escaping, initial references,
and temporaries reset on each loop iteration.
No SAP access is needed for generation or these checks.

Phase 1 number contract (Fix round 1): TypeScript `number` maps to Number/ABAP
`f`, including fields, parameters, arithmetic and collections. Integral literals
and absence of division do not prove an i32 range. On clean open-steamgate
`ad3d1e87cd3c3545b32dd4ba2ddeceb25708f05f`, assignment, arithmetic, return and
comparison overflow probes trap on osgo but do not trap on OSG-JS. Therefore
an i32 trapping contract is unavailable and the frontend does not use it.
`2147483647 + 1` must remain 2147483648 when stored, returned and compared.
Runtime regression sources are under `tsfront/testdata/critic-r1/`.
String indices explicitly truncate toward zero and saturate to the i32 bounds
before the existing clamping/bounds checks. Number remainder is accepted only
with literal divisor 2: binary64 scaling by two, truncation and subtraction
preserve the signed remainder without rounding a division by an arbitrary
number. General division/remainder are blocking frontend diagnostics.
Decimal Number.toString is supported for safe integers; dynamic fractions or
values beyond the safe integer range raise instead of using ABAP formatting.
Arguments to number.toString are blocking diagnostics. Exceptional numbers
and binary64 overflow retain the target limits described above.

Instance field initialization runs in declaration order in a constructor,
including a synthesized constructor when absent. A synthesized derived
constructor forwards its inherited parameters. Static fields and module consts
initialize in declaration order in class_constructor only when a conservative
syntax whitelist proves their initializers pure: literals, constant expressions
over literals/module constants, literal collection elements, and construction of
runtime collections from those elements. Calls to user code, user-class
construction, assignments, and other effectful expressions produce blocking
`unsupported-static-init` diagnostics with file:line:col. Class-static reads are
limited to earlier pure fields of the same class; cross-class reads are rejected
even if their initializers appear pure, because mutable statics may change before
lazy initialization. This is not eager TypeScript module evaluation: purity makes
unused/lazily initialized classes unobservable within this accepted envelope.
Static blocks and every unknown class member kind are blocking
`unsupported-member` diagnostics, never silently dropped.
Pre-super statements retain their order;
those touching this are rejected. Parameter defaults are blocking diagnostics.
Executable module statements other than const initializers are rejected.
Regex replace mapping accepts nonempty literal needles (including decoded
literal escapes), the global flag only, and literal replacements without `$`.
Anchors, metacharacters, substitution strings and dynamic replacements are
blocking source diagnostics.

The lexer differential is part of `.github/ci/hir-unit.sh`: sequential case
blocks check full dumps and each token count, with teardown requiring the corpus
cardinality even after an early RETURN. CI also requires the critic's early-return
and removed-case mutations to fail on both pinned runtimes. The 44-case corpus
is BMP; a separate supplementary probe must pass on both runtimes. String literals
containing lone UTF-16 surrogate units (escaped or raw) are rejected with
`unsupported-lone-surrogate` before rune conversion; valid pairs remain
supplementary code points, not replacement characters. Supplementary slicing,
full BMP Unicode uppercasing, virtual positions and pure static declaration
order have separate runtime probes.
Local runtime results do not establish SAP kernel timing. SAP validation
remains outstanding.

Dynamic charCodeAt and trim decode UTF-16LE bytes directly (low + high * 256),
avoiding the pinned library's erroneous high-byte * 255 implementation on
OSG-JS. Direct euro-code and Unicode whitespace regressions cover this path.

The statement-parser branch contains experimental ClassValue descriptor,
Dynamic, RegExp, Cast and Seq machinery. Seq evaluates its prelude in a local
scope shared with its result expression; declarations do not escape that
scope. Optional primitive Narrow unwraps its box; optional reference Narrow
uses a checked cast when its underlying reference type changes. These changes
preserve the existing fixture and lexer gates, but the complete statement-parser
HIR still fails verification. Run `STMTS_EXPLORE=1 STMTS_GATE=1 go test ./tsfront
-run TestLowerStatementsClosureExploratory -v` before treating that translation
as executable. RegExp, namespace export maps, structural shapes, dynamic calls
and closure lifting are not yet a validated general TypeScript contract.

Erased generic members retain the constraint signature in their virtual and
interface slots. Specialized implementations use forwarding bridges when their
signatures differ. The TypeScript checker proves the instantiated type at each
use site; Narrow records that view, including covariant reference arrays. Such
arrays share one ABAP object table and cast elements on reads, preserving identity
and mutation through all views. Verification still checks the erased signature
and the direction of every narrowing.

In ABAP, inherited override bodies occupy the REDEFINITION slot; specialized
variants forward through `me->` and use checked downcasts for their results.
A void inheritance slot with reference-returning overrides gains an object
result in emitted ABAP, while void callers and interface wrappers discard it.
`super.m()` is emitted only inside that same method (or a constructor's
`super()` call); cross-method super calls produce a blocking emission diagnostic.

The ABAP emitter initializes translated static registries through guarded
class-specific methods on first use. This follows ABAP static initialization
while avoiding OSG-JS module-import constructors calling unregistered modules.
Descriptor singletons are grouped by inheritance root, allocated before their
ancestry links, and exposed through a common descriptor interface for root
object references. Dynamic factories uppercase the ABAP class name and explicitly
name known zero-argument classes for runtime closure discovery.
Constructor defaults run before the constructor body, and Optional constructor
parameters can be omitted by a checker-proven no-argument class-value call.

Regex lowering expands shorthands within character classes without nesting
brackets. Bounded negative lookahead for a first literal character is expressed
through a restricted first class; anchored whole-word exclusions become a
separate rejection match. Both test and replace preserve these exclusions, and
the public source string retains the original pattern.
