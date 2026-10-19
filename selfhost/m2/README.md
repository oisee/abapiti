# Self-hosting M2: emitted ZLNT core as the workload

The fixed-point target is abapiti's translated abaplint checking the ABAP emitted for abaplint itself, with output equal to Node abaplint on exactly the same inputs. This kit freezes that workload, builds the Node oracle and probes TS-HG@Go. The measured outcome and remaining limits are recorded below.

Frozen inputs:

- Compiler/emitter: `oisee/abapiti` **`953eae439f7fe5d39fcf6a4f4d8f4fb8d6a262ab`**, the initial HEAD of `feat/selfhost-m2-workload` and fetched `origin/selfhost`. `prepare.mjs` runs the release builder from a `git archive` of this SHA. Later inventory commits do not move this pin.
- abaplint: **`577f875ebec44cfaf64841cfe71c8ab8dc32622e`**, `@abaplint/core` **2.120.56**. Node compiles the original source afresh using its locked TypeScript **6.0.3**, through `tools/statements-upstream.mjs`. The compiler uses the same pinned embedded core for its TS-HG checker.
- Release-shaped emission: `tools/release-build.sh` calls `tools/a4h-tests-build.mjs`, which materializes the complete 17-shard test union and emits core/tests together with `abapiti abaplint-tests --target a4h`. This preserves their shared names/layouts. The workload comes from the resulting **`zlnt.zip`**, corresponding to release asset **`abaplint-577f875e-a4h.zip`**. Core-only `abapiti abaplint --target a4h` would be a different build scope. The release's shorthand “2,338 classes” is 2,338 source objects: 2,251 CLAS, 85 INTF and two PROG; the DEVC metadata adds one Registry object.
- Comparison asset: [v0.5.1 core ZIP](https://github.com/oisee/abapiti/releases/download/v0.5.1/abaplint-577f875e-a4h.zip), SHA-256 `bba0fbc463ac8aa77860ec3dfe95e6274bc816e14804ca723ce62f7ee0f1f910`. Its bytes are used only for comparison. The fresh workload is emitted locally from the frozen compiler.
- M1 template: `origin/feat/selfhost-m1-workload` at **`4649b6b90a04b95ed01d1f5b777f9dab8d9ed042`**, specifically its README, prepare/reference/verify/package scripts, workload schema and config. The initial branch has no `selfhost/m1/`; those files were read from that feature ref without merging it. The frozen compiler and `origin/selfhost` also predate the M1 translator fixes and #167's multi-file CLI. M2 uses M1's `go-multifile` host approach.
- Original kit contract: [v0.5.1 zabapgit-check-kit.zip](https://github.com/oisee/abapiti/releases/download/v0.5.1/zabapgit-check-kit.zip), SHA-256 `2be51f82cfc1947416faef522c21b52c485e94f43bc6ea65d40fb8e9fb421de8`. Its six rules and generated-object skip flags are retained.

Config is **`v755`, errorNamespace `.*`**, matching `tools/a4h-tests-lint.mjs` at the compiler pin. That release gate explicitly permits PCRE, which requires 7.55, while the remaining emitted constructs use 7.50-style syntax. Thus neither open-abap nor an assumed v758 is the release lint setting. M2 enables the M1 kit's unknown_types, allowed_object_naming, check_syntax, implement_methods, superclass_final and parser_error rules. `global.files` selects the synthetic tree and duplicate filenames are errors. There are no dependencies: this measures the standalone core archive. The release's two-rule lint gate additionally loads open-abap-core and corpus-abap dependencies; its clean result is a different contract.

Every core ZIP member has an entry in workload.json. UTF-8 normalization strips one leading BOM, changes CRLF/CR to LF and preserves final-newline presence, exactly as M1. Object ABAP/XML members, including `src/package.devc.xml`, are main/checkable inputs. `.abapgit.xml` is retained and hashed as archive payload; it is not a Registry object. The Registry's filename-derived DEVC identity is `DEVC PACKAGE`. The test package is built only to reproduce release emission, and contributes no lint inputs. No source objects are pruned to obtain a cleaner result. The inherited generated-object skip flags can still exempt generated objects from rule checks.

The oracle runs the same unchanged `RegistryRun` TypeScript harness as M1. `expected-clean.txt` preserves its issue count, issue order and final LF for byte comparison. `issues-sorted.txt` contains every issue sorted by its pipe-delimited record. The canonical whole hash is M1's `sha256(JSON.stringify(objects) + LF)`: objects sorted by type/name, issues sorted by compact JSON using UTF-16 lexical order, including objects with zero issues. Each issue retains rule, severity, relative filename, 1-based start/end and complete message. `node-reference.json` records both dump hashes and counts by rule/object. If a full reference exceeds 4 MiB, it stays under `.tmp/` with its hash recorded in the tracked summary and is included in the kit.

The TS-HG probe uses full core/all rules and M1's generated `main.go` I/O adapter: `--file files.txt` registers every listed file via `addFile` and retains its relative path. All other generated sources are verified byte-identical before and after rebuilding the adapter. No translator, HIR, checker or runtime changes are made. Each native probe has a 20-minute timeout. `go-probe.mjs` records refusals, incomplete runs or differences and exits successfully after recording them; the kit's `check.sh` itself requires a successful checker run and byte equality. The unadapted CLI accepts one main file; passing other objects as dependencies would change which objects are checked.

The fresh core ZIP is **byte-identical to v0.5.1**, SHA-256 `bba0fbc463ac8aa77860ec3dfe95e6274bc816e14804ca723ce62f7ee0f1f910`. All archive members also match byte-for-byte: zero additions, removals or changed contents. Post-release commits changed CI, nullish-assignment lowering and partial-HIR number-range handling; the measured core output at this pin/scope remains identical. `release-comparison.json` records both archive hashes. Paired emission took **590.416 s**, including the archived compiler build; its 2,167-file lowering gate has zero blocking diagnostics, HIR errors and coverage traps.

The core is **4,678 files / 61,898,658 normalized bytes**, with **4,677 main Registry files / 2,339 objects**:

| Type | Objects | Files |
|---|---:|---:|
| CLAS | 2,251 | 4,502 |
| INTF | 85 | 170 |
| PROG | 2 | 4 |
| DEVC | 1 | 1 |
| Archive metadata | 0 | 1 |

The excluded monolithic tests ZIP has **10,005 members**, **4,994 classes**, six interfaces, one report and a table: **22,543,490 compressed / 117,797,655 unpacked bytes**. The release splitter adds 21 routing helper classes; its seven packages contain **5,015 classes**, **22,226,406 compressed / 123,494,805 unpacked bytes** in total. `excluded-tests.json` records each archive hash and size; the splitter independently verified its manifests and zero cross-part independence violations. These are alternative test distributions, not additional M2 inputs.

| Excluded test part | ZIP bytes | Classes |
|---|---:|---:|
| base | 685,803 | 83 |
| statements | 2,656,531 | 568 |
| rules_1 | 3,864,348 | 1,097 |
| rules_2 | 3,528,640 | 817 |
| syntax | 6,359,827 | 1,394 |
| objects | 2,943,083 | 486 |
| rest | 2,188,174 | 570 |

Node reports **20,179 issues in 1,406 objects**, with **zero parser errors**:

| Rule | Issues |
|---|---:|
| unknown_types | 222 |
| allowed_object_naming | 0 |
| check_syntax | 19,946 |
| implement_methods | 0 |
| superclass_final | 11 |
| parser_error | 0 |

The largest group is 16,036 `CATCH, unknown class CX_SY_MOVE_CAST_ERROR` diagnostics. The standalone core omits SAP/runtime definitions such as CX_ROOT, CX_NO_CHECK, converter classes and MATCH_RESULT; their absence also causes cascaded generated-exception/type diagnostics. All 11 superclass_final diagnostics say `Super class "cx_no_check" not found`. This inventory preserves those results rather than declaring the emitted source clean or treating them as demonstrated compiler bugs.

Top objects (all per-object counts are in node-reference.json):

| Object | Issues |
|---|---:|
| CLAS ZLNT_RUNTIME_CONF_5B63794ECCEF | 846 |
| CLAS ZCL_LNT_SELECT_CORE_MODULE | 364 |
| CLAS ZCL_LNT_DOWNPORT | 210 |
| CLAS ZCL_LNT_ST_SOURCE | 208 |
| CLAS ZCL_LNT_METHOD_DEF | 198 |
| CLAS ZCL_LNT_SQL_AGGREGATION | 196 |
| CLAS ZCL_LNT_DDIC | 192 |
| CLAS ZCL_LNT_DERIVED_TYPES_MODULE | 190 |
| CLAS ZCL_LNT_CDS_EXTEND_VIEW | 186 |
| CLAS ZCL_LNT_ST_SELECTION_SCREEN | 176 |

Node fresh source compilation took **9.988 s**; file loading **0.135 s**; parse/check/report **63.672 s**; process peak RSS **4066.5 MiB**. Stage timings are in node-reference.json and are machine-specific observations. The full reference is **7,272,233 bytes** under `.tmp/selfhost-m2/node-reference-full.json`, with its hash in the tracked summary. The ordered dump is **2,376,160 bytes**. Every full diagnostic is available in both the ordered and sorted dumps and the packaged full JSON.

Canonical whole SHA-256: **`be3771e2dd549e3732642bb525174e16cb09be60a96a0131d5701cf33fc12b71`**. Ordered dump SHA-256 before final LF: `99d7b40fc9e71b68280ab8e3c8ed7678e19eac57f62ac0c5249f67ccd72d2964`; actual expected-clean.txt SHA-256: `6a54bd9e3353589b01b669595e011414c0e91651382c6b0568ef8b8a48de2d53`. Sorted issue dump SHA-256: `67b2f1fb7056689b3fc0c5cd9b1164eded6e7992cee234ad99ca19fb8bd9f764`.

TS-HG generation succeeded: **2,055 classes, zero blocking diagnostics and zero HIR verification errors**. Generation took **78.411 s**, the original native build **48.1 s**, and the multi-file adapter build **44.985 s**. All **7 generated files other than main.go/binary** were checked unchanged before and after the adapter build; original/adapted host hashes are recorded in go-probe.json.

The full **4,677-file** native probe ran for **1113.804 s (18 min 33.8 s)**, then received **SIGKILL from the 32 GiB cgroup OOM limit**. The run used `GOMAXPROCS=8`, a **24 GiB soft Go memory target** (`GOMEMLIMIT=24GiB`), and a 20-minute timeout. The cgroup's `oom_kill` counter rose from 0 to 1 and peak charged memory reached **34,359,742,464 bytes**, at its **34,359,738,368-byte** limit. A Go memory target is soft and did not prevent the cgroup kill. This was **before the timeout**. The checker emitted **zero stdout/stderr bytes**, including no source-located refusal or issue dump.

**Native issue count, byte equality and per-issue differences remain unknown.** `go-probe.json` records an incomplete run (`cgroup_oom`), the signal, timing, empty-output hashes, and cgroup evidence. No issue mismatches are invented from the absent dump. TS-HA@OSGO and SAP-kernel checker execution were not run in this step. **The fixed-point equality goal is not established.** Completing the same frozen workload on a capable host/runtime remains future work; translator changes are outside this commit.

Validation passed: all 4,678 archive member hashes, all 2,789 frozen compiler files, the clean pinned oracle checkout, config/manifest/full-result/canonical hashes, and an independent fresh-source `Registry.parse()` / `findIssues()` run reproducing **all 20,179 issues**. The native OOM run is preserved as measured evidence. Extracted-kit validation checks bytes and shell syntax; the memory-exhausting native run is not repeated. Script syntax, JSON size/path limits and staged whitespace checks also pass. Tracked JSON files are each below 1.4 MB.

Reproduce from the repository root with Git, Python 3, curl, tar, Node/npm and Go **1.26.0** on PATH (the release toolchain). This run used Node **26.9.0**. The build is offline with respect to SAP. The initial paired emission and native checker builds take several minutes; allow several GiB of RAM. Run heavy jobs within the environment's normal admission/locking policy; this workspace has a 32 GiB cgroup ceiling.

```sh
mkdir -p .tmp/selfhost-m2/downloads
curl -L --fail https://github.com/oisee/abapiti/releases/download/v0.5.1/abaplint-577f875e-a4h.zip -o .tmp/selfhost-m2/downloads/v0.5.1-core.zip
git clone https://github.com/abaplint/abaplint .tmp/selfhost-m2/abaplint
git -C .tmp/selfhost-m2/abaplint fetch origin 577f875ebec44cfaf64841cfe71c8ab8dc32622e
git -C .tmp/selfhost-m2/abaplint checkout --detach 577f875ebec44cfaf64841cfe71c8ab8dc32622e
npm ci --ignore-scripts --no-audit --no-fund --prefix .tmp/selfhost-m2/abaplint
npm ci --ignore-scripts --no-audit --no-fund --prefix .tmp/selfhost-m2/abaplint/packages/core
node selfhost/m2/prepare.mjs
NODE_OPTIONS=--max-old-space-size=24576 node selfhost/m2/reference.mjs
NODE_OPTIONS=--max-old-space-size=24576 node selfhost/m2/verify.mjs
GOFLAGS=-buildvcs=false CGO_ENABLED=0 GOMAXPROCS=8 go -C .tmp/selfhost-m2/compiler build -p 2 -trimpath -o ../abapiti ./cmd/abapiti
GOFLAGS=-buildvcs=false CGO_ENABLED=0 GOMAXPROCS=8 .tmp/selfhost-m2/abapiti abaplint --target go --evidence -o .tmp/selfhost-m2/go-build > .tmp/selfhost-m2/go-build.log 2>&1
GOMAXPROCS=8 GOMEMLIMIT=24GiB node selfhost/m2/go-probe.mjs
python3 selfhost/m2/package.py
python3 -m zipfile -e .tmp/selfhost-m2/zlnt-check-kit.zip .tmp/selfhost-m2/zip-check
GOMAXPROCS=8 GOMEMLIMIT=24GiB node selfhost/m2/go-probe.mjs --kit-verify
python3 selfhost/m2/package.py
```

The recorded extracted-kit validation verifies every workload file, file list, empty dependencies, config/oracle hashes and runner shell syntax. To explicitly rerun the full native checker from the extracted ZIP, replace `--kit-verify` with `--kit-runner`; that is a separate potentially expensive probe.

For a fresh emission, remove only `.tmp/selfhost-m2/compiler` and `.tmp/selfhost-m2/a4h` before rerunning prepare.mjs; the clean pinned upstream checkout can be reused. For repeated packaging, the explicit member list excludes execution logs and actual output. Timings and their containing JSON/ZIP hashes change on reruns; workload and canonical issue hashes should remain stable. `kit.json` records the final ZIP size, hash, member count and verification result. The ZIP contains the synthetic tree, files.txt, empty deps.txt, config, full expected/sorted dumps, available licenses, inventory/results and check.sh. It has one original `clean` variant, meaning unmodified input, regardless of its issue count. Clones, emitted ABAP, binaries, raw logs and ZIP stay untracked. The local commit carries the requested identity/trailers; no push is part of this task.
