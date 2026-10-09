This package is a Go prototype of the HIR backend. `Emit(p)` verifies the program
and returns `hir.go` and `runtime.go` in package `main`. `EmitPackage(p, name)`
selects another package name. Neither function writes files. Generated modules
use only the Go standard library. Output identities and class ordering are
deterministic; field, method-body, argument and initialization order are retained.

To call a static method from a main function in the emitted package, its Go
identifier is `hir.NewNames().Get("body." + class + "." + method)`. Virtual
methods use `hir.NewNames().Get("member." + method)` throughout the hierarchy.
Generated identifiers are deliberately internal to the chosen package.

Number is float64, I32 is int32, and I64 is int64. Integer arithmetic uses exact
intermediates to detect native overflow; CheckIntegerOverflow additionally traps
outside [-9007199254740991, 9007199254740991]. Integer division truncates toward
zero. Number literals and arithmetic retain the ABAP backend's finite-number,
nonzero-literal-divisor and unsupported-general-remainder limits.

Strings have a comparable UTF-16LE backing, with sections and indices measured
in code units. Sections can retain individual surrogates. Conversion to Go text
is only for display, where Go's UTF-16 decoder replaces isolated surrogates.
Primitive optionals are value structs; reference optionals are nil references.
Collections are pointer-backed, with linear insertion-ordered lookup and copied
key/value snapshots. Empty classes contain an identity byte so different objects
cannot collapse to equal pointers to zero-sized Go allocations.

Classes embed their base storage. Class-reference interfaces expose storage
accessors and method signatures; every class forwards inherited method bodies
with the original concrete object as receiver, preserving virtual dispatch and
reference identity. Static initialization is guarded and runs on first use.
Generated code assumes sequential execution, like the fixture runtime.

Throw uses a typed panic payload with the exact HIR type identity. Try catches
only that payload type, rethrowing all other panics. Trap uses a distinct panic.
Returns use separate internal control panics so a return crosses try closures
without being mistaken for an exception. This is a correctness-first prototype.

`RuntimeOps()` reports all catalogue names as implemented. Implementations retain
bounded contracts: numeric rendering rejects fractions and unsafe numbers;
ordering rejects characters outside the reviewed ASCII alphabets; JSON is strict;
XML follows the reviewed abapGit subset and rejects comments, DTDs, numeric or
unknown entities, and prototype-sensitive names. Materialization accepts data
shapes with a matching field constructor ABI and retains the original graph.
Class descriptors retain identity, ancestry, own static names and concrete
zero-argument factories (including absent optional parameters); factories that
require arguments fail explicitly.

Reference arrays share `array[any]` storage and cast elements on reads, so typed
views retain identity and mutation. Missing IndexGet reads return the element's
zero value, matching ABAP READ TABLE; callers use optionals for presence.
Unicode casing includes full mappings, contextual final sigma for lowercase,
and preservation of lone surrogates. Finally uses defer, so it runs once on
normal completion, return, throw or Trap.

Regex patterns are deliberately reviewed individually; Go's RE2 syntax alone
is never evidence of JavaScript equivalence. The lexer uses only `/\r/g`,
which the frontend maps to literal `string.replaceAll`. Registry fixtures use:

| JavaScript | Handling |
| --- | --- |
| `/^Y/`, `/^Z/` | Identical RE2 prefix tests |
| `/test$/i` | `[tT][eE][sS][tT]$`, preventing Unicode fold differences |
| `/a.c/i` | ASCII case pairs; dot excludes LF, CR, U+2028 and U+2029 |
| `new RegExp("x/y", "gi")` | `[xX]/[yY]`, canonical escaped source, global state |

Matching operates on one rune per UTF-16 unit, preserving non-Unicode regex
width even for supplementary input and isolated surrogates. Global test advances
and resets lastIndex; match_test resets global state. Replacement preserves
original UTF-16 sections. Other literal patterns fail emission; other dynamic
patterns fail construction. Dollar replacement substitutions are unsupported.

Variadic parameters, primitive covariant array views, cross-method super calls,
and break/continue crossing try/finally closures remain explicitly unsupported.
Non-finite Number literals, general Number remainder and nonliteral/zero Number
divisors retain the round-one restrictions. This is not a general JS runtime.

`go test ./hir/golang -v` runs the six ABAP fixtures, int8 observations, and
runtime semantic edge tests in standalone generated modules. The tests in
`tsfront/golang_emit_test.go` run the same lowered lexer and registry programs
as the ABAP tests. Lexer comparison regenerates the Node oracle, checks all 44
cases and mutates an emitted token type to prove the comparison fails. Registry
array, sort, iterator, feature, typed JSON and tagged XML tests use the existing
Node observations. Integration checks skip if Node or the abaplint build is
absent; set TSFRONT_ABAPLINT to the core directory to select another checkout.
