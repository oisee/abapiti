# Extern modules

An extern configuration maps TypeScript source files to typed Go packages:

```json
{"modules":[{"file":"tools/gogen/host-io.ts","goImport":"github.com/oisee/abapiti/extern/hostio","goDir":"../../extern/hostio"}]}
```

`file` and a relative `goDir` resolve against the tsconfig directory. Extern source files belong to the TypeScript project, but their bodies and classes are excluded from HIR. Exported function declarations supply the ABI. Named imports, import renames and namespace calls resolve to the declaration's original name, with its first letter capitalized for Go. Function values, other exports and namespace values produce located blocking diagnostics.

```sh
abapiti lower --full --tsconfig tsconfig.json --externs externs.json src/main.ts tools/gogen/host-io.ts
abapiti emit-go --tsconfig tsconfig.json --externs externs.json -o out/go --build src/main.ts tools/gogen/host-io.ts
```

`emit-go` emits module initialization and an empty `main`. `--entry file.ts.Class.method` additionally invokes a static method with no parameters. Its return value is discarded. `abaplint --target go` also accepts `--externs`.

Supported annotations are `string`, `boolean`, `number`, `void` (result), `string | undefined` (result), `string[]`, `[string, boolean][]`, and a final rest parameter `...parts: string[]`. A final spread argument can supply the rest array.

The Go ABI uses `string`, `bool`, `float64`, `[]string`, and `[]struct{ F0 string; F1 bool }`. An optional string result uses `(string, bool)`. A host function may append an `error` result; a non-nil error throws a catchable TS `Error` whose message is `err.Error()`. Generated adapters constrain the host function to exactly the two permitted signatures, so parameter or result mismatches fail `go build`. Type switching inside that constraint selects the error convention; arguments never pass through an untyped function call.

Strings cross through the runtime's UTF-16/UTF-8 conversions: non-BMP characters round-trip, and isolated UTF-16 surrogates become U+FFFD when passed to Go. The reference `extern/hostio` module implements filesystem, POSIX path, Git HEAD and environment operations. Its directory results use byte order; directory symlinks report false in `listEntries`.

ABAP emission refuses extern calls with their source location. Host calls conservatively read arguments and global state, may write global state, allocate, and raise.

The host module has its own `go.mod`. To run root and nested-module tests together, use a temporary Go workspace containing the repository root and `extern/hostio`, or test the two modules separately.
