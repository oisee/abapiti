# Rename records on TS-HA@OSGO

Branch `osgo-rename` starts at `origin/integ/shinkansen` **83e9ca4**.
Upstream abaplint is **577f875ebec44cfaf64841cfe71c8ab8dc32622e**;
OSG is the unchanged CI pin **b32d0f6049ec0a3a32a2b5a561fd673fcaf44974**.
The measurement selects all nine `test/objects/rename/rename_*.ts` files.
It certifies this 31-test shard, not the complete 11,060-test ABAP suite.

| Host / step | Pass | FAIL | Traps | Registered | Node active |
| --- | ---: | ---: | ---: | ---: | ---: |
| OSGO before, diagnostic-only emitter | 0 | 0 | 31 | 31 | 31 |
| OSGO after factory presence fix | 31 | 0 | 0 | 31 | 31 |
| Go after fix | 31 | 0 | 0 | 31 | 31 |

Both OSGO runs use the original suite harness and independent, freshly compiled
Node oracle, with exact title reconciliation and no missing, pending, skipped,
unmatched or initialization-failure titles. No assertion was changed.

## Root and implementation

Every `view.refusal` now requires a reason and overrides `get_text` to expose
it. Reasons identify the emitter site and the refused representation or
operation; the structural slot guard additionally names the class and operation.
The detailed baseline reports the same reason for all 31 bodies:

```text
own_keys.go: get node_modules/vscode-languageserver-types/lib/umd/main.d.ts.RenameFile: structural own-key metadata absent
```

The registry-map deletion guess was not the first blocker. `RenameFile.create`
produces a native structural record without `OwnKeys`, because options and
annotations have conditional presence. `TextDocumentEdit.is` first reads that
record through the generic storage interface. Its getter correctly refuses
without presence metadata, before the rename can proceed.

The frontend now preserves checked factory provenance in `New.Op`, separate
from the static `OwnKeys` proof. The ABAP emitter validates the pinned factory
layout, then installs `kind`, `oldUri`, `newUri` in Node's creation order. It adds
`options` only when `overwrite` or `ignoreIfExists` is defined, preserving the
short-circuit read order and including false flags. Empty options are omitted
from the result. A defined annotation is appended, including the empty string.
Options retain their original object identity. The two nested `Position` records
created by four-number `Range.create` also receive their known `line,character`
proof. No Go emitter or runtime code changed.

Ordinary structural `New` expressions without a proof retain the explicit
refusal. No declaration-order fallback or initial-value substitute was added.
The live record-map `has` and `delete` refusals remain, with specific reasons;
the successful rename run does not require removing them. Undefined writes to
required map values, unsupported unbox ABIs, unknown native layouts, prototype
operations and unsupported integer representations remain guarded. No entry
was removed from `unsupportedOps`.

## Static closure and runtime refusals

Both full gap JSON files are identical. The closure has **625 roots / 17 shards /
2,166 files / 7,052 classes / 85 interfaces**, with blocking diagnostics, HIR
errors and coverage traps all zero.

| Counter inventory | Before | After |
| --- | ---: | ---: |
| Unsupported runtime sites, full closure | 0 | 0 |
| Rename-related emitter refusal sites | 0 | 0 |
| Other emitter refusal sites (`json.stringifySubset` / `AbstractToken`) | 1 | 1 |
| Nonfinite literal sites | 0 | 0 |
| Rename bodies reaching a view refusal (execution, not static sites) | 31 | 0 |

The catalogue retains `string.parseFloat` as unsupported with zero occurrences.
The one remaining emitter refusal is outside this task's JSON ownership and
remains explicit. Compact, path-independent evidence is in
[osgo-rename.json](evidence/osgo-rename.json).

The counter from `docs/osgo-suite-gap.md` on `origin/feat/osgo-suite-m0` is used
unchanged: runtime occurrences before ABAP passes, distinct emitter diagnostics
after passes, `ABAPITI_INLINE=classic`, 625 roots in 17 shards / 2,166 files.
Generated runtime `RAISE` statements are not that counter's emitter diagnostics.
The missing-metadata guard remains emitted; the checked constructions now
supply its proof. Executed refusal bodies fall from 31 to zero.

The executed rename closure contains 1,549 files, zero blocking diagnostics,
zero HIR errors, zero unsupported runtime sites and one unrelated JSON emitter
refusal in `dumpDDLS`. That refusal is not reached by these 31 bodies. The
baseline emission has 49 string-key record-map specializations containing 49
`has` and 49 `delete` runtime raises; their implementation is unchanged. These 98 generated raises are not added to the HIR-site census.

## Validation

`TestLSPFactoryPresence` uses `emitABAPNamed` and `storageHostFixture`, executing
its emitted ABAP through pinned gogen and the OSG kernel. It checks absent/empty
options, both false flags, empty/present annotations, key order, option identity,
nested-position storage and refusal of an unproven native construction. Its
independent oracle uses installed **vscode-languageserver-types 3.18.0**.
`TestViewRefusalDiagnostics` executes the exact reason text through the same
host and checks generated raises for missing reasons. The frontend's existing
`TestLSPConstructors` also passes.

Passed `go vet ./...`, `go test -short -p 2 ./...`,
`ABAPITI_GRACECHECK=1 go test -p 2 -count=1 -run TestLexerFactsReport ./tsfront/`,
`TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend` with `-p 2 -count=1`,
and `./.github/ci/lint.sh gate origin/main`: **0 new issues, canary 5/5**,
golangci-lint **2.13.2**. The lexer facts golden is unchanged; no regeneration
was needed. The final OSG envelope is **2,366 files**, at most **255 bytes per
line**, zero overlong lines, and at most **29 bytes per object name**.

Default zabapgit Go output was generated at the same output path with the
requested-base and candidate compilers: **raw `diff -r` exit 0, zero bytes,
9 files including the built executable and `sites.json`**. The rename patch
therefore changes no default Go output.

**The requested `origin/main` comparison is not empty.** Main is `fd77046`;
the requested Shinkansen base already contains later Go sparse-array changes.
The inherited difference affects `go/hir.go`, `go/hir_profile.go`,
`go/runtime.go` and `go/zabaplint-go`: sized-array construction, hole tracking,
and its associated array helpers. This same difference exists before the
rename fix. Those Go files were not edited or reverted by this task. All
other generated output matches main. The origin/main parity gate therefore remains unmet for an inherited reason;
its byte comparison is not credited as passed.

All heavy builds are serial within this task; package parallelism is at most
two. The standard final OSGO run disables the optional frontend cache. A detailed
intermediate diagnostic retained too much cached frontend state to satisfy the
20 GiB free-memory admission gate on the 31 GiB machine; its unchanged generated
Go was built after releasing that process and executed with the existing
`osg-runner-execute.mjs`. It again reported all 31 specific RenameFile refusals.
This intermediate diagnostic is separate from the standard final harness run.

## Reproduction

Use clean pinned checkouts and caller-owned `UPSTREAM`, `OSG_HOME`, and `WORK`
directories. Run from the repository root:

```sh
export GOTOOLCHAIN=go1.26.0 GOMAXPROCS=2
export GOFLAGS='-p=2 -buildvcs=false -gcflags=all=-dwarf=false "-ldflags=-s -w"'
export NODE_OPTIONS=--max-old-space-size=24576
export ABAPITI_INLINE=classic ABAPITI_OSG_HOME="$OSG_HOME"
export ABAPITI_TESTS_FILES="$(node --input-type=module - "$UPSTREAM" <<'JS'
import {discoverShards} from './tools/abaplint-tests-all.mjs';
const files = discoverShards(process.argv[2]).files;
console.log(files.filter(f => /^test\/objects\/rename\/rename_.*\.ts$/.test(f)).join(','));
JS
)"
ABAPITI_TESTS_TARGET=osg node tools/abaplint-tests-all.mjs "$UPSTREAM" "$WORK/osg.json"
ABAPITI_TESTS_TARGET=go node tools/abaplint-tests-all.mjs "$UPSTREAM" "$WORK/go.json"
```

For static counts, materialize `discoverShards(UPSTREAM).files` with the existing
`materializeRegistryClosure` helper and the repository's test runtime, then run
baseline and candidate compilers serially:

```sh
GOMEMLIMIT=20GiB ABAPITI_INLINE=classic "$COMPILER" abaplint-tests "$CLOSURE" \
  --target osg --gap-only -o "$OUT"
```
