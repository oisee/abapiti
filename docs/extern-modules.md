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

Host calls conservatively read arguments and global state, may write global state, allocate, and raise.

## ABAP host classes

ABAP emission refuses calls into a module without `abapClass`, with their source location. A module with `"abapClass": "ZCL_..."` (at most 30 characters) is called as static methods of that global class:

```json
{"modules":[{"file":"tools/gogen/host-io.ts","goImport":"github.com/oisee/abapiti/extern/hostio","goDir":"../../extern/hostio","abapClass":"ZCL_ABAPITI_HOSTIO"}]}
```

`readText(file)` becomes `zcl_abapiti_hostio=>read_text( file = ... )`: the method name and the formal parameter names are the TypeScript names in snake case (at most 30 characters, not `result`). The value types are `string`, `abap_bool` and `f` (the ABAP backend's `number`); `string[]` and a rest parameter are `string_table`; `[string, boolean][]` is the host class's `ty_pairs`, which the class must declare as `STANDARD TABLE OF ty_pair WITH DEFAULT KEY` with `ty_pair` having `f0 TYPE string` and `f1 TYPE abap_bool`. A result is `RETURNING VALUE(result)`; `string | undefined` is `REF TO string`, unbound for undefined. Arguments are evaluated before the call and passed as copies, so the read-only array contract above holds. A host exception that is not a `CX_NO_CHECK` (that is, any `CX_STATIC_CHECK` or `CX_DYNAMIC_CHECK`) becomes a catchable TS `Error` whose message is `get_text( )`. Unlike Go, strings are not checked for lone surrogates at the boundary; the host's UTF-8 conversion decides.

`extern/hostio/abap/zcl_abapiti_hostio.clas.abap` (with abapGit XML) is the reference ABAP host for `host-io.ts` and the step-4 driver's `readText`/`writeStdout`. It is self-contained ABAP 7.50 and needs no OSG runtime extension:

- `read_text`: `OPEN DATASET ... IN BINARY MODE`, then strict UTF-8 decoding; on invalid input, maximal-subpart decoding with one U+FFFD per invalid sequence, like Node and `extern/hostio`.
- `write_text`: UTF-8, `OPEN DATASET ... FOR OUTPUT IN BINARY MODE` (create or truncate).
- `write_stdout`: appends to the stdout file, truncated on the first write of the run. Its path is `configure( stdout = ... )`, else the snapshot's `stdout`, else `gogen.go`.
- `basename`, `dirname`, `join` (and `normalize`): ports of Node's `path.posix`.
- `list_dir`, `list_entries`, `env`, `git_head`: replayed from `fs-snapshot.json` (`configure( snapshot = ... )`, default relative to the dataset home), because ABAP has no directory listing, no process environment and no git. A directory that is missing from the snapshot, or recorded with an error, raises `CX_SY_FILE_OPEN`; so does a file that cannot be opened.

On OSGo, datasets are sandboxed: set `OSD_DATASET_HOME` to the working directory of the Node reference (relative names resolve against it), `OSD_DATASET_READ` to the read roots and `OSD_DATASET_WRITE` to the output roots.

`emit-abap --initialize order.json` adds `ZCL_ABAPITI_MAIN` with a static `RUN` that calls each module's initializer in the given order (the TS-HA runner's order, shared code). ABAP class constructors are lazy, so this replaces Go's eager module initialization; `RUN` is the program entry.

### fs-snapshot.json

`tools/fs-snapshot-recorder.mjs` is a `node --import` preload for the reference run. It records, without changing any result or error, what the program observed through `fs.readdirSync` (names, or `{withFileTypes: true}`), `child_process.execFileSync("git", ["rev-parse", "HEAD"], ...)` and `process.env`, and writes `FS_SNAPSHOT_OUT` at exit:

```json
{
 "version": 1,
 "cwd": "/abs/working/dir",
 "stdout": "/abs/out/gogen.go",
 "dirs": {"/abs/dir": [["name", false], ["sub", true]], "/abs/missing": {"error": "ENOENT: ..."}},
 "env": {"GOGEN_NOLINE": null, "HOME": "/home/x"},
 "gitHead": {"/abs/repo": "40-hex", "/abs/not-a-repo": null}
}
```

Keys of `dirs` and `gitHead` are `path.resolve` of the argument; the host resolves its argument against `cwd` with the same POSIX rules. Entries keep Node's native order and `Dirent.isDirectory()`. `env` holds only the names the program read (`null`: absent). `stdout` comes from `FS_SNAPSHOT_STDOUT` and is optional. The first observation of a key wins. JSON escapes are the ones `JSON.stringify` writes.

`tools/selfhost-step4.py --abap` records the snapshot in its Node reference run and binds both extern modules to `ZCL_ABAPITI_HOSTIO`. `tools/extern-abap-osg-smoke.sh ABAPITI OSG OUT` runs `cmd/abapiti/testdata/extern-abap` under Node and, via `emit-abap --initialize` and `tools/selfhost-step4b-osg.mjs --build --main`, on OSGo, and compares the two outputs byte for byte, running the binary from a different directory. It also checks that an uncaught JS Error reports its payload message and exits with status 3.

`--main` calls `ZCL_ABAPITI_HOSTIO=>configure` before `RUN` with absolute paths fixed at build time: `OUT/fs-snapshot.json` and `OUT/gogen.go` by default. Pass `--snapshot /abs/run/fs-snapshot.json --stdout /abs/run/gogen.go` when the bridge output is a separate directory. Set `OSD_DATASET_HOME` to the Node reference working directory, `OSD_DATASET_READ` to cover the OSG checkout, lib folders and snapshot, and `OSD_DATASET_WRITE` to cover stdout and any other host writes.

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
