# 2026-10-15: abaplint's whole test suite on three runtimes

abaplint, the ABAP linter written in TypeScript, translated by abapiti
(TypeScript → tsgo → object HIR → ABAP or Go), passes its own upstream test
suite on a Go host, on open-steamgate's Go runtime and on a real SAP kernel:
**11,060 / 11,060** active tests on each.

## Input

- abaplint at `577f875e` (`@abaplint/core` 2.120.56), the oisee/abaplint
  fork, byte-identical to upstream for every translated file. The closure is
  1,538 core files plus the test files of the 17 shards below.
- The tests are abaplint's own mocha/chai tests, compiled together with the
  core through a small mocha/chai shim, never rewritten by hand.
- The reference is a Node run of the same tests at the same pin: 11,092
  registered, of which 32 are upstream `it.skip` (outside the denominator).
  Each result is reconciled title by title against that oracle.

## Results

| Host | What runs | Version | Pass / 11,060 | FAIL | Traps |
|---|---|---|---:|---:|---:|
| TS-HG@Go | abaplint translated to Go | abapiti `8ad792f` (#149), re-verified on later trains | **11,060** | 0 | 0 |
| TS-HA@OSGO | abaplint translated to ABAP, run by open-steamgate's Go runtime | OSG `f83dc5c3`, abapiti `8ad792f` (#149) | **11,060** | 0 | 0 |
| TS-HA@A4H | the same ABAP on SAP NetWeaver AS ABAP 7.58 (A4H) | abapiti v0.5.0 archives + #155 (`18cc014`), = v0.5.1 logic | **11,060** | 0 | 0 |

Per shard (identical on all three hosts):

| Shard | Tests | Shard | Tests |
|---|---:|---|---:|
| abap/statements | 4,962 | cds | 358 |
| rules | 3,149 | abap | 216 |
| abap/syntax | 1,363 | abap/lexer | 200 |
| lsp | 188 | abap/structures | 139 |
| objects | 144 | abap/expressions | 107 |
| root | 94 | ddl | 32 |
| objects/rename | 31 | pretty_printer | 31 |
| abap/types | 29 | utils | 15 |
| abap/nodes | 2 | **total** | **11,060** |

The last five A4H traps were the kernel refusing an empty POSIX regex
(`FIND REGEX ''`), which JavaScript and open-steamgate accept; #155 maps an
empty pattern to `(?:)`.

## How to repeat it

From the tag `v0.5.2` (https://github.com/oisee/abapiti/releases/tag/v0.5.2;
the translation is unchanged since v0.5.1, only the A4H archive packaging):

1. **Go host.** Check out abapiti at the tag and abaplint at `577f875e`
   (`npm ci` at the abaplint root and in `packages/core`), then
   `node tools/abaplint-tests-all.mjs <abaplint> out.json`. It compiles every
   shard to Go, runs it and prints the per-shard table against the Node
   oracle.
2. **open-steamgate.** Same command with
   `ABAPITI_TESTS_TARGET=osg ABAPITI_OSG_HOME=<open-steamgate at f83dc5c3, after npm ci and node tools/osd-libs.mjs>`.
3. **A real kernel.** Import the release archives with abapGit in order:
   `abaplint-577f875e-a4h.zip` (core), then
   `abaplint-577f875e-a4h-tests-base.zip` and the six test parts. Run report
   `ZLNT_TESTS_RUN` as a background job once per shard (`P_SHARD` = shard name
   in lower case, `P_RUN` = a run id); results land in table `ZLNT_TESTRUN`.
   Reconcile them with `tools/a4h-tests-collect.mjs` and
   `tools/abaplint-tests-report.mjs` against the Node oracle.

The same system is published as one OCI index
`ghcr.io/oisee/abapiti/zlnt-system:v0.5.2` (digest
`sha256:f33816c89e5e23d305a952a1e0524d7dedc688e281387fe85015626ea62e1ebd`),
which open-steamgate installs with `osd-fetch --system`.
