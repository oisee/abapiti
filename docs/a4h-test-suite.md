# abaplint test suite: A4H build and offline collection

This is **phase 2, stage 1: build only**. None of these tools connect to SAP,
activate objects, create jobs, or fetch database rows. Deployment, activation,
background jobs and ST22 inspection belong to the coordinator.

## Build the core, registry and tests packages

Use the pinned abaplint checkout (commit `577f875e`, core 2.120.56, `npm ci`
already completed), Go 1.26.0 and the project's normal Node dependencies:

```sh
node tools/a4h-tests-build.mjs "$ABAPLINT_HOME" "$BUILD"
```

On the shared OSG host, every heavy command must be admitted at load <12 and
wrapped with `flock "$OSG_HEAVY_LOCK" systemd-run --user --scope
-p MemoryMax=48G nice -n 10`. Use caller-owned scratch and the environment
specified in [the harness guide](osg-harness-speed.md). Never nest the same
flock. Do not use the all-suite scheduler for this preparation.

The tool discovers **all 17 original shards**, materializes their unmodified
static union with separate dependency-first initialization plans, and invokes:

```sh
go run ./cmd/abapiti abaplint-tests "$BUILD/closure" -o "$BUILD" --target a4h
```

`--target a4h` is build-only, forces readable names, adds the zabapgit deployment
harness, and refuses partial/duplicate shard plans. There is no trace-based
pruning or assume-integer contract. A single emission/name table serves all three
packages; shared structural shapes may originate at either a core or test site.

Deploy **`zlnt.zip` to `$ZLNT` first**, then **`zlnt_registry.zip` to
`$ZLNT_A4H`**, then **`zlnt_tests.zip` to `$ZLNT_TESTS`**:

* `$ZLNT`: the full translated core and its shared runtime, including the
  deployment harness used by the zabapgit client. No executable test
  registrations, test lambdas or mocha/chai classes belong here.
* `$ZLNT_A4H`: the readable `ZCL_LNT_REGISTRY_A4H` driver and
  `ZLNT_REGISTRY_RUN`/`CLEAN` reports (plus `NEG` when emitted), corpus/log
  runtime and setup reports, and optional `ZCL_LNT_TEST_LOG_BAL`. These objects
  belong only to the registry archive.
* `$ZLNT_TESTS`: original test registrations/bodies, mocha/chai, test-only
  runtime helpers, `ZCL_LNT_TEST_RUNNER`, `ZCL_LNT_TEST_LOG`,
  `ZLNT_TESTS_RUN`, and the transparent table `ZLNT_TESTRUN`.
* Global constructor/deletion dispatch uses a registered test extension. The
  core has no static dependency on the test package; only the test runner
  installs the extension. The zabapgit client needs no test package.
* Each zip has a companion `*.manifest.json`. Object hashes cover **all actual
  abapGit files**, including generated metadata and DEVC. The manifest describes
  its SHA-256 length-framing algorithm. Compare object keys/hashes for diff
  deployment; a missing old object must be handled as a deletion, not ignored.
* All emitted locations (refusals, HIR spans, test registration metadata and
  diagnostics) are relative to the lowering root, for ABAP/A4H, OSG and Go.
  A two-root regression compares emitted bytes, including `names.json`.
* `names.json` maps ABAP names to TS identities and relative file:line:column.
  Synthetic runtime objects have no original TS declaration; generated wrappers
  identify their generator. Readable declaration names use `ZCL_LNT_`,
  `ZIF_LNT_`, `ZCX_LNT_`; shared synthetic helpers retain the readable emitter's
  deterministic `ZLNT_` fallback. All names are bounded by 30 characters.

`core/` is the exact shared source set. `native/zabaplint.prog.abap` is an OSGO
client for that set, not another translated core. `native/lib/` contains the
same translated core bytes. The SAP corpus driver and its
`ZCL_ABAPITI_CORPUS`, `ZIF_ABAPITI_CORPUS`, `ZABAPITI_CORPUS` and
`ZCL_ABAPITI_LOG` services are packaged together in `$ZLNT_A4H`.
`abap/` is the combined test fixture before replacing the empty OSG checkpoint hook with durable A4H logging.
Do not deploy `abap/` separately or overwrite the old hashed-name package.

`tools/release-build.sh` copies this build's `zlnt.zip`, `zlnt_registry.zip`
and test archives to the release assets. It checks object ownership and scans
core for driver/corpus-log references, once with the monolithic tests and again
with the seven test parts. The monolith and parts are alternative installs.
The standalone corpus archive is retired because registry owns those objects.
Set `RELEASE_PACKAGES_ONLY=1` to run the real release packaging path offline
without building native clients/bundles; supply the usual five arguments and
a fresh WORK/ASSETS pair. This mode builds the full readable union, lints
core/tests and the matching registry driver, splits/verifies the tests, and
checks both archive install sets. SAP BAL adapter activation still requires SAP.

The generated ABAP uses the project's 7.50-style syntax and 255-byte line
formatter. **FIND/REPLACE PCRE itself requires 7.55**; A4H 7.58 supports it. The
existing CI lint gate uses v750 for non-PCRE fixtures and the narrow v755 PCRE
exception. Calling PCRE genuine v750 syntax would be incorrect.

## Parallel offline import: base + independent test parts

Reuse the full build above; **do not re-emit or rename its objects**:

```sh
node tools/a4h-tests-parts.mjs "$BUILD" "$PARTS"
node tools/a4h-tests-parts.mjs "$BUILD" "$PARTS" --verify
node tools/a4h-tests-ddic-test.mjs "$BUILD" "$OSG"
node tools/a4h-tests-parts-test.mjs
```

The splitter uses `closure/closure.json` (each original shard's initialization
files, including borrowed roots) and `names.json` source identities. Objects
shared by original shards, mocha/chai, runner/log/report/table, unmapped synthetic
helpers, and shared rules utilities start in base. Small ABAP shards are grouped
with statements; objects/rename/LSP stay together. Rules is split into two halves
by deterministic, largest-first **emitted-source byte** balancing of indivisible
TS root files. `parts.json` records the roots, provenance, assignments, dependency
promotions, original versus added object counts, zip sizes/hashes and import
prerequisites. The syntax shard's large single registration module is deliberately
not cut apart, so its part is larger than the others.

An ABAP lexical scan checks *all* Z identifiers that designate objects, not just
`TYPE REF TO`: it covers `NEW`, `CAST`, static calls, inheritance, interfaces,
exception/type names and references inside string-template expressions. Literal
test fixtures, comments, locally declared names and member/method selectors are
excluded. Unknown object references fail closed. Cross-part targets are promoted
to base to a fixed point; base is checked too. The verification command reopens
each emitted zip, checks hashes and object coverage, rescans the **actual emitted
ABAP**, rejects duplicate/core-overlapping names, and confirms that all unadapted
original objects retain their original full serialized object hashes.

The original runner does **not** discover tests through package membership or
TADIR. It contains the full manifest's per-shard initialization plans. Only its
zero-argument test `host_init_*` wrappers change to named dynamic calls; the
original selectors, order, registrations, checkpoint/resume ABI and report remain
unchanged. Import **all parts before running**, especially both rules halves when
selecting `P_SHARD=rules`.

Three other suite-wide generated switches would otherwise drag almost all test
classes into base: constructor factory, delete dispatch, and chai entries/invoke.
Their original typed leaf bodies now live in three generated helper classes per
package. Base routers dynamically call the appropriate helper. Constructor
argument conversions and exceptions are unchanged. Delete/chai use the receiver's
RTTI ancestor chain to preserve the original `IS INSTANCE OF` **first-match
order**, rather than grouping execution by package; superclass traversal uses the
standard `SUPER_CLASS_NOT_FOUND` exception. These dynamic routes are runtime test
dispatch, not static activation dependencies between offline repositories. Missing
imports are not swallowed by the routers. All 5,002 original names remain; only
four original implementations change, plus 21 named helper classes.

### Verified split (2026-10-14)

Counts exclude the one DEVC in each archive; include the three new dispatch
helpers per package. Each zip has its own `.abapgit.xml`, `src/package.devc.xml`
and SHA-256 object manifest, using the original length-framed algorithm.

| Part / zip suffix | Package | Original objects | Total objects | Zip bytes | MiB |
| --- | --- | ---: | ---: | ---: | ---: |
| `base` | `$ZLNT_TESTS_BASE` | 82 | 85 | 685,986 | 0.65 |
| `statements` | `$ZLNT_TESTS_STATEMENTS` | 565 | 568 | 2,670,698 | 2.55 |
| `rules_1` | `$ZLNT_TESTS_RULES_1` | 1,086 | 1,089 | 3,874,724 | 3.70 |
| `rules_2` | `$ZLNT_TESTS_RULES_2` | 822 | 825 | 3,603,624 | 3.44 |
| `syntax` | `$ZLNT_TESTS_SYNTAX` | 1,392 | 1,395 | 6,450,684 | 6.15 |
| `objects` | `$ZLNT_TESTS_OBJECTS` | 485 | 488 | 2,966,980 | 2.83 |
| `rest` | `$ZLNT_TESTS_REST` | 570 | 573 | 2,197,314 | 2.10 |

Independence: **5,023 objects, 115,034 unique-per-object reference edges,
zero violations**. The initial base has 81 objects; the source scan promotes
`ZCL_LNT_DUMMY_NODE` from statements because the CDS parser tests in rest also
reference it. All 4,998 unadapted original objects have identical serialized
hashes. Actual emitted-source syntax lint (core + parts, standard read-only
dependencies, v755 PCRE exception): **12,383 files, zero issues**.

Import/activate **`zlnt.zip` → `zlnt_tests_base.zip` → the six shard zips in
parallel**, each into its indicated package with its **own offline abapGit repo**.
Do not also import `zlnt_tests.zip`: these are the same object names, not duplicate
copies. The DEVC files are root packages, with **no superpackage/PARENTCL**;
creating `$ZLNT_TESTS` is unnecessary. They use the same standalone offline repo
layout as the original build. Actual SAP import/activation remains a coordinator
check; this preparation never contacts SAP.

Splitter tests cover synthetic cross-part failure, iterative promotion, lexical
contexts/templates, shard mapping, rules balancing, dispatch preservation,
reproducible standalone archives and hash tampering. Gates: `go vet ./...`;
`GOFLAGS=-buildvcs=false go test -short -p 2 ./tsfront/ ./cmd/... -count=1`;
all **18** `tools/*-test.mjs` scripts individually with Go 1.26.0; lint against
`origin/main`: **0 new issues**, canary 5/5. No OSG heavy runs were needed.

### v0.5.0 download names and path gate

`tools/release-build.sh` retains the monolithic tests zip for compatibility and
also publishes `abaplint-577f875e-a4h-tests-<part>.zip` (base, statements,
rules_1, rules_2, syntax, objects, rest), their manifests, and
`parts.json`. The index's zip filenames refer to the
published downloads; its archive hashes are unchanged by renaming.

The table above records the initial splitter measurement. After root-relative
locations shorten emitted sources, byte-weight balancing is recomputed: the
release parts contain **85 / 568 / 1,097 / 817 / 1,395 / 488 / 573** objects
(base / statements / rules_1 / rules_2 / syntax / objects / rest), still **5,023
objects and zero independence violations**. Current sizes are recorded in
[the release inventory](release-v0.5.0-assets.tsv) and `parts.json`.

Choose **either** the monolith **or** the parts, never both. Import the release
core `abaplint-577f875e-a4h.zip`, then (from v0.5.2)
`abaplint-577f875e-a4h-registry.zip`, then `abaplint-577f875e-a4h-tests-base.zip`,
then the six remaining parts in parallel. Import all parts before running tests.

From v0.5.2 the core archive contains only translated core. The registry archive
has its own `$ZLNT_A4H` package: driver, run/clean reports (neg with `--negative`),
`ZCL_ABAPITI_CORPUS`, `ZCL_ABAPITI_LOG`, their interface and tables, and the corpus
loader/log setup reports, plus the optional `ZCL_LNT_TEST_LOG_BAL` progress adapter.
Runtime files are embedded in the standalone binary.
Run `ZABAPITI_LOG_SETUP` once and load inputs with `ZABAPITI_CORPUS_LOAD` before
using the registry reports. Test packages retain their independent table logging
service.
The OCI import order is `zlnt-core` (lib), `zlnt-registry-a4h` (input), then
`zlnt-tests-base` and the test parts. This fixes the system self-containment gap
found by the E.10 OSGO proof (13 syntax issues in the registry driver).

`tools/a4h-tests-lint.mjs` checks extracted core/tests sources with corpus runtime
as repository dependencies. To verify release self-containment, extract the new
registry archive too and check all layer sources together; do not let the
repository corpus dependency mask a missing registry layer.

Before generating `SHA256SUMS`, the release script runs
`tools/release-path-gate.py` over **every asset**, all decompressed zip/tar entries
and binary `strings -a` output. Build roots, local mount/home/temp prefixes and
the build user's name as an identifier token are forbidden; any match fails the
build. Identifier boundaries distinguish a user name from unrelated words such
as CSS color names; there is no allowlist for path-prefix matches. The gate reports
zero leaked entries per asset rather than relying on a quick binary spot check.

### Bounded generated methods (v0.5.2)

The shared emitter keeps shard selection in `ZCL_LNT_TEST_RUNNER=>RUN` and
moves each branch to a private `run_<shard>` method. Non-identifier shard names
use the emitter's sanitized hash names within ABAP's 30-character limit.
Each shard calls ordered chunks of about 200 initialization statements. The
original `host_init_*` recovery boundaries, checkpoint indices, JSON, and common
RUN prologue/epilogue remain in order. Initialization scratch attributes reset
where the former RUN locals were initialized.

Large translated registration actions also use ordered chunks, with typed private
frames cleared at action entry to retain locals across calls. Deletion and Chai
first-match dispatch use bounded helpers with explicit early-return propagation.
The parts adapter discovers their leaf branches and initializes RTTI locals in
each helper scope. Both archive forms receive these changes before packaging.

To compare with an earlier build and inspect every emitted test class:

```sh
node tools/a4h-runner-split-test.mjs "$BEFORE_BUILD" "$BUILD" "$PARTS"
node tools/a4h-tests-parts.mjs "$BUILD" "$PARTS" --verify
```

The pinned 17-shard closure measures **66,994 → 3,963 lines** for the largest
method across the monolithic and seven-part test archives. RUN itself is now
**152 lines**; the largest runner helper is **192 lines**. Syntax checks report
zero issues for the monolithic install set (7,340 files) and reconstructed parts
(12,383 source/XML files), using the established v755 PCRE exception. Parts
verification checks 5,023 objects with zero dependency violations; both install
sets pass archive hashes and unique ownership checks.

The first check recursively inlines the monolithic and base-part runners and
compares the complete original RUN statement sequence, reversing only the
explicit local-to-attribute declaration/reset plumbing. It also reopens all eight
test ZIPs and asserts a 5,000-line method ceiling. The ordinary unit gates cover
4,962 ordered registration boundaries, unusual shard names, cross-chunk locals,
and dispatch order/return propagation.

## Run and resume (coordinator, after deployment)

Schedule `ZLNT_TESTS_RUN` as a **background report**, not ABAP Unit. Selection:

| Parameter | Meaning |
| --- | --- |
| `P_SHARD` | Exact manifest selector, e.g. `abap/lexer`, `abap/types`, `utils` |
| `P_START` | 0 = read durable next index; positive = explicit start index |
| `P_RUN` | Caller-chosen unique 32-character run ID, reused only for resume |

Use a new run ID for each clean rerun. Never run two jobs concurrently with the
same ID. A run ID is bound to its shard; changing shards is rejected.

There is no `ZCL_ABAPITI_LOG` in this generated closure, so the zip includes a
small independent table/log service. Each checkpoint is split into 255-character
rows (`SEQ`, `CHUNK_NO`, `USED`, `PAYLOAD`) and committed **before the test body**.
`NEXT_IDX` already points to the next test when a dump occurs. Completed tests
emit an indexed result checkpoint; the final report is also written in chunks.
`ZLNT_TESTRUN` is the only source of truth. The tests base layer has no SAP BAL
types or function modules. Optional BAL/SLG1 progress requires importing the
`abaplint-577f875e-a4h-registry.zip` archive, which owns `ZCL_LNT_TEST_LOG_BAL`.
The logger calls its `BEGIN` and `EVENT` methods dynamically and tolerates an
absent adapter. SLG1 is a progress mirror and is never used for resume or collection.

The checkpoint TABL uses the corpus tables' standard-data-element DD03P layout:
`MANDT`, `CHAR32`, `INT4`, `CHAR40`, and `CHAR255`, with `COMPTYPE E`.
The DDIC regression check resolves those elements with abaplint and compares
all eight field types/lengths with the previous primitive definitions. Because
the old `COMPTYPE E` had no `ROLLNAME`, the baseline parses its explicit
`DATATYPE`/`LENG` as built-in fields. Field order, keys and nullability stay unchanged.

Checkpoint commits are intended for these in-memory tests, not arbitrary
transactional application code. Retain logs until collection is verified.

After a dump, inspect ST22 and record its short text against the interrupted
local test index. Restart with the same run ID and `P_START=0`: initialization
replays, completed tests are not rerun, and the interrupted test is represented
as a trap, never a pass. Initialization dumps require coordinator investigation;
there is no automatic kernel-side initialization-skip planner. Explicit backward
replay into the same ID produces duplicate results and is rejected by collection.

## Fetch and collect offline

Fetch **all rows of one run ID** through the coordinator's approved mechanism.
Export a JSON array of column objects; preserve payload padding (or at least
`USED` characters), `SEQ` and `CHUNK_NO`.
`PART` is a DDIC reserved word; the generated table and collector use
`CHUNK_NO` instead. The collector sorts rows, validates chunks,
and reconstructs checkpoint JSON. It also accepts a raw runner JSON object or
an array of decoded checkpoint events.

```sh
node tools/a4h-tests-collect.mjs fetched.json go-abap-lexer.json abap/lexer st22.json
```

Optional `st22.json`: `{"201":"FIND PCRE: ...kernel short text..."}`. Without
ST22 text, an interrupted test remains an explicitly worded trap. An incomplete
job without a final completion event is refused; resume it before collecting.
When collecting resumed jobs, use the indexed checkpoint results, not only the
final report (which contains only that invocation's suffix).

Put `go-<id>.json` beside the original `node-<id>.json`, `shards.json`, and
`<id>/closure.json` from the original manifests. Then run the existing tools
unchanged:

```sh
node tools/abaplint-tests-report.mjs node-abap-lexer.json go-abap-lexer.json report-abap-lexer.json
node tools/abaplint-tests-summary.mjs "$RESULTS" summary.json
```

The collector preserves status and `passed` exactly. `category: "pcre-drift"`
is an **additional** tag for failed/trapped results with explicit FIND/REPLACE
PCRE provenance or kernel regex exception identity. `categories["pcre-drift"]`
and `initializationCategories["pcre-drift"]` give separate counts. A regex test
title or ordinary assertion mismatch is not sufficient provenance. No PCRE
failure becomes a pass or disappears from existing FAIL/trap accounting; the
unchanged summary ignores extra categories, so keep the collected JSON too.

## OSGO preparation checks

```sh
node tools/a4h-tests-osg-check.mjs "$ABAPLINT_HOME" "$RESULTS"
```

This focused tool uses the original harness on a readable three-shard closure,
compares against independently executed Node-active cases (including borrowed
registration ownership), and fails on any nonpassing active case or genuine
FAIL. It does not activate the A4H report/table or claim kernel PCRE parity.
The default hashed-name harness and its checkpoint/resume ABI remain unchanged.

For the full-core zabapgit kit check (copying the kit's data, not editing it):

```sh
node tools/a4h-zabapgit-check.mjs "$BUILD" "$OSG_HOME" "$ZABAPGIT_KIT" "$CHECK"
```

This uses the existing OSG `osabap.mjs` and kit `check.sh`, with
`OSABAP_BUILD_ROOT` outside OSG. A temporary Go launcher injects `-p 1` and
`-gcflags='osg/gogen/cmd/osabap=-N -l -dwarf=false'`; all other arguments pass
through. This is the harness's memory-friendly compile mode, not a rewrite of
ABAP or a changed verdict. Use identical flags on the pruned baseline when
comparing runtimes. The launcher is deleted even on failure.

## Stage-1 verification (2026-10-14)

Full union: **2,167 TS files** including the added deployment harness; zero
blocking diagnostics, zero HIR errors. Actual package-source lint: **7,340 files,
zero issues**, with the existing PCRE v755 exception. Object counts below exclude
each package's DEVC, which is also covered by the manifest.

| Artifact | Objects | Bytes | MiB |
| --- | ---: | ---: | ---: |
| `zlnt.zip` | 2,338 | 11,019,421 | 10.51 |
| `zlnt_tests.zip` | 5,002 | 22,771,762 | 21.72 |

Readable OSGO: **lexer 200/200, types 29/29, utils 15/15**, Node-active only;
FAIL 0, traps 0, registration parity intact. A separate v750-clean 24-file
extension fixture passes **1/1 on OSGO**, exercising core construction with and
without an installed test provider, test constructors (both `new` and
`new_args`), and provider-backed deletion.

The existing zabapgit kit matches Node byte-for-byte: clean **0 issues**, seeded
**5 issues**. Same memory-friendly Go compiler flags on both builds:

| Native build | Clean (s) | Seeded (s) | Combined (s) |
| --- | ---: | ---: | ---: |
| Pruned hashed baseline | 62 | 63 | 125 |
| Full readable core | 68 | 66 | 134 |

That pair is **7.2% slower combined** (+9 s), with one-second kit timer precision;
not a controlled repeated benchmark or a prediction of kernel time. The original
optimized baseline was 45/44 s, but the full optimized Go compilation was
OOM-killed at the 48 GiB scope limit. The bounded full-core Go build completed in
89.12 s at 12.09 GiB peak process RSS. Both comparable runtime measurements use
`-N -l -dwarf=false`, not an optimized baseline against an unoptimized full build.

Gates: `go vet ./...`; the requested short Go suite; all **17**
`tools/*-test.mjs` scripts individually; lint against `origin/main` with **0 new
issues**, canary 5/5, golangci-lint 2.13.2; mandatory `hir-unit.sh` with
`HIR_RUNTIMES=" "`, **exit 0**. For the standard short Go suite, optional OSG
execution is disabled by unsetting `ABAPITI_OSG_HOME`; the explicit OSG checks
above retain the pinned host environment.

An additional opt-in run of the broad short Go suite with OSG enabled encountered
`TestParsedNestedObjectABAPHost` and `TestLSPErasedInNodeOracle` failures. Running
those same two tests from an `origin/main` source archive reproduced both failures
on the same OSG pin. These are recorded baseline optional-host fixture failures,
not silently waived new regressions; the regular short and required CI gates are
green. The default hashed runner/checkpoint ABI remains unchanged.

Kernel activation time, PXA/memory limits, database table activation and actual
PCRE2 behavior remain coordinator-side gates; no A4H access is part of this step.
