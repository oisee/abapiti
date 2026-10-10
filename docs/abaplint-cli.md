# abapiti abaplint

Translates the core of [abaplint](https://github.com/abaplint/abaplint) from TypeScript into ABAP. Generation is pure Go: no Node, npm or SAP system is needed.

```sh
abapiti abaplint -o out                      # abaplint 577f875e is built into abapiti: no network
abapiti abaplint ~/src/abaplint -o out       # or use a checkout (at 577f875e, after npm ci in packages/core)
abapiti abaplint -o out --target native      # one target only: all (default ABAP set), a4h, osg, native, go
```

The ABAP emitter inlines small methods with Grace's inline rules by default.
`ABAPITI_INLINE=classic` runs `hir.Inline` instead, the oracle Grace is checked
against (CI requires both to give the same program), and `ABAPITI_INLINE=0`
disables inlining. `ABAPITI_INLINE_STATS=1` prints the call-site total and
per-callee counts in the same format for both inliners.

Before inlining, calls that pass a one-element array literal to a method that
only iterates its array parameter, `recv.run([e])` in abaplint's combinators,
become `recv.run_one(e)`: the method gets a variant with the loop body run once,
other declarations of it forward to `run([x])`. `ABAPITI_SINGLETON=0` disables
this.

## Input

abaplint commit `577f875ebec44cfaf64841cfe71c8ab8dc32622e` (`@abaplint/core` 2.120.56). Nothing else is accepted.

- **No path**: abapiti unpacks the abaplint sources it carries (the 1,538 closure files of `packages/core` at 577f875e and abaplint's LICENSE, 0.5 MB compressed) and the type declarations of the three npm packages the translation reads (`fast-xml-parser` 5.10.1, `json5` 2.2.3, `vscode-languageserver-types` 3.18.0, with their licenses) into a temporary directory. Nothing is downloaded, so it works on networks that reach neither GitHub nor the npm registry.
- **A checkout path**: no downloads. The packages are looked up in `packages/core/node_modules`, then in the root `node_modules`.

In both cases every source file of the closure (1,538 files reachable from `registry.ts`, `config.ts` and `files/memory_file.ts`) and every pinned file of the three npm packages (type declarations, package.json, license) must match its SHA-256 as recorded at the pin. If one does not match, the command stops and names the expected commit and the first file that differs. The fingerprinted overrides and the reachability manifest are recorded against the same pin. abapiti never builds anything from a different abaplint.

## Output

| Directory | Contents | Next step |
|---|---|---|
| `classes/` | the translated classes and interfaces (2,110 objects), shared by the ABAP targets | |
| `a4h/` | `abaplint-577f875e-a4h.zip`: an abapGit offline repository with the classes, `ZCL_ABAPITI_REGISTRY_A4H` and the report `ZABAPITI_REGISTRY_RUN` (package `$ZABAPLINT`, set with `--package`) | import with abapGit, then run `ZABAPITI_REGISTRY_RUN` as a background job. The driver reads zabapgit from the `ZABAPITI_CORPUS` table through `ZCL_ABAPITI_CORPUS` and logs through `ZCL_ABAPITI_LOG`, so these must already be installed (they are not part of the zip) |
| `osg/` | the classes; with `--input <dir> --deps <dir> --config <file>` also `ZCL_ABAPITI_REGISTRY_RUN`, which embeds those files and compares the issue dump with `--run-sha` | `npm run osgo:unit -- <out>/osg` (or `osgjs:unit`) in an open-steamgate checkout |
| `native/` | `zabaplint.prog.abap` and `lib/` with the classes | in an open-steamgate checkout: `node tools/gogen/osabap.mjs native/zabaplint.prog.abap --lib native/lib` (set GOOS/GOARCH for cross builds), then `.out/osabap --file zabapgit_standalone.prog.abap --config abaplint.json -allow-read .` |

The output is deterministic. The same pinned input gives byte-identical files on every run and from any directory, and they are the same files that the `TestRegistryClosureGate` test path emits. `--evidence` also writes the lowering evidence (the overrides applied, the trapped bodies, and the blocking diagnostics, of which there are none).

## Limits

The build is pruned to the code paths that checking `zabapgit_standalone` with abapGit's `ci/abaplint.json` executes (release v702, six rules). These are the paths recorded in `tsfront/testdata/registrycorpus/reachability.json`. Function bodies outside them are compiled as traps. Other inputs or rules may reach such a trap. In that case the check is refused, and the refusal names the TypeScript location of the missing code (for example `refused: this build has no code for src/rules/….ts:42`). It never gives a silently wrong answer.

## TS-HG@Go

```sh
abapiti abaplint -o out --target go
# Go 1.26 on PATH: out/go/zabaplint-go is built automatically.
# Otherwise only the buildable module is written:
(cd out/go && GOFLAGS=-buildvcs=false go build -o zabaplint-go .)
# Dependency paths are relative to deps.txt; the binary can run from any directory:
/path/to/out/go/zabaplint-go --file zabapgit_standalone.prog.abap --config abaplint.json --deps deps.txt
```

TS-HG@Go translates TypeScript → HIR → Go directly, before ABAP inlining. The
module (`hir.go`, `runtime.go`, `main.go`, `go.mod`) needs only the Go standard
library. The executable prints the same issue dump as the ABAP native driver.
Read access defaults to the directory trees containing the paths passed to
`--file`, `--config` and `--deps`, plus the directory trees containing each
path listed in `deps.txt`. Relative dependency entries resolve against the list's
directory. No other directory is granted access automatically. Repeat
`-allow-read DIR` (or `--allow-read DIR`) to grant additional directory trees.
Paths are made absolute and symlinks resolved before checking containment, so
`..` and symlinks cannot escape an allowed root. A denied read exits nonzero with
`refused: read access denied for "PATH" (resolved to "RESOLVED"); use -allow-read "DIR" to allow it`.
The kit's `check.sh` and `check.ps1` can also pass their existing `-allow-read .`.

`--times`, `--metrics`, `--cpu-profile` and `--mem-profile` add observations.
`--input/--deps/--config` on the generation command still configure the osg target;
pass the executable's flags to check files with Go.

Go remains opt-in, including with `--target all`, to preserve the default output
set and avoid introducing a Go build failure into existing ABAP generation.
`--target all,go` explicitly requests both; the ABAP output remains identical.
Generation and Go build costs are narrated separately. A failed Go build is an
error with compiler diagnostics; absence of Go leaves the sources and prints the
manual build command.

Unsupported static shapes fail generation with a HIR/frontend diagnostic and TS
location. Input-dependent limits refuse the check with a nonzero exit and
`refused: <TS location>: <reason>` on stderr. Runtime helpers capture the nearest
translated TS method only on failure; this names the method's declaration, while
pruned bodies retain their recorded body location. There is no Go panic traceback
for a refused CLI check. These contracts are bounded to the reviewed workload:

| Rejected shape | Diagnostic boundary |
|---|---|
| Unreviewed regex patterns or flags | Literal patterns fail emission; dynamic patterns fail construction. The pinned token grammars and reviewed dynamic macro/SQL-name grammars are supported. |
| Replacement containing `$` | The frontend rejects literal dollar substitutions; dynamic replacement strings are refused at runtime. |
| Exits from the try body of try/finally; try/catch/finally | Frontend/HIR verification diagnostic, before emission. |
| ClassValue factory for an abstract class or constructor requiring nonoptional arguments | Refused when `classvalue.new` invokes the factory. |
| Nonfinite Number literals, general Number remainder, nonliteral or zero Number divisors | Verification/emission diagnostic. Runtime arithmetic rejects overflow, unsafe checked integers, invalid indices/conversions, and fractional/unsafe numeric string rendering. |
| JSON outside the strict JSON grammar or nonfinite JSON numbers | Refused by the JSON subset parser; this is not JSON5. |
| XML outside the reviewed abapGit subset | Refuses malformed nesting, comments, CDATA, DTDs, numeric/unknown entities and prototype-sensitive names. |
| Ordering outside the reviewed ASCII alphabets | Refused by the ordering helper. |
| Escaping/aliased primitive covariant array views or unsupported materialization ABI | Emission diagnostic; only proven unaliased temporary narrowing and matching field constructor shapes are supported. |
| Trapped/pruned/unexecuted bodies | Refusal retains the TS source location; additional rules and workloads are not silently accepted. |

The normal CI job **go target** builds the module/executable, verifies the lexer
against fresh pinned Node output (44/44 plus a mutation check), checks the registry
array/sort/iterator/feature/JSON/XML oracles, and compares clean and seeded output
byte for byte against `zabapgit-check-kit.zip` from release v0.1.1. Go is cached and
the job has a 15-minute timeout. Branch protection is left to the maintainer.
