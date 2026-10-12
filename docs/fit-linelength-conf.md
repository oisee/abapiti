# Primitive config assignment values

Measured only original `test/rules/line_length.ts`, on pinned upstream
`577f875ebec44cfaf64841cfe71c8ab8dc32622e`, against `origin/integ/train-12`.
The Node oracle passes all four tests. No upstream source or assertions changed.

| Compiler | Pass | Test-body trap | FAIL | Unregistered | Initialization traps |
| --- | ---: | ---: | ---: | ---: | ---: |
| Integration base | 2 | 0 | 0 | 2 | 1 |
| This fix | 4 | 0 | 0 | 0 | 0 |

[Focused evidence](evidence/fit-linelength-conf.json) preserves registration
counts and the original initialization refusal.

The existing typed-field assignment expression lowering already stores `400`
into the new config object's `length` and returns `400`, evaluating the receiver
before the RHS exactly once. The refusal was downstream: `testRule` passes that
number to `LineLength.setConfig`, whose native class view refused primitive
backing storage at `src/rules/line_length.ts:35`.

The shared absence analysis now recognizes explicitly optional scalar reads,
including a read saved to a local before testing for undefined. For those layouts,
numeric sibling fields used in relational comparisons preserve absence in optional
storage. The existing optional comparison lowering produces false for the missing
numeric property, matching JavaScript. Native config objects still retain their
assigned fields and identity. This has no line_length-specific branch.

Node-oracle regressions cover numeric, string and boolean assignment values,
receiver-before-RHS evaluation, primitive-backed config reads, native config reads,
and primitive identity. Required numeric reads and writes into primitive backing
storage remain typed refusals. Prototype-bearing layouts retain their existing
refusal. No runtime operation was added; the existing ABAP class-view emitter
explicitly refuses primitive backing values.

The focused runner used the existing materialization, original Node oracle,
FAIL guard and report modules, with its root list restricted to line_length and
its summary marked as a partial run. Baseline compilation used an isolated archive
of the integration base. Go build/test package parallelism was at most two.

The short gate also exposed a stale `TestObjectStringCoercionNodeOracle`
assertion on the untouched integration base: its `ownHook` case expected the
older generic "outside certified" text, but the existing runtime already refuses
with "record string conversion requires the original default Object coercion
hooks". The test now checks that specific refusal; runtime behavior is unchanged.

Validation:

- `go vet ./...`: pass.
- `go test -short -p 2 ./...`: pass; 569 top-level tests passed, 36 skipped. Including subtests: 1088 passes, 40 skips, zero failures.
- `ABAPITI_GRACECHECK=1 go test -p 2 -count=1 -run TestLexerFactsReport ./tsfront/`: pass; exported with `GRACE_FACTS_OUT`, golden unchanged.
- `TestDefaultZabapgitRuntimeOpsSupportedByEveryBackend`: pass, explicitly executed.
- `./.github/ci/lint.sh gate origin/integ/train-12`: zero new issues; five of five canaries caught, CI-pinned golangci-lint 2.13.2.
