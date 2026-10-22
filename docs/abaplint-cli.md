# abapiti abaplint

Translates the core of [abaplint](https://github.com/abaplint/abaplint) from TypeScript into ABAP. Generation is pure Go: no Node, npm or SAP system is needed.

```sh
abapiti abaplint -o out                      # abaplint 9f7f098c is built into abapiti: no network
abapiti abaplint ~/src/abaplint -o out       # or use a checkout (at 9f7f098c, after npm ci in packages/core)
abapiti abaplint -o out --target native      # all / abap (ABAP set), a4h, osg, native, go
abapiti abaplint -o small --scope zabapgit   # explicit legacy six-rule pruned build
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

abaplint commit `9f7f098c21e93b8b534fff0f0148f0a900d29d16` (`@abaplint/core` 2.120.73). Nothing else is accepted.

- **No path**: abapiti unpacks the abaplint sources it carries (the 1,548 closure files of `packages/core` at 9f7f098c and abaplint's LICENSE, 0.5 MB compressed) and the type declarations of the three npm packages the translation reads (`fast-xml-parser` 5.10.1, `json5` 2.2.3, `vscode-languageserver-types` 3.18.0, with their licenses) into a temporary directory. Nothing is downloaded, so it works on networks that reach neither GitHub nor the npm registry.
- **A checkout path**: no downloads. The packages are looked up in `packages/core/node_modules`, then in the root `node_modules`.

In both cases every source file of the closure (1,548 files reachable from `registry.ts`, `config.ts` and `files/memory_file.ts`) and every pinned file of the three npm packages (type declarations, package.json, license) must match its SHA-256 as recorded at the pin. If one does not match, the command stops and names the expected commit and the first file that differs. The fingerprinted overrides and the reachability manifest are recorded against the same pin. abapiti never builds anything from a different abaplint.

## Output

| Directory | Contents | Next step |
|---|---|---|
| `classes/` | the full translated core and runtime, with readable `ZCL_LNT_` / `ZIF_LNT_` names, shared by the ABAP targets | |
| `a4h/` | `abaplint-9f7f098c-a4h.zip`: an abapGit offline repository with the classes, `ZCL_LNT_REGISTRY_A4H` and the report `ZLNT_REGISTRY_RUN` (package `$ZLNT`, set with `--package`; the legacy scope defaults to `$ZABAPLINT`) | import with abapGit, then run `ZLNT_REGISTRY_RUN` as a background job. The driver reads zabapgit from the `ZABAPITI_CORPUS` table through `ZCL_ABAPITI_CORPUS` and logs through `ZCL_ABAPITI_LOG`, so these must already be installed (they are not part of the zip) |
| `osg/` | the classes; with `--input <dir> --deps <dir> --config <file>` also `ZCL_ABAPITI_REGISTRY_RUN`, which embeds those files and compares the issue dump with `--run-sha` | `npm run osgo:unit -- <out>/osg` (or `osgjs:unit`) in an open-steamgate checkout |
| `native/` | `zabaplint.prog.abap` and `lib/` with the classes | in an open-steamgate checkout: `node tools/gogen/osabap.mjs native/zabaplint.prog.abap --lib native/lib` (set GOOS/GOARCH for cross builds), then `.out/osabap --file zabapgit_standalone.prog.abap --config abaplint.json -allow-read .` |

Object identities and layouts are deterministic for the same pinned input and build scope. Raw refusal diagnostics can retain the caller's scratch path; package manifests hash the actual emitted bytes. `--evidence` also writes the lowering evidence (overrides, source-located refusal bodies, and blocking diagnostics). The default is **all of core, all rules**, binary64 numbers, with no reachability pruning. It uses the same fail-closed body-repair policy as the certified test build: unsupported operations are refused, not guessed. `--scope zabapgit` retains the old trace-pruned, assume-integer build for size/speed. Its default numeric policy can be overridden with `--assume-int=false`.

## Full-closure measurement

`abapiti abaplint --full [-o out]` lowers the entire pinned closure without
reachability pruning or coverage traps, then exits successfully even when
lowering or HIR verification reports gaps. It does not emit target code.
Evidence is always written to `<out>/evidence` (default `out`), including
`registry-blocking.json`, `registry-verify.txt`, and `gap-summary.json`.
The summary counts blockers by category and source area, lists the top 30
normalised root messages, and separates root blockers from unresolved-local
cascades caused by an earlier failed declaration in the same function.
`files` counts distinct files with blocking or HIR diagnostics. The default
integer assumption is supported; use `--assume-int=false` to measure binary64
lowering instead. `--quiet` suppresses the narration, not the evidence.
Source/IO failures remain errors. `--full` is still a diagnostic-only mode, not the flag needed to emit all rules; the normal build now emits the full core.

### Other TypeScript projects

```sh
abapiti lower --full --tsconfig scratch/tsconfig.json -o out file.ts other.ts
# Include the pinned core source at src/ to retain its fingerprinted adaptations:
abapiti lower --full --tsconfig scratch/tsconfig.json --overrides registry \
  --scope transpiler/src -o out src/registry.ts other-files.ts
```

File arguments are relative to the tsconfig directory. This command uses the
same full-body lowering, numeric policy and HIR verifier, without reachability
pruning, failure traps or target emission. Overrides default to `none`;
`registry` explicitly enables the pinned abaplint registry inventory and its
source validation. `--assume-int=false` selects binary64 measurement.

Evidence includes the complete closure's lowering evidence and
`closure-gap-summary.json`. `gap-summary.json` and `checker-diagnostics.json`
are restricted by optional `--scope` (a tsconfig-relative directory), but the
closure itself is **never** restricted. Root-message entries include an example
location. Scoped HIR counts include only errors with a source location beneath
that directory; global/synthetic errors remain in the complete evidence.
Checker errors are recorded separately, not counted as lowering blockers.
Gaps do not cause a failing exit status; source/IO/override failures do.
See [the transpiler measurement](generality-transpiler.md) for a second-project
example and its limitations.

## Limits

The default includes every rule, not just the six used by the zabapgit check. Certification covers the pinned core's 11,060 Node-active tests on Go and OSGO, not arbitrary TypeScript or every possible input. Unsupported bodies/operations retain source-located refusals. Only **`--scope zabapgit`** prunes to the trace in `tsfront/testdata/registrycorpus/reachability.json` (v702, six rules); other rules can reach coverage traps in that explicit legacy build. Neither scope silently substitutes an answer.

The release's `$ZLNT` / `$ZLNT_TESTS` pair is built together from the complete 17-shard union so both packages share exactly one name/shape table. Use [the A4H build guide](a4h-test-suite.md) to reproduce those zips; A4H kernel certification is still in progress.

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
for a refused CLI check. The following historical pruned-build limits describe the explicit legacy scope; the full build supports the additional operations exercised by the certified suite:

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

### Embedding the Go checker

```sh
# Fresh output directory; --package still means the ABAP A4H package.
abapiti abaplint --target go --go-package abaplint -o out-library
# out-library/go is an importable module named zabaplint, package abaplint.
# Link it from a consumer with a local module replace; do not copy/edit hir.go.
```

`--go-package` defaults to `main`, preserving CLI generation and its automatic
build. Another valid Go package name uses `hir/golang.EmitPackageWithNames` and emits
`library.go` beside the translated core, without building an executable or
emitting the CLI/site profiler. It requires the full `core` scope. Mixing old
CLI/library output files is refused; use separate fresh output directories.

The wrapper exports:

```go
var checker abaplint.Checker
checker.Prime([]abaplint.File{{Filename: "/src/ztest.prog.abap", Raw: "REPORT ztest."}}, nil, configJSON)
issues := checker.Check("/src/ztest.prog.abap")
checker.Update("/src/ztest.prog.abap", &buffer) // empty buffer is a file
checker.Update("/src/ztest.prog.abap", nil)     // remove
stats := checker.Stats()
```

`Prime(files, dependencies, config)` owns one persistent registry, replacing its
previous state, and indexes lexical readers. File names and source bytes are
preserved. `Check` resolves the owning object and uses `findIssuesObject`,
returning ordered structured issues (`Filename`, `Key`, `Severity`, `Message`,
`Row`, `Column`, `EndRow`, `EndColumn`), without the batch report/dump transport.
Coordinates must be finite integers representable as int32. `Update` dirties the edited owner and its
direct lexical readers using the same conservative word pattern as OSG;
parsing stays lazy until Check, CheckObject, Tree, CDS, DDIC or Files.
Removing an include preserves sibling readers;
removing the last file discards the old object identity. `Stats` exposes work
counters, not a timing or engine parity certificate. `ParseRuns`/`ParseObjects`
count pending invalidations submitted after Update, excluding untouched inert
objects whose no-op parsers leave them permanently dirty. Several Updates before
parsing accumulate a union; no-op Updates retain that pending state.

Failures panic with a cause for the host to catch and report as a refusal.
After failed Prime/Update the host must discard or re-prime that checker.
For an overlay, the host saves the original source (or absence), defers
`Update(filename, original)`, installs the buffer, and calls Check. A failed
Check may be retried after successful restoration; failed restoration requires
discard/re-prime. Serialize that entire host transaction. A global `libraryMu`
serialises all Checkers, including reads: one checking thread per process.
Translated core globals assume sequential access. No file I/O or Node runtime
is used by this API. Configuration and input discovery belong to the consumer.

Focused library tests cover the wrapper/runtime lifecycle with a small core
double and every template placeholder against identifiers from a real emitted
HIR graph and its name table:
`go test ./tsfront -run '^TestRegistryGoLibrary' -count=1`.
The opt-in full-core fixture test below checks the actual pinned parser;
consumer HTTP/performance gates remain separate acceptance work.

Hosts can select a process-wide GC setting before `Prime`, for example
`debug.SetGCPercent(200)` or `GOGC=200`. On the frozen M1 workload, 20 alternating
u7 pairs measured broken checks at 668 → 512 ms with GOGC=100/200, while peak RSS
rose from 1.98 to 2.70 GiB. The library leaves GC policy to the host; select it
with the process memory budget in mind. See the
[measurements](evidence/library-incremental-u7.json) and
[reproduction](../tsfront/testdata/library-incremental/README.md).

The library also exports object checks and detached outline snapshots:

```go
issues, found := checker.CheckObject("CLAS", "ZCL_EXAMPLE")
files, found := checker.Tree("CLAS", "ZCL_EXAMPLE")
text := abaplint.ConcatTokens(files[0].Statements[0].Tokens)
```

Both lookups use `Registry.getObject` directly: names are case insensitive,
object types are case sensitive, and absence returns `(nil, false)`. A present
non-ABAP object returns an empty file slice from `Tree`. `CheckObject` preserves
`findIssuesObject` issue ordering, duplicates and coordinates, just like `Check`.
`Tree` parses pending edits without running rules. Both APIs hold `libraryMu`,
and returned trees contain Go values that callers may retain or edit after the
lock is released. Trees contain statement and structure nodes only; expressions
are represented by their statement's flattened `getTokens()` list.

For an isolated outline without priming a baseline, use
`abaplint.TreeOf(files, "CLAS", "ZCL_EXAMPLE")`. It builds a temporary registry
from just those files, parses them and expands macros, and copies the same
`FileTree` values. It skips reader indexing and global type-definition discovery.
`TreeOfWithConfig(files, typ, name, config)` accepts the same configuration as
`Prime`; `TreeOf` uses the core defaults. Supply the same release/global-macro
settings for parity with an existing Checker. INCLUDEs and macros from objects
outside the supplied files are absent. `Checker.Tree` keeps its existing full
registry semantics. All variants hold `libraryMu` through snapshot copying.

`Node.Kind` uses the core's constructor descriptor name. `Pos` contains raw,
one-based row/column coordinates, and token ends come directly from
`AbstractToken.getEnd()` (the start row and start column plus UTF-16 string
length, even for multiline tokens). Node bounds use the first token's start and
last token's end. An empty structure has no token positions: its `Start` and
`End` are zero positions. It remains a non-nil structure with empty children;
a file with no parsed structure instead has `Structure == nil`.

`FirstStatement` is nil for the pinned parser's structures. In .73,
`StructureNode.getFirstStatement()` immediately recurses through the first
child, rather than trying subsequent siblings. A structure with accessible token
bounds has a nonempty first-child chain terminating at its first depth-first
statement; the two results therefore coincide. Empty structures return undefined
and have no statement descendant. The Node oracle independently compares the
actual `getFirstStatement()` result against a depth-first search.

Class and interface files use `getSequencedFiles()` verbatim, including class
include ordering and the interface's main-file-only behavior. Other ABAP objects
retain `getABAPFiles()` order with `getMainABAPFile()` moved to the front.
`FileTree.Statements` comes from the file's statement list, which can contain
statements omitted from its structure (for example pragma-only statements).

`Token` has one additional field, `Pragma bool`, recording lexer identity.
Matched statements usually remove pragmas from `getTokens()`, but unknown,
empty and pragma-only statements can retain them. `ConcatTokens` skips tokens
with this flag and uses the previous non-pragma token's start and UTF-16 string
length for adjacency, preserving punctuation, chain expansion and spacing.
Looking for a `##` prefix in text would lose the token's original class identity.

The full-core fixture test can be enabled with
`ABAPITI_LIBRARY_TREE_DIR=/absolute/path/to/out-library/go`; see
[`testdata/library-tree`](../tsfront/testdata/library-tree/README.md) for the
Node/Go differential driver and reproducible inputs.


The library exposes CDS, dictionary and raw object-file snapshots as well:

```go
type Expression struct {
    Kind, Text string
    Start, End Pos
    Children []Expression
}
func (c *Checker) CDS(name string) (tree *Expression, sqlView *string, found bool)
func (c *Checker) DDIC(typ, name string) (components, keys []string, found bool)
func (c *Checker) Files(typ, name string) (files []File, found bool)
```

Each call holds `libraryMu` and calls `registry.parse()` before `getObject`,
including missing-object lookups. `CDS` looks up DDLS; undefined trees and SQL
view names return nil, while a defined empty SQL view name returns a pointer to
`""`. Expression kinds use `node.get().constructor.name`, text uses the core's
`concatTokens()`, and positions use the first token's start and last token's
`getEnd()`. Children contain only ExpressionNodes, in core order; TokenNodes are
skipped.

`DDIC` handles TABL and VIEW. It calls `parseType(registry, undefined, undefined)`
and copies component names only when the result is a StructureType. Other types
return nil components; an empty structure would return a non-nil empty slice.
TABL keys come from `listKeys(registry)` and VIEW keys from `listKeys()`, including
VIEW's source field names when exposed fields have aliases. Order and case are
preserved for both components and keys. Empty key lists remain nil.

`Files` returns `getFiles()` in object order, copying each filename and raw source.
All three APIs return `found=false` when `getObject` returns undefined. Returned
snapshots are detached Go values.

Set `ABAPITI_LIBRARY_DATA_DIR=/absolute/path/to/out-library/go` to enable the
full-core data fixtures. [`testdata/library-data`](../tsfront/testdata/library-data/README.md)
documents the original Node oracle and the differential against OSG PR #839's
exact base-library adapter, including all 41 change scenarios and frozen M1.
