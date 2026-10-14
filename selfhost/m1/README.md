# Self-hosting M1: frozen OSGo workload and Node inventory

The target is abapiti’s translated abaplint running as **TS-HA@OSGO**, checking OSG’s own ABAP with the same results as Node. This milestone freezes the input and oracle. The optional TS-HG@Go probe refuses this workload; no translator fixes or TS-HA parity claim are included.

Frozen inputs:

- OSG: `oisee/open-steamgate` at `3ee404ca1260464221f5eba7d86a06a38eabc6c4`, source-clean in `.osg`. Generated sources and fetched packs live in an archive copy at `.tmp/selfhost/osg`.
- Node abaplint: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`, `@abaplint/core` **2.120.56**. The OSG package-lock dependencies are used only for source generation; its abaplint 2.120.73 is not the reference.
- Check-kit shape and six-rule set: [v0.5.1 zabapgit-check-kit.zip](https://github.com/oisee/abapiti/releases/download/v0.5.1/zabapgit-check-kit.zip), archive SHA-256 `2be51f82cfc1947416faef522c21b52c485e94f43bc6ea65d40fb8e9fb421de8`. `tools/release-build.sh` consumes this existing kit; the original kit builder is not tracked. The fresh Node build and unchanged `RegistryRun` harness follow `tools/hir-go-check.mjs` and `tools/statements-upstream.mjs`.
- Compiler for the optional probe: `6ed0e12230c7e491c32f04775d23f6bdff6a374e`, default full `core` scope; go version go1.26.9 linux/amd64.

Library clones follow **libs.lock.json**, including its `oisee/open-abap-core` fork rather than the URL in abap_transpile.json:

| Folder | Repository | Ref |
|---|---|---|
| open-abap-core | oisee/open-abap-core | `84125c41c22e4c0d45e8037e8b60c691bb64de8d` |
| express-icf-shim | open-abap/express-icf-shim | `c1fc36026f69eb0ae7d1830d304e1e1923e9a9a3` |
| open-abap-apc | oisee/open-abap-apc | `bcf43c75b1c3b83ddee24298b2d0cba4637e15cd` |
| abapgit | abapGit/abapGit | `3b6485b5d0b09ef3861006b966e68542446721ce` |
| open-abap-gui | oisee/open-abap-gui | `b65c4431d71c7b23468d08bd955b18785fd2ca6c` |
| open-abap-odata | oisee/open-abap-odata | `bd9f1fb175e7b26678e48eb2e311a278a13ef91b` |
| ajson | sbcgua/ajson | `e5e0fd043f5813ef5a271fcba4b327aa944c3224` |

The workload uses `layers()` after removing `test`, exactly as `tools/gogen/osg-build.mjs` does. Later folders own the entire object. All six discovered default packs are enabled: **o4d, zork, gui-examples, lsd, zvdb, travels-a4h**; travels-a4h contributes no ABAP objects. Pack upstream folders are fetched at their manifest pins before resolving layers. No OSD_PACKS, OSD_WEB_PACKS, OSD_LAYERS or workstation $TMP overlays are used. Pack repositories, refs and filters are recorded in workload.json.

The build’s generator sequence creates `gen` before layer resolution, including AMDP rewrites and converted GUI reports. Three source objects are hidden by later layers. Input testclasses, DDLS XML (CDS has generated DDIC), cockpit web files and folder-owned DEVC metadata are excluded from the check. ABAP and XML object members are retained; binary media and other non-registry payloads are outside this lint workload. Source bytes are checked as stored, before OSGo’s in-memory BOOLX/SUBMIT rewrites.

Libraries use the explicitly requested `libraryFiles(abap_transpile.json)` selection: configured globs and exclude_filter, with testclasses removed. OSGo’s current wrapper has broader hand-written library directories; this manifest deliberately follows libraryFiles as the milestone contract. Input filenames override library copies (`if_amdp_marker_hdb.intf.abap` here). All remaining library objects are main/checkable workload inputs, with **no dependencies**. Synthetic paths retain input folder paths and place libraries under `libs/<folder>/...`.

Config uses OSG’s actual `abaplint.jsonc` syntax settings: `open-abap`, error namespace `.`. The kit’s rule set and generated-object skip flags are retained: unknown_types, allowed_object_naming, check_syntax, implement_methods, superclass_final, parser_error. Thus an object with an empty issues array can also be skipped by the inherited generated-object policy.

**2,785 files, 2,210 objects**, matching Node’s 2,210 registry objects. Counts below count unique object identities, not just executable classes. workload.json records every selected file’s layer, object type/name and normalized SHA-256. UTF-8 normalization strips one initial BOM and changes CRLF/CR to LF, preserving final-newline presence.

| Object type | Objects | Files |
|---|---:|---:|
| CLAS | 1023 | 1402 |
| DOMA | 50 | 50 |
| DTEL | 206 | 206 |
| ENQU | 1 | 1 |
| FUGR | 13 | 109 |
| INTF | 290 | 367 |
| IWMO | 16 | 16 |
| IWPR | 1 | 1 |
| IWSV | 16 | 16 |
| MSAG | 2 | 2 |
| PROG | 17 | 31 |
| SAMC | 1 | 1 |
| SAPC | 4 | 4 |
| SHLP | 2 | 2 |
| SICF | 18 | 18 |
| SUSH | 1 | 1 |
| TABL | 259 | 259 |
| TRAN | 21 | 21 |
| TTYP | 83 | 83 |
| TYPE | 18 | 18 |
| VIEW | 36 | 36 |
| W3MI | 130 | 138 |
| WAPA | 1 | 1 |
| XSLT | 1 | 2 |

| Layer | Objects | Files |
|---|---:|---:|
| gen | 410 | 485 |
| lib:abapgit | 47 | 95 |
| lib:ajson | 7 | 15 |
| lib:express-icf-shim | 5 | 6 |
| lib:open-abap-apc | 46 | 46 |
| lib:open-abap-core | 623 | 680 |
| lib:open-abap-gui | 350 | 358 |
| lib:open-abap-odata | 111 | 114 |
| packs/gui-examples/upstream | 3 | 3 |
| packs/lsd/src | 8 | 11 |
| packs/o4d/src | 1 | 2 |
| packs/o4d/upstream | 115 | 198 |
| packs/zork/src | 5 | 5 |
| packs/zork/upstream | 12 | 24 |
| packs/zvdb/src | 5 | 9 |
| src | 462 | 734 |
| packs/travels-a4h | 0 | 0 |

Node reports **80 issues**. Fresh source build: 10.358 s; loading: 0.051 s; parse/check: 6.960 s; process peak RSS: 726.9 MiB. Node v26.9.0, locked TypeScript 6.0.3; timings are observations on this machine. Detailed stages are in node-reference.json.

| Rule | Issues |
|---|---:|---:|
| allowed_object_naming | 69 |
| unknown_types | 6 |
| check_syntax | 5 |
| parser_error | 0 |
| implement_methods | 0 |
| superclass_final | 0 |

Top objects (ties continue in node-reference.json):

| Object | Issues |
|---|---:|
| CLAS ZCL_OSD_GUI_OSD_JOB_E2E | 4 |
| INTF IF_ALV_MESSAGE | 3 |
| CLAS ZCL_OSD_GUI_OSD_JOB_DUMP | 2 |
| CLAS ZCL_OSD_GUI_OSD_JOB_TICK | 2 |
| INTF ZIF_GG_LIST_PROCESSING_TYPES_V1 | 1 |
| INTF ZIF_GG_SELECTION_SCREEN_BUILDER_V1 | 1 |
| IWMO ZOSD_TEST_MDL                     0001 | 1 |
| IWMO ZSTG_SADL_MDL                     0001 | 1 |
| TABL ZOSD_JOB_IDENTITY | 1 |
| TRAN ZCL_OSD_GUITX_GG_EX_001 | 1 |

Normalized whole SHA-256: **`a8885a1c61e4008bcd339378e344efa23199fdb6175eacf31fcc7ceac917e489`**. The whole is `JSON.stringify(objects) + LF`: objects sorted by type/name, issue records sorted by their compact JSON representation using UTF-16 lexical order. Fields retain rule, severity, relative file, 1-based start/end positions and full message. Objects with zero issues are included. The kit’s original ordered pipe-delimited dump has SHA-256 `e473332a8eea22ef7ec52ec1a0f791860ef418ac9567ceaf7f7419f8f525029c` (before its final output LF).

Go generation succeeded with zero blocking diagnostics and zero HIR verification errors. Generation took 76.663 s and its original binary build 47.000 s. The existing CLI supports one main `--file`; listing the remaining workload as `--deps` checks only that main object and is not full parity. That run exited 1 after 55.575 s. A separate generated host loader registered all files as main files and preserved synthetic paths; only main.go’s I/O was adapted, with no checker, HIR, runtime or translator changes. Its build took 49.469 s and its full run exited 1 after 52.578 s.

The extracted ZIP runner also exited 1 after 50.741 s, with all extracted file hashes verified. All three runs emit the same refusal and no issue dump:

```text
refused: src/ddic.ts:192:3: value outside supported numeric, ordering or conversion domain
```

Node completes with 80 issues; Go completes no comparison. Its issue count and per-issue differences are therefore unknown, rather than zero issues or 80 mismatches. go-probe.json records exit status, output hashes, timings and the refusal. The refusal names the declaration of `DDIC.lookupNoVoid`; the underlying numeric/ordering/conversion operation has not been diagnosed in this inventory.

Reproduce from the abapiti repository root (Git, Python 3, tar, curl, Node/npm and Go required; this run used Node 26.9.0 and Go 1.26.9):

```sh
mkdir -p .tmp/selfhost
curl -L --fail https://github.com/oisee/abapiti/releases/download/v0.5.1/zabapgit-check-kit.zip -o .tmp/zabapgit-check-kit.zip
python3 -m zipfile -e .tmp/zabapgit-check-kit.zip .tmp/kit
git clone https://github.com/abaplint/abaplint .tmp/abaplint
git -C .tmp/abaplint fetch origin 577f875ebec44cfaf64841cfe71c8ab8dc32622e
git -C .tmp/abaplint checkout --detach 577f875ebec44cfaf64841cfe71c8ab8dc32622e
(cd .tmp/abaplint && npm ci --ignore-scripts --no-audit --no-fund)
npm ci --ignore-scripts --no-audit --no-fund --prefix .tmp/abaplint/packages/core
node selfhost/m1/prepare.mjs
node selfhost/m1/reference.mjs
node selfhost/m1/verify.mjs
go build -o .tmp/selfhost/abapiti ./cmd/abapiti
.tmp/selfhost/abapiti abaplint --target go -o .tmp/selfhost/go-build > .tmp/selfhost/go-build.log 2>&1
node selfhost/m1/go-probe.mjs
python3 selfhost/m1/package.py
python3 -m zipfile -e .tmp/selfhost/osg-check-kit.zip .tmp/selfhost/zip-check
node selfhost/m1/go-probe.mjs --kit-runner
python3 selfhost/m1/package.py
```

The extracted kit runner currently exits 1 with the recorded refusal; go-probe.mjs records that expected failure and returns successfully. Repackaging incorporates that runner evidence. For a fresh preparation repeat, remove only `.tmp/selfhost/osg` before rerunning prepare.mjs; frozen clones can be reused. Environment selectors for external packs/layers should be unset. Timings and the containing reference JSON/ZIP hashes change on reruns; the normalized workload and issue hashes are stable.

Validation passed: all 2,785 file hashes, all eight frozen OSG/library checkout pins and source cleanliness, config/manifest hashes, the normalized result hash, and an independent fresh-source `Registry.parse()`/`findIssues()` comparison against the kit harness. A second generator run plus fresh Node build reproduced identical workload bytes and normalized issue hash. Script syntax and git whitespace checks passed. The package script verifies ZIP CRCs and every workload member hash.

The check kit is local at `.tmp/selfhost/osg-check-kit.zip`; kit.json records its size, SHA-256 and member count. It contains the synthetic tree, files.txt, empty deps.txt, config, expected-clean.txt, manifests/results, available root licenses and check.sh. It has one frozen clean variant; no artificial seeded variant was introduced. Original pack licensing remains in its source repositories. Clones, generated ABAP, binaries, raw execution logs and the ZIP remain untracked. Tracked JSON files are each below 1 MB. No push or deployment is part of this milestone.
