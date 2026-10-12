# Final ABI handoffs — train-9

Base: `a8038ab2ddb279a77af2e8119edb557d54f77c4f` (`origin/integ/train-9`).
Frozen semantic source: `49672e4f18d74b84ac390fc9f7605189ee896f92`. Original upstream:
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`, freshly compiled with its locked
TypeScript 6.0.3 and original Mocha/Chai. No upstream source, assertion,
registration or FAIL guard is changed. No new site override or WASM change.

**The task certificate is red:** the specified base already has one genuine LSP
FAIL, unchanged on the final compiler. Local compiler gates pass, but this is
not a FAIL=0 test certificate.

## Measurements: only the five touched shards

251 root files, 4,875 original Node active tests plus 26 pending. Owned results
subtract six independently measured imported SQL tests from LSP; rules owns them.
No borrowed FAIL is hidden. Initializations and missing titles remain separate
in [the evidence](evidence/final-abi.json).

| Shard | Pass before → after | FAIL before → after | Trap before → after | Not-run |
| --- | ---: | ---: | ---: | ---: |
| abap/syntax | 1,362 → 1,363 | 0 → 0 | 1 → 0 | 10 |
| objects | 134 → 138 | 0 → 0 | 10 → 6 | 1 |
| objects/rename | 29 → 31 | 0 → 0 | 2 → 0 | 0 |
| rules | 3,141 → 3,143 | 0 → 0 | 6 → 4 | 12 |
| lsp | 179 → 180 | 1 → 1 | 8 → 7 | 5 |
| Total | **4,845 → 4,855** | **1 → 1** | **27 → 17** | **28** |

Ten real passes; trap message groups **17→12**. Not-run is unchanged:
**28 = 26 skipped/focused + 2 unregistered + 0 async/done**. All traps sum to
27/17; there are no unmatched executions or infrastructure failures. This is a
focused measurement, not a new 11,060-test whole-suite claim.

The inherited FAIL is
`LSP, actual rename, global class rename global class, add namespace`:
`assertion failed: expected 0 to equal 1`. Its final assertion expects an
`object_naming` issue after adding a namespace. The initial base runner stopped
at this FAIL. The remaining base shards were measured separately. Complete raw
LSP runs retained the failure rather than weakening the runner or relabeling it.
Excluded areas remain untouched; no permission to expand their scope was supplied.

## Listed tests and general repairs

| Handoff | Final outcome |
| --- | --- |
| ClassData (3) | All three pass, including syntax, class attribute naming, and LSP hover/static modifier. |
| RenamerHelper (2) | TTYP and AUTH referenced DataElement rename tests pass. |
| ProxyObject line 81 (1) | `Proxy Object basic INTF` passes. |
| ABAPObject parseInt (2) | Function Group findTextFile, Z and namespace cases pass. |
| NumberRange nonfinite parseFloat (1) | `Rule: allowed_object_naming NROB, okay` passes. |
| Chai enumerable keys (2) | XML parser errors outside parseRaw2 passes. Malformed inner metadata progresses through the key assertions, then refuses at the structured Proxy R3_NAME guard. Only one pass is credited. |
| UnusedVariables skipAbstract (2) | `Abstract` and `select, order by dynamic` both pass. They were already green on train-9; no redundant config/literal change is made. |
| Domain .dtel nominal assertions (2) | Both remain true nominal mismatches: the original .dtel filename creates DataElement, not Domain. |

The listed set has **12 passes / 3 traps**, including two config tests already
passing on the base. Exact titles and statuses are retained in the JSON.

- **Erased storage:** a string assertion flowing into an erased destination
  retains the original value. Undefined stays undefined; structured values keep
  identity and live mutations. Explicit `String()` remains a separate operation.
- **Live row arrays:** erased arrays of structured rows use the existing live
  array ABI with a checked row decoder. Reads/writes delegate to the original
  array; row fields use certified live data layouts. No copied rows or arrays.
- **Common/nominal assertions:** sibling static class references sharing a real
  base can reach the actual runtime class check. True mismatches still refuse.
  Optional locals and guard aliases used only through inherited methods or
  genuine ancestor parameters retain the original ancestor view. Subclass fields,
  subclass result escapes and incompatible objects retain their checked boundary.
- **Nonfinite Number values:** full TS closures have separate parseInt/parseFloat
  operations returning binary64 NaN/Infinity. Go comparisons, truthiness,
  arithmetic propagation, isNaN, string conversion and existing JSON handling
  match the Node oracles in the measured domains. Pruned workloads keep their
  old finite operations and bytes. ABAP proves finite safe-integer string literal
  parseInt inputs before emission; dynamic/nonfinite/unsupported signed-zero
  inputs explicitly refuse. Existing Number literal, division and remainder
  restrictions remain.
- **Record provenance/enumeration:** record allocations retain provenance
  distinct from actual Maps. Chai reads original live entries, including own
  undefined values. Shared physical storage alone never grants a Map object keys.
  The allocation metadata is recorded outside literal property lowering; no
  property/default/spread policy is changed.
- **Ordinary record coercion:** the ClassData hover follow-up needed actual JS
  string conversion of its retained structured value. Certified tagged plain
  records use default Object coercion. Own toString/valueOf hooks,
  prototype-affecting keys, arrays and native class instances stay outside this
  new domain. No native class receives a fabricated `[object Object]`. ABAP
  supports the primitive part and explicitly refuses nonprimitive storage.

The remaining Proxy follow-up reads structured XML `R3_NAME` through an optional
native string field before the original `typeof` guard at line 96 can reject it.
That heterogeneous guarded-read ABI belongs to the excluded typeof work. The
Domain tests call `parseType`, which is not inherited from their shared
AbstractObject base and has different sibling signatures. Neither creates a
legitimate Domain instance; no fake class or weakened nominal check is used.
Other unchanged touched-shard traps are listed completely in the evidence.

The compiler Node oracles cover absence, identity, live mutations, optional
ancestor arguments, true nominal mismatch, nonfinite comparisons/propagation,
parser rounding/prefixes, record coercion effects/hook refusals and original Chai.
The existing finite ABAP syntax fixture remains intact and passing.

## Commands and gates

Go package parallelism is two, GOMAXPROCS is two, and shard workers are one.
`BASE` and `CANDIDATE` below are scratch binaries from the named revisions;
`UPSTREAM` is the clean pinned checkout; `SCRATCH` is disposable storage.

```sh
GOFLAGS=-p=2 GOMAXPROCS=2 go build -o "$CANDIDATE" ./cmd/abapiti
"$BASE" abaplint --full -o "$SCRATCH/full-base"
"$CANDIDATE" abaplint --full -o "$SCRATCH/full-after"
ABAPITI_TESTS_SHARDS=objects,objects/rename,rules,abap/syntax \
  ABAPITI_TESTS_WORKERS=1 ABAPITI_TESTS_COMPILER="$CANDIDATE" \
  ABAPITI_TESTS_COMPILER_COMMIT=49672e4 GOFLAGS=-p=2 GOMAXPROCS=2 \
  nice -n 10 node tools/abaplint-tests-all.mjs "$UPSTREAM" "$SCRATCH/final.json"
```

For LSP, materialize its original roots with `tools/registry-closure.mjs`, run
`abapiti abaplint-tests "$LSP_CLOSURE" -o "$SCRATCH/lsp-out"`, retain its raw JSON,
and apply `tools/abaplint-tests-owned.mjs` using an independent original Node
oracle for the six imported SQL tests. The raw CLI returns exit 1 for the genuine
FAIL. The initial base and separate remaining-shard commands are in the JSON.

`abaplint --full` with its default integer policy reports identical summaries:
**26 blocking = 17 roots + 9 cascades, 67 HIR errors, 19 files**. Every blocking
message and count, including cascades, is retained before/after. The owned static
parseFloat integer-evidence diagnostic remains **1→1**; it is not eliminated by
inventing integer evidence. The test shards independently use binary64 Numbers.

All final local gates pass:

- `go vet ./hir/... ./tsfront/... ./internal/... ./cmd/...`
- `go test -short -p 2 -timeout 40m` on the same packages, including
  `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`.
- `ABAPITI_GRACECHECK=1` lexer facts/golden check: all 61 call sites are identical.
  The only golden delta is four operation catalog records, **132→136** and read
  facts **200→204**; conservative count remains 21.
- `./.github/ci/lint.sh gate origin/integ/train-9`: zero new findings, all five
  canaries caught, pinned golangci-lint 2.13.2.
- Default `abaplint -o "$OUT" --target all,go`: base output archived, final output
  regenerated at the same original path. Raw `diff -r` is empty: **6,321 files,
  zero diff bytes**, including generated packages and binaries.

Only compact documentation/evidence is committed. Work remains local on
`feat/fit-final-abi`; no credentials, push to any branch, or PR is used.
