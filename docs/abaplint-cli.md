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

### Stable site maps and profiling certificates

A run that includes `--target go` writes `sites.json` next to `go/`. Its
`schema` is `sites/1`; `sites` is an ordered array of emitted occurrences:

```json
{
  "schema": "sites/1",
  "sites": [{
    "site_id": "src%2Fprobe.ts.Probe.run|src%2Fprobe.ts%3A4%3A3|virtual|0",
    "kind": "virtual_call",
    "source": "src/probe.ts:4:3",
    "method": "src/probe.ts.Probe.run",
    "inline_path": [],
    "locations": {
      "go": {"file": "hir.go", "line": 120},
      "go_profile": {"file": "hir_profile.go", "line": 135}
    }
  }]
}
```

`site_id` consists of URL query-escaped qualified owning method and original
HIR `Source`, followed by HIR node kind and zero-based ordinal, separated by
`|`. The ordinal counts nodes of the same kind and source within that method
in pre-order (statement, X/Y expressions, Body/Else, List; expression,
X/Y/Z, Args, Seq statements). It never uses global serial IDs. Source paths
are frontend-relative, with one-based TS line and column. Synthetic nodes
whose source is a name or empty string omit source from the identity and
use owner/kind/ordinal. The map uses the enclosing method location when
available, otherwise their descriptive synthetic source. Closure
owners derive from the enclosing qualified method plus the arrow location,
so the legacy serial-based generated class name does not enter the ID.
Declarations outside a method use their qualified class/interface as owner.

Kinds of interest are `call` (including super calls), `virtual_call`, `loop`
(`while`/`foreach` in the ID), `new`, and `runtime_op`. Inlining preserves the
callee's original ID, source and owner. `inline_path` contains the ordered
call-site SiteID chain from outermost caller to the callee, excluding the
callee's own ID. Multiple emitted occurrences may share an ID; the path
identifies their inline context. Profile counts aggregate those occurrences
by original SiteID. A location is one-based and relative to `out/go`; it
points to the first emitted token of the operation or its evaluation prelude.

`hir.AssignSiteIDs` is the frontend/transform hook. Backends can use
`hir.NewSiteMap`, `SiteMap.Add` and `Site.Locations` to attach additional
locations. The ABAP backend can attach `locations.abap` with `file`, `class`,
`method`, and `line` without changing IDs or the schema. Metadata has no
influence on target semantics or spelling. `golang.EmitWithSites` exposes
source and location emission together; ordinary `golang.Emit` stays unchanged.

Build the opt-in profiler and run it on the kit:

```sh
cd out/go
go build -tags profile_sites -o zabaplint-go-profile .
./zabaplint-go-profile --profile-sites profile.json \
  --file /path/to/kit/zabapgit_standalone.prog.abap \
  --config /path/to/kit/abaplint.json --deps /path/to/kit/deps.txt
```

The default build selects `hir.go` and `site_profile_off.go`; the profiling
build selects `hir_profile.go` and `site_profile.go`. There are no counter
calls, receiver inspections, loop counters or profiling closures in the
default generated method bodies. Passing `--profile-sites` to that build
fails explicitly. The profiling variant counts even when no output path is
specified; supply the flag to save the certificate. Profiling leaves normal
stdout unchanged.

Certificates have schema `site-profile/1`, `input_sha256`, `binary_sha256`,
and a `sites` object keyed by SiteID. Call counters contain `calls`; virtual
calls also contain `receivers`, mapping fully qualified TS class names to
counts. Allocation sites contain `allocations`. Loops contain `invocations`,
`trips` and `histogram` invocation counts in buckets `0`, `1`, `2-3`, `4-7`,
`8+`. Zero-valued scalar counters may be omitted; histogram buckets are
always present for executed loop sites. Unexecuted sites are absent. An
invocation records actual body entries, including early break, return and
exception exits; it does not predict trips from an array's initial length.

The input hash covers every successful host read, in order: config, primary
ABAP input, dependency list (if present), then dependency contents. Each read
is framed as decimal basename-byte-length, `:`, basename, decimal content-byte-length,
`:`, content. This binds both configuration and dependency data and avoids
absolute-path dependence. The binary hash is SHA-256 of the executed binary's
bytes. Output-file errors are fatal; stdout remains the ordinary issue dump.

To join a certificate back to TS names and print the full receiver histograms,
loop distribution and top allocations:

```sh
python3 tools/site-profile-report.py out/sites.json out/go/profile.json
```
