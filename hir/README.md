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
Strings use ABAP strings and escaped string templates preserve blanks. Length counts
UTF-16 units through code page 4103, including supplementary characters.
Substring currently requires a BMP literal receiver; other uses fail with source
diagnostics until all runtimes support surrogate slicing.
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
an assertion after a check): unwrapping an optional reference is a plain move,
a downcast is emitted as a checked `?=` so an unproven view raises instead of
aliasing the wrong object. A method named class_constructor (static, no
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
Number remainder is rejected until an IEEE remainder runtime is available.
The string operations work on dynamic receivers: length counts UTF-16 code units
through code page 4103, substring/charAt clamp like JavaScript, substr keeps its
legacy negative-start semantics, trim strips exactly the ECMAScript white space
set, and replaceAll replaces every occurrence of a literal needle (the front end
maps the lexer's /\r/g to it and reports the mapping). i32.toString renders like
JavaScript String(int32) through a string template. string.charCodeAt returns the
UTF-16 code unit as i32 and raises cx_sy_range_out_of_bounds when out of range,
where JavaScript yields NaN — a documented divergence, like Number division by
zero. Indices count UTF-16 code units, which equals the character count only
inside the BMP; the runtimes slice by characters, so input outside the BMP
diverges: measured on osgo, the lexer raises on such input instead of producing
wrong output. String literals are chunked so that no chunk contains IN BYTE MODE
or IN CHARACTER MODE: the CONCATENATE statement parser mistakes those sequences
inside a literal for its own clauses. The catalogue is the sole supported
operation list; unknown operations fail verification.
Closures, regular expressions, generators, and null are outside Phase 0.

Six hand-built programs have dump goldens and one ABAP Unit method per fixture.
`ABAPITI_TEST_OUT=/tmp/hir go test ./hir/abap -run TestFixtures -count=1` exports
all sources to `/tmp/hir/TestFixtures`. The existing runtime CI job runs the HIR
corpus through the v750 syntax gate and both osgo and OSG-JS, requiring all six
rows to pass. The default corpus must contain inline DATA and IS INSTANCE OF.
`EmitWithOptions` exposes two explicit osgo workarounds: `OsgoScalarValueFallback`
uses typed DATA/CLEAR instead of scalar VALUE initializers, and
`OsgoInstanceOfFallback` emits checked-cast helpers instead of IS INSTANCE OF.
CI selects these only for the osgo corpus through `ABAPITI_HIR_OSGO_COMPAT=1`;
the default corpus is syntax-checked at v750 and runs unchanged on OSG-JS.
CI also runs a semantic regression for template escaping, initial references,
and temporaries reset on each loop iteration.
No SAP access is needed for generation or these checks.
