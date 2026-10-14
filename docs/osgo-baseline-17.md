# Full 17-shard TS-HA@OSGO baseline

Run A: main `428a8d5acbd4e3be6331b01d5d9e0afc71df0dc4` with OSG `f557e12f4217addcec7834b0607234aecf9b723a`. Run B: `origin/integ/o2` `72ada516c4fb5918ce9a6162a699014ed6b0ce7d` with OSG `ea4522861d75493067d7483adde559bd60323ecd` (character-set `sy-fdpos` fix, #719). No local feature merges or product changes; no push.

Both runs use upstream abaplint `577f875ebec44cfaf64841cfe71c8ab8dc32622e` and transpiler/runtime `ee5c2cd1a45aaac6be503a46fc903ce95a946877`. Fresh whole-suite Node oracle: **11,060 passed, zero failed, 32 pending**. All 625 roots partition into 17 shards; whole-suite and owned-title multisets reconcile.

[Machine-readable evidence](evidence/osgo-baseline-17.json) retains all active outcomes, normalized trap groups, exact unregistered titles, initialization refusals, static inventories, gates, host timings, and report hashes.

Totals below use the **11,060 Node-active tests**; 32 pending tests are outside the denominator. `infra` counts active tests without a trustworthy report, and affected outcome cells read **unmeasured**. An initialization refusal in a returned report may leave active titles unregistered; this differs from a shard infrastructure failure.

## Run A

**521 passes; 0 FAIL; 9,808 traps; 373 unregistered; 358 infrastructure-unmeasured active tests in 1 shard.**

| Shard | Total | Pass | FAIL | Trap | Unregistered | Infra |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| abap | 216 | 2 | 0 | 49 | 165 | 0 |
| abap/expressions | 107 | 1 | 0 | 18 | 88 | 0 |
| abap/lexer | 200 | 85 | 0 | 115 | 0 | 0 |
| abap/nodes | 2 | 0 | 0 | 2 | 0 | 0 |
| abap/statements | 4,962 | 433 | 0 | 4,529 | 0 | 0 |
| abap/structures | 139 | 0 | 0 | 139 | 0 | 0 |
| abap/syntax | 1,363 | 0 | 0 | 1,363 | 0 | 0 |
| abap/types | 29 | 0 | 0 | 29 | 0 | 0 |
| root | 94 | 0 | 0 | 94 | 0 | 0 |
| cds | 358 | unmeasured | unmeasured | unmeasured | unmeasured | 358 |
| ddl | 32 | 0 | 0 | 32 | 0 | 0 |
| lsp | 188 | 0 | 0 | 188 | 0 | 0 |
| objects | 144 | 0 | 0 | 144 | 0 | 0 |
| objects/rename | 31 | 0 | 0 | 31 | 0 | 0 |
| pretty_printer | 31 | 0 | 0 | 2 | 29 | 0 |
| rules | 3,149 | 0 | 0 | 3,064 | 85 | 0 |
| utils | 15 | 0 | 0 | 9 | 6 | 0 |
| **TOTAL** | 11,060 | 521 | 0 | 9,808 | 373 | 358 |

### Top 20 trap groups

Each trapped test counts once in its normalized composite-message group. Repeated reasons in one refused body are collapsed; source identities do not split a group. Counts are measured test impact, not operation executions or static sites.

| Rank | Normalized message | Tests | Example source site | Zone |
| ---: | --- | ---: | --- | --- |
| 1 | runtime operation chai.box has no ABAP emitter; runtime operation chai.deepEqual has no ABAP emitter | 7,544 | `test/abap/abap_parser.ts:43:5` (diagnostic) | abapiti |
| 2 | runtime operation array.has has no ABAP emitter; runtime operation chai.box has no ABAP emitter; runtime operation chai.deepEqual has no ABAP emitter | 1,321 | `test/rules/_utils.ts:44:9` (diagnostic) | abapiti + dell |
| 3 | runtime operation chai.arrayInclude has no ABAP emitter; runtime operation chai.box has no ABAP emitter; runtime operation chai.deepEqual has no ABAP emitter; runtime operation chai.deepInclude has no ABAP emitter | 364 | `test/abap/macros.ts:95:5` (diagnostic) | abapiti |
| 4 | runtime operation json.stringifySubset has no ABAP emitter | 273 | `test/objects/data_element.ts:18:25` (diagnostic) | abapiti |
| 5 | runtime operation chai.arrayInclude has no ABAP emitter; runtime operation chai.box has no ABAP emitter; runtime operation chai.deepInclude has no ABAP emitter | 173 | `test/abap/native_sql_colon.ts:74:5` (diagnostic) | abapiti |
| 6 | runtime operation json5.parse has no ABAP emitter | 97 | `src/config.ts:99:31` (diagnostic) | abapiti |
| 7 | runtime operation chai.box has no ABAP emitter; runtime operation chai.deepEqual has no ABAP emitter; runtime operation json.stringifySubset has no ABAP emitter | 17 | `test/config.ts:40:44` (diagnostic) | abapiti |
| 8 | runtime operation chai.throwMatch has no ABAP emitter | 9 | `test/registry.ts:115:5` (diagnostic) | abapiti |
| 9 | runtime operation chai.box has no ABAP emitter; runtime operation chai.deepEqual has no ABAP emitter; runtime operation chai.members has no ABAP emitter | 3 | `test/abap/syntax/basic_variables.ts:1917:5` (diagnostic) | abapiti |
| 10 | An exception was raised. | 2 | `test/artifacts_objects.ts:9:3` (test-registration) | dell (provisional) |
| 11 | runtime operation chai.arrayInclude has no ABAP emitter; runtime operation chai.box has no ABAP emitter; runtime operation chai.deepEqual has no ABAP emitter; runtime operation chai.deepInclude has no ABAP emitter; runtime operation json.stringifySubset has no ABAP emitter | 1 | `test/lsp/hover.ts:1630:44` (diagnostic) | abapiti |
| 12 | runtime operation chai.arrayInclude has no ABAP emitter; runtime operation chai.box has no ABAP emitter; runtime operation chai.deepInclude has no ABAP emitter; runtime operation json.stringifySubset has no ABAP emitter | 1 | `test/objects/table.ts:960:45` (diagnostic) | abapiti |
| 13 | runtime operation chai.box has no ABAP emitter; runtime operation chai.deepEqual has no ABAP emitter; runtime operation chai.throwMatch has no ABAP emitter | 1 | `test/registry.ts:130:5` (diagnostic) | abapiti |
| 14 | runtime operation chai.box has no ABAP emitter; runtime operation chai.deepEqual has no ABAP emitter; runtime operation dynamic.delete has no ABAP emitter | 1 | `test/registry.ts:460:12` (diagnostic) | abapiti + dell |
| 15 | runtime operation chai.box has no ABAP emitter; runtime operation chai.deepEqual has no ABAP emitter; runtime operation json.indent has no ABAP emitter; runtime operation json.stringifySubset has no ABAP emitter | 1 | `test/config.ts:73:39` (diagnostic) | abapiti |

Only 15 normalized groups were observed; no extra groups are invented.

### Infrastructure failures

- `cds`: OSG harness refused: NOT_COMPILED in FIND PCRE: an escape RE2 reads differently or not at all is not in Go's RE2: \xA0

No genuine FAIL was observed in returned reports; infrastructure-unmeasured tests carry no FAIL-zero claim.

## Run B

**677 passes; 0 FAIL; 5,193 traps; 288 unregistered; 4,902 infrastructure-unmeasured active tests in 4 shards.**

| Shard | Total | Pass | FAIL | Trap | Unregistered | Infra |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| abap | 216 | 4 | 0 | 47 | 165 | 0 |
| abap/expressions | 107 | 2 | 0 | 17 | 88 | 0 |
| abap/lexer | 200 | 200 | 0 | 0 | 0 | 0 |
| abap/nodes | 2 | 2 | 0 | 0 | 0 | 0 |
| abap/statements | 4,962 | 429 | 0 | 4,533 | 0 | 0 |
| abap/structures | 139 | 25 | 0 | 114 | 0 | 0 |
| abap/syntax | 1,363 | unmeasured | unmeasured | unmeasured | unmeasured | 1,363 |
| abap/types | 29 | 0 | 0 | 29 | 0 | 0 |
| root | 94 | 8 | 0 | 86 | 0 | 0 |
| cds | 358 | unmeasured | unmeasured | unmeasured | unmeasured | 358 |
| ddl | 32 | unmeasured | unmeasured | unmeasured | unmeasured | 32 |
| lsp | 188 | 0 | 0 | 188 | 0 | 0 |
| objects | 144 | 0 | 0 | 144 | 0 | 0 |
| objects/rename | 31 | 0 | 0 | 31 | 0 | 0 |
| pretty_printer | 31 | 0 | 0 | 2 | 29 | 0 |
| rules | 3,149 | unmeasured | unmeasured | unmeasured | unmeasured | 3,149 |
| utils | 15 | 7 | 0 | 2 | 6 | 0 |
| **TOTAL** | 11,060 | 677 | 0 | 5,193 | 288 | 4,902 |

### Top 20 trap groups

Each trapped test counts once in its normalized composite-message group. Repeated reasons in one refused body are collapsed; source identities do not split a group. Counts are measured test impact, not operation executions or static sites.

| Rank | Normalized message | Tests | Example source site | Zone |
| ---: | --- | ---: | --- | --- |
| 1 | An exception was raised. | 5,161 | `test/abap/abap_parser.ts:19:3` (test-registration) | dell (provisional) |
| 2 | runtime operation json.stringifySubset has no ABAP emitter | 26 | `test/config.ts:40:44` (diagnostic) | abapiti |
| 3 | dynamic boxing of i64 is not supported | 4 | `test/abap/types/message.ts:6:3` (test-registration) | dell |
| 4 | runtime operation dynamic.delete has no ABAP emitter | 1 | `test/registry.ts:460:12` (diagnostic) | dell |
| 5 | runtime operation json.indent has no ABAP emitter; runtime operation json.stringifySubset has no ABAP emitter | 1 | `test/config.ts:73:39` (diagnostic) | abapiti |

Only 5 normalized groups were observed; no extra groups are invented.

### Infrastructure failures

- `abap/syntax`: FATAL ERROR: Ineffective mark-compacts near heap limit Allocation failed - JavaScript heap out of memory
- `cds`: OSG harness refused: NOT_COMPILED in FIND PCRE: an escape RE2 reads differently or not at all is not in Go's RE2: \xA0
- `ddl`: OSG harness refused: NOT_COMPILED in FIND PCRE: an escape RE2 reads differently or not at all is not in Go's RE2: \xA0
- `rules`: FATAL ERROR: Ineffective mark-compacts near heap limit Allocation failed - JavaScript heap out of memory

No genuine FAIL was observed in returned reports; infrastructure-unmeasured tests carry no FAIL-zero claim.

## Reproduction and limits

Measurement environment: `GOCACHE=/work/.cache/go-build`, `TMPDIR=/work/.cache/tmp`, `NODE_OPTIONS=--max-old-space-size=32768`, `GOMAXPROCS=6`, `GOTOOLCHAIN=go1.26.0`, `GOFLAGS=-buildvcs=false`, `ABAPITI_TESTS_TARGET=osg`, one shard worker per run. Both runs execute concurrently in isolated source/OSG checkouts. Pinned npm dependencies and all OSG locked ABAP libraries are installed; OSG and upstream sources remain clean.

The orchestration is a scratch copy of `tools/abaplint-tests-all.mjs`: it logs genuine FAILs and continues, saves raw reports before ownership filtering, retains compact artifacts after summary, and adds the OSG frontend refusal inventory. A scratch host bridge validates the exact supplied OSG commit so B can use the requested fix rather than its older checked-in pin. Compiler, emitter, runtime, assertions, and upstream tests are unchanged. Exact script hashes and changes are in the JSON metadata.

Run A initially hit the missing locked-library setup gate for `abap` and `abap/expressions`; these attempts had no runner JSON and are retained as infrastructure failures in `setupAttempts`. After syncing the required pins, those two shards were retried. The rules shard was also retried in isolation after B finished, because a concurrent generated-Go compile hit the container memory limit; that original failure is retained in `infrastructureAttempts`. The final A table uses the replacement reports alongside the remaining shard reports, reconciled against the independent full-suite oracle. This is a complete frozen-source map assembled from one all-shard attempt plus two setup retries and one isolated rules retry.

Generic `An exception was raised.` diagnostics retain their own group. The report does not expose their native exception class or failing operation; their dell assignment is a provisional runtime/storage handoff, not a proven root cause. A source site marked test-registration is the original test location, not the failure location. Other groups use the stated operation ownership zones.

The comparison changes both the abapiti integration and the OSG host; pass differences are not attributed to a single feature. Host phase timings include concurrent execution and are not serial performance comparisons.

No fixes, weakened assertions, exclusions, synthesized registrations, or extrapolated test passes are included.
