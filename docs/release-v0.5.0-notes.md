🎉 **100 % of abaplint's own test suite passes on the ABAP host: TS-HA@OSGO.**

All **11,060 / 11,060** Node-active tests of [abaplint](https://github.com/abaplint/abaplint)'s pinned core pass after TypeScript → HIR → ABAP → open-steamgate Go: **FAIL 0, traps 0, unregistered 0**. Certified on the **express train #149** (`8ad792f`, 14 Oct 2026), all 17 shards, OSG **`f83dc5c3`**. **TS-HG@Go still passes 11,060 / 11,060** on that same train. The 32 tests Node itself skips (`it.skip`) remain outside the denominator; neither missing registrations nor host dumps count as passes.

```
TS-HA@OSGO: 521 → 677 → 5,329 → 10,767 → 10,996 → 11,037 → 11,060
```

## What changed since v0.4.0

**The user-facing build now includes all of core and all rules.** v0.4.0's test build was unpruned, but its default CLI and downloadable clients still used the six-rule zabapgit trace. That could surprise a user enabling another rule with `zlnt_exception_un_…`. In v0.5.0:

```sh
abapiti abaplint ~/src/abaplint -o out --target abap
abapiti abaplint ~/src/abaplint -o out --target osg
abapiti abaplint ~/src/abaplint -o out --target go
# Explicit smaller/faster legacy build:
abapiti abaplint ~/src/abaplint -o small --scope zabapgit
```

No reachability pruning in the default build; readable ABAP identities and binary64 numbers. `--full` remains a diagnostic-only measurement command, not a prerequisite for emitting all rules. Unsupported operations still retain source-located refusals rather than guessed answers.

The ABAP backend gained the bounded operations that closed the host gap: checked JSON/JSON5 and XML, structural and nominal class views, inherited presence/deletion and own-key order, constructor descriptors/factories, boxed optional and nonfinite numbers, checked string operations, and Chai observations over **hole-aware arrays**. Regex fixes preserve JavaScript's absolute anchors, dot and whitespace semantics on the POSIX path, and native PCRE word boundaries on non-UCP hosts. Registration of `keyword_case` is repaired. Every genuine mismatch was stopped, root-caused and retained as a regression against Node.

The OSG harness lowers/builds a runtime-selected union once and replays each shard's original initialization plan. The measured five-shard run fell from **34m12s to 7m19s** (4.7×), not a claim that the full suite takes seven minutes. Default grouping is three shards, with automatic build-failure splitting. Checkpoint/resume survives initialization and test-body host dumps without losing earlier FAILs; reconciliation now compares **raw title multisets**, so duplicates and unregistered tests cannot disappear from accounting.

New offline A4H packaging emits a full readable **`$ZLNT`** core plus **`$ZLNT_TESTS`**, with one shared name/shape table, manifest hashes and a durable background test report. Core has no reverse static dependency on the test package. Build and collection tools do not connect to SAP.

The test suite is also available as **base + six independent parts**, with verified object coverage, archive hashes and zero cross-part static dependencies. Import **core → base → the six parts in parallel**, and install all parts before running. Choose the parts or the monolithic tests zip, never both: their object names overlap. Durable logging uses **`ZLNT_TESTRUN-CHUNK_NO`**, replacing `PART`, a DDIC reserved word that A4H refuses.

Every emitted source location is relative to the lowering root, including refusal/trap messages, HIR spans, test-registration metadata and `names.json`, across ABAP/A4H, OSG and Go. A two-absolute-root unit regression requires byte-identical output. The release build now scans **every asset**, decompressed zip/tar entries, archive metadata and binary `strings -a` output; any build-root, local mount/home/temp prefix or build-user identifier token blocks checksum generation. Identifier boundaries avoid mistaking the unrelated CSS color `aliceblue` for a local user identity; path-prefix matches have no exemptions. To keep all assets free of those prefixes, the corpus loader now requires an explicit `P_PATH` instead of a hard-coded temp default, and two embedded declaration comments use the current ISO currency-code documentation URL (no checker/declaration semantics changed).

## Scope, honestly

- The 100 % is the pinned original suite on **Go and OSGO**, not general TypeScript support or every possible input. The standalone full build still contains explicit refusals on unsupported paths; all rules are included, not restricted to six trace paths.
- **A4H (a real SAP kernel, 7.58): 11,055 / 11,060.** These exact archives are imported into A4H: the core, base and all six parts. The mass syntax check is **0 errors** across all 2,338 core classes, 5,015 test classes and 94 interfaces/programs. Two kernel-only refusals were found and fixed on the way: long `WHEN` keys, and conditional `super->constructor` plus empty `CASE`. The zabapgit check through `ZLNT_REGISTRY_CLEAN` matches Node (Node-hash equal; parse 376 s). Test shards run as background jobs via `ZLNT_TESTS_RUN`, with an SLG1 log per run. **All 17 shards have run on A4H: 11,055 / 11,060 Node-active tests pass, with FAIL 0 and unregistered 0.** The 5 others are traps where the kernel refuses a regex (`CX_SY_INVALID_REGEX`). They were reported separately as planned and are now root-caused. They are not PCRE drift: the kernel refuses an **empty** POSIX pattern, which JavaScript's `new RegExp("")` accepts. The fix is #155, which maps an empty pattern to `(?:)`. With that fix deployed (a 2-class delta), the three affected shards were rerun: **A4H reaches 11,060 / 11,060, with FAIL 0, traps 0 and unregistered 0.** **v0.5.1** ships those archives. Every result is reconciled title-by-title against the same Node oracle as OSGO. PCRE requires ABAP 7.55; calling it 7.50 syntax would be incorrect.
- Upstream remains **`577f875e` (`@abaplint/core` 2.120.56)**. The newer core is the next train, not silently substituted into this release.
- Full-core native OSG clients use the bounded `-N -l -dwarf=false` report compile mode. Their size/runtime is not directly comparable with the old optimized pruned downloads.

## Reproduce

Use this repository, Go 1.26.0, Node, a clean abaplint checkout at exactly `577f875e` with its locked npm dependencies installed, and a clean OSG checkout matching `.github/ci/osgo.ref` (`f83dc5c3`). No checkout argument is needed for ordinary CLI builds: pinned core source and declarations are built into `abapiti`, without network access.

For release preparation, with caller-owned scratch and the existing check kit:

```sh
# Set the pinned toolchain/cache environment; admit heavy work at load <12.
flock "$OSG_HEAVY_LOCK" systemd-run --user --scope -p MemoryMax=48G nice -n 10 \
  bash tools/release-build.sh "$ABAPLINT_HOME" "$OSG_HOME" \
  "$KIT_ZIP" "$ASSETS" "$BUILD"
```

This builds the six-platform clients, both A4H packages and the independent tests parts, checks the actual package-source syntax, verifies split-object coverage and dependency independence, compares the zabapgit kit and three extra rules with fresh pinned Node, bundles downloads, runs the per-asset path gate, and generates `SHA256SUMS`. See [the A4H guide](https://github.com/oisee/abapiti/blob/main/docs/a4h-test-suite.md), [harness measurements](https://github.com/oisee/abapiti/blob/main/docs/osg-harness-speed.md) and [checkpoint accounting](https://github.com/oisee/abapiti/blob/main/docs/osg-runner-resume.md).

## Release smoke checks

The full-default clients and the full readable release clients match Node byte-for-byte on zabapgit: **clean 0 / seeded 5**, on **Go and OSGO**. Fresh original pinned Node was rebuilt, not inferred from the old binary. A small sample also gives the exact same issue dumps for three rules outside the old six: **`keyword_case` 2, `line_only_punc` 1, `sequential_blank` 1**. Both produced binaries exit successfully; no `zlnt_exception_un_` traps.

The complete rebuilt download set has **47 assets** (including `SHA256SUMS`).
The final path gate reports **zero leaked entries for each asset**, including all
zip/tar members, binary strings, package manifests, `names.json` and `parts.json`.
`SHA256SUMS` was regenerated and all **46 covered downloads** verify. See the
[per-asset size/path-gate inventory](release-v0.5.0-assets.tsv). The two-root unit
regression passes for all source targets, including A4H's readable-name mode.

Mandatory CI HIR lint (exit 0), vet, the regular short Go suite, all 19 individual Node tool-test scripts, and lint against main (0 new) are green. The regular short suite disables optional host execution. With `ABAPITI_OSG_HOME` enabled, the exact broad short command fails two pre-existing optional host fixtures (`TestParsedNestedObjectABAPHost`, `TestLSPErasedInNodeOracle`); both were reproduced independently from an `origin/main` source archive on the same pin. This is a recorded baseline gate gap, not a passing run or an omitted new failure.

## Downloads

As in v0.4.0, now with **full core / all rules**:
- `abapiti`, `zabaplint-go` (TS-HG@Go), and `zabaplint` (TS-HA@OSGO), each for Linux/macOS/Windows × amd64/arm64;
- six `abaplint-abap-<platform>` bundles and `zabapgit-check-kit.zip`;
- `abaplint-577f875e-a4h.zip`: full readable `$ZLNT` core;
- **new** `abaplint-577f875e-a4h-tests.zip`: `$ZLNT_TESTS` (install core first), retained for compatibility;
- **new** `abaplint-577f875e-a4h-tests-<part>.zip`: `base`, `statements`, `rules_1`, `rules_2`, `syntax`, `objects`, `rest`, with companion manifests and **`parts.json`** describing published zip names, hashes, assignments and prerequisites;
- companion package manifests and `abaplint-577f875e-names.json` for diff deployment and tracing;
- the small `abapiti-corpus-a4h.zip` repository (same corpus services; loader path now explicit) and `SHA256SUMS`.

Import the new readable packages into their named packages; do not overwrite or mix them with the old hashed `$ZABAPLINT` package. For split imports, install `abaplint-577f875e-a4h.zip` first, then `abaplint-577f875e-a4h-tests-base.zip`, then the six remaining parts in parallel into their own named packages/offline repositories. Do not also import the monolith. The test zip is not needed to run the standalone zabapgit client. Checksums cover every download except `SHA256SUMS` itself.
