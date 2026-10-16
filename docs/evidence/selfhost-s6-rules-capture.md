# s6 follow-up: lexical numeric capture storage

Base: `c62b85f208a8f1aa753c83433bf9fd9012bdfd82`.

## Reproduction and mapping

The failing Node-active title is:

`test 7bit_ascii rule "WRITE: / 'æøå'." should have 1 issue(s)`

The generated `abap/names.json` maps
`Z_SRC_RULES_7B_A8808C0DE90679` to
`src/rules/7bit_ascii.ts.SevenBitAscii` (line 11), and
`Z_MEMBER_RUN_71BE1BCA84374A` to `member.run`, including that class's
method at line 42. `Z_RUNTIME_OPTI_8DF4F98F9A3A52` is
`runtime.optional<number>`.

The original compiler reproduces the exact reported line-404 refusal on native
OSG: `CAST z_runtime_opti_8df4f98f9a3a52( t57 )`, with `t57` declared `TYPE f`.
The seven-test `test/rules/7bit_ascii.ts` file produces 6 pass, 0 FAIL, 1 trap.
The same file passes 7/7 with 0 FAIL and 0 traps using the compiler built at
`d470394`, pinning the regression to c62b85f. Native reports are
`/tmp/s6ref/bad-execute.json` and `/tmp/s6ref/before-execute.json`.

## Root cause and repair

`SevenBitAscii.run` declares `const column` in two sibling blocks. The numeric
storage pass previously flattened every method's declarations into one map
keyed by name. Backward propagation from the optional numeric `Position`
constructor parameter marked the last `column` declaration, but forward
propagation treated reads of either declaration as tagged. The first local
remained `TYPE f` while its consuming expression expected an optional reference.
The pre-c62b85f emitter instead boxes that finite scalar at the call boundary.

Resolve each local expression to its lexical declaration before storage
propagation, respecting blocks, sequence preludes/results, loop bindings and
catch bindings. Widen a variable declaration's own type. This keeps each
capture's declared storage and reads consistent and preserves unrelated
shadowed locals. Parser values still reach optional storage before scalar
finite checks. The change is confined to ABAP storage lowering; source HIR and
Go emission remain unchanged.

`TestNonfiniteParserCaptureLexicalStorage` covers sibling captures with the
same name, a sequence capture and an unrelated outer scalar with that name.
On unmodified c62b85f it fails with declaration `number` versus read
`optional<number>` in the first block. With the repair it passes, including
native OSG execution. `TestNonfiniteParserCaptureStorage` also passes native
OSG for both widened and already-optional destinations, including NaN and
finite parser results.

## Reproduction environment

Abaplint `577f875ebec44cfaf64841cfe71c8ab8dc32622e`, with root and
`packages/core` npm installs. Clean OSG
`f83dc5c3e05bfaea71268fd6ad0e9ff88532cce6`, with npm and `osd-libs.mjs`.
Node 26.9.0 and Go 1.26.0. All heavy runs use `ulimit -v 16777216` and Go
package parallelism at most 2. Native shard builds run serially.

The local Node wrapper disables the Wasm trap handler and uses explicit bounds
checks with `--wasm-max-mem-pages=4096`. Focused builds use a 12 GiB JS
heap; the final shared native build uses 13 GiB, within the same 16 GiB
virtual-memory cap. The harness split the initial three-shard union after
its 12 GiB frontend heap was insufficient.
Without `--disable-wasm-trap-handler`, Node reserved an unused 8 GiB region and
hit the virtual-memory cap. Infrastructure-only attempts are excluded from
results. An optional persistent frontend-cache attempt also exceeded the JS
heap; the successful comparisons and final shards use the uncached frontend.
Raw reports and retained generated sources are under `/tmp/s6ref`
and `/tmp/abapiti-test-shards`.

Focused runs use `tools/abaplint-tests-all.mjs` with
`ABAPITI_TESTS_FILES=test/rules/7bit_ascii.ts`, `ABAPITI_TESTS_TARGET=osg`,
and `ABAPITI_OSG_HOME=/tmp/s6ref/osgo`. The pre-c62b85f compiler is selected
with `ABAPITI_TESTS_COMPILER` and `ABAPITI_TESTS_COMPILER_COMMIT=d470394`.

## Final gates

All fixed gates ran on c62b85f plus this patch. Cells are **passed / FAIL / trap**.
Both runtimes have registration parity, 0 unregistered, 0 async-pending and
0 active tests not passing. The 11 Node-pending titles (10 rules, 1 objects)
are outside the active denominator.

| Shard | TS-HG | TS-HA@OSGO |
| --- | ---: | ---: |
| rules | 3149 / 0 / 0 | 3149 / 0 / 0 |
| objects | 144 / 0 / 0 | 144 / 0 / 0 |
| utils | 15 / 0 / 0 | 15 / 0 / 0 |
| Total active | 3308 / 0 / 0 | 3308 / 0 / 0 |

Native OSG passes the exact Unicode test, both FUGR `findTextFile` Z/namespace
cases, `malformed object XML ignores structured values where text is expected`,
and both utils ignored-name/pattern cases.

- `go build ./...`: passed.
- `go vet ./...`: passed.
- `go test -short -p 2 ./hir/... ./tsfront/... ./cmd/abapiti/... ./internal/abapfixture/...`: passed.
- Native OSG `TestNonfiniteParserCaptureStorage` (both variants) and
  `TestNonfiniteParserCaptureLexicalStorage`: passed.
- Exact extra gate over generated default abaplint Go: **2** occurrences of
  `assignment type mismatch` (one each in `hir.go` and `hir_profile.go`).

Full shard runs use `ABAPITI_TESTS_SHARDS=rules,objects,utils`, selecting
`ABAPITI_TESTS_TARGET=osg` or `go` as appropriate. The frozen invocation and
result assertions are `/tmp/s6ref/gates.sh` and `/tmp/s6ref/check-shards.py`.
Final summaries are `/tmp/s6ref/fixed-osg.json` and
`/tmp/s6ref/fixed-hg.json`; per-title reports are retained in
`/tmp/abapiti-test-shards/all-SRSwg2` (OSG) and
`/tmp/abapiti-test-shards/all-VEN5tl` (Go). The repaired class and source mapping
are also retained under `/tmp/s6ref/fixed-source`.

No push.
