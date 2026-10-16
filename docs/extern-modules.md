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

Extern array parameters (`string[]`, `[string, boolean][]`, and rest arrays) are read-only inputs. Go receives a copy, including tuple fields; host mutations are not written back to the TypeScript array. Host implementations must treat these parameters as read-only and return any changed data explicitly.

Strings cross through the runtime's UTF-16/UTF-8 conversions: non-BMP characters round-trip, and a string argument containing an isolated UTF-16 surrogate raises a located trap (`extern string argument has a lone surrogate`) before the host call, including strings in arrays, tuples, and rest arguments. The reference `extern/hostio` module implements filesystem, POSIX path, Git HEAD and environment operations. Its directory results use byte order; directory symlinks report false in `listEntries`.

ABAP emission refuses extern calls with their source location. Host calls conservatively read arguments and global state, may write global state, allocate, and raise.

The host module has its own `go.mod`. To run root and nested-module tests together, use a temporary Go workspace containing the repository root and `extern/hostio`, or test the two modules separately.

For a complete module that retains unsupported functions as located runtime
traps, use `emit-go --trap-on-failure`. This applies the same fail-closed body
repair as the full abaplint Go pipeline and writes lowering evidence under
`<output>/evidence`. Default emission still rejects lowering failures.
`--overrides registry` selects the source-pinned registry overrides;
`--reachability <manifest>` accepts a generic workload manifest (no upstream
pin or recorded inputs). See [selfhost-step4.md](selfhost-step4.md) for the
end-to-end gogen reference harness.

`--initialize order.json` takes a JSON array of source files from the selected
closure. It calls their generated module initializers in that order, once,
before the optional static entry. Files with no module state (including
extern declarations) need no call. This is required for top-level program
code and imported side effects that no later static read would trigger.
The step-4 harness records runtime import postorder after TypeScript erases
type-only imports, with the reviewed package roots initialized first.
