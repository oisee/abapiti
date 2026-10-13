# OSGO: Node-active tests that never registered

The 17-shard OSGO baseline (`docs/osgo-baseline-17.md`, run B) left **288
Node-active tests unregistered** on TS-HA@OSGO with no attributed cause:
`abap` 165, `abap/expressions` 88, `pretty_printer` 29, `utils` 6. On the Go
host all of them register.

## Root: `array.has` in registration closures (dell, O3)

Every one of the 288 has one root. abaplint test files register their mocha
cases from `forEach` over a case table at describe level, for example
`tests.forEach(t => it(t.abap, ...))`. Presence-checked callbacks
(`forEach`/`every`/`find`/`lastIndexOf`) lower to `array.has`, which has no ABAP
emitter on main. `TrapABAPTestRefusals` therefore refuses the **whole
describe/member-test closure**, so none of its `it` calls run and the
titles never register.

The exact refusal, reported as an initialization record with status `trapped`:

```
lowering trap: node 0 (): runtime operation array.has has no ABAP emitter
```

| Shard | Refused closures (files) | Unregistered active |
| --- | --- | ---: |
| abap | `ast.ts`, `combi.ts`, `combi_size.ts`, `concat_tokens.ts`, `nesting.ts`, `structure_errors.ts` (parser table), `unknown.ts` | 165 |
| abap/expressions | `field_sub.ts`, `reuse.ts`, `source.ts`, `string_template.ts` | 88 |
| pretty_printer | `pretty_printer.ts` (7 describe closures) | 29 |
| utils | `method_length_stats.ts` | 6 |

`node 0` means the site is synthesized: it is the presence check inside the
callback loop (`tsfront/dense_callbacks.go`), not a source expression. The
mocha shim, `describe`/`it` registration and module initializers are not
involved. No harness or registration-lowering defect was found.

**Status: blocked on `array.has` → dell O3 (`osgo-storage`).** Array presence,
`array.delete` and sparse storage are dell's zone. O3's presence-aware storage
emits `array.has`. This branch makes no change to array storage. A bounds-check
emission was prototyped and then reverted at the coordinator's request, so that
it does not conflict with O3.

### Measured on top of O3

The same four shards were run with a compiler built from `origin/osgo-storage`
(`0a61c21`). The harness, tools, runtime testdata and OSG pin (`368baab28`)
came from this branch; tools and runtime testdata are identical between the
two branches.

| Shard | main 3da6606: pass / FAIL / trap / unregistered | O3 0a61c21: pass / FAIL / trap / unregistered |
| --- | --- | --- |
| abap | 4 / 0 / 47 / 165 | 2 / 0 / 214 / 0 |
| abap/expressions | 2 / 0 / 17 / 88 | 1 / 0 / 106 / 0 |
| pretty_printer | 0 / 0 / 2 / 29 | 0 / 0 / 31 / 0 |
| utils | 7 / 0 / 2 / 6 | 0 / 0 / 15 / 0 |
| **Total (369 active)** | **13 / 0 / 68 / 288** | **3 / 0 / 366 / 0** |

With O3, all 288 tests register and FAIL stays 0. The pass drop from 13 to 3
does not come from O3's storage. `osgo-storage` branches from before O2
(`3da6606`), so it lacks the ABAP Chai emitters, and its dominant traps are
`chai.box`/`chai.deepEqual has no ABAP emitter` (187 + 147 + 18 tests) plus
`json5.parse` (8). These are the O2 features that main already has.

### Handoff to dell (O3)

1. Rebase or merge `osgo-storage` onto main (O2, pin `368baab28`). A trial merge
   conflicts in `hir/abap/emit.go`, `hir/abap/runtime.go` and
   `tsfront/abaplint_tests_osg_test.go`. The merged build must keep both
   presence-aware `array.has`/`array.delete` and O2's Chai, JSON5 and nested
   views.
2. Under Chai, keep the existing guards that refuse sparse writes and nonempty
   `new Array(n)` before padding, or replace them with true presence. In the
   harness these closures run inside Chai programs.
3. After the merge, rerun `abap,abap/expressions,pretty_printer,utils` on OSGO.
   Expect unregistered 0 and FAIL 0, and passes at or above main's 13, with
   these titles now executed. The remaining traps belong to the next roots.

## Pending-only root (backlog, not in the denominator)

`test/abap/structure_errors.ts` has a first describe that registers only
`it.skip` cases (4 Node-pending titles). Its closure refuses in lowering:

```
lowering trap: test/abap/structure_errors.ts:15:54: HIR verification: node 36346
(test/abap/structure_errors.ts:17:13): invalid narrowing classref<shape.c4b3…>
to classref<shape.a075…>
```

The likely cause (not yet verified) is the explicit parameter annotation
`(c: {abap, error, structure: IStructure, errorMatched})` on a `forEach`
callback, whose element shape differs from the inferred row shape. The
refusal persists on O3, and it hides no Node-active test. It is an abapiti
lowering item: a structural row narrowing between equivalent object-literal
shapes.

## Reproduction

```
ABAPITI_TESTS_TARGET=osg ABAPITI_OSG_HOME=<clean OSG at .github/ci/osgo.ref> \
ABAPITI_TESTS_SHARDS=abap,abap/expressions,pretty_printer,utils \
[ABAPITI_TESTS_COMPILER=<abapiti built from osgo-storage> ABAPITI_TESTS_COMPILER_COMMIT=0a61c21] \
node tools/abaplint-tests-all.mjs <abaplint at 577f875e> <summary.json>
```

Environment: `GOTOOLCHAIN=go1.26.0`, `GOFLAGS=-buildvcs=false`, `GOMAXPROCS=4`,
`NODE_OPTIONS=--max-old-space-size=32768`, transpiler/runtime `ee5c2cd1`, one
shard worker. The Node oracle reports 369 active and 4 pending tests in these
shards.
