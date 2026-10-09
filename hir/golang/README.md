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

`RuntimeOps()` returns sorted supported and unsupported catalogue entries. The
fixtures' operations, numeric conversion/rendering helpers, and dynamic boxes
and object/string extraction are supported. Every other catalogue operation
fails with `not supported in the Go prototype: <op>` before source generation.
ClassValue and RegExp types, variadic parameters, covariant array views,
cross-method super calls, and break/continue crossing a try closure are also
explicitly rejected. This is not a general TypeScript runtime.

`go test ./hir/golang -v` executes each of the six copied ABAP fixture programs
in a temporary standalone module via `go run`. The copied constructors are
checked against the ABAP HIR goldens. Collection extra and all 24 int8 oracle
values are included. Additional execution tests exercise UTF-16 surrogate
sections, collection snapshots, reference reads, dynamic tags, native and
JS-safe integer overflow, typed catches and uncaught traps. Unsupported catalogue
operations are tested through the public emitter using verified programs.
