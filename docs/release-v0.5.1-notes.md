🎉 **abaplint's whole test suite passes on a real SAP kernel: TS-HA@A4H 11,060 / 11,060.**

v0.5.1 is v0.5.0 plus one kernel fix, and these are the archives that reach 100 % on A4H as shipped.

| Host | Shards | Tests passing | Wrong answers | Traps |
|---|---:|---:|---:|---:|
| TS-HG@Go | 17 | **11,060** / 11,060 | 0 | 0 |
| TS-HA@OSGO | 17 | **11,060** / 11,060 | 0 | 0 |
| **TS-HA@A4H** (SAP 7.58) | 17 | **11,060** / 11,060 | 0 | 0 |

## What changed since v0.5.0

- **Empty regular expressions are kernel-safe (#155).** A real kernel refuses an empty POSIX pattern (`FIND REGEX ''` raises `CX_SY_INVALID_REGEX`). JavaScript's `new RegExp("")` and open-steamgate accept it. The regexp runtime now maps an empty pattern to `(?:)`. This was the cause of the last 5 A4H traps: empty naming patterns in `class_attribute_names` / `unknown_types`, a CI-include parse, and two renames.
- **A4H test runner (since v0.5.0):** `ZLNT_TESTS_RUN` takes the shard name in lower case (`P_SHARD`) and a run id (`P_RUN`). It writes `ZLNT_TESTRUN` checkpoints and an SLG1 log, `ZABAPITI/CHECK`, with the external id `TESTS <run> <shard>`.
- **OCI artifacts:** the eight A4H archives are published to `ghcr.io/oisee/abapiti/abaplint-a4h`. There is one layer per archive (`core-v0.5.1`, `tests-base-v0.5.1`, `tests-<part>-v0.5.1`), each blob byte-identical to its release asset, plus a system index `v0.5.1` in import order.

## How A4H was certified

The readable `$ZLNT` core (2,338 classes) and the `$ZLNT_TESTS_*` packages (5,015 test classes) are imported with abapGit. The syntax check gives 0 errors. Each of the 17 shards runs as a background job. Results are reconciled title by title against the pinned Node oracle (upstream `577f875e`), and the reconciliation was done twice, independently. Accounting note: the `abap/syntax` and `lsp` shards also register the 6 `sql_value_conversion` tests of `rules`. That makes 11,072 executions for 11,060 Node-active titles; the 12 are duplicates, not extra passes.

Downloads, import order and everything else are as in [v0.5.0](https://github.com/oisee/abapiti/releases/tag/v0.5.0).
