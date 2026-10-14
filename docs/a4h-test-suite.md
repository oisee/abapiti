# abaplint test suite: A4H build and offline collection

This is **phase 2, stage 1: build only**. None of these tools connect to SAP,
activate objects, create jobs, or fetch database rows. Deployment, activation,
background jobs and ST22 inspection belong to the coordinator.

## Build the two packages

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
pruning or assume-integer contract. A single emission/name table serves both
packages; shared structural shapes may originate at either a core or test site.

Deploy **`zlnt.zip` to `$ZLNT` first**, then **`zlnt_tests.zip` to `$ZLNT_TESTS`**:

* `$ZLNT`: the full translated core and its shared runtime, including the
  deployment harness used by the zabapgit client. No executable test
  registrations, test lambdas or mocha/chai classes belong here.
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
* `names.json` maps ABAP names to TS identities and relative file:line:column.
  Synthetic runtime objects have no original TS declaration; generated wrappers
  identify their generator. Readable declaration names use `ZCL_LNT_`,
  `ZIF_LNT_`, `ZCX_LNT_`; shared synthetic helpers retain the readable emitter's
  deterministic `ZLNT_` fallback. All names are bounded by 30 characters.

`core/` is the exact shared source set. `native/zabaplint.prog.abap` is an OSGO
client for that set, not another translated core. `native/lib/` contains the
same translated core bytes, excluding only the generated SAP corpus driver
and its two reports. The SAP corpus driver in `$ZLNT` expects the existing
`ZCL_ABAPITI_CORPUS`, `ZIF_ABAPITI_CORPUS`, `ZABAPITI_CORPUS` and
`ZCL_ABAPITI_LOG` services from `tools/allbackends/corpus-abap`; these services
are not copied into either new package. `abap/` is the combined test
fixture before replacing the empty OSG checkpoint hook with durable A4H logging.
Do not deploy `abap/` separately or overwrite the old hashed-name package.

The generated ABAP uses the project's 7.50-style syntax and 255-byte line
formatter. **FIND/REPLACE PCRE itself requires 7.55**; A4H 7.58 supports it. The
existing CI lint gate uses v750 for non-PCRE fixtures and the narrow v755 PCRE
exception. Calling PCRE genuine v750 syntax would be incorrect.

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
rows (`SEQ`, `PART`, `USED`, `PAYLOAD`) and committed **before the test body**.
`NEXT_IDX` already points to the next test when a dump occurs. Completed tests
emit an indexed result checkpoint; the final report is also written in chunks.
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
`USED` characters), `SEQ` and `PART`. The collector sorts rows, validates chunks,
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
