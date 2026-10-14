# Syntax dynamic-array casts and absent scalar indices

Baseline compiler: `55008d96733e8b2b3d1372ffceee8e52c5ac1c8a`.
Upstream abaplint: `577f875ebec44cfaf64841cfe71c8ab8dc32622e`.
OSGO: `16c024c8` (the repository's unchanged pin).

## Baseline and grouping

`test/abap/syntax/syntax.ts`: **1070 pass / 0 FAIL / 67 traps** out of
**1137 Node-active**, plus 10 Node-pending tests outside that denominator.
Per-test titles, normalized messages and source groups are in
[`evidence/syntax-move-cast.json`](evidence/syntax-move-cast.json).

All **65 MOVE_CAST_ERRORs** group at:

- TypeScript: `src/xml_utils.ts`, `xmlToArray`, the `return data` after
  `Array.isArray(data)`;
- ABAP: `z_src_xml_util_1a864ff5911e43.clas.abap`, method
  `z_member_xmlto_57d3e1fca40c8a`, **line 63**, `t13 ?= t15.`;
- host function:
  `Z_SRC_XML_UTIL_1A864FF5911E43_Z_MEMBER_XMLTO_57D3E1FCA40C8A`;
- dynamic source: `runtime.dynamic`; target: `runtime.array<dynamic>`.

The baseline kept run was `all-Utjxlq`. Its `go-abap-syntax.json` contained
owned per-test results; `shared-0-out/abap` contains emitted ABAP and
`names.json`; `shared-0-out/osg/go` contains the external OSG runtime copy
and generated host source. To obtain locations without changing OSG, only
that external copy's failed `Cast` diagnostic was enriched with dynamic
source/target types and the caller. Cast success/failure behavior was unchanged.
The giant generated Go file's reported line saturates at 1048574; use the
function and emitted ABAP statement, not that host line alone.

## Root cause and repair

The HIR is `dynamic.asRef` with result `array<dynamic>`. In ABAP, a parsed
array is a tagged graph node: `as_ref()` returns that node itself. It is
**not** an instance of the separately emitted typed-table wrapper. The
subsequent checked cast is therefore correctly rejected by OSG.

The Go parser instead stores an actual `*array[any]` inside the dynamic
array value. `dynRef` returns that array, so `castRef[*array[any]]` succeeds
and preserves its identity. See `hir/golang/parsed_array_test.go`.

The ABAP emitter now activates live-array storage for array-valued
`dynamic.asRef` and uses the existing `arrayView` boundary. A graph-backed
array gets a wrapper bound to its original graph; a boxed native array
retains its native wrapper. Reads refresh and writes publish to the same
storage. This is not a snapshot, nominal-class substitution, exception
swallowing, or an OSG workaround.

`TestDynamicArrayRefUsesLiveStorage` uses `emitABAPNamed`, a Node oracle,
and an optional pinned OSG unit execution. It checks alias identity,
`Array.isArray`, writes in both directions and native-array identity,
without another class/array-view operation activating the ABI accidentally.
Pinned OSG result: **1 SUCCESS row**.

## The two range traps

A separate diagnostic run (`ranges.json`) enriched only the external
runtime copy's explicitly raised range exceptions with stacks. The roots
are required scalar `IndexGet` reads, not substring offsets:

| Test | Method | Observation |
| --- | --- | --- |
| instantiating global interface, error | `StatementParser.nativeAfter` | `words[words.length - 2] === "ENDEXEC"` on a short statement |
| FILTER with infer field | `BasicTypes.lookupQualifiedName` | truthiness of `split[2]` on a two-part qualified name |

These reads can return JavaScript `undefined` even though the checker
annotates the array element as `string`. Emitting a required scalar read
raises at the array membership guard before the observation can occur.

`tsfront.observedArrayIndex` now retains presence through `array.get` only
for immediate truthiness and strict-equality observations in the unpruned
lowering. It also preserves expression preludes and evaluation order.
Ordinary required scalar reads remain checked; no parameter/return/local
ABI is globally widened and loose equality is not newly supported.

`TestObservedScalarArrayIndexNodeOracle` compares against Node and the Go
emitter, invokes emitted ABAP on OSG, checks negative/missing indices,
undefined versus empty string/zero/false, truthiness, exactly-once operand
effects, and preservation of an ordinary checked read. Pinned OSG result:
**1 SUCCESS row**.

## Validation

The final unmodified-runtime OSG run (`all-0OoXbV`) selects the complete
`abap/lexer` shard and only `test/abap/syntax/syntax.ts` from syntax. The
shared binary is runtime-selected per shard; independent upstream Node
oracles and the harness's genuine-FAIL checks remain enabled.

| Measurement | Pass | FAIL | Trap | Node-active |
| --- | ---: | ---: | ---: | ---: |
| Baseline syntax, OSG | 1070 | 0 | 67 | 1137 |
| Repaired syntax, OSG | 1137 | 0 | 0 | 1137 |
| Repaired abap/lexer, OSG | 200 | 0 | 0 | 200 |
| Repaired syntax, Go | 1137 | 0 | 0 | 1137 |

The Go file-focused rerun was kept as `all-m6hi7p`. Syntax's 10 pending
tests remain outside the denominator on both targets. The final OSG run
has **1337/1337** Node-active passing across its two selected shards.

Both fixes are committed separately after their pinned OSG unit steps:
`87b9470` (array storage) and `5544b61` (scalar observations).

Required gates all pass:

- `go vet ./...`;
- `GOFLAGS=-buildvcs=false go test -short -p 2 ./hir/... ./tsfront/ ./cmd/... -count=1`;
- `.github/ci/lint.sh gate origin/main`: **0 new issues**, canary **5/5**,
  golangci-lint **2.13.2**.

The run names above identify evidence kept during diagnosis. Compact
results and every original trapped title are committed in the linked JSON;
the RAM scratch and OSG's disposable `tools/gogen/.out/go-cache` are cleaned
at completion, as requested.

All heavy commands use the shared flock, a 48 GiB user scope and nice 10,
admitted below load 12. New run artifacts are kept under the caller's RAM
scratch, not on the nearly full SSD. No OSG source or pin is modified,
and nothing is pushed.
