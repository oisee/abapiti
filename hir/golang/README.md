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

The pinned abaplint regex token grammars are listed in `regex_patterns.go`.
They use ASCII word/digit sets and ASCII case pairs; dot excludes LF, CR,
U+2028 and U+2029. Negative keyword lookaheads are separate exclusions.
The dynamic grammar admits macro placeholders (`&` plus a positive decimal
integer) and anchored ASCII literal alternatives used for SQL names.
Patterns outside these reviewed languages fail emission when literal and
construction when dynamic. Matching uses one rune per UTF-16 unit. Global
state and replacement preserve UTF-16 sections; dollar substitutions remain
explicitly unsupported. The Node oracle checks 117 patterns against 1,033
inputs, including Unicode and line terminators.

The frontend packs variadic arguments into trailing arrays. Super calls resolve
from the immediate base while retaining the concrete receiver. Typed internal
loop-control panics cross try closures and unwind to the correct loop; catches
continue to accept only exception payloads. The frontend/HIR verifier still
reject exits from the try block of try/finally, and try/catch/finally. General primitive
covariant array views remain unsupported. A fresh, unaliased temporary whose
only uses are pushes before a final narrowing can be repacked after checking
that every element is present; escaping/aliased arrays are rejected.
Non-finite Number literals, general Number remainder and nonliteral/zero Number
divisors retain the round-one restrictions. This is not a general JS runtime.

The complete CLI closure compiles directly from HIR, without `hir.Inline`:

```sh
go run ./cmd/abapiti abaplint --target go -o /tmp/abaplint-go
cd /tmp/abaplint-go/go
GOFLAGS=-buildvcs=false go build -o ../zabaplint .
```

`--file`, `--config`, `--deps` and `--times` use the same RegistryRun harness
as the ABAP native command. `--cpu-profile`, `--mem-profile` and `--metrics`
collect full-check observations. The binary reads dependency list paths relative
to its working directory, as the release check kit does.

`ABAPITI_GO_FULL_TEST=1 go test ./cmd/abapiti -run TestGoFullClosure` compiles
all 1,927 classes and 73 interfaces from the verified 1,538-file source closure,
with CLI-default assume-int, pinned overrides and the reachability manifest.
The same 1,995 unexecuted bodies remain explicit traps. Existing feature,
lexer, statement, structure, MemoryFile and split differential tests run under
`go test ./hir/... ./tsfront/...`.

`node tools/hir-go-check.mjs KIT GO_BINARY OUTPUT_DIR [RELEASE_BINARY] [UPSTREAM]`
rebuilds clean original Node abaplint 577f875e, checks both kit variants byte
for byte, and writes stage timings, peak RSS, CPU/allocation profiles and the
Go top ten CPU entries. It runs each host sequentially under GNU time.

`go test ./hir/golang -v` runs the six ABAP fixtures, int8 observations, and
runtime semantic edge tests in standalone generated modules. The tests in
`tsfront/golang_emit_test.go` run the same lowered lexer and registry programs
as the ABAP tests. Lexer comparison regenerates the Node oracle, checks all 44
cases and mutates an emitted token type to prove the comparison fails. Registry
array, sort, iterator, feature, typed JSON and tagged XML tests use the existing
Node observations. Integration checks skip if Node or the abaplint build is
absent; set TSFRONT_ABAPLINT to the core directory to select another checkout.

Run `node tools/lexer-go-timing.mjs [iterations]` from this clone for the
same-machine lexer benchmark (default 100, 30 warmups). It builds a temporary
Go executable, verifies token counts, excludes compilation/startup/dump output,
and reports milliseconds per lexer run against the original Node implementation.
The correctness-first Go emitter uses checked integer helpers and panic/recover
returns; these measurements do not claim an optimized lexer.

Measured on 2026-10-09, 100 runs per file after 30 warmups:

| Corpus file | Go ms/run | Node ms/run | Go/Node | Tokens |
| --- | ---: | ---: | ---: | ---: |
| real_wasm_compiler | 26.456 | 0.415 | 63.78 | 1840 |
| real_bench_mem | 28.272 | 0.320 | 88.35 | 2056 |
| real_abapgit | 8.862 | 0.142 | 62.47 | 505 |
